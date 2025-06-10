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

package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	"github.com/voicekit/voicekit-server/pkg/config"
	"github.com/voicekit/voicekit-server/pkg/routing"
	"github.com/voicekit/voicekit-server/pkg/service"
	"github.com/voicekit/voicekit-server/pkg/service/servicefakes"
	"github.com/voicekit/voicekit-server/pkg/telemetry/prometheus"
	"github.com/voicekit/protocol/voicekit"
	"github.com/voicekit/protocol/logger"
	"github.com/voicekit/psrpc"
)

func init() {
	prometheus.Init("node", voicekit.NodeType_CONTROLLER)
}

func TestSignal(t *testing.T) {
	cfg := config.SignalRelayConfig{
		RetryTimeout:     30 * time.Second,
		MinRetryInterval: 500 * time.Millisecond,
		MaxRetryInterval: 5 * time.Second,
		StreamBufferSize: 1000,
	}

	t.Run("messages are delivered", func(t *testing.T) {
		bus := psrpc.NewLocalMessageBus()

		reqMessageIn := &voicekit.SignalRequest{
			Message: &voicekit.SignalRequest_Ping{Ping: 123},
		}
		resMessageIn := &voicekit.SignalResponse{
			Message: &voicekit.SignalResponse_Pong{Pong: 321},
		}

		var reqMessageOut proto.Message
		var resErr error
		done := make(chan struct{})

		client, err := routing.NewSignalClient(voicekit.NodeID("node0"), bus, cfg)
		require.NoError(t, err)

		handler := &servicefakes.FakeSessionHandler{
			LoggerStub: func(context.Context) logger.Logger { return logger.GetLogger() },
			HandleSessionStub: func(
				ctx context.Context,
				pi routing.ParticipantInit,
				connectionID voicekit.ConnectionID,
				requestSource routing.MessageSource,
				responseSink routing.MessageSink,
			) error {
				go func() {
					reqMessageOut = <-requestSource.ReadChan()
					resErr = responseSink.WriteMessage(resMessageIn)
					responseSink.Close()
					close(done)
				}()
				return nil
			},
		}
		server, err := service.NewSignalServer(voicekit.NodeID("node1"), "region", bus, cfg, handler)
		require.NoError(t, err)

		err = server.Start()
		require.NoError(t, err)

		_, reqSink, resSource, err := client.StartParticipantSignal(
			context.Background(),
			voicekit.RoomName("room1"),
			routing.ParticipantInit{},
			voicekit.NodeID("node1"),
		)
		require.NoError(t, err)

		err = reqSink.WriteMessage(reqMessageIn)
		require.NoError(t, err)

		<-done
		require.True(t, proto.Equal(reqMessageIn, reqMessageOut), "req message should match %s %s", protojson.Format(reqMessageIn), protojson.Format(reqMessageOut))
		require.NoError(t, resErr)

		resMessageOut := <-resSource.ReadChan()
		require.True(t, proto.Equal(resMessageIn, resMessageOut), "res message should match %s %s", protojson.Format(resMessageIn), protojson.Format(resMessageOut))
	})

	t.Run("messages are delivered when session handler fails", func(t *testing.T) {
		bus := psrpc.NewLocalMessageBus()

		resMessageIn := &voicekit.SignalResponse{
			Message: &voicekit.SignalResponse_Pong{Pong: 321},
		}

		var resErr error
		done := make(chan struct{})

		client, err := routing.NewSignalClient(voicekit.NodeID("node0"), bus, cfg)
		require.NoError(t, err)

		handler := &servicefakes.FakeSessionHandler{
			LoggerStub: func(context.Context) logger.Logger { return logger.GetLogger() },
			HandleSessionStub: func(
				ctx context.Context,
				pi routing.ParticipantInit,
				connectionID voicekit.ConnectionID,
				requestSource routing.MessageSource,
				responseSink routing.MessageSink,
			) error {
				defer close(done)
				resErr = responseSink.WriteMessage(resMessageIn)
				return errors.New("start session failed")
			},
		}
		server, err := service.NewSignalServer(voicekit.NodeID("node1"), "region", bus, cfg, handler)
		require.NoError(t, err)

		err = server.Start()
		require.NoError(t, err)

		_, _, resSource, err := client.StartParticipantSignal(
			context.Background(),
			voicekit.RoomName("room1"),
			routing.ParticipantInit{},
			voicekit.NodeID("node1"),
		)
		require.NoError(t, err)

		<-done
		require.NoError(t, resErr)

		resMessageOut := <-resSource.ReadChan()
		require.True(t, proto.Equal(resMessageIn, resMessageOut), "res message should match %s %s", protojson.Format(resMessageIn), protojson.Format(resMessageOut))
	})
}
