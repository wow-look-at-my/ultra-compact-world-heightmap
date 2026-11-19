package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

func TestBenchReportsEachZoomAndATotal(t *testing.T) {
	t.Serial("the commands share one root command and its flags")
	out, err := run(t, "bench", "--per-tile", "../../testdata/tiles")
	require.NoError(t, err)
	assert.Contains(t, out, "2-0-2.webp")
	assert.Contains(t, out, "z2 ")
	assert.Contains(t, out, "z12 ")
	assert.Contains(t, out, "total     2 tiles")
}

func writePNG(t *testing.T, path string, tile *terrarium.Tile) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, terrarium.EncodePNG(f, tile))
	require.NoError(t, f.Close())
}

func TestBenchCountsATileThatItsChildrenRebuild(t *testing.T) {
	t.Serial("the commands share one root command and its flags")
	dir := t.TempDir()
	writeFamily(t, dir)
	out, err := run(t, "bench", dir)
	require.NoError(t, err)
	assert.Contains(t, out, "4 with parent")
	assert.Contains(t, out, "without the 1 tiles that their children rebuild exactly")

	out, err = run(t, "bench", "--mode", "alone", dir)
	require.NoError(t, err)
	assert.NotContains(t, out, "with parent")
}

func TestBenchFailsWithoutTiles(t *testing.T) {
	t.Serial("the commands share one root command and its flags")
	_, err := run(t, "bench", t.TempDir())
	require.Error(t, err)
	_, err = run(t, "bench", "no-such-directory")
	require.Error(t, err)
}

func TestTileZoom(t *testing.T) {
	assert.Equal(t, 12, tileZoom("a/12-3-4.webp"))
	assert.Equal(t, -1, tileZoom("a/tile.webp"))
}
