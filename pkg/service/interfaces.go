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
	"time"

	"github.com/voicekit/protocol/voicekit"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// encapsulates CRUD operations for room settings
//
//counterfeiter:generate . ObjectStore
type ObjectStore interface {
	ServiceStore

	// enable locking on a specific room to prevent race
	// returns a (lock uuid, error)
	LockRoom(ctx context.Context, roomName voicekit.RoomName, duration time.Duration) (string, error)
	UnlockRoom(ctx context.Context, roomName voicekit.RoomName, uid string) error

	StoreRoom(ctx context.Context, room *voicekit.Room, internal *voicekit.RoomInternal) error

	StoreParticipant(ctx context.Context, roomName voicekit.RoomName, participant *voicekit.ParticipantInfo) error
	DeleteParticipant(ctx context.Context, roomName voicekit.RoomName, identity voicekit.ParticipantIdentity) error
}

//counterfeiter:generate . ServiceStore
type ServiceStore interface {
	LoadRoom(ctx context.Context, roomName voicekit.RoomName, includeInternal bool) (*voicekit.Room, *voicekit.RoomInternal, error)
	DeleteRoom(ctx context.Context, roomName voicekit.RoomName) error

	// ListRooms returns currently active rooms. if names is not nil, it'll filter and return
	// only rooms that match
	ListRooms(ctx context.Context, roomNames []voicekit.RoomName) ([]*voicekit.Room, error)
	LoadParticipant(ctx context.Context, roomName voicekit.RoomName, identity voicekit.ParticipantIdentity) (*voicekit.ParticipantInfo, error)
	ListParticipants(ctx context.Context, roomName voicekit.RoomName) ([]*voicekit.ParticipantInfo, error)
}

type OSSServiceStore interface {
	HasParticipant(context.Context, voicekit.RoomName, voicekit.ParticipantIdentity) (bool, error)
}

//counterfeiter:generate . EgressStore
type EgressStore interface {
	StoreEgress(ctx context.Context, info *voicekit.EgressInfo) error
	LoadEgress(ctx context.Context, egressID string) (*voicekit.EgressInfo, error)
	ListEgress(ctx context.Context, roomName voicekit.RoomName, active bool) ([]*voicekit.EgressInfo, error)
	UpdateEgress(ctx context.Context, info *voicekit.EgressInfo) error
}

//counterfeiter:generate . IngressStore
type IngressStore interface {
	StoreIngress(ctx context.Context, info *voicekit.IngressInfo) error
	LoadIngress(ctx context.Context, ingressID string) (*voicekit.IngressInfo, error)
	LoadIngressFromStreamKey(ctx context.Context, streamKey string) (*voicekit.IngressInfo, error)
	ListIngress(ctx context.Context, roomName voicekit.RoomName) ([]*voicekit.IngressInfo, error)
	UpdateIngress(ctx context.Context, info *voicekit.IngressInfo) error
	UpdateIngressState(ctx context.Context, ingressId string, state *voicekit.IngressState) error
	DeleteIngress(ctx context.Context, info *voicekit.IngressInfo) error
}

//counterfeiter:generate . RoomAllocator
type RoomAllocator interface {
	AutoCreateEnabled(ctx context.Context) bool
	SelectRoomNode(ctx context.Context, roomName voicekit.RoomName, nodeID voicekit.NodeID) error
	CreateRoom(ctx context.Context, req *voicekit.CreateRoomRequest, isExplicit bool) (*voicekit.Room, *voicekit.RoomInternal, bool, error)
	ValidateCreateRoom(ctx context.Context, roomName voicekit.RoomName) error
}

//counterfeiter:generate . SIPStore
type SIPStore interface {
	StoreSIPTrunk(ctx context.Context, info *voicekit.SIPTrunkInfo) error
	StoreSIPInboundTrunk(ctx context.Context, info *voicekit.SIPInboundTrunkInfo) error
	StoreSIPOutboundTrunk(ctx context.Context, info *voicekit.SIPOutboundTrunkInfo) error
	LoadSIPTrunk(ctx context.Context, sipTrunkID string) (*voicekit.SIPTrunkInfo, error)
	LoadSIPInboundTrunk(ctx context.Context, sipTrunkID string) (*voicekit.SIPInboundTrunkInfo, error)
	LoadSIPOutboundTrunk(ctx context.Context, sipTrunkID string) (*voicekit.SIPOutboundTrunkInfo, error)
	ListSIPTrunk(ctx context.Context, opts *voicekit.ListSIPTrunkRequest) (*voicekit.ListSIPTrunkResponse, error)
	ListSIPInboundTrunk(ctx context.Context, opts *voicekit.ListSIPInboundTrunkRequest) (*voicekit.ListSIPInboundTrunkResponse, error)
	ListSIPOutboundTrunk(ctx context.Context, opts *voicekit.ListSIPOutboundTrunkRequest) (*voicekit.ListSIPOutboundTrunkResponse, error)
	DeleteSIPTrunk(ctx context.Context, sipTrunkID string) error

	StoreSIPDispatchRule(ctx context.Context, info *voicekit.SIPDispatchRuleInfo) error
	LoadSIPDispatchRule(ctx context.Context, sipDispatchRuleID string) (*voicekit.SIPDispatchRuleInfo, error)
	ListSIPDispatchRule(ctx context.Context, opts *voicekit.ListSIPDispatchRuleRequest) (*voicekit.ListSIPDispatchRuleResponse, error)
	DeleteSIPDispatchRule(ctx context.Context, sipDispatchRuleID string) error
}

//counterfeiter:generate . AgentStore
type AgentStore interface {
	StoreAgentDispatch(ctx context.Context, dispatch *voicekit.AgentDispatch) error
	DeleteAgentDispatch(ctx context.Context, dispatch *voicekit.AgentDispatch) error
	ListAgentDispatches(ctx context.Context, roomName voicekit.RoomName) ([]*voicekit.AgentDispatch, error)

	StoreAgentJob(ctx context.Context, job *voicekit.Job) error
	DeleteAgentJob(ctx context.Context, job *voicekit.Job) error
}
