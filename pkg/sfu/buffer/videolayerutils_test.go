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

package buffer

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/voicekit/protocol/voicekit"
)

func TestRidConversion(t *testing.T) {
	type RidAndLayer struct {
		rid   string
		layer int32
	}
	tests := []struct {
		name       string
		trackInfo  *voicekit.TrackInfo
		ridToLayer map[string]RidAndLayer
	}{
		{
			"no track info",
			nil,
			map[string]RidAndLayer{
				"":                {rid: QuarterResolution, layer: 0},
				QuarterResolution: {rid: QuarterResolution, layer: 0},
				HalfResolution:    {rid: HalfResolution, layer: 1},
				FullResolution:    {rid: FullResolution, layer: 2},
			},
		},
		{
			"no layers",
			&voicekit.TrackInfo{},
			map[string]RidAndLayer{
				"":                {rid: QuarterResolution, layer: 0},
				QuarterResolution: {rid: QuarterResolution, layer: 0},
				HalfResolution:    {rid: HalfResolution, layer: 1},
				FullResolution:    {rid: FullResolution, layer: 2},
			},
		},
		{
			"single layer, low",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_LOW},
				},
			},
			map[string]RidAndLayer{
				"":                {rid: QuarterResolution, layer: 0},
				QuarterResolution: {rid: QuarterResolution, layer: 0},
				HalfResolution:    {rid: QuarterResolution, layer: 0},
				FullResolution:    {rid: QuarterResolution, layer: 0},
			},
		},
		{
			"single layer, medium",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_MEDIUM},
				},
			},
			map[string]RidAndLayer{
				"":                {rid: QuarterResolution, layer: 0},
				QuarterResolution: {rid: QuarterResolution, layer: 0},
				HalfResolution:    {rid: QuarterResolution, layer: 0},
				FullResolution:    {rid: QuarterResolution, layer: 0},
			},
		},
		{
			"single layer, high",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_HIGH},
				},
			},
			map[string]RidAndLayer{
				"":                {rid: QuarterResolution, layer: 0},
				QuarterResolution: {rid: QuarterResolution, layer: 0},
				HalfResolution:    {rid: QuarterResolution, layer: 0},
				FullResolution:    {rid: QuarterResolution, layer: 0},
			},
		},
		{
			"two layers, low and medium",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_LOW},
					{Quality: voicekit.VideoQuality_MEDIUM},
				},
			},
			map[string]RidAndLayer{
				"":                {rid: QuarterResolution, layer: 0},
				QuarterResolution: {rid: QuarterResolution, layer: 0},
				HalfResolution:    {rid: HalfResolution, layer: 1},
				FullResolution:    {rid: HalfResolution, layer: 1},
			},
		},
		{
			"two layers, low and high",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_LOW},
					{Quality: voicekit.VideoQuality_HIGH},
				},
			},
			map[string]RidAndLayer{
				"":                {rid: QuarterResolution, layer: 0},
				QuarterResolution: {rid: QuarterResolution, layer: 0},
				HalfResolution:    {rid: HalfResolution, layer: 1},
				FullResolution:    {rid: HalfResolution, layer: 1},
			},
		},
		{
			"two layers, medium and high",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_MEDIUM},
					{Quality: voicekit.VideoQuality_HIGH},
				},
			},
			map[string]RidAndLayer{
				"":                {rid: QuarterResolution, layer: 0},
				QuarterResolution: {rid: QuarterResolution, layer: 0},
				HalfResolution:    {rid: HalfResolution, layer: 1},
				FullResolution:    {rid: HalfResolution, layer: 1},
			},
		},
		{
			"three layers",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_LOW},
					{Quality: voicekit.VideoQuality_MEDIUM},
					{Quality: voicekit.VideoQuality_HIGH},
				},
			},
			map[string]RidAndLayer{
				"":                {rid: QuarterResolution, layer: 0},
				QuarterResolution: {rid: QuarterResolution, layer: 0},
				HalfResolution:    {rid: HalfResolution, layer: 1},
				FullResolution:    {rid: FullResolution, layer: 2},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for testRid, expectedResult := range test.ridToLayer {
				actualLayer := RidToSpatialLayer(testRid, test.trackInfo)
				require.Equal(t, expectedResult.layer, actualLayer)

				actualRid := SpatialLayerToRid(actualLayer, test.trackInfo)
				require.Equal(t, expectedResult.rid, actualRid)
			}
		})
	}
}

func TestQualityConversion(t *testing.T) {
	type QualityAndLayer struct {
		quality voicekit.VideoQuality
		layer   int32
	}
	tests := []struct {
		name           string
		trackInfo      *voicekit.TrackInfo
		qualityToLayer map[voicekit.VideoQuality]QualityAndLayer
	}{
		{
			"no track info",
			nil,
			map[voicekit.VideoQuality]QualityAndLayer{
				voicekit.VideoQuality_LOW:    {quality: voicekit.VideoQuality_LOW, layer: 0},
				voicekit.VideoQuality_MEDIUM: {quality: voicekit.VideoQuality_MEDIUM, layer: 1},
				voicekit.VideoQuality_HIGH:   {quality: voicekit.VideoQuality_HIGH, layer: 2},
			},
		},
		{
			"no layers",
			&voicekit.TrackInfo{},
			map[voicekit.VideoQuality]QualityAndLayer{
				voicekit.VideoQuality_LOW:    {quality: voicekit.VideoQuality_LOW, layer: 0},
				voicekit.VideoQuality_MEDIUM: {quality: voicekit.VideoQuality_MEDIUM, layer: 1},
				voicekit.VideoQuality_HIGH:   {quality: voicekit.VideoQuality_HIGH, layer: 2},
			},
		},
		{
			"single layer, low",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_LOW},
				},
			},
			map[voicekit.VideoQuality]QualityAndLayer{
				voicekit.VideoQuality_LOW:    {quality: voicekit.VideoQuality_LOW, layer: 0},
				voicekit.VideoQuality_MEDIUM: {quality: voicekit.VideoQuality_LOW, layer: 0},
				voicekit.VideoQuality_HIGH:   {quality: voicekit.VideoQuality_LOW, layer: 0},
			},
		},
		{
			"single layer, medium",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_MEDIUM},
				},
			},
			map[voicekit.VideoQuality]QualityAndLayer{
				voicekit.VideoQuality_LOW:    {quality: voicekit.VideoQuality_MEDIUM, layer: 0},
				voicekit.VideoQuality_MEDIUM: {quality: voicekit.VideoQuality_MEDIUM, layer: 0},
				voicekit.VideoQuality_HIGH:   {quality: voicekit.VideoQuality_MEDIUM, layer: 0},
			},
		},
		{
			"single layer, high",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_HIGH},
				},
			},
			map[voicekit.VideoQuality]QualityAndLayer{
				voicekit.VideoQuality_LOW:    {quality: voicekit.VideoQuality_HIGH, layer: 0},
				voicekit.VideoQuality_MEDIUM: {quality: voicekit.VideoQuality_HIGH, layer: 0},
				voicekit.VideoQuality_HIGH:   {quality: voicekit.VideoQuality_HIGH, layer: 0},
			},
		},
		{
			"two layers, low and medium",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_LOW},
					{Quality: voicekit.VideoQuality_MEDIUM},
				},
			},
			map[voicekit.VideoQuality]QualityAndLayer{
				voicekit.VideoQuality_LOW:    {quality: voicekit.VideoQuality_LOW, layer: 0},
				voicekit.VideoQuality_MEDIUM: {quality: voicekit.VideoQuality_MEDIUM, layer: 1},
				voicekit.VideoQuality_HIGH:   {quality: voicekit.VideoQuality_MEDIUM, layer: 1},
			},
		},
		{
			"two layers, low and high",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_LOW},
					{Quality: voicekit.VideoQuality_HIGH},
				},
			},
			map[voicekit.VideoQuality]QualityAndLayer{
				voicekit.VideoQuality_LOW:    {quality: voicekit.VideoQuality_LOW, layer: 0},
				voicekit.VideoQuality_MEDIUM: {quality: voicekit.VideoQuality_HIGH, layer: 1},
				voicekit.VideoQuality_HIGH:   {quality: voicekit.VideoQuality_HIGH, layer: 1},
			},
		},
		{
			"two layers, medium and high",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_MEDIUM},
					{Quality: voicekit.VideoQuality_HIGH},
				},
			},
			map[voicekit.VideoQuality]QualityAndLayer{
				voicekit.VideoQuality_LOW:    {quality: voicekit.VideoQuality_MEDIUM, layer: 0},
				voicekit.VideoQuality_MEDIUM: {quality: voicekit.VideoQuality_MEDIUM, layer: 0},
				voicekit.VideoQuality_HIGH:   {quality: voicekit.VideoQuality_HIGH, layer: 1},
			},
		},
		{
			"three layers",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_LOW},
					{Quality: voicekit.VideoQuality_MEDIUM},
					{Quality: voicekit.VideoQuality_HIGH},
				},
			},
			map[voicekit.VideoQuality]QualityAndLayer{
				voicekit.VideoQuality_LOW:    {quality: voicekit.VideoQuality_LOW, layer: 0},
				voicekit.VideoQuality_MEDIUM: {quality: voicekit.VideoQuality_MEDIUM, layer: 1},
				voicekit.VideoQuality_HIGH:   {quality: voicekit.VideoQuality_HIGH, layer: 2},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for testQuality, expectedResult := range test.qualityToLayer {
				actualLayer := VideoQualityToSpatialLayer(testQuality, test.trackInfo)
				require.Equal(t, expectedResult.layer, actualLayer)

				actualQuality := SpatialLayerToVideoQuality(actualLayer, test.trackInfo)
				require.Equal(t, expectedResult.quality, actualQuality)
			}
		})
	}
}

func TestVideoQualityToRidConversion(t *testing.T) {
	tests := []struct {
		name         string
		trackInfo    *voicekit.TrackInfo
		qualityToRid map[voicekit.VideoQuality]string
	}{
		{
			"no track info",
			nil,
			map[voicekit.VideoQuality]string{
				voicekit.VideoQuality_LOW:    QuarterResolution,
				voicekit.VideoQuality_MEDIUM: HalfResolution,
				voicekit.VideoQuality_HIGH:   FullResolution,
			},
		},
		{
			"no layers",
			&voicekit.TrackInfo{},
			map[voicekit.VideoQuality]string{
				voicekit.VideoQuality_LOW:    QuarterResolution,
				voicekit.VideoQuality_MEDIUM: HalfResolution,
				voicekit.VideoQuality_HIGH:   FullResolution,
			},
		},
		{
			"single layer, low",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_LOW},
				},
			},
			map[voicekit.VideoQuality]string{
				voicekit.VideoQuality_LOW:    QuarterResolution,
				voicekit.VideoQuality_MEDIUM: QuarterResolution,
				voicekit.VideoQuality_HIGH:   QuarterResolution,
			},
		},
		{
			"single layer, medium",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_MEDIUM},
				},
			},
			map[voicekit.VideoQuality]string{
				voicekit.VideoQuality_LOW:    QuarterResolution,
				voicekit.VideoQuality_MEDIUM: QuarterResolution,
				voicekit.VideoQuality_HIGH:   QuarterResolution,
			},
		},
		{
			"single layer, high",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_HIGH},
				},
			},
			map[voicekit.VideoQuality]string{
				voicekit.VideoQuality_LOW:    QuarterResolution,
				voicekit.VideoQuality_MEDIUM: QuarterResolution,
				voicekit.VideoQuality_HIGH:   QuarterResolution,
			},
		},
		{
			"two layers, low and medium",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_LOW},
					{Quality: voicekit.VideoQuality_MEDIUM},
				},
			},
			map[voicekit.VideoQuality]string{
				voicekit.VideoQuality_LOW:    QuarterResolution,
				voicekit.VideoQuality_MEDIUM: HalfResolution,
				voicekit.VideoQuality_HIGH:   HalfResolution,
			},
		},
		{
			"two layers, low and high",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_LOW},
					{Quality: voicekit.VideoQuality_HIGH},
				},
			},
			map[voicekit.VideoQuality]string{
				voicekit.VideoQuality_LOW:    QuarterResolution,
				voicekit.VideoQuality_MEDIUM: HalfResolution,
				voicekit.VideoQuality_HIGH:   HalfResolution,
			},
		},
		{
			"two layers, medium and high",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_MEDIUM},
					{Quality: voicekit.VideoQuality_HIGH},
				},
			},
			map[voicekit.VideoQuality]string{
				voicekit.VideoQuality_LOW:    QuarterResolution,
				voicekit.VideoQuality_MEDIUM: QuarterResolution,
				voicekit.VideoQuality_HIGH:   HalfResolution,
			},
		},
		{
			"three layers",
			&voicekit.TrackInfo{
				Layers: []*voicekit.VideoLayer{
					{Quality: voicekit.VideoQuality_LOW},
					{Quality: voicekit.VideoQuality_MEDIUM},
					{Quality: voicekit.VideoQuality_HIGH},
				},
			},
			map[voicekit.VideoQuality]string{
				voicekit.VideoQuality_LOW:    QuarterResolution,
				voicekit.VideoQuality_MEDIUM: HalfResolution,
				voicekit.VideoQuality_HIGH:   FullResolution,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for testQuality, expectedRid := range test.qualityToRid {
				actualRid := VideoQualityToRid(testQuality, test.trackInfo)
				require.Equal(t, expectedRid, actualRid)
			}
		})
	}
}
