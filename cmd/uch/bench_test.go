package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBenchReportsEachZoomAndATotal(t *testing.T) {
	out, err := run(t, "bench", "--per-tile", "../../testdata/tiles")
	require.NoError(t, err)
	assert.Contains(t, out, "2-0-2.webp")
	assert.Contains(t, out, "z2 ")
	assert.Contains(t, out, "z12 ")
	assert.Contains(t, out, "total     2 tiles")
}

func TestBenchFailsWithoutTiles(t *testing.T) {
	_, err := run(t, "bench", t.TempDir())
	require.Error(t, err)
	_, err = run(t, "bench", "no-such-directory")
	require.Error(t, err)
}

func TestTileZoom(t *testing.T) {
	assert.Equal(t, 12, tileZoom("a/12-3-4.webp"))
	assert.Equal(t, -1, tileZoom("a/tile.webp"))
}
