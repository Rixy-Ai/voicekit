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

package rtc

import (
	"github.com/voicekit/protocol/voicekit"
	"github.com/voicekit/protocol/logger"
	"github.com/voicekit/protocol/observability/roomobs"
	"github.com/voicekit/protocol/utils"
	"github.com/voicekit/protocol/utils/guid"

	"github.com/voicekit/voicekit-server/pkg/rtc/types"
	"github.com/voicekit/voicekit-server/pkg/rtc/types/typesfakes"
)

func NewMockParticipant(identity voicekit.ParticipantIdentity, protocol types.ProtocolVersion, hidden bool, publisher bool) *typesfakes.FakeLocalParticipant {
	p := &typesfakes.FakeLocalParticipant{}
	sid := guid.New(utils.ParticipantPrefix)
	p.IDReturns(voicekit.ParticipantID(sid))
	p.IdentityReturns(identity)
	p.StateReturns(voicekit.ParticipantInfo_JOINED)
	p.ProtocolVersionReturns(protocol)
	p.CanSubscribeReturns(true)
	p.CanPublishSourceReturns(!hidden)
	p.CanPublishDataReturns(!hidden)
	p.HiddenReturns(hidden)
	p.ToProtoReturns(&voicekit.ParticipantInfo{
		Sid:         sid,
		Identity:    string(identity),
		State:       voicekit.ParticipantInfo_JOINED,
		IsPublisher: publisher,
	})
	p.ToProtoWithVersionReturns(&voicekit.ParticipantInfo{
		Sid:         sid,
		Identity:    string(identity),
		State:       voicekit.ParticipantInfo_JOINED,
		IsPublisher: publisher,
	}, utils.TimedVersion(0))

	p.SetMetadataCalls(func(m string) {
		var f func(participant types.LocalParticipant)
		if p.OnParticipantUpdateCallCount() > 0 {
			f = p.OnParticipantUpdateArgsForCall(p.OnParticipantUpdateCallCount() - 1)
		}
		if f != nil {
			f(p)
		}
	})
	updateTrack := func() {
		var f func(participant types.LocalParticipant, track types.MediaTrack)
		if p.OnTrackUpdatedCallCount() > 0 {
			f = p.OnTrackUpdatedArgsForCall(p.OnTrackUpdatedCallCount() - 1)
		}
		if f != nil {
			f(p, NewMockTrack(voicekit.TrackType_VIDEO, "testcam"))
		}
	}

	p.SetTrackMutedCalls(func(sid voicekit.TrackID, muted bool, fromServer bool) *voicekit.TrackInfo {
		updateTrack()
		return nil
	})
	p.AddTrackCalls(func(req *voicekit.AddTrackRequest) {
		updateTrack()
	})
	p.GetLoggerReturns(logger.GetLogger())
	p.GetReporterReturns(roomobs.NewNoopParticipantSessionReporter())

	return p
}

func NewMockTrack(kind voicekit.TrackType, name string) *typesfakes.FakeMediaTrack {
	t := &typesfakes.FakeMediaTrack{}
	t.IDReturns(voicekit.TrackID(guid.New(utils.TrackPrefix)))
	t.KindReturns(kind)
	t.NameReturns(name)
	t.ToProtoReturns(&voicekit.TrackInfo{
		Type: kind,
		Name: name,
	})
	return t
}
