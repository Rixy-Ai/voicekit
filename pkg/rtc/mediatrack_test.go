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

func TestTrackInfo(t *testing.T) {
	// ensures that persisted trackinfo is being returned
	ti := voicekit.TrackInfo{
		Sid:       "testsid",
		Name:      "testtrack",
		Source:    voicekit.TrackSource_SCREEN_SHARE,
		Type:      voicekit.TrackType_VIDEO,
		Simulcast: false,
		Width:     100,
		Height:    80,
		Muted:     true,
	}

	mt := NewMediaTrack(MediaTrackParams{}, &ti)
	outInfo := mt.ToProto()
	require.Equal(t, ti.Muted, outInfo.Muted)
	require.Equal(t, ti.Name, outInfo.Name)
	require.Equal(t, ti.Name, mt.Name())
	require.Equal(t, voicekit.TrackID(ti.Sid), mt.ID())
	require.Equal(t, ti.Type, outInfo.Type)
	require.Equal(t, ti.Type, mt.Kind())
	require.Equal(t, ti.Source, outInfo.Source)
	require.Equal(t, ti.Width, outInfo.Width)
	require.Equal(t, ti.Height, outInfo.Height)
	require.Equal(t, ti.Simulcast, outInfo.Simulcast)

	// make it simulcasted
	mt.SetSimulcast(true)
	require.True(t, mt.ToProto().Simulcast)
}

func TestGetQualityForDimension(t *testing.T) {
	t.Run("landscape source", func(t *testing.T) {
		mt := NewMediaTrack(MediaTrackParams{}, &voicekit.TrackInfo{
			Type:   voicekit.TrackType_VIDEO,
			Width:  1080,
			Height: 720,
		})

		require.Equal(t, voicekit.VideoQuality_LOW, mt.GetQualityForDimension(120, 120))
		require.Equal(t, voicekit.VideoQuality_LOW, mt.GetQualityForDimension(300, 200))
		require.Equal(t, voicekit.VideoQuality_MEDIUM, mt.GetQualityForDimension(200, 250))
		require.Equal(t, voicekit.VideoQuality_HIGH, mt.GetQualityForDimension(700, 480))
		require.Equal(t, voicekit.VideoQuality_HIGH, mt.GetQualityForDimension(500, 1000))
	})

	t.Run("portrait source", func(t *testing.T) {
		mt := NewMediaTrack(MediaTrackParams{}, &voicekit.TrackInfo{
			Type:   voicekit.TrackType_VIDEO,
			Width:  540,
			Height: 960,
		})

		require.Equal(t, voicekit.VideoQuality_LOW, mt.GetQualityForDimension(200, 400))
		require.Equal(t, voicekit.VideoQuality_MEDIUM, mt.GetQualityForDimension(400, 400))
		require.Equal(t, voicekit.VideoQuality_MEDIUM, mt.GetQualityForDimension(400, 700))
		require.Equal(t, voicekit.VideoQuality_HIGH, mt.GetQualityForDimension(600, 900))
	})

	t.Run("layers provided", func(t *testing.T) {
		mt := NewMediaTrack(MediaTrackParams{}, &voicekit.TrackInfo{
			Type:   voicekit.TrackType_VIDEO,
			Width:  1080,
			Height: 720,
			Layers: []*voicekit.VideoLayer{
				{
					Quality: voicekit.VideoQuality_LOW,
					Width:   480,
					Height:  270,
				},
				{
					Quality: voicekit.VideoQuality_MEDIUM,
					Width:   960,
					Height:  540,
				},
				{
					Quality: voicekit.VideoQuality_HIGH,
					Width:   1080,
					Height:  720,
				},
			},
		})

		require.Equal(t, voicekit.VideoQuality_LOW, mt.GetQualityForDimension(120, 120))
		require.Equal(t, voicekit.VideoQuality_LOW, mt.GetQualityForDimension(300, 300))
		require.Equal(t, voicekit.VideoQuality_MEDIUM, mt.GetQualityForDimension(800, 500))
		require.Equal(t, voicekit.VideoQuality_HIGH, mt.GetQualityForDimension(1000, 700))
	})

	t.Run("highest layer with smallest dimensions", func(t *testing.T) {
		mt := NewMediaTrack(MediaTrackParams{}, &voicekit.TrackInfo{
			Type:   voicekit.TrackType_VIDEO,
			Width:  1080,
			Height: 720,
			Layers: []*voicekit.VideoLayer{
				{
					Quality: voicekit.VideoQuality_LOW,
					Width:   480,
					Height:  270,
				},
				{
					Quality: voicekit.VideoQuality_MEDIUM,
					Width:   1080,
					Height:  720,
				},
				{
					Quality: voicekit.VideoQuality_HIGH,
					Width:   1080,
					Height:  720,
				},
			},
		})

		require.Equal(t, voicekit.VideoQuality_LOW, mt.GetQualityForDimension(120, 120))
		require.Equal(t, voicekit.VideoQuality_LOW, mt.GetQualityForDimension(300, 300))
		require.Equal(t, voicekit.VideoQuality_HIGH, mt.GetQualityForDimension(800, 500))
		require.Equal(t, voicekit.VideoQuality_HIGH, mt.GetQualityForDimension(1000, 700))
		require.Equal(t, voicekit.VideoQuality_HIGH, mt.GetQualityForDimension(1200, 800))

		mt = NewMediaTrack(MediaTrackParams{}, &voicekit.TrackInfo{
			Type:   voicekit.TrackType_VIDEO,
			Width:  1080,
			Height: 720,
			Layers: []*voicekit.VideoLayer{
				{
					Quality: voicekit.VideoQuality_LOW,
					Width:   480,
					Height:  270,
				},
				{
					Quality: voicekit.VideoQuality_MEDIUM,
					Width:   480,
					Height:  270,
				},
				{
					Quality: voicekit.VideoQuality_HIGH,
					Width:   1080,
					Height:  720,
				},
			},
		})

		require.Equal(t, voicekit.VideoQuality_MEDIUM, mt.GetQualityForDimension(120, 120))
		require.Equal(t, voicekit.VideoQuality_MEDIUM, mt.GetQualityForDimension(300, 300))
		require.Equal(t, voicekit.VideoQuality_HIGH, mt.GetQualityForDimension(800, 500))
		require.Equal(t, voicekit.VideoQuality_HIGH, mt.GetQualityForDimension(1000, 700))
		require.Equal(t, voicekit.VideoQuality_HIGH, mt.GetQualityForDimension(1200, 800))
	})

}
