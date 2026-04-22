// Copyright 2026 Sergio Slobodrian
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

package mom

import "testing"

func TestRemapSegIndex_ZeroStaysZero(t *testing.T) {
	cases := [][2]int{{25, 51}, {101, 203}, {3, 7}, {1, 3}}
	for _, c := range cases {
		oldSeg, newSeg := c[0], c[1]
		got := remapSegIndex(0, oldSeg, newSeg)
		if got != 0 {
			t.Errorf("remapSegIndex(0, %d, %d) = %d, want 0", oldSeg, newSeg, got)
		}
	}
}

func TestRemapSegIndex_LastStaysLast(t *testing.T) {
	cases := [][2]int{{25, 51}, {101, 203}, {3, 7}}
	for _, c := range cases {
		oldSeg, newSeg := c[0], c[1]
		got := remapSegIndex(oldSeg-1, oldSeg, newSeg)
		if got != newSeg-1 {
			t.Errorf("remapSegIndex(%d, %d, %d) = %d, want %d", oldSeg-1, oldSeg, newSeg, got, newSeg-1)
		}
	}
}

func TestRemapSegIndex_MiddleProportional(t *testing.T) {
	// Middle index should map near the proportional midpoint of the new mesh.
	// For oldIndex=12, oldSeg=25, newSeg=51: expect ~25 (middle of 0-50).
	got := remapSegIndex(12, 25, 51)
	if got < 23 || got > 27 {
		t.Errorf("remapSegIndex(12, 25, 51) = %d, expected near 25", got)
	}
}

func TestRemapSegIndex_EdgeCases(t *testing.T) {
	// oldSeg=0: should not panic, return 0
	if got := remapSegIndex(5, 0, 10); got != 0 {
		t.Errorf("remapSegIndex(5, 0, 10) = %d, want 0", got)
	}
	// negative oldIndex: clamps to 0
	if got := remapSegIndex(-1, 25, 51); got != 0 {
		t.Errorf("remapSegIndex(-1, 25, 51) = %d, want 0", got)
	}
}
