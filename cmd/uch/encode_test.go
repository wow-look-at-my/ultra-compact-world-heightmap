package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/pyramid"
	"github.com/wow-look-at-my/ultra-compact-world-heightmap/internal/terrarium"
)

const fixture = "../../testdata/tiles/12-2144-1434.webp"

func TestEncodeAndDecodeGiveTheSamplesBack(t *testing.T) {
	t.Serial()
	dir := t.TempDir()
	packed, restored := filepath.Join(dir, "tile.uch"), filepath.Join(dir, "tile.png")
	out, err := run(t, "encode", fixture, packed)
	require.NoError(t, err)
	assert.Contains(t, out, "decodes alone")
	_, err = run(t, "decode", packed, restored)
	require.NoError(t, err)
	want, _, err := loadTile(fixture)
	require.NoError(t, err)
	got, _, err := loadTile(restored)
	require.NoError(t, err)
	assert.Equal(t, want.Samples, got.Samples)
}

// writeParent builds the tile above the fixture, with sea where the fixture has no sibling.
func writeParent(t *testing.T, dir string) string {
	t.Helper()
	child, _, err := loadTile(fixture)
	require.NoError(t, err)
	parent, err := pyramid.Parent([4]*terrarium.Tile{child}, 11)
	require.NoError(t, err)
	path := filepath.Join(dir, "11-1072-717.png")
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, terrarium.EncodePNG(f, parent))
	require.NoError(t, f.Close())
	return path
}

func TestEncodeAgainstAParentIsSmallerAndDecodesOnlyWithIt(t *testing.T) {
	t.Serial()
	dir := t.TempDir()
	parent := writeParent(t, dir)
	alone, packed, restored := filepath.Join(dir, "alone.uch"), filepath.Join(dir, "12-2144-1434.uch"), filepath.Join(dir, "tile.png")
	_, err := run(t, "encode", fixture, alone)
	require.NoError(t, err)
	out, err := run(t, "encode", "--parent", parent, fixture, packed)
	require.NoError(t, err)
	assert.Contains(t, out, "decodes with its parent")
	aloneInfo, err := os.Stat(alone)
	require.NoError(t, err)
	packedInfo, err := os.Stat(packed)
	require.NoError(t, err)
	assert.Less(t, packedInfo.Size(), aloneInfo.Size())

	_, err = run(t, "decode", packed, restored)
	require.ErrorContains(t, err, "--parent")
	_, err = run(t, "decode", "--parent", parent, packed, restored)
	require.NoError(t, err)
	want, _, err := loadTile(fixture)
	require.NoError(t, err)
	got, _, err := loadTile(restored)
	require.NoError(t, err)
	assert.Equal(t, want.Samples, got.Samples)
}

func TestTheQuadrantComesFromTheNamesOrTheFlag(t *testing.T) {
	t.Serial()
	dir := t.TempDir()
	parent := writeParent(t, dir)
	unnamed := filepath.Join(dir, "parent.png")
	data, err := os.ReadFile(parent)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(unnamed, data, 0o644))
	packed := filepath.Join(dir, "tile.uch")
	_, err = run(t, "encode", "--parent", unnamed, fixture, packed)
	require.ErrorContains(t, err, "--quadrant")
	_, err = run(t, "encode", "--parent", unnamed, "--quadrant", "7", fixture, packed)
	require.Error(t, err)
	out, err := run(t, "encode", "--parent", unnamed, "--quadrant", "0", fixture, packed)
	require.NoError(t, err)
	assert.Contains(t, out, "decodes with its parent")
	_, err = run(t, "encode", "--parent", filepath.Join(dir, "missing.png"), "--quadrant", "0", fixture, packed)
	require.Error(t, err)
}

func TestEncodeAndDecodeFailOnWhatTheyCannotRead(t *testing.T) {
	t.Serial()
	dir := t.TempDir()
	_, err := run(t, "encode", "no-such-tile.webp", filepath.Join(dir, "out.uch"))
	require.Error(t, err)
	_, err = run(t, "decode", "no-such-tile.uch", filepath.Join(dir, "out.png"))
	require.Error(t, err)
	bad := filepath.Join(dir, "bad.uch")
	require.NoError(t, os.WriteFile(bad, []byte{0}, 0o644))
	_, err = run(t, "decode", bad, filepath.Join(dir, "out.png"))
	require.Error(t, err)
}
