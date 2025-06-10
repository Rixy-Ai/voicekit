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

	"go.uber.org/atomic"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/voicekit/protocol/voicekit"
	"github.com/voicekit/protocol/logger"
	"github.com/voicekit/protocol/observability/roomobs"
	"github.com/voicekit/protocol/rpc"
	"github.com/voicekit/protocol/utils/guid"

	"github.com/voicekit/voicekit-server/pkg/config"
	"github.com/voicekit/voicekit-server/pkg/routing"
)

//counterfeiter:generate . AnalyticsService
type AnalyticsService interface {
	SendStats(ctx context.Context, stats []*voicekit.AnalyticsStat)
	SendEvent(ctx context.Context, events *voicekit.AnalyticsEvent)
	SendNodeRoomStates(ctx context.Context, nodeRooms *voicekit.AnalyticsNodeRooms)
	RoomProjectReporter(ctx context.Context) roomobs.ProjectReporter
}

type analyticsService struct {
	analyticsKey   string
	nodeID         string
	sequenceNumber atomic.Uint64

	events    rpc.AnalyticsRecorderService_IngestEventsClient
	stats     rpc.AnalyticsRecorderService_IngestStatsClient
	nodeRooms rpc.AnalyticsRecorderService_IngestNodeRoomStatesClient
}

func NewAnalyticsService(_ *config.Config, currentNode routing.LocalNode) AnalyticsService {
	return &analyticsService{
		analyticsKey: "", // TODO: conf.AnalyticsKey
		nodeID:       string(currentNode.NodeID()),
	}
}

func (a *analyticsService) SendStats(_ context.Context, stats []*voicekit.AnalyticsStat) {
	if a.stats == nil {
		return
	}

	for _, stat := range stats {
		stat.Id = guid.New("AS_")
		stat.AnalyticsKey = a.analyticsKey
		stat.Node = a.nodeID
	}
	if err := a.stats.Send(&voicekit.AnalyticsStats{Stats: stats}); err != nil {
		logger.Errorw("failed to send stats", err)
	}
}

func (a *analyticsService) SendEvent(_ context.Context, event *voicekit.AnalyticsEvent) {
	if a.events == nil {
		return
	}

	event.Id = guid.New("AE_")
	event.NodeId = a.nodeID
	event.AnalyticsKey = a.analyticsKey
	if err := a.events.Send(&voicekit.AnalyticsEvents{
		Events: []*voicekit.AnalyticsEvent{event},
	}); err != nil {
		logger.Errorw("failed to send event", err, "eventType", event.Type.String())
	}
}

func (a *analyticsService) SendNodeRoomStates(_ context.Context, nodeRooms *voicekit.AnalyticsNodeRooms) {
	if a.nodeRooms == nil {
		return
	}

	nodeRooms.NodeId = a.nodeID
	nodeRooms.SequenceNumber = a.sequenceNumber.Add(1)
	nodeRooms.Timestamp = timestamppb.Now()
	if err := a.nodeRooms.Send(nodeRooms); err != nil {
		logger.Errorw("failed to send node room states", err)
	}
}

func (a *analyticsService) RoomProjectReporter(_ context.Context) roomobs.ProjectReporter {
	return roomobs.NewNoopProjectReporter()
}
