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

package types

import (
	"fmt"
	"time"

	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v4"

	"github.com/voicekit/protocol/auth"
	"github.com/voicekit/protocol/voicekit"
	"github.com/voicekit/protocol/logger"
	"github.com/voicekit/protocol/observability/roomobs"
	"github.com/voicekit/protocol/utils"

	"github.com/voicekit/voicekit-server/pkg/routing"
	"github.com/voicekit/voicekit-server/pkg/sfu"
	"github.com/voicekit/voicekit-server/pkg/sfu/buffer"
	"github.com/voicekit/voicekit-server/pkg/sfu/mime"
	"github.com/voicekit/voicekit-server/pkg/sfu/pacer"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

//counterfeiter:generate . WebsocketClient
type WebsocketClient interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteMessage(messageType int, data []byte) error
	WriteControl(messageType int, data []byte, deadline time.Time) error
	SetReadDeadline(deadline time.Time) error
	Close() error
}

type AddSubscriberParams struct {
	AllTracks bool
	TrackIDs  []voicekit.TrackID
}

// ---------------------------------------------

type MigrateState int32

const (
	MigrateStateInit MigrateState = iota
	MigrateStateSync
	MigrateStateComplete
)

func (m MigrateState) String() string {
	switch m {
	case MigrateStateInit:
		return "MIGRATE_STATE_INIT"
	case MigrateStateSync:
		return "MIGRATE_STATE_SYNC"
	case MigrateStateComplete:
		return "MIGRATE_STATE_COMPLETE"
	default:
		return fmt.Sprintf("%d", int(m))
	}
}

// ---------------------------------------------

type SubscribedCodecQuality struct {
	CodecMime mime.MimeType
	Quality   voicekit.VideoQuality
}

// ---------------------------------------------

type ParticipantCloseReason int

const (
	ParticipantCloseReasonNone ParticipantCloseReason = iota
	ParticipantCloseReasonClientRequestLeave
	ParticipantCloseReasonRoomManagerStop
	ParticipantCloseReasonVerifyFailed
	ParticipantCloseReasonJoinFailed
	ParticipantCloseReasonJoinTimeout
	ParticipantCloseReasonMessageBusFailed
	ParticipantCloseReasonPeerConnectionDisconnected
	ParticipantCloseReasonDuplicateIdentity
	ParticipantCloseReasonMigrationComplete
	ParticipantCloseReasonStale
	ParticipantCloseReasonServiceRequestRemoveParticipant
	ParticipantCloseReasonServiceRequestDeleteRoom
	ParticipantCloseReasonSimulateMigration
	ParticipantCloseReasonSimulateNodeFailure
	ParticipantCloseReasonSimulateServerLeave
	ParticipantCloseReasonSimulateLeaveRequest
	ParticipantCloseReasonNegotiateFailed
	ParticipantCloseReasonMigrationRequested
	ParticipantCloseReasonPublicationError
	ParticipantCloseReasonSubscriptionError
	ParticipantCloseReasonDataChannelError
	ParticipantCloseReasonMigrateCodecMismatch
	ParticipantCloseReasonSignalSourceClose
	ParticipantCloseReasonRoomClosed
	ParticipantCloseReasonUserUnavailable
	ParticipantCloseReasonUserRejected
	ParticipantCloseReasonMoveFailed
)

func (p ParticipantCloseReason) String() string {
	switch p {
	case ParticipantCloseReasonNone:
		return "NONE"
	case ParticipantCloseReasonClientRequestLeave:
		return "CLIENT_REQUEST_LEAVE"
	case ParticipantCloseReasonRoomManagerStop:
		return "ROOM_MANAGER_STOP"
	case ParticipantCloseReasonVerifyFailed:
		return "VERIFY_FAILED"
	case ParticipantCloseReasonJoinFailed:
		return "JOIN_FAILED"
	case ParticipantCloseReasonJoinTimeout:
		return "JOIN_TIMEOUT"
	case ParticipantCloseReasonMessageBusFailed:
		return "MESSAGE_BUS_FAILED"
	case ParticipantCloseReasonPeerConnectionDisconnected:
		return "PEER_CONNECTION_DISCONNECTED"
	case ParticipantCloseReasonDuplicateIdentity:
		return "DUPLICATE_IDENTITY"
	case ParticipantCloseReasonMigrationComplete:
		return "MIGRATION_COMPLETE"
	case ParticipantCloseReasonStale:
		return "STALE"
	case ParticipantCloseReasonServiceRequestRemoveParticipant:
		return "SERVICE_REQUEST_REMOVE_PARTICIPANT"
	case ParticipantCloseReasonServiceRequestDeleteRoom:
		return "SERVICE_REQUEST_DELETE_ROOM"
	case ParticipantCloseReasonSimulateMigration:
		return "SIMULATE_MIGRATION"
	case ParticipantCloseReasonSimulateNodeFailure:
		return "SIMULATE_NODE_FAILURE"
	case ParticipantCloseReasonSimulateServerLeave:
		return "SIMULATE_SERVER_LEAVE"
	case ParticipantCloseReasonSimulateLeaveRequest:
		return "SIMULATE_LEAVE_REQUEST"
	case ParticipantCloseReasonNegotiateFailed:
		return "NEGOTIATE_FAILED"
	case ParticipantCloseReasonMigrationRequested:
		return "MIGRATION_REQUESTED"
	case ParticipantCloseReasonPublicationError:
		return "PUBLICATION_ERROR"
	case ParticipantCloseReasonSubscriptionError:
		return "SUBSCRIPTION_ERROR"
	case ParticipantCloseReasonDataChannelError:
		return "DATA_CHANNEL_ERROR"
	case ParticipantCloseReasonMigrateCodecMismatch:
		return "MIGRATE_CODEC_MISMATCH"
	case ParticipantCloseReasonSignalSourceClose:
		return "SIGNAL_SOURCE_CLOSE"
	case ParticipantCloseReasonRoomClosed:
		return "ROOM_CLOSED"
	case ParticipantCloseReasonUserUnavailable:
		return "USER_UNAVAILABLE"
	case ParticipantCloseReasonUserRejected:
		return "USER_REJECTED"
	case ParticipantCloseReasonMoveFailed:
		return "MOVE_FAILED"
	default:
		return fmt.Sprintf("%d", int(p))
	}
}

func (p ParticipantCloseReason) ToDisconnectReason() voicekit.DisconnectReason {
	switch p {
	case ParticipantCloseReasonClientRequestLeave, ParticipantCloseReasonSimulateLeaveRequest:
		return voicekit.DisconnectReason_CLIENT_INITIATED
	case ParticipantCloseReasonRoomManagerStop:
		return voicekit.DisconnectReason_SERVER_SHUTDOWN
	case ParticipantCloseReasonVerifyFailed, ParticipantCloseReasonJoinFailed, ParticipantCloseReasonJoinTimeout, ParticipantCloseReasonMessageBusFailed:
		// expected to be connected but is not
		return voicekit.DisconnectReason_JOIN_FAILURE
	case ParticipantCloseReasonPeerConnectionDisconnected:
		return voicekit.DisconnectReason_CONNECTION_TIMEOUT
	case ParticipantCloseReasonDuplicateIdentity, ParticipantCloseReasonStale:
		return voicekit.DisconnectReason_DUPLICATE_IDENTITY
	case ParticipantCloseReasonMigrationRequested, ParticipantCloseReasonMigrationComplete, ParticipantCloseReasonSimulateMigration:
		return voicekit.DisconnectReason_MIGRATION
	case ParticipantCloseReasonServiceRequestRemoveParticipant:
		return voicekit.DisconnectReason_PARTICIPANT_REMOVED
	case ParticipantCloseReasonServiceRequestDeleteRoom:
		return voicekit.DisconnectReason_ROOM_DELETED
	case ParticipantCloseReasonSimulateNodeFailure, ParticipantCloseReasonSimulateServerLeave:
		return voicekit.DisconnectReason_SERVER_SHUTDOWN
	case ParticipantCloseReasonNegotiateFailed, ParticipantCloseReasonPublicationError, ParticipantCloseReasonSubscriptionError,
		ParticipantCloseReasonDataChannelError, ParticipantCloseReasonMigrateCodecMismatch, ParticipantCloseReasonMoveFailed:
		return voicekit.DisconnectReason_STATE_MISMATCH
	case ParticipantCloseReasonSignalSourceClose:
		return voicekit.DisconnectReason_SIGNAL_CLOSE
	case ParticipantCloseReasonRoomClosed:
		return voicekit.DisconnectReason_ROOM_CLOSED
	case ParticipantCloseReasonUserUnavailable:
		return voicekit.DisconnectReason_USER_UNAVAILABLE
	case ParticipantCloseReasonUserRejected:
		return voicekit.DisconnectReason_USER_REJECTED
	default:
		// the other types will map to unknown reason
		return voicekit.DisconnectReason_UNKNOWN_REASON
	}
}

// ---------------------------------------------

type SignallingCloseReason int

const (
	SignallingCloseReasonUnknown SignallingCloseReason = iota
	SignallingCloseReasonMigration
	SignallingCloseReasonResume
	SignallingCloseReasonTransportFailure
	SignallingCloseReasonFullReconnectPublicationError
	SignallingCloseReasonFullReconnectSubscriptionError
	SignallingCloseReasonFullReconnectDataChannelError
	SignallingCloseReasonFullReconnectNegotiateFailed
	SignallingCloseReasonParticipantClose
	SignallingCloseReasonDisconnectOnResume
	SignallingCloseReasonDisconnectOnResumeNoMessages
)

func (s SignallingCloseReason) String() string {
	switch s {
	case SignallingCloseReasonUnknown:
		return "UNKNOWN"
	case SignallingCloseReasonMigration:
		return "MIGRATION"
	case SignallingCloseReasonResume:
		return "RESUME"
	case SignallingCloseReasonTransportFailure:
		return "TRANSPORT_FAILURE"
	case SignallingCloseReasonFullReconnectPublicationError:
		return "FULL_RECONNECT_PUBLICATION_ERROR"
	case SignallingCloseReasonFullReconnectSubscriptionError:
		return "FULL_RECONNECT_SUBSCRIPTION_ERROR"
	case SignallingCloseReasonFullReconnectDataChannelError:
		return "FULL_RECONNECT_DATA_CHANNEL_ERROR"
	case SignallingCloseReasonFullReconnectNegotiateFailed:
		return "FULL_RECONNECT_NEGOTIATE_FAILED"
	case SignallingCloseReasonParticipantClose:
		return "PARTICIPANT_CLOSE"
	case SignallingCloseReasonDisconnectOnResume:
		return "DISCONNECT_ON_RESUME"
	case SignallingCloseReasonDisconnectOnResumeNoMessages:
		return "DISCONNECT_ON_RESUME_NO_MESSAGES"
	default:
		return fmt.Sprintf("%d", int(s))
	}
}

// ---------------------------------------------

//counterfeiter:generate . Participant
type Participant interface {
	ID() voicekit.ParticipantID
	Identity() voicekit.ParticipantIdentity
	State() voicekit.ParticipantInfo_State
	ConnectedAt() time.Time
	CloseReason() ParticipantCloseReason
	Kind() voicekit.ParticipantInfo_Kind
	IsRecorder() bool
	IsDependent() bool
	IsAgent() bool

	CanSkipBroadcast() bool
	Version() utils.TimedVersion
	ToProto() *voicekit.ParticipantInfo

	IsPublisher() bool
	GetPublishedTrack(trackID voicekit.TrackID) MediaTrack
	GetPublishedTracks() []MediaTrack
	RemovePublishedTrack(track MediaTrack, isExpectedToResume bool, shouldClose bool)

	GetAudioLevel() (smoothedLevel float64, active bool)

	// HasPermission checks permission of the subscriber by identity. Returns true if subscriber is allowed to subscribe
	// to the track with trackID
	HasPermission(trackID voicekit.TrackID, subIdentity voicekit.ParticipantIdentity) bool

	// permissions
	Hidden() bool

	Close(sendLeave bool, reason ParticipantCloseReason, isExpectedToResume bool) error

	SubscriptionPermission() (*voicekit.SubscriptionPermission, utils.TimedVersion)

	// updates from remotes
	UpdateSubscriptionPermission(
		subscriptionPermission *voicekit.SubscriptionPermission,
		timedVersion utils.TimedVersion,
		resolverBySid func(participantID voicekit.ParticipantID) LocalParticipant,
	) error

	DebugInfo() map[string]interface{}

	OnMetrics(callback func(Participant, *voicekit.DataPacket))
}

// -------------------------------------------------------

type AddTrackParams struct {
	Stereo bool
	Red    bool
}

type MoveToRoomParams struct {
	RoomName      voicekit.RoomName
	ParticipantID voicekit.ParticipantID
	Helper        LocalParticipantHelper
}

//counterfeiter:generate . LocalParticipantHelper
type LocalParticipantHelper interface {
	ResolveMediaTrack(LocalParticipant, voicekit.TrackID) MediaResolverResult
	GetParticipantInfo(pID voicekit.ParticipantID) *voicekit.ParticipantInfo
	GetRegionSettings(ip string) *voicekit.RegionSettings
	GetSubscriberForwarderState(p LocalParticipant) (map[voicekit.TrackID]*voicekit.RTPForwarderState, error)
	ShouldRegressCodec() bool
}

//counterfeiter:generate . LocalParticipant
type LocalParticipant interface {
	Participant

	ToProtoWithVersion() (*voicekit.ParticipantInfo, utils.TimedVersion)

	// getters
	GetTrailer() []byte
	GetLogger() logger.Logger
	GetLoggerResolver() logger.DeferredFieldResolver
	GetReporter() roomobs.ParticipantSessionReporter
	GetReporterResolver() roomobs.ParticipantReporterResolver
	GetAdaptiveStream() bool
	ProtocolVersion() ProtocolVersion
	SupportsSyncStreamID() bool
	SupportsTransceiverReuse() bool
	IsClosed() bool
	IsReady() bool
	IsDisconnected() bool
	Disconnected() <-chan struct{}
	IsIdle() bool
	SubscriberAsPrimary() bool
	GetClientInfo() *voicekit.ClientInfo
	GetClientConfiguration() *voicekit.ClientConfiguration
	GetBufferFactory() *buffer.Factory
	GetPlayoutDelayConfig() *voicekit.PlayoutDelay
	GetPendingTrack(trackID voicekit.TrackID) *voicekit.TrackInfo
	GetICEConnectionInfo() []*ICEConnectionInfo
	HasConnected() bool
	GetEnabledPublishCodecs() []*voicekit.Codec
	GetPublisherICESessionUfrag() (string, error)
	SupportsMoving() bool

	SetResponseSink(sink routing.MessageSink)
	CloseSignalConnection(reason SignallingCloseReason)
	UpdateLastSeenSignal()
	SetSignalSourceValid(valid bool)
	HandleSignalSourceClose()

	// updates
	CheckMetadataLimits(name string, metadata string, attributes map[string]string) error
	SetName(name string)
	SetMetadata(metadata string)
	SetAttributes(attributes map[string]string)
	UpdateAudioTrack(update *voicekit.UpdateLocalAudioTrack) error
	UpdateVideoTrack(update *voicekit.UpdateLocalVideoTrack) error

	// permissions
	ClaimGrants() *auth.ClaimGrants
	SetPermission(permission *voicekit.ParticipantPermission) bool
	CanPublish() bool
	CanPublishSource(source voicekit.TrackSource) bool
	CanSubscribe() bool
	CanPublishData() bool

	// PeerConnection
	AddICECandidate(candidate webrtc.ICECandidateInit, target voicekit.SignalTarget)
	HandleOffer(sdp webrtc.SessionDescription) error
	GetAnswer() (webrtc.SessionDescription, error)
	HandleICETrickleSDPFragment(sdpFragment string) error
	HandleICERestartSDPFragment(sdpFragment string) (string, error)
	AddTrack(req *voicekit.AddTrackRequest)
	SetTrackMuted(trackID voicekit.TrackID, muted bool, fromAdmin bool) *voicekit.TrackInfo

	HandleAnswer(sdp webrtc.SessionDescription)
	Negotiate(force bool)
	ICERestart(iceConfig *voicekit.ICEConfig)
	AddTrackLocal(trackLocal webrtc.TrackLocal, params AddTrackParams) (*webrtc.RTPSender, *webrtc.RTPTransceiver, error)
	AddTransceiverFromTrackLocal(trackLocal webrtc.TrackLocal, params AddTrackParams) (*webrtc.RTPSender, *webrtc.RTPTransceiver, error)
	RemoveTrackLocal(sender *webrtc.RTPSender) error

	WriteSubscriberRTCP(pkts []rtcp.Packet) error

	// subscriptions
	SubscribeToTrack(trackID voicekit.TrackID)
	UnsubscribeFromTrack(trackID voicekit.TrackID)
	UpdateSubscribedTrackSettings(trackID voicekit.TrackID, settings *voicekit.UpdateTrackSettings)
	GetSubscribedTracks() []SubscribedTrack
	IsTrackNameSubscribed(publisherIdentity voicekit.ParticipantIdentity, trackName string) bool
	Verify() bool
	VerifySubscribeParticipantInfo(pID voicekit.ParticipantID, version uint32)
	// WaitUntilSubscribed waits until all subscriptions have been settled, or if the timeout
	// has been reached. If the timeout expires, it will return an error.
	WaitUntilSubscribed(timeout time.Duration) error
	StopAndGetSubscribedTracksForwarderState() map[voicekit.TrackID]*voicekit.RTPForwarderState
	SupportsCodecChange() bool

	// returns list of participant identities that the current participant is subscribed to
	GetSubscribedParticipants() []voicekit.ParticipantID
	IsSubscribedTo(sid voicekit.ParticipantID) bool

	GetConnectionQuality() *voicekit.ConnectionQualityInfo

	// server sent messages
	SendJoinResponse(joinResponse *voicekit.JoinResponse) error
	SendParticipantUpdate(participants []*voicekit.ParticipantInfo) error
	SendSpeakerUpdate(speakers []*voicekit.SpeakerInfo, force bool) error
	SendDataMessage(kind voicekit.DataPacket_Kind, data []byte) error
	SendDataMessageUnlabeled(data []byte, useRaw bool, sender voicekit.ParticipantIdentity) error
	SendRoomUpdate(room *voicekit.Room) error
	SendConnectionQualityUpdate(update *voicekit.ConnectionQualityUpdate) error
	SubscriptionPermissionUpdate(publisherID voicekit.ParticipantID, trackID voicekit.TrackID, allowed bool)
	SendRefreshToken(token string) error
	SendRequestResponse(requestResponse *voicekit.RequestResponse) error
	HandleReconnectAndSendResponse(reconnectReason voicekit.ReconnectReason, reconnectResponse *voicekit.ReconnectResponse) error
	IssueFullReconnect(reason ParticipantCloseReason)
	SendRoomMovedResponse(moved *voicekit.RoomMovedResponse) error

	// callbacks
	OnStateChange(func(p LocalParticipant))
	OnSubscriberReady(callback func(LocalParticipant))
	OnMigrateStateChange(func(p LocalParticipant, migrateState MigrateState))
	// OnTrackPublished - remote added a track
	OnTrackPublished(func(LocalParticipant, MediaTrack))
	// OnTrackUpdated - one of its publishedTracks changed in status
	OnTrackUpdated(callback func(LocalParticipant, MediaTrack))
	// OnTrackUnpublished - a track was unpublished
	OnTrackUnpublished(callback func(LocalParticipant, MediaTrack))
	// OnParticipantUpdate - metadata or permission is updated
	OnParticipantUpdate(callback func(LocalParticipant))
	OnDataPacket(callback func(LocalParticipant, voicekit.DataPacket_Kind, *voicekit.DataPacket))
	OnDataMessage(callback func(LocalParticipant, []byte))
	OnSubscribeStatusChanged(fn func(publisherID voicekit.ParticipantID, subscribed bool))
	OnClose(callback func(LocalParticipant))
	OnClaimsChanged(callback func(LocalParticipant))

	HandleReceiverReport(dt *sfu.DownTrack, report *rtcp.ReceiverReport)

	// session migration
	MaybeStartMigration(force bool, onStart func()) bool
	NotifyMigration()
	SetMigrateState(s MigrateState)
	MigrateState() MigrateState
	SetMigrateInfo(
		previousOffer, previousAnswer *webrtc.SessionDescription,
		mediaTracks []*voicekit.TrackPublishedResponse,
		dataChannels []*voicekit.DataChannelInfo,
	)
	IsReconnect() bool
	MoveToRoom(params MoveToRoomParams)

	UpdateMediaRTT(rtt uint32)
	UpdateSignalingRTT(rtt uint32)

	CacheDownTrack(trackID voicekit.TrackID, rtpTransceiver *webrtc.RTPTransceiver, downTrackState sfu.DownTrackState)
	UncacheDownTrack(rtpTransceiver *webrtc.RTPTransceiver)
	GetCachedDownTrack(trackID voicekit.TrackID) (*webrtc.RTPTransceiver, sfu.DownTrackState)

	SetICEConfig(iceConfig *voicekit.ICEConfig)
	GetICEConfig() *voicekit.ICEConfig
	OnICEConfigChanged(callback func(participant LocalParticipant, iceConfig *voicekit.ICEConfig))

	UpdateSubscribedQuality(nodeID voicekit.NodeID, trackID voicekit.TrackID, maxQualities []SubscribedCodecQuality) error
	UpdateMediaLoss(nodeID voicekit.NodeID, trackID voicekit.TrackID, fractionalLoss uint32) error

	// down stream bandwidth management
	SetSubscriberAllowPause(allowPause bool)
	SetSubscriberChannelCapacity(channelCapacity int64)

	GetPacer() pacer.Pacer

	GetDisableSenderReportPassThrough() bool

	HandleMetrics(senderParticipantID voicekit.ParticipantID, batch *voicekit.MetricsBatch) error
}

// Room is a container of participants, and can provide room-level actions
//
//counterfeiter:generate . Room
type Room interface {
	Name() voicekit.RoomName
	ID() voicekit.RoomID
	RemoveParticipant(identity voicekit.ParticipantIdentity, pID voicekit.ParticipantID, reason ParticipantCloseReason)
	UpdateSubscriptions(participant LocalParticipant, trackIDs []voicekit.TrackID, participantTracks []*voicekit.ParticipantTracks, subscribe bool)
	UpdateSubscriptionPermission(participant LocalParticipant, permissions *voicekit.SubscriptionPermission) error
	SyncState(participant LocalParticipant, state *voicekit.SyncState) error
	SimulateScenario(participant LocalParticipant, scenario *voicekit.SimulateScenario) error
	ResolveMediaTrackForSubscriber(sub LocalParticipant, trackID voicekit.TrackID) MediaResolverResult
	GetLocalParticipants() []LocalParticipant
	IsDataMessageUserPacketDuplicate(ip *voicekit.UserPacket) bool
}

// MediaTrack represents a media track
//
//counterfeiter:generate . MediaTrack
type MediaTrack interface {
	ID() voicekit.TrackID
	Kind() voicekit.TrackType
	Name() string
	Source() voicekit.TrackSource
	Stream() string

	UpdateTrackInfo(ti *voicekit.TrackInfo)
	UpdateAudioTrack(update *voicekit.UpdateLocalAudioTrack)
	UpdateVideoTrack(update *voicekit.UpdateLocalVideoTrack)
	ToProto() *voicekit.TrackInfo

	PublisherID() voicekit.ParticipantID
	PublisherIdentity() voicekit.ParticipantIdentity
	PublisherVersion() uint32
	Logger() logger.Logger

	IsMuted() bool
	SetMuted(muted bool)

	IsSimulcast() bool

	GetAudioLevel() (level float64, active bool)

	Close(isExpectedToResume bool)
	IsOpen() bool

	// callbacks
	AddOnClose(func(isExpectedToResume bool))

	// subscribers
	AddSubscriber(participant LocalParticipant) (SubscribedTrack, error)
	RemoveSubscriber(participantID voicekit.ParticipantID, isExpectedToResume bool)
	IsSubscriber(subID voicekit.ParticipantID) bool
	RevokeDisallowedSubscribers(allowedSubscriberIdentities []voicekit.ParticipantIdentity) []voicekit.ParticipantIdentity
	GetAllSubscribers() []voicekit.ParticipantID
	GetNumSubscribers() int
	OnTrackSubscribed()

	// returns quality information that's appropriate for width & height
	GetQualityForDimension(width, height uint32) voicekit.VideoQuality

	// returns temporal layer that's appropriate for fps
	GetTemporalLayerForSpatialFps(spatial int32, fps uint32, mime mime.MimeType) int32

	Receivers() []sfu.TrackReceiver
	ClearAllReceivers(isExpectedToResume bool)

	IsEncrypted() bool
}

//counterfeiter:generate . LocalMediaTrack
type LocalMediaTrack interface {
	MediaTrack

	Restart()

	SignalCid() string
	HasSdpCid(cid string) bool

	GetConnectionScoreAndQuality() (float32, voicekit.ConnectionQuality)
	GetTrackStats() *voicekit.RTPStats

	SetRTT(rtt uint32)

	NotifySubscriberNodeMaxQuality(nodeID voicekit.NodeID, qualities []SubscribedCodecQuality)
	ClearSubscriberNodesMaxQuality()
	NotifySubscriberNodeMediaLoss(nodeID voicekit.NodeID, fractionalLoss uint8)
}

//counterfeiter:generate . SubscribedTrack
type SubscribedTrack interface {
	AddOnBind(f func(error))
	IsBound() bool
	Close(isExpectedToResume bool)
	OnClose(f func(isExpectedToResume bool))
	ID() voicekit.TrackID
	PublisherID() voicekit.ParticipantID
	PublisherIdentity() voicekit.ParticipantIdentity
	PublisherVersion() uint32
	SubscriberID() voicekit.ParticipantID
	SubscriberIdentity() voicekit.ParticipantIdentity
	Subscriber() LocalParticipant
	DownTrack() *sfu.DownTrack
	MediaTrack() MediaTrack
	RTPSender() *webrtc.RTPSender
	IsMuted() bool
	SetPublisherMuted(muted bool)
	UpdateSubscriberSettings(settings *voicekit.UpdateTrackSettings, isImmediate bool)
	// selects appropriate video layer according to subscriber preferences
	UpdateVideoLayer()
	NeedsNegotiation() bool
}

type ChangeNotifier interface {
	AddObserver(key string, onChanged func())
	RemoveObserver(key string)
	HasObservers() bool
	NotifyChanged()
}

type MediaResolverResult struct {
	TrackChangedNotifier ChangeNotifier
	TrackRemovedNotifier ChangeNotifier
	Track                MediaTrack
	// is permission given to the requesting participant
	HasPermission     bool
	PublisherID       voicekit.ParticipantID
	PublisherIdentity voicekit.ParticipantIdentity
}

// MediaTrackResolver locates a specific media track for a subscriber
type MediaTrackResolver func(LocalParticipant, voicekit.TrackID) MediaResolverResult

// Supervisor/operation monitor related definitions
type OperationMonitorEvent int

const (
	OperationMonitorEventPublisherPeerConnectionConnected OperationMonitorEvent = iota
	OperationMonitorEventAddPendingPublication
	OperationMonitorEventSetPublicationMute
	OperationMonitorEventSetPublishedTrack
	OperationMonitorEventClearPublishedTrack
)

func (o OperationMonitorEvent) String() string {
	switch o {
	case OperationMonitorEventPublisherPeerConnectionConnected:
		return "PUBLISHER_PEER_CONNECTION_CONNECTED"
	case OperationMonitorEventAddPendingPublication:
		return "ADD_PENDING_PUBLICATION"
	case OperationMonitorEventSetPublicationMute:
		return "SET_PUBLICATION_MUTE"
	case OperationMonitorEventSetPublishedTrack:
		return "SET_PUBLISHED_TRACK"
	case OperationMonitorEventClearPublishedTrack:
		return "CLEAR_PUBLISHED_TRACK"
	default:
		return fmt.Sprintf("%d", int(o))
	}
}

type OperationMonitorData interface{}

type OperationMonitor interface {
	PostEvent(ome OperationMonitorEvent, omd OperationMonitorData)
	Check() error
	IsIdle() bool
}
