package geometry

import "fmt"

// ValidateGround checks that the ground configuration is valid.
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
