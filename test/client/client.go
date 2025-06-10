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

package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/thoas/go-funk"
	"go.uber.org/atomic"
	"google.golang.org/protobuf/proto"

	"github.com/voicekit/mediatransportutil/pkg/rtcconfig"
	"github.com/voicekit/protocol/auth"
	"github.com/voicekit/protocol/voicekit"
	"github.com/voicekit/protocol/logger"

	"github.com/voicekit/voicekit-server/pkg/rtc"
	"github.com/voicekit/voicekit-server/pkg/rtc/transport/transportfakes"
	"github.com/voicekit/voicekit-server/pkg/rtc/types"
	"github.com/voicekit/voicekit-server/pkg/sfu/buffer"
	"github.com/voicekit/voicekit-server/pkg/sfu/mime"
)

type SignalRequestHandler func(msg *voicekit.SignalRequest) error
type SignalRequestInterceptor func(msg *voicekit.SignalRequest, next SignalRequestHandler) error
type SignalResponseHandler func(msg *voicekit.SignalResponse) error
type SignalResponseInterceptor func(msg *voicekit.SignalResponse, next SignalResponseHandler) error

type RTCClient struct {
	id         voicekit.ParticipantID
	conn       *websocket.Conn
	publisher  *rtc.PCTransport
	subscriber *rtc.PCTransport
	// sid => track
	localTracks        map[string]webrtc.TrackLocal
	trackSenders       map[string]*webrtc.RTPSender
	lock               sync.Mutex
	wsLock             sync.Mutex
	ctx                context.Context
	cancel             context.CancelFunc
	me                 *webrtc.MediaEngine // optional, populated only when receiving tracks
	subscribedTracks   map[voicekit.ParticipantID][]*webrtc.TrackRemote
	localParticipant   *voicekit.ParticipantInfo
	remoteParticipants map[voicekit.ParticipantID]*voicekit.ParticipantInfo

	signalRequestInterceptor  SignalRequestInterceptor
	signalResponseInterceptor SignalResponseInterceptor

	icQueue [2]atomic.Pointer[webrtc.ICECandidate]

	subscriberAsPrimary        atomic.Bool
	publisherFullyEstablished  atomic.Bool
	subscriberFullyEstablished atomic.Bool
	pongReceivedAt             atomic.Int64
	lastAnswer                 atomic.Pointer[webrtc.SessionDescription]

	// tracks waiting to be acked, cid => trackInfo
	pendingPublishedTracks map[string]*voicekit.TrackInfo

	pendingTrackWriters     []*TrackWriter
	OnConnected             func()
	OnDataReceived          func(data []byte, sid string)
	OnDataUnlabeledReceived func(data []byte)
	refreshToken            string

	// map of voicekit.ParticipantID and last packet
	lastPackets   map[voicekit.ParticipantID]*rtp.Packet
	bytesReceived map[voicekit.ParticipantID]uint64

	subscriptionResponse atomic.Pointer[voicekit.SubscriptionResponse]
}

var (
	// minimal settings only with stun server
	rtcConf = webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}
	extMimeMapping = map[string]string{
		".ivf":  mime.MimeTypeVP8.String(),
		".h264": mime.MimeTypeH264.String(),
		".ogg":  mime.MimeTypeOpus.String(),
	}
)

type Options struct {
	AutoSubscribe             bool
	Publish                   string
	Attributes                map[string]string
	ClientInfo                *voicekit.ClientInfo
	DisabledCodecs            []webrtc.RTPCodecCapability
	TokenCustomizer           func(token *auth.AccessToken, grants *auth.VideoGrant)
	SignalRequestInterceptor  SignalRequestInterceptor
	SignalResponseInterceptor SignalResponseInterceptor
}

func NewWebSocketConn(host, token string, opts *Options) (*websocket.Conn, error) {
	u, err := url.Parse(host + fmt.Sprintf("/rtc?protocol=%d", types.CurrentProtocol))
	if err != nil {
		return nil, err
	}
	requestHeader := make(http.Header)
	SetAuthorizationToken(requestHeader, token)

	connectUrl := u.String()
	sdk := "go"
	if opts != nil {
		connectUrl = fmt.Sprintf("%s&auto_subscribe=%t", connectUrl, opts.AutoSubscribe)
		if opts.Publish != "" {
			connectUrl += encodeQueryParam("publish", opts.Publish)
		}
		if len(opts.Attributes) != 0 {
			data, err := json.Marshal(opts.Attributes)
			if err != nil {
				return nil, err
			}
			connectUrl += encodeQueryParam("attributes", base64.URLEncoding.EncodeToString(data))
		}
		if opts.ClientInfo != nil {
			if opts.ClientInfo.DeviceModel != "" {
				connectUrl += encodeQueryParam("device_model", opts.ClientInfo.DeviceModel)
			}
			if opts.ClientInfo.Os != "" {
				connectUrl += encodeQueryParam("os", opts.ClientInfo.Os)
			}
			if opts.ClientInfo.Sdk != voicekit.ClientInfo_UNKNOWN {
				sdk = opts.ClientInfo.Sdk.String()
			}
		}
	}
	connectUrl += encodeQueryParam("sdk", sdk)
	conn, _, err := websocket.DefaultDialer.Dial(connectUrl, requestHeader)
	return conn, err
}

func SetAuthorizationToken(header http.Header, token string) {
	header.Set("Authorization", "Bearer "+token)
}

func NewRTCClient(conn *websocket.Conn, opts *Options) (*RTCClient, error) {
	var err error

	c := &RTCClient{
		conn:                   conn,
		localTracks:            make(map[string]webrtc.TrackLocal),
		trackSenders:           make(map[string]*webrtc.RTPSender),
		pendingPublishedTracks: make(map[string]*voicekit.TrackInfo),
		subscribedTracks:       make(map[voicekit.ParticipantID][]*webrtc.TrackRemote),
		remoteParticipants:     make(map[voicekit.ParticipantID]*voicekit.ParticipantInfo),
		me:                     &webrtc.MediaEngine{},
		lastPackets:            make(map[voicekit.ParticipantID]*rtp.Packet),
		bytesReceived:          make(map[voicekit.ParticipantID]uint64),
	}
	c.ctx, c.cancel = context.WithCancel(context.Background())

	conf := rtc.WebRTCConfig{
		WebRTCConfig: rtcconfig.WebRTCConfig{
			Configuration: rtcConf,
		},
	}
	conf.SettingEngine.SetLite(false)
	conf.SettingEngine.SetAnsweringDTLSRole(webrtc.DTLSRoleClient)
	ff := buffer.NewFactoryOfBufferFactory(500, 200)
	conf.SetBufferFactory(ff.CreateBufferFactory())
	var codecs []*voicekit.Codec
	for _, codec := range []*voicekit.Codec{
		{
			Mime: "audio/opus",
		},
		{
			Mime: "video/vp8",
		},
		{
			Mime: "video/h264",
		},
	} {
		var disabled bool
		if opts != nil {
			for _, dc := range opts.DisabledCodecs {
				if mime.IsMimeTypeStringEqual(dc.MimeType, codec.Mime) && (dc.SDPFmtpLine == "" || dc.SDPFmtpLine == codec.FmtpLine) {
					disabled = true
					break
				}
			}
		}
		if !disabled {
			codecs = append(codecs, codec)
		}
	}

	//
	// The signal targets are from point of view of server.
	// From client side, they are flipped,
	// i. e. the publisher transport on client side has SUBSCRIBER signal target (i. e. publisher is offerer).
	// Same applies for subscriber transport also
	//
	publisherHandler := &transportfakes.FakeHandler{}
	c.publisher, err = rtc.NewPCTransport(rtc.TransportParams{
		Config:                   &conf,
		DirectionConfig:          conf.Subscriber,
		EnabledCodecs:            codecs,
		IsOfferer:                true,
		IsSendSide:               true,
		Handler:                  publisherHandler,
		DatachannelSlowThreshold: 1024 * 1024 * 1024,
	})
	if err != nil {
		return nil, err
	}
	subscriberHandler := &transportfakes.FakeHandler{}
	c.subscriber, err = rtc.NewPCTransport(rtc.TransportParams{
		Config:                           &conf,
		DirectionConfig:                  conf.Publisher,
		EnabledCodecs:                    codecs,
		Handler:                          subscriberHandler,
		DatachannelMaxReceiverBufferSize: 1500,
		FireOnTrackBySdp:                 true,
	})
	if err != nil {
		return nil, err
	}

	publisherHandler.OnICECandidateCalls(func(ic *webrtc.ICECandidate, t voicekit.SignalTarget) error {
		return c.SendIceCandidate(ic, voicekit.SignalTarget_PUBLISHER)
	})
	publisherHandler.OnOfferCalls(c.onOffer)
	publisherHandler.OnFullyEstablishedCalls(func() {
		logger.Debugw("publisher fully established", "participant", c.localParticipant.Identity, "pID", c.localParticipant.Sid)
		c.publisherFullyEstablished.Store(true)
	})

	ordered := true
	if err := c.publisher.CreateDataChannel(rtc.ReliableDataChannel, &webrtc.DataChannelInit{
		Ordered: &ordered,
	}); err != nil {
		return nil, err
	}

	ordered = false
	maxRetransmits := uint16(0)
	if err := c.publisher.CreateDataChannel(rtc.LossyDataChannel, &webrtc.DataChannelInit{
		Ordered:        &ordered,
		MaxRetransmits: &maxRetransmits,
	}); err != nil {
		return nil, err
	}

	if err := c.publisher.CreateDataChannel("pubraw", &webrtc.DataChannelInit{
		Ordered: &ordered,
	}); err != nil {
		return nil, err
	}

	if err := c.subscriber.CreateReadableDataChannel("subraw", &webrtc.DataChannelInit{
		Ordered: &ordered,
	}); err != nil {
		return nil, err
	}

	subscriberHandler.OnICECandidateCalls(func(ic *webrtc.ICECandidate, t voicekit.SignalTarget) error {
		if ic == nil {
			return nil
		}
		return c.SendIceCandidate(ic, voicekit.SignalTarget_SUBSCRIBER)
	})
	subscriberHandler.OnTrackCalls(func(track *webrtc.TrackRemote, rtpReceiver *webrtc.RTPReceiver) {
		go c.processTrack(track)
	})
	subscriberHandler.OnDataMessageCalls(c.handleDataMessage)
	subscriberHandler.OnDataMessageUnlabeledCalls(c.handleDataMessageUnlabeled)
	subscriberHandler.OnInitialConnectedCalls(func() {
		logger.Debugw("subscriber initial connected", "participant", c.localParticipant.Identity)

		c.lock.Lock()
		defer c.lock.Unlock()
		for _, tw := range c.pendingTrackWriters {
			if err := tw.Start(); err != nil {
				logger.Errorw("track writer error", err)
			}
		}

		c.pendingTrackWriters = nil

		if c.OnConnected != nil {
			go c.OnConnected()
		}
	})
	subscriberHandler.OnFullyEstablishedCalls(func() {
		logger.Debugw("subscriber fully established", "participant", c.localParticipant.Identity, "pID", c.localParticipant.Sid)
		c.subscriberFullyEstablished.Store(true)
	})
	subscriberHandler.OnAnswerCalls(func(answer webrtc.SessionDescription) error {
		// send remote an answer
		logger.Infow("sending subscriber answer",
			"participant", c.localParticipant.Identity,
			// "sdp", answer,
		)
		return c.SendRequest(&voicekit.SignalRequest{
			Message: &voicekit.SignalRequest_Answer{
				Answer: rtc.ToProtoSessionDescription(answer),
			},
		})
	})

	if opts != nil {
		c.signalRequestInterceptor = opts.SignalRequestInterceptor
		c.signalResponseInterceptor = opts.SignalResponseInterceptor
	}

	return c, nil
}

func (c *RTCClient) ID() voicekit.ParticipantID {
	return c.id
}

// create an offer for the server
func (c *RTCClient) Run() error {
	c.conn.SetCloseHandler(func(code int, text string) error {
		// when closed, stop connection
		logger.Infow("connection closed", "code", code, "text", text)
		c.Stop()
		return nil
	})

	// run the session
	for {
		res, err := c.ReadResponse()
		if errors.Is(io.EOF, err) {
			return nil
		} else if err != nil {
			logger.Errorw("error while reading", err)
			return err
		}
		if c.signalResponseInterceptor != nil {
			err = c.signalResponseInterceptor(res, c.handleSignalResponse)
		} else {
			err = c.handleSignalResponse(res)
		}
		if err != nil {
			return err
		}
	}
}

func (c *RTCClient) handleSignalResponse(res *voicekit.SignalResponse) error {
	switch msg := res.Message.(type) {
	case *voicekit.SignalResponse_Join:
		c.localParticipant = msg.Join.Participant
		c.id = voicekit.ParticipantID(msg.Join.Participant.Sid)
		c.lock.Lock()
		for _, p := range msg.Join.OtherParticipants {
			c.remoteParticipants[voicekit.ParticipantID(p.Sid)] = p
		}
		c.lock.Unlock()
		// if publish only, negotiate
		if !msg.Join.SubscriberPrimary {
			c.subscriberAsPrimary.Store(false)
			c.publisher.Negotiate(false)
		} else {
			c.subscriberAsPrimary.Store(true)
		}

		logger.Infow("join accepted, awaiting offer", "participant", msg.Join.Participant.Identity)
	case *voicekit.SignalResponse_Answer:
		// logger.Debugw("received server answer",
		//	"participant", c.localParticipant.Identity,
		//	"answer", msg.Answer.Sdp)
		c.handleAnswer(rtc.FromProtoSessionDescription(msg.Answer))
	case *voicekit.SignalResponse_Offer:
		logger.Infow("received server offer",
			"participant", c.localParticipant.Identity,
		)
		desc := rtc.FromProtoSessionDescription(msg.Offer)
		c.handleOffer(desc)
	case *voicekit.SignalResponse_Trickle:
		candidateInit, err := rtc.FromProtoTrickle(msg.Trickle)
		if err != nil {
			return err
		}
		if msg.Trickle.Target == voicekit.SignalTarget_PUBLISHER {
			c.publisher.AddICECandidate(candidateInit)
		} else {
			c.subscriber.AddICECandidate(candidateInit)
		}
	case *voicekit.SignalResponse_Update:
		c.lock.Lock()
		for _, p := range msg.Update.Participants {
			if voicekit.ParticipantID(p.Sid) != c.id {
				if p.State != voicekit.ParticipantInfo_DISCONNECTED {
					c.remoteParticipants[voicekit.ParticipantID(p.Sid)] = p
				} else {
					delete(c.remoteParticipants, voicekit.ParticipantID(p.Sid))
				}
			}
		}
		c.lock.Unlock()

	case *voicekit.SignalResponse_TrackPublished:
		logger.Debugw("track published", "trackID", msg.TrackPublished.Track.Name, "participant", c.localParticipant.Sid,
			"cid", msg.TrackPublished.Cid, "trackSid", msg.TrackPublished.Track.Sid)
		c.lock.Lock()
		c.pendingPublishedTracks[msg.TrackPublished.Cid] = msg.TrackPublished.Track
		c.lock.Unlock()
	case *voicekit.SignalResponse_RefreshToken:
		c.lock.Lock()
		c.refreshToken = msg.RefreshToken
		c.lock.Unlock()
	case *voicekit.SignalResponse_TrackUnpublished:
		sid := msg.TrackUnpublished.TrackSid
		c.lock.Lock()
		sender := c.trackSenders[sid]
		if sender != nil {
			if err := c.publisher.RemoveTrack(sender); err != nil {
				logger.Errorw("Could not unpublish track", err)
			}
			c.publisher.Negotiate(false)
		}
		delete(c.trackSenders, sid)
		delete(c.localTracks, sid)
		c.lock.Unlock()
	case *voicekit.SignalResponse_Pong:
		c.pongReceivedAt.Store(msg.Pong)
	case *voicekit.SignalResponse_SubscriptionResponse:
		c.subscriptionResponse.Store(msg.SubscriptionResponse)
	}
	return nil
}

func (c *RTCClient) WaitUntilConnected() error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			id := string(c.ID())
			if c.localParticipant != nil {
				id = c.localParticipant.Identity
			}
			return fmt.Errorf("%s could not connect after timeout", id)
		case <-time.After(10 * time.Millisecond):
			if c.subscriberAsPrimary.Load() {
				if c.subscriberFullyEstablished.Load() {
					return nil
				}
			} else {
				if c.publisherFullyEstablished.Load() {
					return nil
				}
			}
		}
	}
}

func (c *RTCClient) ReadResponse() (*voicekit.SignalResponse, error) {
	for {
		// handle special messages and pass on the rest
		messageType, payload, err := c.conn.ReadMessage()
		if err != nil {
			return nil, err
		}

		if c.ctx.Err() != nil {
			return nil, c.ctx.Err()
		}

		msg := &voicekit.SignalResponse{}
		switch messageType {
		case websocket.PingMessage:
			_ = c.conn.WriteMessage(websocket.PongMessage, nil)
			continue
		case websocket.BinaryMessage:
			// protobuf encoded
			err := proto.Unmarshal(payload, msg)
			return msg, err
		default:
			return nil, fmt.Errorf("unexpected message received: %v", messageType)
		}
	}
}

func (c *RTCClient) SubscribedTracks() map[voicekit.ParticipantID][]*webrtc.TrackRemote {
	// create a copy of this
	c.lock.Lock()
	defer c.lock.Unlock()
	tracks := make(map[voicekit.ParticipantID][]*webrtc.TrackRemote, len(c.subscribedTracks))
	for key, val := range c.subscribedTracks {
		tracks[key] = val
	}
	return tracks
}

func (c *RTCClient) RemoteParticipants() []*voicekit.ParticipantInfo {
	c.lock.Lock()
	defer c.lock.Unlock()
	return funk.Values(c.remoteParticipants).([]*voicekit.ParticipantInfo)
}

func (c *RTCClient) GetRemoteParticipant(sid voicekit.ParticipantID) *voicekit.ParticipantInfo {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.remoteParticipants[sid]
}

func (c *RTCClient) Stop() {
	logger.Infow("stopping client", "ID", c.ID())
	_ = c.SendRequest(&voicekit.SignalRequest{
		Message: &voicekit.SignalRequest_Leave{
			Leave: &voicekit.LeaveRequest{
				Reason: voicekit.DisconnectReason_CLIENT_INITIATED,
				Action: voicekit.LeaveRequest_DISCONNECT,
			},
		},
	})
	c.publisherFullyEstablished.Store(false)
	c.subscriberFullyEstablished.Store(false)
	_ = c.conn.Close()
	c.publisher.Close()
	c.subscriber.Close()
	c.cancel()
}

func (c *RTCClient) RefreshToken() string {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.refreshToken
}

func (c *RTCClient) PongReceivedAt() int64 {
	return c.pongReceivedAt.Load()
}

func (c *RTCClient) GetSubscriptionResponseAndClear() *voicekit.SubscriptionResponse {
	return c.subscriptionResponse.Swap(nil)
}

func (c *RTCClient) SendPing() error {
	return c.SendRequest(&voicekit.SignalRequest{
		Message: &voicekit.SignalRequest_Ping{
			Ping: time.Now().UnixNano(),
		},
	})
}

func (c *RTCClient) SendRequest(msg *voicekit.SignalRequest) error {
	if c.signalRequestInterceptor != nil {
		return c.signalRequestInterceptor(msg, c.sendRequest)
	} else {
		return c.sendRequest(msg)
	}
}

func (c *RTCClient) sendRequest(msg *voicekit.SignalRequest) error {
	payload, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	c.wsLock.Lock()
	defer c.wsLock.Unlock()
	return c.conn.WriteMessage(websocket.BinaryMessage, payload)
}

func (c *RTCClient) SendIceCandidate(ic *webrtc.ICECandidate, target voicekit.SignalTarget) error {
	prevIC := c.icQueue[target].Swap(ic)
	if prevIC == nil {
		return nil
	}

	return c.SendRequest(&voicekit.SignalRequest{
		Message: &voicekit.SignalRequest_Trickle{
			Trickle: rtc.ToProtoTrickle(prevIC.ToJSON(), target, ic == nil),
		},
	})
}

func (c *RTCClient) SetAttributes(attrs map[string]string) error {
	return c.SendRequest(&voicekit.SignalRequest{
		Message: &voicekit.SignalRequest_UpdateMetadata{
			UpdateMetadata: &voicekit.UpdateParticipantMetadata{
				Attributes: attrs,
			},
		},
	})
}

func (c *RTCClient) hasPrimaryEverConnected() bool {
	if c.subscriberAsPrimary.Load() {
		return c.subscriber.HasEverConnected()
	} else {
		return c.publisher.HasEverConnected()
	}
}

type AddTrackParams struct {
	NoWriter bool
}

type AddTrackOption func(params *AddTrackParams)

func AddTrackNoWriter() AddTrackOption {
	return func(params *AddTrackParams) {
		params.NoWriter = true
	}
}

func (c *RTCClient) AddTrack(track *webrtc.TrackLocalStaticSample, path string, opts ...AddTrackOption) (writer *TrackWriter, err error) {
	var params AddTrackParams
	for _, opt := range opts {
		opt(&params)
	}
	trackType := voicekit.TrackType_AUDIO
	if track.Kind() == webrtc.RTPCodecTypeVideo {
		trackType = voicekit.TrackType_VIDEO
	}

	if err = c.SendAddTrack(track.ID(), track.StreamID(), trackType); err != nil {
		return
	}

	// wait till track published message is received
	timeout := time.After(5 * time.Second)
	var ti *voicekit.TrackInfo
	for {
		select {
		case <-timeout:
			return nil, errors.New("could not publish track after timeout")
		default:
			c.lock.Lock()
			ti = c.pendingPublishedTracks[track.ID()]
			c.lock.Unlock()
			if ti != nil {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		if ti != nil {
			break
		}
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	sender, _, err := c.publisher.AddTrack(track, types.AddTrackParams{})
	if err != nil {
		logger.Errorw("add track failed", err, "trackID", ti.Sid, "participant", c.localParticipant.Identity, "pID", c.localParticipant.Sid)
		return
	}
	c.localTracks[ti.Sid] = track
	c.trackSenders[ti.Sid] = sender
	c.publisher.Negotiate(false)

	if !params.NoWriter {
		writer = NewTrackWriter(c.ctx, track, path)

		// write tracks only after connection established
		if c.hasPrimaryEverConnected() {
			err = writer.Start()
		} else {
			c.pendingTrackWriters = append(c.pendingTrackWriters, writer)
		}
	}

	return
}

func (c *RTCClient) AddStaticTrack(mime string, id string, label string, opts ...AddTrackOption) (writer *TrackWriter, err error) {
	return c.AddStaticTrackWithCodec(webrtc.RTPCodecCapability{MimeType: mime}, id, label, opts...)
}

func (c *RTCClient) AddStaticTrackWithCodec(codec webrtc.RTPCodecCapability, id string, label string, opts ...AddTrackOption) (writer *TrackWriter, err error) {
	track, err := webrtc.NewTrackLocalStaticSample(codec, id, label)
	if err != nil {
		return
	}

	return c.AddTrack(track, "", opts...)
}

func (c *RTCClient) AddFileTrack(path string, id string, label string) (writer *TrackWriter, err error) {
	// determine file mime
	mime, ok := extMimeMapping[filepath.Ext(path)]
	if !ok {
		return nil, fmt.Errorf("%s has an unsupported extension", filepath.Base(path))
	}

	logger.Debugw("adding file track",
		"mime", mime,
	)

	track, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: mime},
		id,
		label,
	)
	if err != nil {
		return
	}

	return c.AddTrack(track, path)
}

// send AddTrack command to server to initiate server-side negotiation
func (c *RTCClient) SendAddTrack(cid string, name string, trackType voicekit.TrackType) error {
	return c.SendRequest(&voicekit.SignalRequest{
		Message: &voicekit.SignalRequest_AddTrack{
			AddTrack: &voicekit.AddTrackRequest{
				Cid:  cid,
				Name: name,
				Type: trackType,
			},
		},
	})
}

func (c *RTCClient) PublishData(data []byte, kind voicekit.DataPacket_Kind) error {
	if err := c.ensurePublisherConnected(); err != nil {
		return err
	}

	dpData, err := proto.Marshal(&voicekit.DataPacket{
		Value: &voicekit.DataPacket_User{
			User: &voicekit.UserPacket{Payload: data},
		},
	})
	if err != nil {
		return err
	}

	return c.publisher.SendDataMessage(kind, dpData)
}

func (c *RTCClient) PublishDataUnlabeled(data []byte) error {
	if err := c.ensurePublisherConnected(); err != nil {
		return err
	}

	return c.publisher.SendDataMessageUnlabeled(data, true, "test")
}

func (c *RTCClient) GetPublishedTrackIDs() []string {
	c.lock.Lock()
	defer c.lock.Unlock()
	var trackIDs []string
	for key := range c.localTracks {
		trackIDs = append(trackIDs, key)
	}
	return trackIDs
}

// LastAnswer return SDP of the last answer for the publisher connection
func (c *RTCClient) LastAnswer() *webrtc.SessionDescription {
	return c.lastAnswer.Load()
}

func (c *RTCClient) ensurePublisherConnected() error {
	if c.publisher.HasEverConnected() {
		return nil
	}

	// start negotiating
	c.publisher.Negotiate(false)

	// wait until connected, increase wait time since it takes more than 10s sometimes on GH
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("could not connect publisher after timeout")
		case <-time.After(10 * time.Millisecond):
			if c.publisherFullyEstablished.Load() {
				return nil
			}
		}
	}
}

func (c *RTCClient) handleDataMessage(kind voicekit.DataPacket_Kind, data []byte) {
	dp := &voicekit.DataPacket{}
	err := proto.Unmarshal(data, dp)
	if err != nil {
		return
	}
	dp.Kind = kind
	if val, ok := dp.Value.(*voicekit.DataPacket_User); ok {
		if c.OnDataReceived != nil {
			c.OnDataReceived(val.User.Payload, val.User.ParticipantSid)
		}
	}
}

func (c *RTCClient) handleDataMessageUnlabeled(data []byte) {
	if c.OnDataUnlabeledReceived != nil {
		c.OnDataUnlabeledReceived(data)
	}
}

// handles a server initiated offer, handle on subscriber PC
func (c *RTCClient) handleOffer(desc webrtc.SessionDescription) {
	c.subscriber.HandleRemoteDescription(desc)
}

// the client handles answer on the publisher PC
func (c *RTCClient) handleAnswer(desc webrtc.SessionDescription) {
	logger.Infow("handling server answer", "participant", c.localParticipant.Identity)

	c.lastAnswer.Store(&desc)
	// remote answered the offer, establish connection
	c.publisher.HandleRemoteDescription(desc)
}

func (c *RTCClient) onOffer(offer webrtc.SessionDescription) error {
	if c.localParticipant != nil {
		logger.Infow("starting negotiation", "participant", c.localParticipant.Identity)
	}
	return c.SendRequest(&voicekit.SignalRequest{
		Message: &voicekit.SignalRequest_Offer{
			Offer: rtc.ToProtoSessionDescription(offer),
		},
	})
}

func (c *RTCClient) processTrack(track *webrtc.TrackRemote) {
	lastUpdate := time.Time{}
	pId, trackId := rtc.UnpackStreamID(track.StreamID())
	if trackId == "" {
		trackId = voicekit.TrackID(track.ID())
	}
	c.lock.Lock()
	c.subscribedTracks[pId] = append(c.subscribedTracks[pId], track)
	c.lock.Unlock()

	logger.Infow("client added track", "participant", c.localParticipant.Identity,
		"pID", pId,
		"trackID", trackId,
		"codec", track.Codec(),
	)

	defer func() {
		c.lock.Lock()
		c.subscribedTracks[pId] = funk.Without(c.subscribedTracks[pId], track).([]*webrtc.TrackRemote)
		c.lock.Unlock()
	}()

	numBytes := 0
	for {
		pkt, _, err := track.ReadRTP()
		if c.ctx.Err() != nil {
			break
		}
		if rtc.IsEOF(err) {
			break
		}
		if err != nil {
			logger.Warnw("error reading RTP", err)
			continue
		}
		c.lock.Lock()
		c.lastPackets[pId] = pkt
		c.bytesReceived[pId] += uint64(pkt.MarshalSize())
		c.lock.Unlock()
		numBytes += pkt.MarshalSize()
		if time.Since(lastUpdate) > 30*time.Second {
			logger.Infow("consumed from participant",
				"trackID", trackId, "pID", pId,
				"size", numBytes)
			lastUpdate = time.Now()
		}
	}
}

func (c *RTCClient) BytesReceived() uint64 {
	var total uint64
	c.lock.Lock()
	for _, size := range c.bytesReceived {
		total += size
	}
	c.lock.Unlock()
	return total
}

func (c *RTCClient) SendNacks(count int) {
	var packets []rtcp.Packet
	c.lock.Lock()
	for _, pkt := range c.lastPackets {
		seqs := make([]uint16, 0, count)
		for i := 0; i < count; i++ {
			seqs = append(seqs, pkt.SequenceNumber-uint16(i))
		}
		packets = append(packets, &rtcp.TransportLayerNack{
			MediaSSRC: pkt.SSRC,
			Nacks:     rtcp.NackPairsFromSequenceNumbers(seqs),
		})
	}
	c.lock.Unlock()

	_ = c.subscriber.WriteRTCP(packets)
}

func encodeQueryParam(key, value string) string {
	return fmt.Sprintf("&%s=%s", url.QueryEscape(key), url.QueryEscape(value))
}
