// tune.go holds the knobs that experiments turn without a rebuild.

package codec

// Tune overrides a model parameter by name. Set it before any tile is coded.
var Tune = map[string]float64{}

func knob(name string, fallback float64) float64 {
	if v, ok := Tune[name]; ok {
		return v
	}
	return fallback
}
