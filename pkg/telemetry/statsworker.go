// Copyright 2025 Rixy Ai.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package telemetry

import (
	"context"
	"sync"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/voicekit/voicekit-server/pkg/utils"
	"github.com/voicekit/protocol/voicekit"
	"github.com/voicekit/protocol/logger"
	protoutils "github.com/voicekit/protocol/utils"
)

// StatsWorker handles participant stats
type StatsWorker struct {
	next *StatsWorker

	ctx                 context.Context
	t                   TelemetryService
	roomID              voicekit.RoomID
	roomName            voicekit.RoomName
	participantID       voicekit.ParticipantID
	participantIdentity voicekit.ParticipantIdentity
	isConnected         bool

	lock             sync.RWMutex
	outgoingPerTrack map[voicekit.TrackID][]*voicekit.AnalyticsStat
	incomingPerTrack map[voicekit.TrackID][]*voicekit.AnalyticsStat
	closedAt         time.Time
}

func newStatsWorker(
	ctx context.Context,
	t TelemetryService,
	roomID voicekit.RoomID,
	roomName voicekit.RoomName,
	participantID voicekit.ParticipantID,
	identity voicekit.ParticipantIdentity,
) *StatsWorker {
	s := &StatsWorker{
		ctx:                 ctx,
		t:                   t,
		roomID:              roomID,
		roomName:            roomName,
		participantID:       participantID,
		participantIdentity: identity,
		outgoingPerTrack:    make(map[voicekit.TrackID][]*voicekit.AnalyticsStat),
		incomingPerTrack:    make(map[voicekit.TrackID][]*voicekit.AnalyticsStat),
	}
	return s
}

func (s *StatsWorker) OnTrackStat(trackID voicekit.TrackID, direction voicekit.StreamType, stat *voicekit.AnalyticsStat) {
	s.lock.Lock()
	if direction == voicekit.StreamType_DOWNSTREAM {
		s.outgoingPerTrack[trackID] = append(s.outgoingPerTrack[trackID], stat)
	} else {
		s.incomingPerTrack[trackID] = append(s.incomingPerTrack[trackID], stat)
	}
	s.lock.Unlock()
}

func (s *StatsWorker) ParticipantID() voicekit.ParticipantID {
	return s.participantID
}

func (s *StatsWorker) SetConnected() {
	s.lock.Lock()
	s.isConnected = true
	s.lock.Unlock()
}

func (s *StatsWorker) IsConnected() bool {
	s.lock.RLock()
	defer s.lock.RUnlock()

	return s.isConnected
}

func (s *StatsWorker) Flush(now time.Time) bool {
	ts := timestamppb.New(now)

	s.lock.Lock()
	stats := make([]*voicekit.AnalyticsStat, 0, len(s.incomingPerTrack)+len(s.outgoingPerTrack))

	incomingPerTrack := s.incomingPerTrack
	s.incomingPerTrack = make(map[voicekit.TrackID][]*voicekit.AnalyticsStat)

	outgoingPerTrack := s.outgoingPerTrack
	s.outgoingPerTrack = make(map[voicekit.TrackID][]*voicekit.AnalyticsStat)

	closed := !s.closedAt.IsZero() && now.Sub(s.closedAt) > workerCleanupWait
	s.lock.Unlock()

	stats = s.collectStats(ts, voicekit.StreamType_UPSTREAM, incomingPerTrack, stats)
	stats = s.collectStats(ts, voicekit.StreamType_DOWNSTREAM, outgoingPerTrack, stats)
	if len(stats) > 0 {
		s.t.SendStats(s.ctx, stats)
	}

	return closed
}

func (s *StatsWorker) Close() bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	ok := s.closedAt.IsZero()
	if ok {
		s.closedAt = time.Now()
	}
	return ok
}

func (s *StatsWorker) Closed() bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return !s.closedAt.IsZero()
}

func (s *StatsWorker) collectStats(
	ts *timestamppb.Timestamp,
	streamType voicekit.StreamType,
	perTrack map[voicekit.TrackID][]*voicekit.AnalyticsStat,
	stats []*voicekit.AnalyticsStat,
) []*voicekit.AnalyticsStat {
	for trackID, analyticsStats := range perTrack {
		coalesced := coalesce(analyticsStats)
		if coalesced == nil {
			continue
		}

		coalesced.TimeStamp = ts
		coalesced.TrackId = string(trackID)
		coalesced.Kind = streamType
		coalesced.RoomId = string(s.roomID)
		coalesced.ParticipantId = string(s.participantID)
		coalesced.RoomName = string(s.roomName)
		stats = append(stats, coalesced)
	}
	return stats
}

// -------------------------------------------------------------------------

// create a single stream and single video layer post aggregation
func coalesce(stats []*voicekit.AnalyticsStat) *voicekit.AnalyticsStat {
	if len(stats) == 0 {
		return nil
	}

	// find aggregates across streams
	startTime := time.Time{}
	endTime := time.Time{}
	scoreSum := float32(0.0) // used for average
	minScore := float32(0.0) // min score in batched stats
	var scores []float32     // used for median
	maxRtt := uint32(0)
	maxJitter := uint32(0)
	coalescedVideoLayers := make(map[int32]*voicekit.AnalyticsVideoLayer)
	coalescedStream := &voicekit.AnalyticsStream{}
	for _, stat := range stats {
		if !isValid(stat) {
			logger.Warnw("telemetry skipping invalid stat", nil, "stat", stat)
			continue
		}

		// only consider non-zero scores
		if stat.Score > 0 {
			if minScore == 0 {
				minScore = stat.Score
			} else if stat.Score < minScore {
				minScore = stat.Score
			}
			scoreSum += stat.Score
			scores = append(scores, stat.Score)
		}

		for _, analyticsStream := range stat.Streams {
			start := analyticsStream.StartTime.AsTime()
			if startTime.IsZero() || startTime.After(start) {
				startTime = start
			}

			end := analyticsStream.EndTime.AsTime()
			if endTime.IsZero() || endTime.Before(end) {
				endTime = end
			}

			if analyticsStream.Rtt > maxRtt {
				maxRtt = analyticsStream.Rtt
			}

			if analyticsStream.Jitter > maxJitter {
				maxJitter = analyticsStream.Jitter
			}

			coalescedStream.PrimaryPackets += analyticsStream.PrimaryPackets
			coalescedStream.PrimaryBytes += analyticsStream.PrimaryBytes
			coalescedStream.RetransmitPackets += analyticsStream.RetransmitPackets
			coalescedStream.RetransmitBytes += analyticsStream.RetransmitBytes
			coalescedStream.PaddingPackets += analyticsStream.PaddingPackets
			coalescedStream.PaddingBytes += analyticsStream.PaddingBytes
			coalescedStream.PacketsLost += analyticsStream.PacketsLost
			coalescedStream.PacketsOutOfOrder += analyticsStream.PacketsOutOfOrder
			coalescedStream.Frames += analyticsStream.Frames
			coalescedStream.Nacks += analyticsStream.Nacks
			coalescedStream.Plis += analyticsStream.Plis
			coalescedStream.Firs += analyticsStream.Firs

			for _, videoLayer := range analyticsStream.VideoLayers {
				coalescedVideoLayer := coalescedVideoLayers[videoLayer.Layer]
				if coalescedVideoLayer == nil {
					coalescedVideoLayer = protoutils.CloneProto(videoLayer)
					coalescedVideoLayers[videoLayer.Layer] = coalescedVideoLayer
				} else {
					coalescedVideoLayer.Packets += videoLayer.Packets
					coalescedVideoLayer.Bytes += videoLayer.Bytes
					coalescedVideoLayer.Frames += videoLayer.Frames
				}
			}
		}
	}
	coalescedStream.StartTime = timestamppb.New(startTime)
	coalescedStream.EndTime = timestamppb.New(endTime)
	coalescedStream.Rtt = maxRtt
	coalescedStream.Jitter = maxJitter

	// whittle it down to one video layer, just the max available layer
	maxVideoLayer := int32(-1)
	for _, coalescedVideoLayer := range coalescedVideoLayers {
		if maxVideoLayer == -1 || maxVideoLayer < coalescedVideoLayer.Layer {
			maxVideoLayer = coalescedVideoLayer.Layer
			coalescedStream.VideoLayers = []*voicekit.AnalyticsVideoLayer{coalescedVideoLayer}
		}
	}

	stat := &voicekit.AnalyticsStat{
		MinScore:    minScore,
		MedianScore: utils.MedianFloat32(scores),
		Streams:     []*voicekit.AnalyticsStream{coalescedStream},
		Mime:        stats[len(stats)-1].Mime, // use the latest Mime
	}
	numScores := len(scores)
	if numScores > 0 {
		stat.Score = scoreSum / float32(numScores)
	}
	return stat
}

type CondensedStat struct {
	StartTime   time.Time
	EndTime     time.Time
	Bytes       uint64
	Packets     uint32
	PacketsLost uint32
	Frames      uint32
}

func CondenseStat(stat *voicekit.AnalyticsStat) (ps CondensedStat, ok bool) {
	if ok = isValid(stat); !ok {
		return
	}

	for _, stream := range stat.Streams {
		startTime := stream.StartTime.AsTime()
		endTime := stream.EndTime.AsTime()
		if ps.StartTime.IsZero() || startTime.Before(ps.StartTime) {
			ps.StartTime = startTime
		}
		if endTime.After(ps.EndTime) {
			ps.EndTime = endTime
		}

		ps.Bytes += stream.PrimaryBytes
		ps.Packets += stream.PrimaryPackets
		ps.PacketsLost += stream.PacketsLost
		ps.Frames += stream.Frames
	}

	return
}

func isValid(stat *voicekit.AnalyticsStat) bool {
	for _, analyticsStream := range stat.Streams {
		if int32(analyticsStream.PrimaryPackets) < 0 ||
			int64(analyticsStream.PrimaryBytes) < 0 ||
			int32(analyticsStream.RetransmitPackets) < 0 ||
			int64(analyticsStream.RetransmitBytes) < 0 ||
			int32(analyticsStream.PaddingPackets) < 0 ||
			int64(analyticsStream.PaddingBytes) < 0 ||
			int32(analyticsStream.PacketsLost) < 0 ||
			int32(analyticsStream.PacketsOutOfOrder) < 0 ||
			int32(analyticsStream.Frames) < 0 ||
			int32(analyticsStream.Nacks) < 0 ||
			int32(analyticsStream.Plis) < 0 ||
			int32(analyticsStream.Firs) < 0 {
			return false
		}

		for _, videoLayer := range analyticsStream.VideoLayers {
			if int32(videoLayer.Packets) < 0 ||
				int64(videoLayer.Bytes) < 0 ||
				int32(videoLayer.Frames) < 0 {
				return false
			}
		}
	}

	return true
}
