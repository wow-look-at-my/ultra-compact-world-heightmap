package terrarium

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeWebPReadsARealTile(t *testing.T) {
	data, err := os.ReadFile("../../testdata/tiles/12-2144-1434.webp")
	require.NoError(t, err)
	tile, err := DecodeWebP(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, 512, tile.Width)
	assert.Equal(t, 512, tile.Height)
	require.Len(t, tile.Samples, 512*512)
	lo, hi := tile.Samples[0], tile.Samples[0]
	for _, s := range tile.Samples {
		lo, hi = min(lo, s), max(hi, s)
	}
	// The tile holds the Bernese Oberland: valley floors to four-thousanders.
	assert.Greater(t, Metres(lo), 400.0)
	assert.Less(t, Metres(lo), 1000.0)
	assert.Greater(t, Metres(hi), 3500.0)
	assert.Less(t, Metres(hi), 4500.0)
}

func TestDecodeWebPRejectsWhatIsNotATile(t *testing.T) {
	_, err := DecodeWebP(bytes.NewReader([]byte("not a webp")))
	require.Error(t, err)
}

func TestMetres(t *testing.T) {
	assert.InDelta(t, 0.0, Metres(32768*256), 0)
	assert.InDelta(t, 0.5, Metres(32768*256+128), 0)
}
