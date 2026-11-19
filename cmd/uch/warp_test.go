package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWarpWritesTheHeightsOfATile(t *testing.T) {
	t.Serial("the commands share one root command and its flags")
	dir := t.TempDir()
	values := make([]float32, 64*64)
	for i := range values {
		values[i] = 250
	}
	grid, out := filepath.Join(dir, "grid.f32"), filepath.Join(dir, "tile.f32")
	require.NoError(t, os.WriteFile(grid, heights(values...), 0o644))
	place := []string{"--x", "2161", "--y", "1766", "--columns", "64", "--west", "9.92", "--north", "24.06", "--step-lon", "0.002", "--step-lat", "0.002"}
	_, err := run(t, append(append([]string{"warp"}, place...), grid, out)...)
	require.NoError(t, err)
	raw, err := os.ReadFile(out)
	require.NoError(t, err)
	tile, err := parseHeights(raw, 512, 512)
	require.NoError(t, err)
	for _, v := range tile {
		require.InDelta(t, 250, v, 1e-3)
	}

	// The heights are a reference that a tile of the same ground decodes against.
	rounded, packed, restored := filepath.Join(dir, "tile.png"), filepath.Join(dir, "tile.uch"), filepath.Join(dir, "back.png")
	_, err = run(t, "round", "--zoom", "12", out, rounded)
	require.NoError(t, err)
	_, err = run(t, "encode", "--reference", out, rounded, packed)
	require.NoError(t, err)
	info, err := os.Stat(packed)
	require.NoError(t, err)
	assert.Less(t, info.Size(), int64(8))
	_, err = run(t, "decode", "--reference", out, packed, restored)
	require.NoError(t, err)

	_, err = run(t, append(append([]string{"warp"}, place...), filepath.Join(dir, "missing.f32"), out)...)
	require.Error(t, err)
	_, err = run(t, "warp", "--x", "2161", "--y", "1766", "--columns", "7", grid, out)
	require.ErrorContains(t, err, "rows of 7")
	_, err = run(t, "warp", "--x", "9", "--y", "9", "--columns", "64", grid, out)
	require.ErrorContains(t, err, "does not reach")
}
