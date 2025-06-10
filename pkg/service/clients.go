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

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/voicekit/voicekit-server/pkg/rtc"
	"github.com/voicekit/protocol/voicekit"
	"github.com/voicekit/protocol/logger"
	"github.com/voicekit/protocol/rpc"
	"github.com/voicekit/protocol/utils"
	"github.com/voicekit/protocol/utils/guid"
)

//counterfeiter:generate . IOClient
type IOClient interface {
	CreateEgress(ctx context.Context, info *voicekit.EgressInfo) (*emptypb.Empty, error)
	GetEgress(ctx context.Context, req *rpc.GetEgressRequest) (*voicekit.EgressInfo, error)
	ListEgress(ctx context.Context, req *voicekit.ListEgressRequest) (*voicekit.ListEgressResponse, error)
	CreateIngress(ctx context.Context, req *voicekit.IngressInfo) (*emptypb.Empty, error)
	UpdateIngressState(ctx context.Context, req *rpc.UpdateIngressStateRequest) (*emptypb.Empty, error)
}

type egressLauncher struct {
	client rpc.EgressClient
	io     IOClient
}

func NewEgressLauncher(client rpc.EgressClient, io IOClient) rtc.EgressLauncher {
	if client == nil {
		return nil
	}
	return &egressLauncher{
		client: client,
		io:     io,
	}
}

func (s *egressLauncher) StartEgress(ctx context.Context, req *rpc.StartEgressRequest) (*voicekit.EgressInfo, error) {
	if s.client == nil {
		return nil, ErrEgressNotConnected
	}

	// Ensure we have an Egress ID
	if req.EgressId == "" {
		req.EgressId = guid.New(utils.EgressPrefix)
	}

	info, err := s.client.StartEgress(ctx, "", req)
	if err != nil {
		return nil, err
	}

	_, err = s.io.CreateEgress(ctx, info)
	if err != nil {
		logger.Errorw("failed to create egress", err)
	}

	return info, nil
}
