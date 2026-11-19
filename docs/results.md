# Results

Every number here is a real tile from `https://tiles.mapterhorn.com`, compressed, decompressed again, and compared sample by sample with the WebP it came from. A tile that does not match stops the run, so no size below belongs to a lossy result.

`webp` is the size of the tile as Mapterhorn serves it, which is a lossless WebP.

## How to reproduce

```bash
corpus/fetch.sh tiles
```

```bash
uch bench tiles/test
```

`corpus/` holds tile lists. The codec was tuned on `dev` only. `test` is a held-out set of other places. The `pyramid` lists are complete: every tile under one root, down to the deepest zoom.

## A whole pyramid

This is the result that matters for storing the data set. A pyramid has one tile at its root, and a few times as many at each zoom below.

Ways to store it:

- **alone**: every tile decodes without any other.
- **against a parent**: every tile but the root is coded against the tile above it. Reading a tile means decoding the few tiles above it first.
- **leaves only**: a tile that its children rebuild exactly is not stored. `uch bench` rebuilds each one and compares it before it counts. Reading a tile near the top means decoding every tile under it.

```
Everest, zoom 9 to 12, 85 tiles, global 30 m data

  webp as served               11230100
  each tile alone               5327486   47.44%
  each tile against a parent    3879085   34.54%
  leaves only                   3295474   29.35%    21 tiles rebuilt, all exact

Interlaken, zoom 14 to 17, 85 tiles, 0.5 m data

  webp as served                9108348
  each tile alone               5124154   56.26%
  each tile against a parent    4164383   45.72%
  leaves only                   3677224   40.37%    21 tiles rebuilt, all exact
```

`uch pack` writes such an archive, and `uch unpack` restores every tile from it.

## Tile by tile, on places the codec was not tuned on

```
test set, 187 tiles

zoom  tiles      webp     alone  of webp | with parent     child  of webp  of alone
  1       2    212686    172617   81.16% |
  2       8    830564    677497   81.57% |      4         263556   62.35%    75.58%
  3      31   2422496   1953928   80.66% |     15         932642   61.01%    75.11%
  4      12   1695762   1435099   84.63% |      5         430277   64.73%    75.54%
  5      13   2066376   1738476   84.13% |     13        1307501   63.28%    75.21%
  6      13   2215214   1826622   82.46% |     13        1368710   61.79%    74.93%
  7      14   2346164   1898408   80.92% |     14        1418987   60.48%    74.75%
  8      14   2120130   1644732   77.58% |     14        1221237   57.60%    74.25%
  9      14   1962156   1473990   75.12% |     14        1084115   55.25%    73.55%
 10      14   1583558   1127901   71.23% |     14         811656   51.26%    71.96%
 11      14   1124518    747962   66.51% |     14         529703   47.10%    70.82%
 12      14   1221180    636506   52.12% |     14         515213   42.19%    80.94%
 13       6    696532    413759   59.40% |      6         346040   49.68%    83.63%
 14       5    569214    352490   61.93% |      5         299950   52.70%    85.09%
 15       5    558532    325940   58.36% |      5         273534   48.97%    83.92%
 16       5    463056    225798   48.76% |      5         187139   40.41%    82.88%
 17       3    196356    104140   53.04% |      3          87862   44.75%    84.37%
all     187  22284494  16755865   75.19% |    158       11078122   56.12%    75.48%
```

The `with parent` columns count only the tiles whose parent is in the set. All of those followed the parent rule, so none fell back to a tile that decodes alone.

The `dev` set, which the codec was tuned on, gives the same picture: 75.18% alone and 56.72% against a parent.

## What the numbers say

- **Deep zooms compress best.** They are smooth, so prediction leaves little. They are also almost all of the data, because each zoom has a few times the tiles of the above.
- **Shallow zooms are rough.** One sample covers kilometres, so neighbours differ by tens of metres and little is predictable. Alone, the gain over WebP is small. Against a parent it is larger, because the parent fixes the sum of every block of multiple samples.
- **The parent rule is exact.** In every family checked, the served parent is the rounded mean of its served children, sample for sample.
- **An all-water tile with a few islands is smaller than the WebP too.** It was larger until the stream asked first whether a sample is at sea level.

```
testdata/tiles/2-0-2.webp    webp 566    alone 358
```

## Cost

```
one tile of 512 by 512 samples, one core, measured with the uch command

encode alone              2.35 s
decode alone              2.40 s
encode against a parent   2.28 s
decode against a parent   2.27 s
```

Decoding costs the same as encoding, because the decoder rebuilds the model that the encoder used. This codec trades time for size on purpose.
