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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ua-parser/uap-go/uaparser"
	"go.uber.org/atomic"
	"golang.org/x/exp/maps"

	"github.com/voicekit/protocol/voicekit"
	"github.com/voicekit/protocol/logger"
	"github.com/voicekit/psrpc"

	"github.com/voicekit/voicekit-server/pkg/config"
	"github.com/voicekit/voicekit-server/pkg/routing"
	"github.com/voicekit/voicekit-server/pkg/routing/selector"
	"github.com/voicekit/voicekit-server/pkg/rtc"
	"github.com/voicekit/voicekit-server/pkg/telemetry"
	"github.com/voicekit/voicekit-server/pkg/telemetry/prometheus"
	"github.com/voicekit/voicekit-server/pkg/utils"
)

type RTCService struct {
	router        routing.MessageRouter
	roomAllocator RoomAllocator
	upgrader      websocket.Upgrader
	config        *config.Config
	isDev         bool
	limits        config.LimitConfig
	parser        *uaparser.Parser
	telemetry     telemetry.TelemetryService

	mu          sync.Mutex
	connections map[*websocket.Conn]struct{}
}

func NewRTCService(
	conf *config.Config,
	ra RoomAllocator,
	router routing.MessageRouter,
	telemetry telemetry.TelemetryService,
) *RTCService {
	s := &RTCService{
		router:        router,
		roomAllocator: ra,
		config:        conf,
		isDev:         conf.Development,
		limits:        conf.Limit,
		parser:        uaparser.NewFromSaved(),
		telemetry:     telemetry,
		connections:   map[*websocket.Conn]struct{}{},
	}

	s.upgrader = websocket.Upgrader{
		EnableCompression: true,

		// allow connections from any origin, since script may be hosted anywhere
		// security is enforced by access tokens
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	return s
}

func (s *RTCService) SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/rtc/validate", s.validate)
}

func (s *RTCService) validate(w http.ResponseWriter, r *http.Request) {
	log := utils.GetLogger(r.Context())
	_, _, code, err := s.validateInternal(log, r, true)
	if err != nil {
		HandleError(w, r, code, err)
		return
	}
	_, _ = w.Write([]byte("success"))
}

func decodeAttributes(str string) (map[string]string, error) {
	data, err := base64.URLEncoding.DecodeString(str)
	if err != nil {
		return nil, err
	}
	var attrs map[string]string
	if err := json.Unmarshal(data, &attrs); err != nil {
		return nil, err
	}
	return attrs, nil
}

func (s *RTCService) validateInternal(log logger.Logger, r *http.Request, strict bool) (voicekit.RoomName, routing.ParticipantInit, int, error) {
	claims := GetGrants(r.Context())
	var pi routing.ParticipantInit

	// require a claim
	if claims == nil || claims.Video == nil {
		return "", pi, http.StatusUnauthorized, rtc.ErrPermissionDenied
	}

	onlyName, err := EnsureJoinPermission(r.Context())
	if err != nil {
		return "", pi, http.StatusUnauthorized, err
	}

	if claims.Identity == "" {
		return "", pi, http.StatusBadRequest, ErrIdentityEmpty
	}
	if !s.config.Limit.CheckParticipantIdentityLength(claims.Identity) {
		return "", pi, http.StatusBadRequest, fmt.Errorf("%w: max length %d", ErrParticipantIdentityExceedsLimits, s.config.Limit.MaxParticipantIdentityLength)
	}

	if claims.RoomConfig != nil {
		if err := claims.RoomConfig.CheckCredentials(); err != nil {
			logger.Warnw("credentials found in token", nil)
			// TODO(dz): in a future version, we'll reject these connections
		}
	}

	roomName := voicekit.RoomName(r.FormValue("room"))
	reconnectParam := r.FormValue("reconnect")
	reconnectReason, _ := strconv.Atoi(r.FormValue("reconnect_reason")) // 0 means unknown reason
	autoSubParam := r.FormValue("auto_subscribe")
	publishParam := r.FormValue("publish")
	adaptiveStreamParam := r.FormValue("adaptive_stream")
	participantID := r.FormValue("sid")
	subscriberAllowPauseParam := r.FormValue("subscriber_allow_pause")
	disableICELite := r.FormValue("disable_ice_lite")
	attributesStr := r.FormValue("attributes")

	if onlyName != "" {
		roomName = onlyName
	}
	if !s.config.Limit.CheckRoomNameLength(string(roomName)) {
		return "", pi, http.StatusBadRequest, fmt.Errorf("%w: max length %d", ErrRoomNameExceedsLimits, s.config.Limit.MaxRoomNameLength)
	}

	// this is new connection for existing participant -  with publish only permissions
	if publishParam != "" {
		// Make sure grant has GetCanPublish set,
		if !claims.Video.GetCanPublish() {
			return "", routing.ParticipantInit{}, http.StatusUnauthorized, rtc.ErrPermissionDenied
		}
		// Make sure by default subscribe is off
		claims.Video.SetCanSubscribe(false)
		claims.Identity += "#" + publishParam
	}

	// room allocator validations
	err = s.roomAllocator.ValidateCreateRoom(r.Context(), roomName)
	if err != nil {
		if errors.Is(err, ErrRoomNotFound) {
			return "", pi, http.StatusNotFound, err
		} else {
			return "", pi, http.StatusInternalServerError, err
		}
	}

	region := ""
	if router, ok := s.router.(routing.Router); ok {
		region = router.GetRegion()
		if foundNode, err := router.GetNodeForRoom(r.Context(), roomName); err == nil {
			if selector.LimitsReached(s.limits, foundNode.Stats) {
				return "", pi, http.StatusServiceUnavailable, rtc.ErrLimitExceeded
			}
		}
	}

	createRequest := &voicekit.CreateRoomRequest{
		Name:       string(roomName),
		RoomPreset: claims.RoomPreset,
	}
	SetRoomConfiguration(createRequest, claims.GetRoomConfiguration())

	// Add extra attributes to the participant
	if attributesStr != "" {
		// Make sure grant has GetCanUpdateOwnMetadata set
		if !claims.Video.GetCanUpdateOwnMetadata() {
			return "", routing.ParticipantInit{}, http.StatusUnauthorized, rtc.ErrPermissionDenied
		}
		attrs, err := decodeAttributes(attributesStr)
		if err != nil {
			if strict {
				return "", pi, http.StatusBadRequest, errors.New("cannot decode attributes")
			}
			log.Debugw("failed to decode attributes", "error", err)
			// attrs will be empty here, so just proceed
		}
		if len(attrs) != 0 && claims.Attributes == nil {
			claims.Attributes = make(map[string]string, len(attrs))
		}
		for k, v := range attrs {
			if v == "" {
				continue // do not allow deleting existing attributes
			}
			claims.Attributes[k] = v
		}
	}

	pi = routing.ParticipantInit{
		Reconnect:       boolValue(reconnectParam),
		ReconnectReason: voicekit.ReconnectReason(reconnectReason),
		Identity:        voicekit.ParticipantIdentity(claims.Identity),
		Name:            voicekit.ParticipantName(claims.Name),
		AutoSubscribe:   true,
		Client:          s.ParseClientInfo(r),
		Grants:          claims,
		Region:          region,
		CreateRoom:      createRequest,
	}
	if pi.Reconnect {
		pi.ID = voicekit.ParticipantID(participantID)
	}

	if autoSubParam != "" {
		pi.AutoSubscribe = boolValue(autoSubParam)
	}
	if adaptiveStreamParam != "" {
		pi.AdaptiveStream = boolValue(adaptiveStreamParam)
	}
	if subscriberAllowPauseParam != "" {
		subscriberAllowPause := boolValue(subscriberAllowPauseParam)
		pi.SubscriberAllowPause = &subscriberAllowPause
	}
	if disableICELite != "" {
		pi.DisableICELite = boolValue(disableICELite)
	}

	return roomName, pi, http.StatusOK, nil
}

func (s *RTCService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// reject non websocket requests
	if !websocket.IsWebSocketUpgrade(r) {
		w.WriteHeader(404)
		return
	}

	var (
		roomName            voicekit.RoomName
		roomID              voicekit.RoomID
		participantIdentity voicekit.ParticipantIdentity
		pID                 voicekit.ParticipantID
		loggerResolved      bool

		pi   routing.ParticipantInit
		code int
		err  error
	)

	pLogger, loggerResolver := utils.GetLogger(r.Context()).WithDeferredValues()

	getLoggerFields := func() []any {
		return []any{
			"room", roomName,
			"roomID", roomID,
			"participant", participantIdentity,
			"pID", pID,
		}
	}

	resolveLogger := func(force bool) {
		if loggerResolved {
			return
		}

		if force || (roomName != "" && roomID != "" && participantIdentity != "" && pID != "") {
			loggerResolved = true
			loggerResolver.Resolve(getLoggerFields()...)
		}
	}

	resetLogger := func() {
		loggerResolver.Reset()

		roomName = ""
		roomID = ""
		participantIdentity = ""
		pID = ""
		loggerResolved = false
	}

	roomName, pi, code, err = s.validateInternal(pLogger, r, false)
	if err != nil {
		HandleError(w, r, code, err)
		return
	}

	participantIdentity = pi.Identity
	if pi.ID != "" {
		pID = pi.ID
	}

	// give it a few attempts to start session
	var cr connectionResult
	var initialResponse *voicekit.SignalResponse
	for attempt := 0; attempt < s.config.SignalRelay.ConnectAttempts; attempt++ {
		connectionTimeout := 3 * time.Second * time.Duration(attempt+1)
		ctx := utils.ContextWithAttempt(r.Context(), attempt)
		cr, initialResponse, err = s.startConnection(ctx, roomName, pi, connectionTimeout)
		if err == nil || errors.Is(err, context.Canceled) {
			break
		}
	}

	if err != nil {
		prometheus.IncrementParticipantJoinFail(1)
		status := http.StatusInternalServerError
		var psrpcErr psrpc.Error
		if errors.As(err, &psrpcErr) {
			status = psrpcErr.ToHttp()
		}
		HandleError(w, r, status, err, getLoggerFields()...)
		return
	}

	prometheus.IncrementParticipantJoin(1)

	pLogger = pLogger.WithValues("connID", cr.ConnectionID)
	if !pi.Reconnect && initialResponse.GetJoin() != nil {
		joinRoomID := voicekit.RoomID(initialResponse.GetJoin().GetRoom().GetSid())
		if joinRoomID != "" {
			roomID = joinRoomID
		}

		pi.ID = voicekit.ParticipantID(initialResponse.GetJoin().GetParticipant().GetSid())
		pID = pi.ID

		resolveLogger(false)
	}

	signalStats := telemetry.NewBytesSignalStats(r.Context(), s.telemetry)
	if join := initialResponse.GetJoin(); join != nil {
		signalStats.ResolveRoom(join.GetRoom())
		signalStats.ResolveParticipant(join.GetParticipant())
	}
	if pi.Reconnect && pi.ID != "" {
		signalStats.ResolveParticipant(&voicekit.ParticipantInfo{
			Sid:      string(pi.ID),
			Identity: string(pi.Identity),
		})
	}

	closedByClient := atomic.NewBool(false)
	done := make(chan struct{})
	// function exits when websocket terminates, it'll close the event reading off of request sink and response source as well
	defer func() {
		pLogger.Debugw("finishing WS connection", "closedByClient", closedByClient.Load())
		cr.ResponseSource.Close()
		cr.RequestSink.Close()
		close(done)

		signalStats.Stop()
	}()

	// upgrade only once the basics are good to go
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		HandleError(w, r, http.StatusInternalServerError, err, getLoggerFields()...)
		return
	}

	s.mu.Lock()
	s.connections[conn] = struct{}{}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.connections, conn)
		s.mu.Unlock()
	}()

	// websocket established
	sigConn := NewWSSignalConnection(conn)
	pLogger.Debugw("sending initial response", "response", logger.Proto(initialResponse))
	count, err := sigConn.WriteResponse(initialResponse)
	if err != nil {
		resolveLogger(true)
		pLogger.Warnw("could not write initial response", err)
		return
	}
	signalStats.AddBytes(uint64(count), true)

	pLogger.Debugw(
		"new client WS connected",
		"reconnect", pi.Reconnect,
		"reconnectReason", pi.ReconnectReason,
		"adaptiveStream", pi.AdaptiveStream,
		"selectedNodeID", cr.NodeID,
		"nodeSelectionReason", cr.NodeSelectionReason,
	)

	// handle responses
	go func() {
		defer func() {
			// when the source is terminated, this means Participant.Close had been called and RTC connection is done
			// we would terminate the signal connection as well
			closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
			_ = conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
			_ = conn.Close()
		}()
		defer func() {
			if r := rtc.Recover(pLogger); r != nil {
				os.Exit(1)
			}
		}()
		for {
			select {
			case <-done:
				return
			case msg := <-cr.ResponseSource.ReadChan():
				if msg == nil {
					resolveLogger(true)
					pLogger.Debugw("nothing to read from response source")
					return
				}
				res, ok := msg.(*voicekit.SignalResponse)
				if !ok {
					pLogger.Errorw(
						"unexpected message type", nil,
						"type", fmt.Sprintf("%T", msg),
					)
					continue
				}

				switch m := res.Message.(type) {
				case *voicekit.SignalResponse_Offer:
					pLogger.Debugw("sending offer", "offer", m)
				case *voicekit.SignalResponse_Answer:
					pLogger.Debugw("sending answer", "answer", m)
				case *voicekit.SignalResponse_Join:
					pLogger.Debugw("sending join", "join", m)
					signalStats.ResolveRoom(m.Join.GetRoom())
					signalStats.ResolveParticipant(m.Join.GetParticipant())
				case *voicekit.SignalResponse_RoomUpdate:
					updateRoomID := voicekit.RoomID(m.RoomUpdate.GetRoom().GetSid())
					if updateRoomID != "" {
						roomID = updateRoomID
						resolveLogger(false)
					}
					pLogger.Debugw("sending room update", "roomUpdate", m)
					signalStats.ResolveRoom(m.RoomUpdate.GetRoom())
				case *voicekit.SignalResponse_Update:
					pLogger.Debugw("sending participant update", "participantUpdate", m)
				case *voicekit.SignalResponse_RoomMoved:
					resetLogger()
					signalStats.Reset()
					roomName = voicekit.RoomName(m.RoomMoved.GetRoom().GetName())
					moveRoomID := voicekit.RoomID(m.RoomMoved.GetRoom().GetSid())
					if moveRoomID != "" {
						roomID = moveRoomID
					}
					participantIdentity = voicekit.ParticipantIdentity(m.RoomMoved.GetParticipant().GetIdentity())
					pID = voicekit.ParticipantID(m.RoomMoved.GetParticipant().GetSid())
					resolveLogger(false)
					signalStats.ResolveRoom(m.RoomMoved.GetRoom())
					signalStats.ResolveParticipant(m.RoomMoved.GetParticipant())
					pLogger.Debugw("sending room moved", "roomMoved", m)
				}

				if count, err := sigConn.WriteResponse(res); err != nil {
					pLogger.Warnw("error writing to websocket", err)
					return
				} else {
					signalStats.AddBytes(uint64(count), true)
				}
			}
		}
	}()

	// handle incoming requests from websocket
	for {
		req, count, err := sigConn.ReadRequest()
		if err != nil {
			if IsWebSocketCloseError(err) {
				closedByClient.Store(true)
			} else {
				pLogger.Errorw("error reading from websocket", err)
			}
			return
		}
		signalStats.AddBytes(uint64(count), false)

		switch m := req.Message.(type) {
		case *voicekit.SignalRequest_Ping:
			count, perr := sigConn.WriteResponse(&voicekit.SignalResponse{
				Message: &voicekit.SignalResponse_Pong{
					//
					// Although this field is int64, some clients (like JS) cause overflow if nanosecond granularity is used.
					// So. use UnixMillis().
					//
					Pong: time.Now().UnixMilli(),
				},
			})
			if perr == nil {
				signalStats.AddBytes(uint64(count), true)
			}
		case *voicekit.SignalRequest_PingReq:
			count, perr := sigConn.WriteResponse(&voicekit.SignalResponse{
				Message: &voicekit.SignalResponse_PongResp{
					PongResp: &voicekit.Pong{
						LastPingTimestamp: m.PingReq.Timestamp,
						Timestamp:         time.Now().UnixMilli(),
					},
				},
			})
			if perr == nil {
				signalStats.AddBytes(uint64(count), true)
			}
		}

		switch m := req.Message.(type) {
		case *voicekit.SignalRequest_Offer:
			pLogger.Debugw("received offer", "offer", m)
		case *voicekit.SignalRequest_Answer:
			pLogger.Debugw("received answer", "answer", m)
		}

		if err := cr.RequestSink.WriteMessage(req); err != nil {
			pLogger.Warnw("error writing to request sink", err)
			return
		}
	}
}

func (s *RTCService) ParseClientInfo(r *http.Request) *voicekit.ClientInfo {
	values := r.Form
	ci := &voicekit.ClientInfo{}
	if pv, err := strconv.Atoi(values.Get("protocol")); err == nil {
		ci.Protocol = int32(pv)
	}
	sdkString := values.Get("sdk")
	switch sdkString {
	case "js":
		ci.Sdk = voicekit.ClientInfo_JS
	case "ios", "swift":
		ci.Sdk = voicekit.ClientInfo_SWIFT
	case "android":
		ci.Sdk = voicekit.ClientInfo_ANDROID
	case "flutter":
		ci.Sdk = voicekit.ClientInfo_FLUTTER
	case "go":
		ci.Sdk = voicekit.ClientInfo_GO
	case "unity":
		ci.Sdk = voicekit.ClientInfo_UNITY
	case "reactnative":
		ci.Sdk = voicekit.ClientInfo_REACT_NATIVE
	case "rust":
		ci.Sdk = voicekit.ClientInfo_RUST
	case "python":
		ci.Sdk = voicekit.ClientInfo_PYTHON
	case "cpp":
		ci.Sdk = voicekit.ClientInfo_CPP
	case "unityweb":
		ci.Sdk = voicekit.ClientInfo_UNITY_WEB
	case "node":
		ci.Sdk = voicekit.ClientInfo_NODE
	}

	ci.Version = values.Get("version")
	ci.Os = values.Get("os")
	ci.OsVersion = values.Get("os_version")
	ci.Browser = values.Get("browser")
	ci.BrowserVersion = values.Get("browser_version")
	ci.DeviceModel = values.Get("device_model")
	ci.Network = values.Get("network")
	// get real address (forwarded http header) - check Cloudflare headers first, fall back to X-Forwarded-For
	ci.Address = GetClientIP(r)

	// attempt to parse types for SDKs that support browser as a platform
	if ci.Sdk == voicekit.ClientInfo_JS ||
		ci.Sdk == voicekit.ClientInfo_REACT_NATIVE ||
		ci.Sdk == voicekit.ClientInfo_FLUTTER ||
		ci.Sdk == voicekit.ClientInfo_UNITY {
		client := s.parser.Parse(r.UserAgent())
		if ci.Browser == "" {
			ci.Browser = client.UserAgent.Family
			ci.BrowserVersion = client.UserAgent.ToVersionString()
		}
		if ci.Os == "" {
			ci.Os = client.Os.Family
			ci.OsVersion = client.Os.ToVersionString()
		}
		if ci.DeviceModel == "" {
			model := client.Device.Family
			if model != "" && client.Device.Model != "" && model != client.Device.Model {
				model += " " + client.Device.Model
			}

			ci.DeviceModel = model
		}
	}

	return ci
}

func (s *RTCService) DrainConnections(interval time.Duration) {
	s.mu.Lock()
	conns := maps.Clone(s.connections)
	s.mu.Unlock()

	// jitter drain start
	time.Sleep(time.Duration(rand.Int63n(int64(interval))))

	t := time.NewTicker(interval)
	defer t.Stop()

	for c := range conns {
		_ = c.Close()
		<-t.C
	}
}

type connectionResult struct {
	routing.StartParticipantSignalResults
	Room *voicekit.Room
}

func (s *RTCService) startConnection(
	ctx context.Context,
	roomName voicekit.RoomName,
	pi routing.ParticipantInit,
	timeout time.Duration,
) (connectionResult, *voicekit.SignalResponse, error) {
	var cr connectionResult
	var err error

	if err := s.roomAllocator.SelectRoomNode(ctx, roomName, ""); err != nil {
		return cr, nil, err
	}

	// this needs to be started first *before* using router functions on this node
	cr.StartParticipantSignalResults, err = s.router.StartParticipantSignal(ctx, roomName, pi)
	if err != nil {
		return cr, nil, err
	}

	// wait for the first message before upgrading to websocket. If no one is
	// responding to our connection attempt, we should terminate the connection
	// instead of waiting forever on the WebSocket
	initialResponse, err := readInitialResponse(cr.ResponseSource, timeout)
	if err != nil {
		// close the connection to avoid leaking
		cr.RequestSink.Close()
		cr.ResponseSource.Close()
		return cr, nil, err
	}

	return cr, initialResponse, nil
}

func readInitialResponse(source routing.MessageSource, timeout time.Duration) (*voicekit.SignalResponse, error) {
	responseTimer := time.NewTimer(timeout)
	defer responseTimer.Stop()
	for {
		select {
		case <-responseTimer.C:
			return nil, errors.New("timed out while waiting for signal response")
		case msg := <-source.ReadChan():
			if msg == nil {
				return nil, errors.New("connection closed by media")
			}
			res, ok := msg.(*voicekit.SignalResponse)
			if !ok {
				return nil, fmt.Errorf("unexpected message type: %T", msg)
			}
			return res, nil
		}
	}
}
