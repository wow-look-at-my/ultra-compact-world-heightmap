# ultra-compact-world-heightmap

A lossless codec for the [Mapterhorn](https://mapterhorn.com/) terrain tiles. It stores the exact samples of a Terrarium WebP tile in a fraction of the bytes.

A general image format has to work for any picture. This only has to work for the ground, as one pipeline renders it:

- Heights are smooth, so a predictor that learns the local shape leaves a small error.
- Heights are rounded to a step that depends on the zoom, so the low bits carry nothing.
- Every tile above the deepest zoom is the rounded mean of the tiles below it. A tile can therefore be coded against its parent, or left out and rebuilt from its children.

## Results

See [docs/results.md](docs/results.md) for the numbers, how to reproduce them, and what was tried and dropped.

## Use

```bash
uch encode tile.webp tile.uch
```

```bash
uch decode tile.uch tile.png
```

Code a tile against the tile one zoom above it. Decoding then needs the same parent:

```bash
uch encode --parent 11-1072-717.webp 12-2144-1434.webp 12-2144-1434.uch
```

```bash
uch decode --parent 11-1072-717.webp 12-2144-1434.uch 12-2144-1434.png
```

Measure a directory of tiles named `z-x-y.webp`. Every tile is decoded again and compared before its size counts:

```bash
uch bench tiles/
```

## Build

```bash
go-toolchain
```

The binary lands in `build/uch`.

## More

- [docs/format.md](docs/format.md) describes the format.
- [AGENTS.md](AGENTS.md) describes the code.

The test tiles in `testdata/tiles/` are © [Mapterhorn](https://mapterhorn.com/attribution) and its data sources.
