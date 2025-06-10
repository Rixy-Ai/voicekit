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
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"

	"github.com/pion/webrtc/v4"

	"github.com/voicekit/voicekit-server/pkg/sfu/mime"
	"github.com/voicekit/protocol/voicekit"
	"github.com/voicekit/protocol/logger"
)

const (
	trackIdSeparator = "|"

	cMinIPTruncateLen = 8
)

func UnpackStreamID(packed string) (participantID voicekit.ParticipantID, trackID voicekit.TrackID) {
	parts := strings.Split(packed, trackIdSeparator)
	if len(parts) > 1 {
		return voicekit.ParticipantID(parts[0]), voicekit.TrackID(packed[len(parts[0])+1:])
	}
	return voicekit.ParticipantID(packed), ""
}

func PackStreamID(participantID voicekit.ParticipantID, trackID voicekit.TrackID) string {
	return string(participantID) + trackIdSeparator + string(trackID)
}

func PackSyncStreamID(participantID voicekit.ParticipantID, stream string) string {
	return string(participantID) + trackIdSeparator + stream
}

func StreamFromTrackSource(source voicekit.TrackSource) string {
	// group camera/mic, screenshare/audio together
	switch source {
	case voicekit.TrackSource_SCREEN_SHARE:
		return "screen"
	case voicekit.TrackSource_SCREEN_SHARE_AUDIO:
		return "screen"
	case voicekit.TrackSource_CAMERA:
		return "camera"
	case voicekit.TrackSource_MICROPHONE:
		return "camera"
	}
	return "unknown"
}

func PackDataTrackLabel(participantID voicekit.ParticipantID, trackID voicekit.TrackID, label string) string {
	return string(participantID) + trackIdSeparator + string(trackID) + trackIdSeparator + label
}

func UnpackDataTrackLabel(packed string) (participantID voicekit.ParticipantID, trackID voicekit.TrackID, label string) {
	parts := strings.Split(packed, trackIdSeparator)
	if len(parts) != 3 {
		return "", voicekit.TrackID(packed), ""
	}
	participantID = voicekit.ParticipantID(parts[0])
	trackID = voicekit.TrackID(parts[1])
	label = parts[2]
	return
}

func ToProtoSessionDescription(sd webrtc.SessionDescription) *voicekit.SessionDescription {
	return &voicekit.SessionDescription{
		Type: sd.Type.String(),
		Sdp:  sd.SDP,
	}
}

func FromProtoSessionDescription(sd *voicekit.SessionDescription) webrtc.SessionDescription {
	var sdType webrtc.SDPType
	switch sd.Type {
	case webrtc.SDPTypeOffer.String():
		sdType = webrtc.SDPTypeOffer
	case webrtc.SDPTypeAnswer.String():
		sdType = webrtc.SDPTypeAnswer
	case webrtc.SDPTypePranswer.String():
		sdType = webrtc.SDPTypePranswer
	case webrtc.SDPTypeRollback.String():
		sdType = webrtc.SDPTypeRollback
	}
	return webrtc.SessionDescription{
		Type: sdType,
		SDP:  sd.Sdp,
	}
}

func ToProtoTrickle(candidateInit webrtc.ICECandidateInit, target voicekit.SignalTarget, final bool) *voicekit.TrickleRequest {
	data, _ := json.Marshal(candidateInit)
	return &voicekit.TrickleRequest{
		CandidateInit: string(data),
		Target:        target,
		Final:         final,
	}
}

func FromProtoTrickle(trickle *voicekit.TrickleRequest) (webrtc.ICECandidateInit, error) {
	ci := webrtc.ICECandidateInit{}
	err := json.Unmarshal([]byte(trickle.CandidateInit), &ci)
	if err != nil {
		return webrtc.ICECandidateInit{}, err
	}
	return ci, nil
}

func ToProtoTrackKind(kind webrtc.RTPCodecType) voicekit.TrackType {
	switch kind {
	case webrtc.RTPCodecTypeVideo:
		return voicekit.TrackType_VIDEO
	case webrtc.RTPCodecTypeAudio:
		return voicekit.TrackType_AUDIO
	}
	panic("unsupported track direction")
}

func IsEOF(err error) bool {
	return err == io.ErrClosedPipe || err == io.EOF
}

func Recover(l logger.Logger) any {
	if l == nil {
		l = logger.GetLogger()
	}
	r := recover()
	if r != nil {
		var err error
		switch e := r.(type) {
		case string:
			err = errors.New(e)
		case error:
			err = e
		default:
			err = errors.New("unknown panic")
		}
		l.Errorw("recovered panic", err, "panic", r)
	}

	return r
}

// logger helpers
func LoggerWithParticipant(l logger.Logger, identity voicekit.ParticipantIdentity, sid voicekit.ParticipantID, isRemote bool) logger.Logger {
	values := make([]interface{}, 0, 4)
	if identity != "" {
		values = append(values, "participant", identity)
	}
	if sid != "" {
		values = append(values, "pID", sid)
	}
	values = append(values, "remote", isRemote)
	// enable sampling per participant
	return l.WithValues(values...)
}

func LoggerWithRoom(l logger.Logger, name voicekit.RoomName, roomID voicekit.RoomID) logger.Logger {
	values := make([]interface{}, 0, 2)
	if name != "" {
		values = append(values, "room", name)
	}
	if roomID != "" {
		values = append(values, "roomID", roomID)
	}
	// also sample for the room
	return l.WithItemSampler().WithValues(values...)
}

func LoggerWithTrack(l logger.Logger, trackID voicekit.TrackID, isRelayed bool) logger.Logger {
	// sampling not required because caller already passing in participant's logger
	if trackID != "" {
		return l.WithValues("trackID", trackID, "relayed", isRelayed)
	}
	return l
}

func LoggerWithPCTarget(l logger.Logger, target voicekit.SignalTarget) logger.Logger {
	return l.WithValues("transport", target)
}

func LoggerWithCodecMime(l logger.Logger, mimeType mime.MimeType) logger.Logger {
	if mimeType != mime.MimeTypeUnknown {
		return l.WithValues("mime", mimeType.String())
	}
	return l
}

func MaybeTruncateIP(addr string) string {
	ipAddr := net.ParseIP(addr)
	if ipAddr == nil {
		return ""
	}

	if ipAddr.IsPrivate() || len(addr) <= cMinIPTruncateLen {
		return addr
	}

	return addr[:len(addr)-3] + "..."
}
