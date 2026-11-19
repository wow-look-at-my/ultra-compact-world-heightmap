# The tile format

A compressed tile is a short header and one arithmetic-coded stream. The decoder rebuilds the model that the encoder used, sample by sample, so the stream holds nothing but the decisions the model can not predict.

## Header

```
byte 0     bits 0-4  shift: the low bits that are zero in every sample
           bit 5     the tile is 512 by 512
           bit 6     every sample is the same
           bit 7     the tile is coded against its parent
                     bits 6 and 7 together: coded against a reference, see below
byte 1     only with bit 7: the shift that the parent was rounded to
varint     only without bit 5: width
varint     only without bit 5: height
zigzag     the first sample, in units of 2^shift, measured from sea level
stream     only without bit 6
```

A sample is the Terrarium value `R<<16 | G<<8 | B`. The codec works on `sample >> shift`.

A tile of one value is the header alone.

## The stream

Samples go in row order. For each the model predicts a value. The stream holds the error.

```
at sea level?    one decision, only where a neighbour is at sea level
zero?            one decision
sign             one decision
(|e|-1) >> k     unary, one decision for each step
low k bits       one decision for each bit
```

`k` comes from the errors of the neighbours already coded. Each decision has a probability, and a binary arithmetic coder writes it at close to its ideal cost.

Sea level is the height that fills whole regions. Next to water, the stream first says whether the sample is water too. An island in open water then costs a few decisions, and not a large error on every side of it.

### Prediction

Several predictors run side by side:

- fixed ones, such as the plane through multiple neighbours
- least-squares fits that learn their weights from the samples already coded

A first answer is their weighted mean. A predictor that was right on the neighbours of the sample weighs more.

A second least-squares fit then reads every prediction and corrects the mean.

### Probability

Tables of adaptive probabilities feed each decision. Each table reads a different context:

- the size of the errors nearby
- where the prediction falls between a pair of whole values
- the errors of the neighbours to the left and above
- how far predictors disagree
- which neighbours are equal, and whether the prediction is even or odd
- the local slope

A mixer joins them with weights that it learns for each context. A last stage maps the mixed probability to the frequency that was seen at that probability.

## A tile coded against its parent

The Mapterhorn pipeline builds every tile above the deepest zoom from the tiles below it. Each parent sample is the mean of a block of multiple samples, rounded half to even to the quantum of the parent zoom. So a parent tile bounds the sum of every block in the tile below it:

```
step = 4 << (parentShift - shift)
q    = parentSample >> parentShift
lo   = q*step - step/2 + (q odd ? 1 : 0)
hi   = q*step + step/2 - (q odd ? 1 : 0)
lo <= sum of the block <= hi
```

The encoder checks this bound on every block before it uses the mode. A tile that fails it is coded alone.

The model then changes in multiple ways:

- **The last sample of a block has a known range.** samples and the bound leave only a few values. The stream spends nothing on a decision that the range settles.
- **The parent is known on every side.** A sample is predicted from the parent samples below and to the right of it too.
- **Each place in a block has its own predictors and contexts.**
- **The model rehearses before it starts.** Terrain looks alike from one zoom to the next. So the model first codes the parent against a parent of the parent, which it builds with the same rule, and writes nothing. It then starts the tile with what it learned. The decoder holds the same parent, and rehearses to the same state.

Decoding such a tile needs the parent, and so the parent of the parent, up to a tile that was coded alone.

## A tile coded against a reference

A reference is a tile of the same size whose samples are almost all equal to the samples of the tile. An older version of the tile is one. The output of the pipeline, run again on the source data, is another.

```
byte 0     bits 0-4  shift
           bit 5     the tile is 512 by 512
           bits 6-7  both set
varint     only without bit 5: width
varint     only without bit 5: height
stream     one difference for each sample, in row order
```

The difference is `(sample >> shift) - ((reference + half a step) >> shift)`.

```
changed?     one decision, with the differences of the neighbours as its context
sign         one decision
size - 1     a bit count, then the bits
```

A sample that equals its reference costs a small part of a bit. The model does no prediction. This mode is fast. It is large when the tiles differ in many places.

## An archive of one pyramid

`uch pack` joins the tiles of a pyramid into one file.

```
"UCHA" 0x01
byte       mode: 0 chain, 1 leaves
varint     count of stored tiles
for each   varint zoom, varint column, varint row, varint length, the compressed tile
varint     count of tiles to rebuild
for each   varint zoom, varint column, varint row
```

The stored tiles are in order of zoom. A parent comes before its children.

- **chain** stores every tile, each against its parent where the parent is in the set.
- **leaves** stores a tile alone, and only when its children do not rebuild it exactly. The other tiles are listed, and rebuilt from the deepest zoom up.

## What the format does not hold

- **No end marker and no checksum.** The encoder drops the zero bytes at the end of the stream. A truncated tile decodes to incorrect samples without an error.
- **No tile address.** The container names the tile and its parent.
