package codec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

func TestATileThatStartsFromTheTileBeforeItIsSmaller(t *testing.T) {
	tile, _ := loadFixture(t, "12-2144-1434.webp")
	first, start, err := EncodeFrom(tile, nil)
	require.NoError(t, err)
	alone, err := Encode(tile, nil)
	require.NoError(t, err)
	assert.Equal(t, alone, first)

	// A flat tile has no model, so it hands the start on as it got it.
	flat := &terrarium.Tile{Width: 512, Height: 512, Samples: make([]int32, 512*512)}
	between, start, err := EncodeFrom(flat, start)
	require.NoError(t, err)
	second, _, err := EncodeFrom(tile, start)
	require.NoError(t, err)
	assert.Less(t, len(second), len(first))

	back, restart, err := DecodeFrom(first, nil)
	require.NoError(t, err)
	require.Equal(t, tile.Samples, back.Samples)
	back, restart, err = DecodeFrom(between, restart)
	require.NoError(t, err)
	require.Equal(t, flat.Samples, back.Samples)
	back, _, err = DecodeFrom(second, restart)
	require.NoError(t, err)
	require.Equal(t, tile.Samples, back.Samples)

	// Without the start, the second tile does not give its samples back.
	wrong, err := Decode(second, nil)
	if err == nil {
		assert.NotEqual(t, tile.Samples, wrong.Samples)
	}
	_, _, err = DecodeFrom(nil, nil)
	require.Error(t, err)
	_, _, err = EncodeFrom(&terrarium.Tile{Width: 2, Height: 2, Samples: []int32{1}}, nil)
	require.Error(t, err)
}

func TestAStartFromATileOfAnotherSizeIsNotUsed(t *testing.T) {
	_, children := family(64, 7, 8)
	_, start, err := EncodeFrom(children[0], nil)
	require.NoError(t, err)
	small := &terrarium.Tile{Width: 33, Height: 17, Samples: make([]int32, 33*17)}
	for i := range small.Samples {
		small.Samples[i] = int32(i*37%1000) << 7
	}
	packed, _, err := EncodeFrom(small, start)
	require.NoError(t, err)
	alone, err := Encode(small, nil)
	require.NoError(t, err)
	assert.Equal(t, alone, packed)
	_, restart, err := DecodeFrom(mustEncode(t, children[0]), nil)
	require.NoError(t, err)
	back, _, err := DecodeFrom(packed, restart)
	require.NoError(t, err)
	assert.Equal(t, small.Samples, back.Samples)
}

func mustEncode(t *testing.T, tile *terrarium.Tile) []byte {
	t.Helper()
	packed, err := Encode(tile, nil)
	require.NoError(t, err)
	return packed
}
