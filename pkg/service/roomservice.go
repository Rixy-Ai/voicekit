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

package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/twitchtv/twirp"

	"github.com/voicekit/voicekit-server/pkg/config"
	"github.com/voicekit/voicekit-server/pkg/routing"
	"github.com/voicekit/voicekit-server/pkg/rtc"
	"github.com/voicekit/protocol/egress"
	"github.com/voicekit/protocol/voicekit"
	"github.com/voicekit/protocol/logger"
	"github.com/voicekit/protocol/rpc"
	"github.com/voicekit/protocol/utils"
)

type RoomService struct {
	limitConf         config.LimitConfig
	apiConf           config.APIConfig
	router            routing.MessageRouter
	roomAllocator     RoomAllocator
	roomStore         ServiceStore
	egressLauncher    rtc.EgressLauncher
	topicFormatter    rpc.TopicFormatter
	roomClient        rpc.TypedRoomClient
	participantClient rpc.TypedParticipantClient
}

func NewRoomService(
	limitConf config.LimitConfig,
	apiConf config.APIConfig,
	router routing.MessageRouter,
	roomAllocator RoomAllocator,
	serviceStore ServiceStore,
	egressLauncher rtc.EgressLauncher,
	topicFormatter rpc.TopicFormatter,
	roomClient rpc.TypedRoomClient,
	participantClient rpc.TypedParticipantClient,
) (svc *RoomService, err error) {
	svc = &RoomService{
		limitConf:         limitConf,
		apiConf:           apiConf,
		router:            router,
		roomAllocator:     roomAllocator,
		roomStore:         serviceStore,
		egressLauncher:    egressLauncher,
		topicFormatter:    topicFormatter,
		roomClient:        roomClient,
		participantClient: participantClient,
	}
	return
}

func (s *RoomService) CreateRoom(ctx context.Context, req *voicekit.CreateRoomRequest) (*voicekit.Room, error) {
	redactedReq := redactCreateRoomRequest(req)
	RecordRequest(ctx, redactedReq)

	AppendLogFields(ctx, "room", req.Name, "request", logger.Proto(redactedReq))
	if err := EnsureCreatePermission(ctx); err != nil {
		return nil, twirpAuthError(err)
	} else if req.Egress != nil && s.egressLauncher == nil {
		return nil, ErrEgressNotConnected
	}

	if !s.limitConf.CheckRoomNameLength(req.Name) {
		return nil, fmt.Errorf("%w: max length %d", ErrRoomNameExceedsLimits, s.limitConf.MaxRoomNameLength)
	}

	err := s.roomAllocator.SelectRoomNode(ctx, voicekit.RoomName(req.Name), voicekit.NodeID(req.NodeId))
	if err != nil {
		return nil, err
	}

	room, err := s.router.CreateRoom(ctx, req)
	RecordResponse(ctx, room)
	return room, err
}

func (s *RoomService) ListRooms(ctx context.Context, req *voicekit.ListRoomsRequest) (*voicekit.ListRoomsResponse, error) {
	RecordRequest(ctx, req)

	AppendLogFields(ctx, "room", req.Names)
	err := EnsureListPermission(ctx)
	if err != nil {
		return nil, twirpAuthError(err)
	}

	var names []voicekit.RoomName
	if len(req.Names) > 0 {
		names = voicekit.StringsAsIDs[voicekit.RoomName](req.Names)
	}
	rooms, err := s.roomStore.ListRooms(ctx, names)
	if err != nil {
		// TODO: translate error codes to Twirp
		return nil, err
	}

	res := &voicekit.ListRoomsResponse{
		Rooms: rooms,
	}
	RecordResponse(ctx, res)
	return res, nil
}

func (s *RoomService) DeleteRoom(ctx context.Context, req *voicekit.DeleteRoomRequest) (*voicekit.DeleteRoomResponse, error) {
	RecordRequest(ctx, req)

	AppendLogFields(ctx, "room", req.Room)
	if err := EnsureCreatePermission(ctx); err != nil {
		return nil, twirpAuthError(err)
	}

	_, _, err := s.roomStore.LoadRoom(ctx, voicekit.RoomName(req.Room), false)
	if err != nil {
		return nil, err
	}

	// ensure at least one node is available to handle the request
	room, err := s.router.CreateRoom(ctx, &voicekit.CreateRoomRequest{Name: req.Room})
	if err != nil {
		return nil, err
	}

	_, err = s.roomClient.DeleteRoom(ctx, s.topicFormatter.RoomTopic(ctx, voicekit.RoomName(req.Room)), req)
	if err != nil {
		return nil, err
	}

	err = s.roomStore.DeleteRoom(ctx, voicekit.RoomName(req.Room))
	res := &voicekit.DeleteRoomResponse{}
	RecordResponse(ctx, room)
	return res, err
}

func (s *RoomService) ListParticipants(ctx context.Context, req *voicekit.ListParticipantsRequest) (*voicekit.ListParticipantsResponse, error) {
	RecordRequest(ctx, req)

	AppendLogFields(ctx, "room", req.Room)
	if err := EnsureAdminPermission(ctx, voicekit.RoomName(req.Room)); err != nil {
		return nil, twirpAuthError(err)
	}

	participants, err := s.roomStore.ListParticipants(ctx, voicekit.RoomName(req.Room))
	if err != nil {
		return nil, err
	}

	res := &voicekit.ListParticipantsResponse{
		Participants: participants,
	}
	RecordResponse(ctx, res)
	return res, nil
}

func (s *RoomService) GetParticipant(ctx context.Context, req *voicekit.RoomParticipantIdentity) (*voicekit.ParticipantInfo, error) {
	RecordRequest(ctx, req)

	AppendLogFields(ctx, "room", req.Room, "participant", req.Identity)
	if err := EnsureAdminPermission(ctx, voicekit.RoomName(req.Room)); err != nil {
		return nil, twirpAuthError(err)
	}

	participant, err := s.roomStore.LoadParticipant(ctx, voicekit.RoomName(req.Room), voicekit.ParticipantIdentity(req.Identity))
	if err != nil {
		return nil, err
	}

	RecordResponse(ctx, participant)
	return participant, nil
}

func (s *RoomService) RemoveParticipant(ctx context.Context, req *voicekit.RoomParticipantIdentity) (*voicekit.RemoveParticipantResponse, error) {
	RecordRequest(ctx, req)

	AppendLogFields(ctx, "room", req.Room, "participant", req.Identity)

	if err := EnsureAdminPermission(ctx, voicekit.RoomName(req.Room)); err != nil {
		return nil, twirpAuthError(err)
	}

	if _, err := s.roomStore.LoadParticipant(ctx, voicekit.RoomName(req.Room), voicekit.ParticipantIdentity(req.Identity)); err == ErrParticipantNotFound {
		return nil, twirp.NotFoundError("participant not found")
	}

	res, err := s.participantClient.RemoveParticipant(ctx, s.topicFormatter.ParticipantTopic(ctx, voicekit.RoomName(req.Room), voicekit.ParticipantIdentity(req.Identity)), req)
	RecordResponse(ctx, res)
	return res, err
}

func (s *RoomService) MutePublishedTrack(ctx context.Context, req *voicekit.MuteRoomTrackRequest) (*voicekit.MuteRoomTrackResponse, error) {
	RecordRequest(ctx, req)

	AppendLogFields(ctx, "room", req.Room, "participant", req.Identity, "trackID", req.TrackSid, "muted", req.Muted)
	if err := EnsureAdminPermission(ctx, voicekit.RoomName(req.Room)); err != nil {
		return nil, twirpAuthError(err)
	}

	res, err := s.participantClient.MutePublishedTrack(ctx, s.topicFormatter.ParticipantTopic(ctx, voicekit.RoomName(req.Room), voicekit.ParticipantIdentity(req.Identity)), req)
	RecordResponse(ctx, res)
	return res, err
}

func (s *RoomService) UpdateParticipant(ctx context.Context, req *voicekit.UpdateParticipantRequest) (*voicekit.ParticipantInfo, error) {
	RecordRequest(ctx, redactUpdateParticipantRequest(req))

	AppendLogFields(ctx, "room", req.Room, "participant", req.Identity)

	if !s.limitConf.CheckParticipantNameLength(req.Name) {
		return nil, twirp.InvalidArgumentError(ErrNameExceedsLimits.Error(), strconv.Itoa(s.limitConf.MaxParticipantNameLength))
	}

	if !s.limitConf.CheckMetadataSize(req.Metadata) {
		return nil, twirp.InvalidArgumentError(ErrMetadataExceedsLimits.Error(), strconv.Itoa(int(s.limitConf.MaxMetadataSize)))
	}

	if !s.limitConf.CheckAttributesSize(req.Attributes) {
		return nil, twirp.InvalidArgumentError(ErrAttributeExceedsLimits.Error(), strconv.Itoa(int(s.limitConf.MaxAttributesSize)))
	}

	if err := EnsureAdminPermission(ctx, voicekit.RoomName(req.Room)); err != nil {
		return nil, twirpAuthError(err)
	}

	if os, ok := s.roomStore.(OSSServiceStore); ok {
		found, err := os.HasParticipant(ctx, voicekit.RoomName(req.Room), voicekit.ParticipantIdentity(req.Identity))
		if err != nil {
			return nil, err
		} else if !found {
			return nil, ErrParticipantNotFound
		}
	}

	res, err := s.participantClient.UpdateParticipant(ctx, s.topicFormatter.ParticipantTopic(ctx, voicekit.RoomName(req.Room), voicekit.ParticipantIdentity(req.Identity)), req)
	RecordResponse(ctx, res)
	return res, err
}

func (s *RoomService) UpdateSubscriptions(ctx context.Context, req *voicekit.UpdateSubscriptionsRequest) (*voicekit.UpdateSubscriptionsResponse, error) {
	RecordRequest(ctx, req)

	trackSIDs := append(make([]string, 0), req.TrackSids...)
	for _, pt := range req.ParticipantTracks {
		trackSIDs = append(trackSIDs, pt.TrackSids...)
	}
	AppendLogFields(ctx, "room", req.Room, "participant", req.Identity, "trackID", trackSIDs)

	if err := EnsureAdminPermission(ctx, voicekit.RoomName(req.Room)); err != nil {
		return nil, twirpAuthError(err)
	}

	res, err := s.participantClient.UpdateSubscriptions(ctx, s.topicFormatter.ParticipantTopic(ctx, voicekit.RoomName(req.Room), voicekit.ParticipantIdentity(req.Identity)), req)
	RecordResponse(ctx, res)
	return res, err
}

func (s *RoomService) SendData(ctx context.Context, req *voicekit.SendDataRequest) (*voicekit.SendDataResponse, error) {
	RecordRequest(ctx, redactSendDataRequest(req))

	roomName := voicekit.RoomName(req.Room)
	AppendLogFields(ctx, "room", roomName, "size", len(req.Data))
	if err := EnsureAdminPermission(ctx, roomName); err != nil {
		return nil, twirpAuthError(err)
	}

	// nonce is either absent or 128-bit UUID
	if len(req.Nonce) != 0 && len(req.Nonce) != 16 {
		return nil, twirp.NewError(twirp.InvalidArgument, fmt.Sprintf("nonce should be 16-bytes or not present, got: %d bytes", len(req.Nonce)))
	}

	res, err := s.roomClient.SendData(ctx, s.topicFormatter.RoomTopic(ctx, voicekit.RoomName(req.Room)), req)
	RecordResponse(ctx, res)
	return res, err
}

func (s *RoomService) UpdateRoomMetadata(ctx context.Context, req *voicekit.UpdateRoomMetadataRequest) (*voicekit.Room, error) {
	RecordRequest(ctx, redactUpdateRoomMetadataRequest(req))

	AppendLogFields(ctx, "room", req.Room, "size", len(req.Metadata))
	maxMetadataSize := int(s.limitConf.MaxMetadataSize)
	if maxMetadataSize > 0 && len(req.Metadata) > maxMetadataSize {
		return nil, twirp.InvalidArgumentError(ErrMetadataExceedsLimits.Error(), strconv.Itoa(maxMetadataSize))
	}

	if err := EnsureAdminPermission(ctx, voicekit.RoomName(req.Room)); err != nil {
		return nil, twirpAuthError(err)
	}

	_, _, err := s.roomStore.LoadRoom(ctx, voicekit.RoomName(req.Room), false)
	if err != nil {
		return nil, err
	}

	room, err := s.roomClient.UpdateRoomMetadata(ctx, s.topicFormatter.RoomTopic(ctx, voicekit.RoomName(req.Room)), req)
	if err != nil {
		return nil, err
	}

	RecordResponse(ctx, room)
	return room, nil
}

func (s *RoomService) ForwardParticipant(ctx context.Context, req *voicekit.ForwardParticipantRequest) (*voicekit.ForwardParticipantResponse, error) {
	RecordRequest(ctx, req)

	roomName := voicekit.RoomName(req.Room)
	AppendLogFields(ctx, "room", roomName, "participant", req.Identity)
	if err := EnsureDestRoomPermission(ctx, roomName, voicekit.RoomName(req.DestinationRoom)); err != nil {
		return nil, twirpAuthError(err)
	}

	if req.Room == req.DestinationRoom {
		return nil, twirp.InvalidArgumentError(ErrDestinationSameAsSourceRoom.Error(), "")
	}

	res, err := s.participantClient.ForwardParticipant(ctx, s.topicFormatter.ParticipantTopic(ctx, voicekit.RoomName(req.Room), voicekit.ParticipantIdentity(req.Identity)), req)
	RecordResponse(ctx, res)
	return res, err
}

func (s *RoomService) MoveParticipant(ctx context.Context, req *voicekit.MoveParticipantRequest) (*voicekit.MoveParticipantResponse, error) {
	RecordRequest(ctx, req)

	roomName := voicekit.RoomName(req.Room)
	AppendLogFields(ctx, "room", roomName, "participant", req.Identity)
	if err := EnsureDestRoomPermission(ctx, roomName, voicekit.RoomName(req.DestinationRoom)); err != nil {
		return nil, twirpAuthError(err)
	}

	if req.Room == req.DestinationRoom {
		return nil, twirp.InvalidArgumentError(ErrDestinationSameAsSourceRoom.Error(), "")
	}

	res, err := s.participantClient.MoveParticipant(ctx, s.topicFormatter.ParticipantTopic(ctx, voicekit.RoomName(req.Room), voicekit.ParticipantIdentity(req.Identity)), req)
	RecordResponse(ctx, res)
	return res, err
}

func redactCreateRoomRequest(req *voicekit.CreateRoomRequest) *voicekit.CreateRoomRequest {
	if req.Egress == nil && req.Metadata == "" {
		// nothing to redact
		return req
	}

	clone := utils.CloneProto(req)

	if clone.Egress != nil {
		if clone.Egress.Room != nil {
			egress.RedactEncodedOutputs(clone.Egress.Room)
		}
		if clone.Egress.Participant != nil {
			egress.RedactAutoEncodedOutput(clone.Egress.Participant)
		}
		if clone.Egress.Tracks != nil {
			egress.RedactUpload(clone.Egress.Tracks)
		}
	}

	// replace with size of metadata to provide visibility on request size
	if clone.Metadata != "" {
		clone.Metadata = fmt.Sprintf("__size: %d", len(clone.Metadata))
	}

	return clone
}

func redactUpdateParticipantRequest(req *voicekit.UpdateParticipantRequest) *voicekit.UpdateParticipantRequest {
	if req.Metadata == "" && len(req.Attributes) == 0 {
		return req
	}

	clone := utils.CloneProto(req)

	// replace with size of metadata/attributes to provide visibility on request size
	if clone.Metadata != "" {
		clone.Metadata = fmt.Sprintf("__size: %d", len(clone.Metadata))
	}

	if len(clone.Attributes) != 0 {
		keysSize := 0
		valuesSize := 0
		for k, v := range clone.Attributes {
			keysSize += len(k)
			valuesSize += len(v)
		}

		clone.Attributes = map[string]string{
			"__num_elements": fmt.Sprintf("%d", len(clone.Attributes)),
			"__keys_size":    fmt.Sprintf("%d", keysSize),
			"__values_size":  fmt.Sprintf("%d", valuesSize),
		}
	}

	return clone
}

func redactSendDataRequest(req *voicekit.SendDataRequest) *voicekit.SendDataRequest {
	if len(req.Data) == 0 {
		return req
	}

	clone := utils.CloneProto(req)

	// replace with size of data to provide visibility on request size
	clone.Data = []byte(fmt.Sprintf("__size: %d", len(clone.Data)))

	return clone
}

func redactUpdateRoomMetadataRequest(req *voicekit.UpdateRoomMetadataRequest) *voicekit.UpdateRoomMetadataRequest {
	if req.Metadata == "" {
		return req
	}

	clone := utils.CloneProto(req)

	// replace with size of metadata to provide visibility on request size
	clone.Metadata = fmt.Sprintf("__size: %d", len(clone.Metadata))

	return clone
}
