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
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/voicekit/protocol/voicekit"
)

func TestPackStreamId(t *testing.T) {
	packed := "PA_123abc|uuid-id"
	pID, trackID := UnpackStreamID(packed)
	require.Equal(t, voicekit.ParticipantID("PA_123abc"), pID)
	require.Equal(t, voicekit.TrackID("uuid-id"), trackID)

	require.Equal(t, packed, PackStreamID(pID, trackID))
}

func TestPackDataTrackLabel(t *testing.T) {
	pID := voicekit.ParticipantID("PA_123abc")
	trackID := voicekit.TrackID("TR_b3da25")
	label := "trackLabel"
	packed := "PA_123abc|TR_b3da25|trackLabel"
	require.Equal(t, packed, PackDataTrackLabel(pID, trackID, label))

	p, tr, l := UnpackDataTrackLabel(packed)
	require.Equal(t, pID, p)
	require.Equal(t, trackID, tr)
	require.Equal(t, label, l)
}
