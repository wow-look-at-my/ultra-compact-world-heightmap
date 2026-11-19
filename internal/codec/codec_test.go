package codec

import (
	"bytes"
	"math/rand"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

func loadFixture(t *testing.T, name string) (*terrarium.Tile, int) {
	t.Helper()
	data, err := os.ReadFile("../../testdata/tiles/" + name)
	require.NoError(t, err)
	tile, err := terrarium.DecodeWebP(bytes.NewReader(data))
	require.NoError(t, err)
	return tile, len(data)
}

func roundTrip(t *testing.T, tile *terrarium.Tile) []byte {
	t.Helper()
	packed, err := Encode(tile, nil)
	require.NoError(t, err)
	back, err := Decode(packed, nil)
	require.NoError(t, err)
	require.Equal(t, tile.Width, back.Width)
	require.Equal(t, tile.Height, back.Height)
	require.Equal(t, tile.Samples, back.Samples)
	return packed
}

func TestRealTilesRoundTripAndBeatWebP(t *testing.T) {
	for _, name := range []string{"12-2144-1434.webp", "2-0-2.webp"} {
		tile, webp := loadFixture(t, name)
		packed := roundTrip(t, tile)
		assert.Less(t, len(packed), webp, name)
	}
}

func TestAConstantTileIsAHeader(t *testing.T) {
	tile := &terrarium.Tile{Width: 512, Height: 512, Samples: make([]int32, 512*512)}
	for i := range tile.Samples {
		tile.Samples[i] = seaLevel
	}
	assert.Len(t, roundTrip(t, tile), 2)
}

func TestOddSizesAndRoughDataRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, size := range [][2]int{{1, 1}, {1, 7}, {5, 1}, {33, 17}, {64, 64}} {
		tile := &terrarium.Tile{Width: size[0], Height: size[1], Samples: make([]int32, size[0]*size[1])}
		for i := range tile.Samples {
			tile.Samples[i] = rng.Int31n(1 << 24)
		}
		roundTrip(t, tile)
	}
}

func TestEveryKnobStillRoundTrips(t *testing.T) {
	t.Serial("the knobs are package state that every other test reads")
	defer clear(Tune)
	for name, value := range map[string]float64{"w1n": 7, "w2n": 0, "cw": 0, "sse": 0, "stack": 0, "wexp": 3, "share": 63, "warm": 0, "c1c": 0, "c1p": 0, "r3n": 0} {
		Tune[name] = value
	}
	tile := &terrarium.Tile{Width: 70, Height: 50, Samples: make([]int32, 70*50)}
	for y := 0; y < 50; y++ {
		for x := 0; x < 70; x++ {
			tile.Samples[y*70+x] = seaLevel + int32(3*x*x+5*y*x-y*y)*4
		}
	}
	roundTrip(t, tile)
	parent, children := family(32, 7, 8)
	p := &Parent{Tile: parent, Quadrant: 1}
	packed, err := Encode(children[1], p)
	require.NoError(t, err)
	back, err := Decode(packed, p)
	require.NoError(t, err)
	require.Equal(t, children[1].Samples, back.Samples)
}

func TestEncodeRejectsWhatIsNotATile(t *testing.T) {
	_, err := Encode(&terrarium.Tile{Width: 2, Height: 2, Samples: []int32{1}}, nil)
	require.Error(t, err)
	_, err = Encode(&terrarium.Tile{Width: 1, Height: 1, Samples: []int32{1 << 24}}, nil)
	require.Error(t, err)
}

func TestDecodeRejectsCorruptData(t *testing.T) {
	for _, data := range [][]byte{nil, {0}, {0x80, 0}, {31, 0}, {0, 0}, {0, 1, 0}, {flagStandardSize | flagConstant, 0xff}, {flagChild | flagConstant, 8, 0}} {
		_, err := Decode(data, nil)
		require.Error(t, err, "%v", data)
	}
}

// family makes a rough parent and those children it is the rounded mean of.
func family(size int, childShift, parentShift uint) (*terrarium.Tile, [4]*terrarium.Tile) {
	rng := rand.New(rand.NewSource(7))
	side := 2 * size
	full := make([]int32, side*side)
	for y := 0; y < side; y++ {
		for x := 0; x < side; x++ {
			full[y*side+x] = (seaLevel>>childShift + int32(x*y/40) + rng.Int31n(9)) << childShift
		}
	}
	parent := &terrarium.Tile{Width: size, Height: size, Samples: make([]int32, size*size)}
	step := int64(4) << parentShift
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			sum := int64(full[2*y*side+2*x]) + int64(full[2*y*side+2*x+1]) + int64(full[(2*y+1)*side+2*x]) + int64(full[(2*y+1)*side+2*x+1])
			q := sum / step
			if r := sum % step; 2*r > step || (2*r == step && q&1 != 0) {
				q++
			}
			parent.Samples[y*size+x] = int32(q << parentShift)
		}
	}
	var children [4]*terrarium.Tile
	for k := range children {
		c := &terrarium.Tile{Width: size, Height: size, Samples: make([]int32, size*size)}
		for y := 0; y < size; y++ {
			copy(c.Samples[y*size:(y+1)*size], full[(k/2*size+y)*side+k%2*size:])
		}
		children[k] = c
	}
	return parent, children
}

func TestAChildCodedAgainstItsParentRoundTripsAndIsSmaller(t *testing.T) {
	for _, shifts := range [][2]uint{{8, 8}, {7, 8}, {3, 4}} {
		parent, children := family(64, shifts[0], shifts[1])
		for k, child := range children {
			p := &Parent{Tile: parent, Quadrant: k}
			packed, err := Encode(child, p)
			require.NoError(t, err)
			require.True(t, NeedsParent(packed))
			back, err := Decode(packed, p)
			require.NoError(t, err)
			require.Equal(t, child.Samples, back.Samples)
			assert.Less(t, len(packed), len(roundTrip(t, child)))
			_, err = Decode(packed, nil)
			require.Error(t, err)
			_, err = Decode(packed, &Parent{Tile: child, Quadrant: 4})
			require.Error(t, err)
		}
	}
}

func TestATileThatDoesNotFollowTheParentDecodesAlone(t *testing.T) {
	parent, children := family(64, 8, 8)
	children[0].Samples[5] += 40 << 8
	packed, err := Encode(children[0], &Parent{Tile: parent})
	require.NoError(t, err)
	require.False(t, NeedsParent(packed))
	back, err := Decode(packed, nil)
	require.NoError(t, err)
	require.Equal(t, children[0].Samples, back.Samples)
}
