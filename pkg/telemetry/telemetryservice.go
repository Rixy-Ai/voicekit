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

	"github.com/voicekit/voicekit-server/pkg/sfu/mime"
	"github.com/voicekit/voicekit-server/pkg/utils"
	"github.com/voicekit/protocol/voicekit"
	"github.com/voicekit/protocol/logger"
	"github.com/voicekit/protocol/webhook"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . TelemetryService
type TelemetryService interface {
	// TrackStats is called periodically for each track in both directions (published/subscribed)
	TrackStats(key StatsKey, stat *voicekit.AnalyticsStat)

	// events
	RoomStarted(ctx context.Context, room *voicekit.Room)
	RoomEnded(ctx context.Context, room *voicekit.Room)
	// ParticipantJoined - a participant establishes signal connection to a room
	ParticipantJoined(ctx context.Context, room *voicekit.Room, participant *voicekit.ParticipantInfo, clientInfo *voicekit.ClientInfo, clientMeta *voicekit.AnalyticsClientMeta, shouldSendEvent bool)
	// ParticipantActive - a participant establishes media connection
	ParticipantActive(ctx context.Context, room *voicekit.Room, participant *voicekit.ParticipantInfo, clientMeta *voicekit.AnalyticsClientMeta, isMigration bool)
	// ParticipantResumed - there has been an ICE restart or connection resume attempt, and we've received their signal connection
	ParticipantResumed(ctx context.Context, room *voicekit.Room, participant *voicekit.ParticipantInfo, nodeID voicekit.NodeID, reason voicekit.ReconnectReason)
	// ParticipantLeft - the participant leaves the room, only sent if ParticipantActive has been called before
	ParticipantLeft(ctx context.Context, room *voicekit.Room, participant *voicekit.ParticipantInfo, shouldSendEvent bool)
	// TrackPublishRequested - a publication attempt has been received
	TrackPublishRequested(ctx context.Context, participantID voicekit.ParticipantID, identity voicekit.ParticipantIdentity, track *voicekit.TrackInfo)
	// TrackPublished - a publication attempt has been successful
	TrackPublished(ctx context.Context, participantID voicekit.ParticipantID, identity voicekit.ParticipantIdentity, track *voicekit.TrackInfo)
	// TrackUnpublished - a participant unpublished a track
	TrackUnpublished(ctx context.Context, participantID voicekit.ParticipantID, identity voicekit.ParticipantIdentity, track *voicekit.TrackInfo, shouldSendEvent bool)
	// TrackSubscribeRequested - a participant requested to subscribe to a track
	TrackSubscribeRequested(ctx context.Context, participantID voicekit.ParticipantID, track *voicekit.TrackInfo)
	// TrackSubscribed - a participant subscribed to a track successfully
	TrackSubscribed(ctx context.Context, participantID voicekit.ParticipantID, track *voicekit.TrackInfo, publisher *voicekit.ParticipantInfo, shouldSendEvent bool)
	// TrackUnsubscribed - a participant unsubscribed from a track successfully
	TrackUnsubscribed(ctx context.Context, participantID voicekit.ParticipantID, track *voicekit.TrackInfo, shouldSendEvent bool)
	// TrackSubscribeFailed - failure to subscribe to a track
	TrackSubscribeFailed(ctx context.Context, participantID voicekit.ParticipantID, trackID voicekit.TrackID, err error, isUserError bool)
	// TrackMuted - the publisher has muted the Track
	TrackMuted(ctx context.Context, participantID voicekit.ParticipantID, track *voicekit.TrackInfo)
	// TrackUnmuted - the publisher has muted the Track
	TrackUnmuted(ctx context.Context, participantID voicekit.ParticipantID, track *voicekit.TrackInfo)
	// TrackPublishedUpdate - track metadata has been updated
	TrackPublishedUpdate(ctx context.Context, participantID voicekit.ParticipantID, track *voicekit.TrackInfo)
	// TrackMaxSubscribedVideoQuality - publisher is notified of the max quality subscribers desire
	TrackMaxSubscribedVideoQuality(ctx context.Context, participantID voicekit.ParticipantID, track *voicekit.TrackInfo, mime mime.MimeType, maxQuality voicekit.VideoQuality)
	TrackPublishRTPStats(ctx context.Context, participantID voicekit.ParticipantID, trackID voicekit.TrackID, mimeType mime.MimeType, layer int, stats *voicekit.RTPStats)
	TrackSubscribeRTPStats(ctx context.Context, participantID voicekit.ParticipantID, trackID voicekit.TrackID, mimeType mime.MimeType, stats *voicekit.RTPStats)
	EgressStarted(ctx context.Context, info *voicekit.EgressInfo)
	EgressUpdated(ctx context.Context, info *voicekit.EgressInfo)
	EgressEnded(ctx context.Context, info *voicekit.EgressInfo)
	IngressCreated(ctx context.Context, info *voicekit.IngressInfo)
	IngressDeleted(ctx context.Context, info *voicekit.IngressInfo)
	IngressStarted(ctx context.Context, info *voicekit.IngressInfo)
	IngressUpdated(ctx context.Context, info *voicekit.IngressInfo)
	IngressEnded(ctx context.Context, info *voicekit.IngressInfo)
	LocalRoomState(ctx context.Context, info *voicekit.AnalyticsNodeRooms)
	Report(ctx context.Context, reportInfo *voicekit.ReportInfo)
	APICall(ctx context.Context, apiCallInfo *voicekit.APICallInfo)
	Webhook(ctx context.Context, webhookInfo *voicekit.WebhookInfo)

	// helpers
	AnalyticsService
	NotifyEgressEvent(ctx context.Context, event string, info *voicekit.EgressInfo)
	FlushStats()
}

const (
	workerCleanupWait = 3 * time.Minute
	jobsQueueMinSize  = 2048

	telemetryStatsUpdateInterval = time.Second * 30
)

type telemetryService struct {
	AnalyticsService

	notifier  webhook.QueuedNotifier
	jobsQueue *utils.OpsQueue

	workersMu  sync.RWMutex
	workers    map[voicekit.ParticipantID]*StatsWorker
	workerList *StatsWorker

	flushMu sync.Mutex
}

func NewTelemetryService(notifier webhook.QueuedNotifier, analytics AnalyticsService) TelemetryService {
	t := &telemetryService{
		AnalyticsService: analytics,
		notifier:         notifier,
		jobsQueue: utils.NewOpsQueue(utils.OpsQueueParams{
			Name:        "telemetry",
			MinSize:     jobsQueueMinSize,
			FlushOnStop: true,
			Logger:      logger.GetLogger(),
		}),
		workers: make(map[voicekit.ParticipantID]*StatsWorker),
	}
	if t.notifier != nil {
		t.notifier.RegisterProcessedHook(func(ctx context.Context, whi *voicekit.WebhookInfo) {
			t.Webhook(ctx, whi)
		})
	}

	t.jobsQueue.Start()
	go t.run()

	return t
}

func (t *telemetryService) FlushStats() {
	t.flushMu.Lock()
	defer t.flushMu.Unlock()

	t.workersMu.RLock()
	worker := t.workerList
	t.workersMu.RUnlock()

	now := time.Now()
	var prev, reap *StatsWorker
	for worker != nil {
		next := worker.next
		if closed := worker.Flush(now); closed {
			if prev == nil {
				// this worker was at the head of the list
				t.workersMu.Lock()
				p := &t.workerList
				for *p != worker {
					// new workers have been added. scan until we find the one
					// immediately before this
					prev = *p
					p = &prev.next
				}
				*p = worker.next
				t.workersMu.Unlock()
			} else {
				prev.next = worker.next
			}

			worker.next = reap
			reap = worker
		} else {
			prev = worker
		}
		worker = next
	}

	if reap != nil {
		t.workersMu.Lock()
		for reap != nil {
			if reap == t.workers[reap.participantID] {
				delete(t.workers, reap.participantID)
			}
			reap = reap.next
		}
		t.workersMu.Unlock()
	}
}

func (t *telemetryService) run() {
	for range time.Tick(telemetryStatsUpdateInterval) {
		t.FlushStats()
	}
}

func (t *telemetryService) enqueue(op func()) {
	t.jobsQueue.Enqueue(op)
}

func (t *telemetryService) getWorker(participantID voicekit.ParticipantID) (worker *StatsWorker, ok bool) {
	t.workersMu.RLock()
	defer t.workersMu.RUnlock()

	worker, ok = t.workers[participantID]
	return
}

func (t *telemetryService) getOrCreateWorker(
	ctx context.Context,
	roomID voicekit.RoomID,
	roomName voicekit.RoomName,
	participantID voicekit.ParticipantID,
	participantIdentity voicekit.ParticipantIdentity,
) (*StatsWorker, bool) {
	t.workersMu.Lock()
	defer t.workersMu.Unlock()

	worker, ok := t.workers[participantID]
	if ok && !worker.Closed() {
		return worker, true
	}

	existingIsConnected := false
	if ok {
		existingIsConnected = worker.IsConnected()
	}

	worker = newStatsWorker(
		ctx,
		t,
		roomID,
		roomName,
		participantID,
		participantIdentity,
	)
	if existingIsConnected {
		worker.SetConnected()
	}

	t.workers[participantID] = worker

	worker.next = t.workerList
	t.workerList = worker

	return worker, false
}

func (t *telemetryService) LocalRoomState(ctx context.Context, info *voicekit.AnalyticsNodeRooms) {
	t.enqueue(func() {
		t.SendNodeRoomStates(ctx, info)
	})
}
