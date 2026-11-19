package codec

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/pyramid"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

// golden holds the SHA-256 of each stream, as an amd64 machine wrote it.
var golden = map[string]string{
	"alone":            "",
	"sea":              "",
	"against a parent": "",
	"after a tile":     "",
	"against heights":  "",
}

func TestEveryMachineWritesTheSameBytes(t *testing.T) {
	tile, _ := loadFixture(t, "12-2144-1434.webp")
	sea, _ := loadFixture(t, "2-0-2.webp")
	streams := map[string][]byte{}
	var err error
	streams["alone"], err = Encode(tile, nil)
	require.NoError(t, err)
	streams["sea"], err = Encode(sea, nil)
	require.NoError(t, err)

	parent, err := pyramid.Parent([4]*terrarium.Tile{tile}, 11)
	require.NoError(t, err)
	streams["against a parent"], err = Encode(tile, &Parent{Tile: parent, Quadrant: 0})
	require.NoError(t, err)
	require.True(t, NeedsParent(streams["against a parent"]))

	_, start, err := EncodeFrom(tile, nil)
	require.NoError(t, err)
	streams["after a tile"], _, err = EncodeFrom(tile, start)
	require.NoError(t, err)

	metres := make([]float32, len(tile.Samples))
	for i, v := range tile.Samples {
		metres[i] = float32(float64(v)/256-32768) + float32(i%97-48)/200
	}
	streams["against heights"], err = EncodeReference(tile, ReferenceHeights(metres, tile.Width, tile.Height))
	require.NoError(t, err)

	for name, want := range golden {
		sum := sha256.Sum256(streams[name])
		assert.Equal(t, want, hex.EncodeToString(sum[:]), name)
	}
}
