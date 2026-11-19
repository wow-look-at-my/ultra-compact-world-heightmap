package warp

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheSeriesAgreeWithTheMathLibrary(t *testing.T) {
	for x := -3.2; x <= 3.2; x += 0.0137 {
		assert.InEpsilon(t, math.Exp(x), exp(x), 1e-12, "exp %v", x)
		assert.InDelta(t, math.Atan(x), atan(x), 1e-14, "atan %v", x)
		assert.InDelta(t, math.Atan(math.Sinh(x))*180/math.Pi, latitude(x*earthRadius), 1e-11, "latitude %v", x)
	}
	assert.Zero(t, latitude(0))
}

func TestTheWeightsOfTheSplineSumToOne(t *testing.T) {
	for _, at := range []float64{0, 0.1, 0.5, 0.999} {
		w := spline(at)
		assert.InDelta(t, 1, w[0]+w[1]+w[2]+w[3], 1e-15)
		// The spline gives back a line: the weights place their mean at the sample.
		assert.InDelta(t, at, -w[0]+w[2]+2*w[3], 1e-15)
	}
}

func slope(east, south float64) *Grid {
	g := &Grid{Width: 64, Height: 64, West: 9.92, North: 24.06, StepLon: 0.002, StepLat: 0.002, Heights: make([]float32, 64*64)}
	for j := 0; j < 64; j++ {
		for i := 0; i < 64; i++ {
			g.Heights[j*64+i] = float32(100 + east*float64(i) + south*float64(j))
		}
	}
	return g
}

func TestAPlaneComesBackAsThePlane(t *testing.T) {
	flat, err := Tile(slope(0, 0), 12, 2161, 1766, 512)
	require.NoError(t, err)
	for _, v := range flat {
		require.InDelta(t, 100, v, 1e-4)
	}
	tile, err := Tile(slope(2, 0.5), 12, 2161, 1766, 512)
	require.NoError(t, err)
	// The centre of the first sample of the tile, in grid samples from the first grid sample.
	west := (2161.0/4096*360 - 180 + 360.0/4096/512/2 - 9.92) / 0.002
	perSample := 360.0 / 4096 / 512 / 0.002
	for c := 0; c < 512; c += 37 {
		// Along one row the height rises by the slope to the east alone.
		assert.InDelta(t, float64(tile[c])-float64(tile[0]), 2*perSample*float64(c), 1e-3)
	}
	assert.InDelta(t, 100+2*west, float64(tile[0])-0.5*rowOf(t, 0), 1e-3)
	// Rows go south, so the height rises down the tile.
	assert.Greater(t, tile[511*512], tile[0])
}

func rowOf(t *testing.T, r int) float64 {
	t.Helper()
	span := 2 * halfWorld / 4096
	return (24.06 - latitude(halfWorld-1766*span-(float64(r)+0.5)*span/512)) / 0.002
}

func TestAGridThatDoesNotReachIsRefused(t *testing.T) {
	g := slope(1, 1)
	_, err := Tile(g, 12, 2162, 1766, 512)
	require.ErrorContains(t, err, "does not reach")
	_, err = Tile(g, 12, 2161, 1765, 512)
	require.ErrorContains(t, err, "does not reach")
	_, err = Tile(g, 12, 5000, 1766, 512)
	require.ErrorContains(t, err, "not a tile")
	_, err = Tile(&Grid{Width: 64, Height: 64, StepLon: 0.002, StepLat: 0.002}, 12, 2161, 1766, 512)
	require.ErrorContains(t, err, "cannot be warped")
	_, err = Tile(&Grid{Width: 64, Height: 64, Heights: make([]float32, 64*64)}, 12, 2161, 1766, 512)
	require.ErrorContains(t, err, "cannot be warped")
}

// rough builds a grid with no pattern that a rounding error can hide in.
func rough() *Grid {
	g := slope(0, 0)
	for j := 0; j < 64; j++ {
		for i := 0; i < 64; i++ {
			g.Heights[j*64+i] = float32((i*7+j*13)%100)*4.25 + float32(i*j%5)*0.125
		}
	}
	return g
}

func TestEveryMachineWarpsToTheSameBits(t *testing.T) {
	tile, err := Tile(rough(), 12, 2161, 1766, 512)
	require.NoError(t, err)
	raw := make([]byte, 0, 4*len(tile))
	for _, v := range tile {
		raw = binary.LittleEndian.AppendUint32(raw, math.Float32bits(v))
	}
	sum := sha256.Sum256(raw)
	assert.Equal(t, goldenWarp, hex.EncodeToString(sum[:]))
}
