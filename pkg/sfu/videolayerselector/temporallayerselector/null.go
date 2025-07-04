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

package temporallayerselector

import (
	"github.com/voicekit/voicekit-server/pkg/sfu/buffer"
)

type Null struct{}

func NewNull() *Null {
	return &Null{}
}

func Select(_extPkt *buffer.ExtPacket, current int32, _target int32) (this int32, next int32) {
	this = current
	next = current
	return
}
