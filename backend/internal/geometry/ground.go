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

package geometry

import "fmt"

// ValidateGround checks that the ground plane configuration is physically valid.
//
// Ground types and their requirements:
//   - "" or "free_space": no ground plane; antenna radiates in free space.
//     No additional parameters needed.
//   - "perfect": ideal infinite perfectly-conducting ground plane (PEC).
//     The solver uses image theory to mirror currents. No material parameters needed.
//   - "real": lossy ground with finite conductivity and permittivity.
//     Conductivity (S/m) and relative permittivity must both be positive
//     because they define the Fresnel reflection coefficients used by the
//     solver for the ground-reflected field contributions.
func ValidateGround(g GroundDTO) error {
	switch g.Type {
	case "", "free_space":
		return nil
	case "perfect":
		return nil
	case "real":
		if g.Conductivity <= 0 {
			return fmt.Errorf("real ground requires positive conductivity, got %f", g.Conductivity)
		}
		if g.Permittivity <= 0 {
			return fmt.Errorf("real ground requires positive relative permittivity, got %f", g.Permittivity)
		}
		return nil
	default:
		return fmt.Errorf("unknown ground type %q; valid types: free_space, perfect, real", g.Type)
	}
}
