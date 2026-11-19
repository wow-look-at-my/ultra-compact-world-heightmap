package codec

import (
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

func referenceRoundTrip(t *testing.T, tile, like *terrarium.Tile) []byte {
	t.Helper()
	return valuesRoundTrip(t, tile, ReferenceTile(like))
}

func valuesRoundTrip(t *testing.T, tile *terrarium.Tile, ref *Reference) []byte {
	t.Helper()
	packed, err := EncodeReference(tile, ref)
	require.NoError(t, err)
	require.True(t, NeedsReference(packed))
	require.False(t, NeedsParent(packed))
	back, err := DecodeReference(packed, ref)
	require.NoError(t, err)
	require.Equal(t, tile.Width, back.Width)
	require.Equal(t, tile.Height, back.Height)
	require.Equal(t, tile.Samples, back.Samples)
	return packed
}

func TestATileEqualToItsReferenceIsAFewBytes(t *testing.T) {
	tile, _ := loadFixture(t, "12-2144-1434.webp")
	assert.Less(t, len(referenceRoundTrip(t, tile, tile)), 8)
}

func TestAFewChangedSamplesCostAFewBytesEach(t *testing.T) {
	tile, _ := loadFixture(t, "12-2144-1434.webp")
	alone, err := Encode(tile, nil)
	require.NoError(t, err)
	ref := &terrarium.Tile{Width: tile.Width, Height: tile.Height, Samples: append([]int32(nil), tile.Samples...)}
	rng := rand.New(rand.NewSource(7))
	const changed = 200
	for i := 0; i < changed; i++ {
		ref.Samples[rng.Intn(len(ref.Samples))] += 128 * int32(rng.Intn(2)*2-1)
	}
	packed := referenceRoundTrip(t, tile, ref)
	assert.Less(t, len(packed), 3*changed)
	assert.Less(t, len(packed), len(alone)/20)
}

func TestAReferenceAtAnotherStepOrFarAwayStillRoundTrips(t *testing.T) {
	tile, _ := loadFixture(t, "12-2144-1434.webp")
	rng := rand.New(rand.NewSource(3))
	ref := &terrarium.Tile{Width: tile.Width, Height: tile.Height, Samples: make([]int32, len(tile.Samples))}
	for i, v := range tile.Samples {
		ref.Samples[i] = v + rng.Int31n(4001) - 2000
	}
	referenceRoundTrip(t, tile, ref)

	for _, size := range [][2]int{{1, 1}, {5, 1}, {33, 17}} {
		small := &terrarium.Tile{Width: size[0], Height: size[1], Samples: make([]int32, size[0]*size[1])}
		far := &terrarium.Tile{Width: size[0], Height: size[1], Samples: make([]int32, size[0]*size[1])}
		for i := range small.Samples {
			small.Samples[i], far.Samples[i] = rng.Int31n(1<<24), rng.Int31n(1<<24)
		}
		referenceRoundTrip(t, small, far)
	}
	flat := &terrarium.Tile{Width: 4, Height: 4, Samples: make([]int32, 16)}
	referenceRoundTrip(t, flat, flat)
}

func TestReferenceModeRefusesWhatItCannotCode(t *testing.T) {
	tile := &terrarium.Tile{Width: 2, Height: 2, Samples: []int32{1, 2, 3, 4}}
	same := ReferenceTile(tile)
	other := ReferenceTile(&terrarium.Tile{Width: 4, Height: 1, Samples: []int32{1, 2, 3, 4}})
	_, err := EncodeReference(tile, other)
	require.ErrorContains(t, err, "reference")
	_, err = EncodeReference(&terrarium.Tile{Width: 2, Height: 2, Samples: []int32{1}}, same)
	require.Error(t, err)
	_, err = EncodeReference(&terrarium.Tile{Width: 1, Height: 1, Samples: []int32{-1}}, &Reference{Width: 1, Height: 1, Values: []float64{0}})
	require.ErrorContains(t, err, "24 bits")

	packed, err := EncodeReference(tile, same)
	require.NoError(t, err)
	width, height, err := ReferenceSize(packed)
	require.NoError(t, err)
	assert.Equal(t, [2]int{2, 2}, [2]int{width, height})
	_, err = DecodeReference(packed, nil)
	require.ErrorContains(t, err, "reference")
	_, err = DecodeReference(packed, other)
	require.ErrorContains(t, err, "reference")
	_, err = Decode(packed, nil)
	require.ErrorContains(t, err, "reference")
	alone, err := Encode(tile, nil)
	require.NoError(t, err)
	_, err = DecodeReference(alone, same)
	require.ErrorIs(t, err, errCorrupt)
	_, err = DecodeReference([]byte{flagReference}, same)
	require.ErrorIs(t, err, errCorrupt)
	_, err = DecodeReference([]byte{flagReference | 31}, same)
	require.ErrorIs(t, err, errCorrupt)

	// A reference that the encoder did not read gives samples outside a tile.
	high := &terrarium.Tile{Width: 2, Height: 2, Samples: []int32{1<<24 - 1, 1<<24 - 1, 1<<24 - 1, 1<<24 - 1}}
	low := &terrarium.Tile{Width: 2, Height: 2, Samples: []int32{0, 0, 0, 0}}
	packed, err = EncodeReference(high, ReferenceTile(low))
	require.NoError(t, err)
	_, err = DecodeReference(packed, ReferenceTile(high))
	require.ErrorIs(t, err, errCorrupt)
}

func TestHeightsBeforeRoundingMakeADifferenceCheaper(t *testing.T) {
	tile, _ := loadFixture(t, "12-2144-1434.webp")
	// The fixture has a step of half a metre.
	rng := rand.New(rand.NewSource(11))
	metres := make([]float32, len(tile.Samples))
	rounded := &terrarium.Tile{Width: tile.Width, Height: tile.Height, Samples: append([]int32(nil), tile.Samples...)}
	moved := 0
	for i, v := range tile.Samples {
		offset := rng.Float64()*0.49 - 0.245
		if rng.Intn(300) == 0 {
			offset = 0.2505
			rounded.Samples[i] += 128
			moved++
		}
		metres[i] = float32(float64(v)/256 - 32768 + offset)
	}
	require.Greater(t, moved, 500)
	exact := valuesRoundTrip(t, tile, ReferenceHeights(metres, tile.Width, tile.Height))
	coarse := referenceRoundTrip(t, tile, rounded)
	assert.Less(t, len(exact), len(coarse)*2/3)

	// Values that no tile holds still give the tile back.
	odd := &Reference{Width: 2, Height: 2, Values: []float64{math.NaN(), math.Inf(1), -1e300, 5}}
	valuesRoundTrip(t, &terrarium.Tile{Width: 2, Height: 2, Samples: []int32{7, 8, 9, 10}}, odd)
}
