package internal

// TrackConfig represents a track configuration
type TrackConfig struct {
	Name    string
	TrackID string
}

// GetTracks returns all configured tracks for GTR 3 (class 1703)
func GetTracks() []TrackConfig {
	return []TrackConfig{
		{"Anderstorp Raceway - Grand Prix", "5301"},
		{"Anderstorp Raceway - South", "6164"},
		{"Autodrom Most - Grand Prix", "7112"},
		{"Bathurst Circuit - Mount Panorama", "1846"},
		{"Bilster Berg - Gesamtstrecke", "7819"},
	}
}
