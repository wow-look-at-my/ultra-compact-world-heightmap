package geo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTheWorldTileSpansTheGlobe(t *testing.T) {
	assert.InDelta(t, -180*3600+0.5*360*3600/512, LonArcsec(0, 0, 0, 512), 1e-6)
	assert.InDelta(t, 0, LonArcsec(1, 1, 0, 512)-0.5*360*3600/1024, 1e-6)
	assert.InDelta(t, 85.05*3600, LatArcsec(0, 0, 0, 512), 400)
	assert.InDelta(t, 0, LatArcsec(1, 0, 511, 512)+LatArcsec(1, 1, 0, 512), 1e-6)
}

func TestGLO30LonStepWidensTowardThePoles(t *testing.T) {
	for lat, step := range map[float64]float64{0: 1, -49: 1, 55: 1.5, 65: 2, -75: 3, 82: 5, 88: 10} {
		assert.InDelta(t, step, GLO30LonStep(lat*3600), 0, "%v", lat)
	}
}

func TestPhase(t *testing.T) {
	assert.InDelta(t, 0.25, Phase(7.25, 1), 1e-12)
	assert.InDelta(t, 0.75, Phase(-7.25, 1), 1e-12)
	assert.InDelta(t, 0.5, Phase(3, 2), 1e-12)
}
