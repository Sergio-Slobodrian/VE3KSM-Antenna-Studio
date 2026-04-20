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

// Package nec2 parses and writes NEC-2 input files (.nec / .nec2).
//
// Supported card subset (the most common ones in amateur and engineering
// use; sufficient to round-trip dipoles, Yagis, verticals, loops, and
// designs with lumped loads or transmission-line elements):
//
//   CM, CE  - comment / comment-end
//   GW      - wire geometry (tag, segments, x1,y1,z1, x2,y2,z2, radius)
//   GS      - geometry scale factor
//   GE      - geometry end (with ground flag)
//   GN      - ground type (free-space / perfect / real Sommerfeld)
//   LD      - lumped load (series RLC, parallel RLC, impedance, conductivity)
//   EX      - excitation (voltage source via delta-gap)
//   TL      - transmission line element (2-port, with stub via tag<0)
//   FR      - frequency / frequency sweep
//   EN      - end of input (terminates parsing)
//
// All other cards are accepted but ignored, with the line preserved so
// callers can warn the user.  NEC's "fixed columns 3-12, 14-23, ..." layout
// is supported by the more lenient free-format reader, which is what every
// modern editor produces.
package nec2
