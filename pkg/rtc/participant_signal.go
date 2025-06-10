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
	"fmt"
	"time"

	"github.com/pion/webrtc/v4"

	"github.com/voicekit/protocol/voicekit"
	"github.com/voicekit/protocol/logger"
	"github.com/voicekit/protocol/utils"
	"github.com/voicekit/psrpc"

	"github.com/voicekit/voicekit-server/pkg/routing"
	"github.com/voicekit/voicekit-server/pkg/rtc/types"
)

func (p *ParticipantImpl) getResponseSink() routing.MessageSink {
	p.resSinkMu.Lock()
	defer p.resSinkMu.Unlock()
	return p.resSink
}

func (p *ParticipantImpl) SetResponseSink(sink routing.MessageSink) {
	p.resSinkMu.Lock()
	defer p.resSinkMu.Unlock()
	p.resSink = sink
}

func (p *ParticipantImpl) SendJoinResponse(joinResponse *voicekit.JoinResponse) error {
	// keep track of participant updates and versions
	p.updateLock.Lock()
	for _, op := range joinResponse.OtherParticipants {
		p.updateCache.Add(voicekit.ParticipantID(op.Sid), participantUpdateInfo{
			identity:  voicekit.ParticipantIdentity(op.Identity),
			version:   op.Version,
			state:     op.State,
			updatedAt: time.Now(),
		})
	}
	p.updateLock.Unlock()

	// send Join response
	err := p.writeMessage(&voicekit.SignalResponse{
		Message: &voicekit.SignalResponse_Join{
			Join: joinResponse,
		},
	})
	if err != nil {
		return err
	}

	// update state after sending message, so that no participant updates could slip through before JoinResponse is sent
	p.updateLock.Lock()
	if p.State() == voicekit.ParticipantInfo_JOINING {
		p.updateState(voicekit.ParticipantInfo_JOINED)
	}
	queuedUpdates := p.queuedUpdates
	p.queuedUpdates = nil
	p.updateLock.Unlock()

	if len(queuedUpdates) > 0 {
		return p.SendParticipantUpdate(queuedUpdates)
	}

	return nil
}

func (p *ParticipantImpl) SendParticipantUpdate(participantsToUpdate []*voicekit.ParticipantInfo) error {
	p.updateLock.Lock()
	if p.IsDisconnected() {
		p.updateLock.Unlock()
		return nil
	}

	if !p.IsReady() {
		// queue up updates
		p.queuedUpdates = append(p.queuedUpdates, participantsToUpdate...)
		p.updateLock.Unlock()
		return nil
	}
	validUpdates := make([]*voicekit.ParticipantInfo, 0, len(participantsToUpdate))
	for _, pi := range participantsToUpdate {
		isValid := true
		pID := voicekit.ParticipantID(pi.Sid)
		if lastVersion, ok := p.updateCache.Get(pID); ok {
			// this is a message delivered out of order, a more recent version of the message had already been
			// sent.
			if pi.Version < lastVersion.version {
				p.params.Logger.Debugw(
					"skipping outdated participant update",
					"otherParticipant", pi.Identity,
					"otherPID", pi.Sid,
					"version", pi.Version,
					"lastVersion", lastVersion,
				)
				isValid = false
			}
		}
		if pi.Permission != nil && pi.Permission.Hidden && pi.Sid != string(p.ID()) {
			p.params.Logger.Debugw("skipping hidden participant update", "otherParticipant", pi.Identity)
			isValid = false
		}
		if isValid {
			p.updateCache.Add(pID, participantUpdateInfo{
				identity:  voicekit.ParticipantIdentity(pi.Identity),
				version:   pi.Version,
				state:     pi.State,
				updatedAt: time.Now(),
			})
			validUpdates = append(validUpdates, pi)
		}
	}
	p.updateLock.Unlock()

	if len(validUpdates) == 0 {
		return nil
	}

	return p.writeMessage(&voicekit.SignalResponse{
		Message: &voicekit.SignalResponse_Update{
			Update: &voicekit.ParticipantUpdate{
				Participants: validUpdates,
			},
		},
	})
}

// SendSpeakerUpdate notifies participant changes to speakers. only send members that have changed since last update
func (p *ParticipantImpl) SendSpeakerUpdate(speakers []*voicekit.SpeakerInfo, force bool) error {
	if !p.IsReady() {
		return nil
	}

	var scopedSpeakers []*voicekit.SpeakerInfo
	if force {
		scopedSpeakers = speakers
	} else {
		for _, s := range speakers {
			participantID := voicekit.ParticipantID(s.Sid)
			if p.IsSubscribedTo(participantID) || participantID == p.ID() {
				scopedSpeakers = append(scopedSpeakers, s)
			}
		}
	}

	if len(scopedSpeakers) == 0 {
		return nil
	}

	return p.writeMessage(&voicekit.SignalResponse{
		Message: &voicekit.SignalResponse_SpeakersChanged{
			SpeakersChanged: &voicekit.SpeakersChanged{
				Speakers: scopedSpeakers,
			},
		},
	})
}

func (p *ParticipantImpl) SendRoomUpdate(room *voicekit.Room) error {
	return p.writeMessage(&voicekit.SignalResponse{
		Message: &voicekit.SignalResponse_RoomUpdate{
			RoomUpdate: &voicekit.RoomUpdate{
				Room: room,
			},
		},
	})
}

func (p *ParticipantImpl) SendConnectionQualityUpdate(update *voicekit.ConnectionQualityUpdate) error {
	return p.writeMessage(&voicekit.SignalResponse{
		Message: &voicekit.SignalResponse_ConnectionQuality{
			ConnectionQuality: update,
		},
	})
}

func (p *ParticipantImpl) SendRefreshToken(token string) error {
	return p.writeMessage(&voicekit.SignalResponse{
		Message: &voicekit.SignalResponse_RefreshToken{
			RefreshToken: token,
		},
	})
}

func (p *ParticipantImpl) SendRequestResponse(requestResponse *voicekit.RequestResponse) error {
	if requestResponse.RequestId == 0 || !p.params.ClientInfo.SupportErrorResponse() {
		return nil
	}

	if requestResponse.Reason == voicekit.RequestResponse_OK && !p.ProtocolVersion().SupportsNonErrorSignalResponse() {
		return nil
	}

	return p.writeMessage(&voicekit.SignalResponse{
		Message: &voicekit.SignalResponse_RequestResponse{
			RequestResponse: requestResponse,
		},
	})
}

func (p *ParticipantImpl) SendRoomMovedResponse(roomMovedResponse *voicekit.RoomMovedResponse) error {
	return p.writeMessage(&voicekit.SignalResponse{
		Message: &voicekit.SignalResponse_RoomMoved{
			RoomMoved: roomMovedResponse,
		},
	})
}

func (p *ParticipantImpl) HandleReconnectAndSendResponse(reconnectReason voicekit.ReconnectReason, reconnectResponse *voicekit.ReconnectResponse) error {
	p.TransportManager.HandleClientReconnect(reconnectReason)

	if !p.params.ClientInfo.CanHandleReconnectResponse() {
		return nil
	}
	if err := p.writeMessage(&voicekit.SignalResponse{
		Message: &voicekit.SignalResponse_Reconnect{
			Reconnect: reconnectResponse,
		},
	}); err != nil {
		return err
	}

	if p.params.ProtocolVersion.SupportHandlesDisconnectedUpdate() {
		return p.sendDisconnectUpdatesForReconnect()
	}

	return nil
}

func (p *ParticipantImpl) sendDisconnectUpdatesForReconnect() error {
	lastSignalAt := p.TransportManager.LastSeenSignalAt()
	var disconnectedParticipants []*voicekit.ParticipantInfo
	p.updateLock.Lock()
	keys := p.updateCache.Keys()
	for i := len(keys) - 1; i >= 0; i-- {
		if info, ok := p.updateCache.Get(keys[i]); ok {
			if info.updatedAt.Before(lastSignalAt) {
				break
			} else if info.state == voicekit.ParticipantInfo_DISCONNECTED {
				disconnectedParticipants = append(disconnectedParticipants, &voicekit.ParticipantInfo{
					Sid:      string(keys[i]),
					Identity: string(info.identity),
					Version:  info.version,
					State:    voicekit.ParticipantInfo_DISCONNECTED,
				})
			}
		}
	}
	p.updateLock.Unlock()

	if len(disconnectedParticipants) == 0 {
		return nil
	}

	return p.writeMessage(&voicekit.SignalResponse{
		Message: &voicekit.SignalResponse_Update{
			Update: &voicekit.ParticipantUpdate{
				Participants: disconnectedParticipants,
			},
		},
	})
}

func (p *ParticipantImpl) sendICECandidate(ic *webrtc.ICECandidate, target voicekit.SignalTarget) error {
	prevIC := p.icQueue[target].Swap(ic)
	if prevIC == nil {
		return nil
	}

	trickle := ToProtoTrickle(prevIC.ToJSON(), target, ic == nil)
	p.params.Logger.Debugw("sending ICE candidate", "transport", target, "trickle", logger.Proto(trickle))

	return p.writeMessage(&voicekit.SignalResponse{
		Message: &voicekit.SignalResponse_Trickle{
			Trickle: trickle,
		},
	})
}

func (p *ParticipantImpl) sendTrackMuted(trackID voicekit.TrackID, muted bool) {
	_ = p.writeMessage(&voicekit.SignalResponse{
		Message: &voicekit.SignalResponse_Mute{
			Mute: &voicekit.MuteTrackRequest{
				Sid:   string(trackID),
				Muted: muted,
			},
		},
	})
}

func (p *ParticipantImpl) sendTrackUnpublished(trackID voicekit.TrackID) {
	_ = p.writeMessage(&voicekit.SignalResponse{
		Message: &voicekit.SignalResponse_TrackUnpublished{
			TrackUnpublished: &voicekit.TrackUnpublishedResponse{
				TrackSid: string(trackID),
			},
		},
	})
}

func (p *ParticipantImpl) sendTrackHasBeenSubscribed(trackID voicekit.TrackID) {
	if !p.params.ClientInfo.SupportTrackSubscribedEvent() {
		return
	}
	_ = p.writeMessage(&voicekit.SignalResponse{
		Message: &voicekit.SignalResponse_TrackSubscribed{
			TrackSubscribed: &voicekit.TrackSubscribed{
				TrackSid: string(trackID),
			},
		},
	})
	p.params.Logger.Debugw("track has been subscribed", "trackID", trackID)
}

func (p *ParticipantImpl) writeMessage(msg *voicekit.SignalResponse) error {
	if p.IsDisconnected() || (!p.IsReady() && msg.GetJoin() == nil) {
		return nil
	}

	sink := p.getResponseSink()
	if sink == nil {
		p.params.Logger.Debugw("could not send message to participant", "messageType", fmt.Sprintf("%T", msg.Message))
		return nil
	}

	err := sink.WriteMessage(msg)
	if utils.ErrorIsOneOf(err, psrpc.Canceled, routing.ErrChannelClosed) {
		p.params.Logger.Debugw(
			"could not send message to participant",
			"error", err,
			"messageType", fmt.Sprintf("%T", msg.Message),
		)
		return nil
	} else if err != nil {
		p.params.Logger.Warnw(
			"could not send message to participant", err,
			"messageType", fmt.Sprintf("%T", msg.Message),
		)
		return err
	}
	return nil
}

// closes signal connection to notify client to resume/reconnect
func (p *ParticipantImpl) CloseSignalConnection(reason types.SignallingCloseReason) {
	sink := p.getResponseSink()
	if sink != nil {
		p.params.Logger.Debugw("closing signal connection", "reason", reason, "connID", sink.ConnectionID())
		sink.Close()
		p.SetResponseSink(nil)
	}
}

func (p *ParticipantImpl) sendLeaveRequest(
	reason types.ParticipantCloseReason,
	isExpectedToResume bool,
	isExpectedToReconnect bool,
	sendOnlyIfSupportingLeaveRequestWithAction bool,
) error {
	var leave *voicekit.LeaveRequest
	if p.ProtocolVersion().SupportsRegionsInLeaveRequest() {
		leave = &voicekit.LeaveRequest{
			Reason: reason.ToDisconnectReason(),
		}
		switch {
		case isExpectedToResume:
			leave.Action = voicekit.LeaveRequest_RESUME
		case isExpectedToReconnect:
			leave.Action = voicekit.LeaveRequest_RECONNECT
		default:
			leave.Action = voicekit.LeaveRequest_DISCONNECT
		}
		if leave.Action != voicekit.LeaveRequest_DISCONNECT {
			// sending region settings even for RESUME just in case client wants to a full reconnect despite server saying RESUME
			leave.Regions = p.helper().GetRegionSettings(p.params.ClientInfo.Address)
		}
	} else {
		if !sendOnlyIfSupportingLeaveRequestWithAction {
			leave = &voicekit.LeaveRequest{
				CanReconnect: isExpectedToReconnect,
				Reason:       reason.ToDisconnectReason(),
			}
		}
	}
	if leave != nil {
		return p.writeMessage(&voicekit.SignalResponse{
			Message: &voicekit.SignalResponse_Leave{
				Leave: leave,
			},
		})
	}

	return nil
}
