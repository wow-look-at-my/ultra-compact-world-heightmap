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
	packed, err := Encode(tile)
	require.NoError(t, err)
	back, err := Decode(packed)
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
	defer clear(Tune)
	Tune["w1n"] = 7
	Tune["w2n"] = 13
	tile := &terrarium.Tile{Width: 70, Height: 50, Samples: make([]int32, 70*50)}
	for y := 0; y < 50; y++ {
		for x := 0; x < 70; x++ {
			tile.Samples[y*70+x] = seaLevel + int32(3*x*x+5*y*x-y*y)*4
		}
	}
	roundTrip(t, tile)
}

func TestEncodeRejectsWhatIsNotATile(t *testing.T) {
	_, err := Encode(&terrarium.Tile{Width: 2, Height: 2, Samples: []int32{1}})
	require.Error(t, err)
	_, err = Encode(&terrarium.Tile{Width: 1, Height: 1, Samples: []int32{1 << 24}})
	require.Error(t, err)
}

func TestDecodeRejectsCorruptData(t *testing.T) {
	for _, data := range [][]byte{nil, {0}, {0x80, 0}, {31, 0}, {0, 0}, {0, 1, 0}, {flagStandardSize | flagConstant, 0xff}} {
		_, err := Decode(data)
		require.Error(t, err, "%v", data)
	}
}
