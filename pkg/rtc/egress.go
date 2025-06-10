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
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/voicekit/voicekit-server/pkg/rtc/types"
	"github.com/voicekit/voicekit-server/pkg/telemetry"
	"github.com/voicekit/protocol/voicekit"
	"github.com/voicekit/protocol/rpc"
	"github.com/voicekit/protocol/webhook"
)

type EgressLauncher interface {
	StartEgress(context.Context, *rpc.StartEgressRequest) (*voicekit.EgressInfo, error)
}

func StartParticipantEgress(
	ctx context.Context,
	launcher EgressLauncher,
	ts telemetry.TelemetryService,
	opts *voicekit.AutoParticipantEgress,
	identity voicekit.ParticipantIdentity,
	roomName voicekit.RoomName,
	roomID voicekit.RoomID,
) error {
	if req, err := startParticipantEgress(ctx, launcher, opts, identity, roomName, roomID); err != nil {
		// send egress failed webhook

		info := &voicekit.EgressInfo{
			RoomId:   string(roomID),
			RoomName: string(roomName),
			Status:   voicekit.EgressStatus_EGRESS_FAILED,
			Error:    err.Error(),
			Request:  &voicekit.EgressInfo_Participant{Participant: req},
		}

		ts.NotifyEgressEvent(ctx, webhook.EventEgressEnded, info)

		return err
	}
	return nil
}

func startParticipantEgress(
	ctx context.Context,
	launcher EgressLauncher,
	opts *voicekit.AutoParticipantEgress,
	identity voicekit.ParticipantIdentity,
	roomName voicekit.RoomName,
	roomID voicekit.RoomID,
) (*voicekit.ParticipantEgressRequest, error) {
	req := &voicekit.ParticipantEgressRequest{
		RoomName:       string(roomName),
		Identity:       string(identity),
		FileOutputs:    opts.FileOutputs,
		SegmentOutputs: opts.SegmentOutputs,
	}

	switch o := opts.Options.(type) {
	case *voicekit.AutoParticipantEgress_Preset:
		req.Options = &voicekit.ParticipantEgressRequest_Preset{Preset: o.Preset}
	case *voicekit.AutoParticipantEgress_Advanced:
		req.Options = &voicekit.ParticipantEgressRequest_Advanced{Advanced: o.Advanced}
	}

	if launcher == nil {
		return req, errors.New("egress launcher not found")
	}

	_, err := launcher.StartEgress(ctx, &rpc.StartEgressRequest{
		Request: &rpc.StartEgressRequest_Participant{
			Participant: req,
		},
		RoomId: string(roomID),
	})
	return req, err
}

func StartTrackEgress(
	ctx context.Context,
	launcher EgressLauncher,
	ts telemetry.TelemetryService,
	opts *voicekit.AutoTrackEgress,
	track types.MediaTrack,
	roomName voicekit.RoomName,
	roomID voicekit.RoomID,
) error {
	if req, err := startTrackEgress(ctx, launcher, opts, track, roomName, roomID); err != nil {
		// send egress failed webhook

		info := &voicekit.EgressInfo{
			RoomId:   string(roomID),
			RoomName: string(roomName),
			Status:   voicekit.EgressStatus_EGRESS_FAILED,
			Error:    err.Error(),
			Request:  &voicekit.EgressInfo_Track{Track: req},
		}
		ts.NotifyEgressEvent(ctx, webhook.EventEgressEnded, info)

		return err
	}
	return nil
}

func startTrackEgress(
	ctx context.Context,
	launcher EgressLauncher,
	opts *voicekit.AutoTrackEgress,
	track types.MediaTrack,
	roomName voicekit.RoomName,
	roomID voicekit.RoomID,
) (*voicekit.TrackEgressRequest, error) {
	output := &voicekit.DirectFileOutput{
		Filepath: getFilePath(opts.Filepath),
	}

	switch out := opts.Output.(type) {
	case *voicekit.AutoTrackEgress_Azure:
		output.Output = &voicekit.DirectFileOutput_Azure{Azure: out.Azure}
	case *voicekit.AutoTrackEgress_Gcp:
		output.Output = &voicekit.DirectFileOutput_Gcp{Gcp: out.Gcp}
	case *voicekit.AutoTrackEgress_S3:
		output.Output = &voicekit.DirectFileOutput_S3{S3: out.S3}
	}

	req := &voicekit.TrackEgressRequest{
		RoomName: string(roomName),
		TrackId:  string(track.ID()),
		Output: &voicekit.TrackEgressRequest_File{
			File: output,
		},
	}

	if launcher == nil {
		return req, errors.New("egress launcher not found")
	}

	_, err := launcher.StartEgress(ctx, &rpc.StartEgressRequest{
		Request: &rpc.StartEgressRequest_Track{
			Track: req,
		},
		RoomId: string(roomID),
	})
	return req, err
}

func getFilePath(filepath string) string {
	if filepath == "" || strings.HasSuffix(filepath, "/") || strings.Contains(filepath, "{track_id}") {
		return filepath
	}

	idx := strings.Index(filepath, ".")
	if idx == -1 {
		return fmt.Sprintf("%s-{track_id}", filepath)
	} else {
		return fmt.Sprintf("%s-%s%s", filepath[:idx], "{track_id}", filepath[idx:])
	}
}
