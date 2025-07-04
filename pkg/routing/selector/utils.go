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

package selector

import (
	"sort"
	"time"

	"github.com/thoas/go-funk"

	"github.com/voicekit/protocol/voicekit"

	"github.com/voicekit/voicekit-server/pkg/config"
)

const AvailableSeconds = 5

// checks if a node has been updated recently to be considered for selection
func IsAvailable(node *voicekit.Node) bool {
	if node.Stats == nil {
		// available till stats are available
		return true
	}

	delta := time.Now().Unix() - node.Stats.UpdatedAt
	return int(delta) < AvailableSeconds
}

func GetAvailableNodes(nodes []*voicekit.Node) []*voicekit.Node {
	return funk.Filter(nodes, func(node *voicekit.Node) bool {
		return IsAvailable(node) && node.State == voicekit.NodeState_SERVING
	}).([]*voicekit.Node)
}

func GetNodeSysload(node *voicekit.Node) float32 {
	stats := node.Stats
	numCpus := stats.NumCpus
	if numCpus == 0 {
		numCpus = 1
	}
	return stats.LoadAvgLast1Min / float32(numCpus)
}

// TODO: check remote node configured limit, instead of this node's config
func LimitsReached(limitConfig config.LimitConfig, nodeStats *voicekit.NodeStats) bool {
	if nodeStats == nil {
		return false
	}

	if limitConfig.NumTracks > 0 && limitConfig.NumTracks <= nodeStats.NumTracksIn+nodeStats.NumTracksOut {
		return true
	}

	rate := &voicekit.NodeStatsRate{}
	if len(nodeStats.Rates) > 0 {
		rate = nodeStats.Rates[0]
	}
	if limitConfig.BytesPerSec > 0 && limitConfig.BytesPerSec <= rate.BytesIn+rate.BytesOut {
		return true
	}

	return false
}

func SelectSortedNode(nodes []*voicekit.Node, sortBy string) (*voicekit.Node, error) {
	if sortBy == "" {
		return nil, ErrSortByNotSet
	}

	// Return a node based on what it should be sorted by for priority
	switch sortBy {
	case "random":
		idx := funk.RandomInt(0, len(nodes))
		return nodes[idx], nil
	case "sysload":
		sort.Slice(nodes, func(i, j int) bool {
			return GetNodeSysload(nodes[i]) < GetNodeSysload(nodes[j])
		})
		return nodes[0], nil
	case "cpuload":
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].Stats.CpuLoad < nodes[j].Stats.CpuLoad
		})
		return nodes[0], nil
	case "rooms":
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].Stats.NumRooms < nodes[j].Stats.NumRooms
		})
		return nodes[0], nil
	case "clients":
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].Stats.NumClients < nodes[j].Stats.NumClients
		})
		return nodes[0], nil
	case "tracks":
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].Stats.NumTracksIn+nodes[i].Stats.NumTracksOut < nodes[j].Stats.NumTracksIn+nodes[j].Stats.NumTracksOut
		})
		return nodes[0], nil
	case "bytespersec":
		sort.Slice(nodes, func(i, j int) bool {
			ratei := &voicekit.NodeStatsRate{}
			if len(nodes[i].Stats.Rates) > 0 {
				ratei = nodes[i].Stats.Rates[0]
			}

			ratej := &voicekit.NodeStatsRate{}
			if len(nodes[j].Stats.Rates) > 0 {
				ratej = nodes[j].Stats.Rates[0]
			}
			return ratei.BytesIn+ratei.BytesOut < ratej.BytesIn+ratej.BytesOut
		})
		return nodes[0], nil
	default:
		return nil, ErrSortByUnknown
	}
}
