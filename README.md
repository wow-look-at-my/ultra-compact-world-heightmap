# ultra-compact-world-heightmap

A lossless codec for the [Mapterhorn](https://mapterhorn.com/) terrain tiles. It stores the exact samples of a Terrarium WebP tile in a fraction of the bytes.

A general image format has to work for any picture. This only has to work for the ground, as one pipeline renders it:

- Heights are smooth, so a predictor that learns the local shape leaves a small error.
- Heights are rounded to a step that depends on the zoom, so the low bits carry nothing.
- Every tile above the deepest zoom is the rounded mean of the tiles below it. A tile can therefore be coded against its parent, or left out and rebuilt from its children.

## Results

Complete pyramids of served tiles, every tile decoded again and compared sample by sample:

```
                               Everest, zoom 9 to 12    Interlaken, zoom 14 to 17
                               global 30 m data         0.5 m data
webp as served                 11230100  100.0%          9108348  100.0%
each tile alone                 5327486   47.4%          5124154   56.3%
each tile against its parent    3879085   34.5%          4164383   45.7%
leaves only, parents rebuilt    3295474   29.4%          3677224   40.4%
```

[docs/results.md](docs/results.md) has the tables for each zoom and how to reproduce them. [docs/tried.md](docs/tried.md) lists what worked and what did not.

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

Code a tile as its difference from a tile that is almost equal to it, such as an older version. Decoding needs the same reference:

```bash
uch encode --reference old.webp new.webp new.uch
```

```bash
uch decode --reference old.webp new.uch new.png
```

Pack every tile of a pyramid into one file. The default stores only the tiles that their children do not rebuild exactly:

```bash
uch pack tiles/ tiles.ucha
```

```bash
uch unpack tiles.ucha restored/
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
