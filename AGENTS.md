# delta — Notes for AI Agents

- **The "original" baseline is captured at construction *and* at unmarshal, and moves at a save only when the caller says so.** `NewString(v)` sets `original = current = v`, and `UnmarshalJSON`/`UnmarshalBSONValue` reset `original` to the freshly-decoded value. So a record loaded from the database reads as `IsChanged() == false` until application code calls `Set`. After a save, call `Reset()` to make the saved value the baseline. Nothing calls it for you, so a record that is saved twice without a `Reset` still compares against the value it was loaded with.

- **Unmarshal sets `original` and `current` together, and only on success.** The BSON decoders populate `original` first and mirror it into `current`. The JSON decoders decode into a local zero value and assign both fields afterwards, for two reasons: a JSON `null` then becomes the zero value for the type, and a decode that fails part-way cannot leave a corrupted baseline behind (the driver's `primitive.ObjectID.UnmarshalJSON` writes the valid prefix of a hex string before rejecting the rest). If you add a new wrapped type, follow the same pattern or change-tracking will be wrong from the first load. The `Fuzz*_Unmarshal*` targets check it.

- **A failed unmarshal must change neither value, so `ObjectID.UnmarshalJSON` decodes into a copy.** mongo-driver's `primitive.ObjectID.UnmarshalJSON` hex-decodes straight into the ID and stops at the first bad character, so decoding into `original` in place left a failed decode with a baseline half old bytes and half new. `Bool` and `String` cannot be partially written, so they still decode in place. Every type's tests and fuzz targets pin the rule; keep it for any new type.

- **A JSON `null` discards a pending edit without an error.** Decoding `null` leaves `original` alone, and the copy into `current` then throws away whatever `Set` wrote. The tests pin this for all three types.

- **mongo-driver copies any 12-byte JSON input straight into an ObjectID.** Its `UnmarshalJSON` checks `len == 12` before reading the input as JSON, so a 10-character JSON string like `"abcdefghij"` decodes to its own raw bytes, quotes included, rather than failing. `TestObjectID_JSON` pins it; it is the driver's behavior, not this package's.

- **`String.SetValue` formats numbers through rosetta's `convert.String`.** A whole-number float is written without decimals (`150`, since rosetta v0.46.0), and any other float is rounded to two (`33.33`). `TestString_SetValue` pins both, so a rosetta change fails here first.

- **`Original()` reads the baseline, and it is the *stored* value, not the previous one.** It answers "what does the database still hold", which is what an update needs to address the old row or clean up whatever the old value pointed at. Because the baseline only moves at construction and unmarshal (above), calling `Set` twice does not make `Original()` return the intermediate value — it still returns what was loaded.

- **`ObjectID` JSON is a hex string and nothing else.** It encodes as 24 lowercase hex characters, and decodes only from 24 hex characters, an empty string, or `null` (the last two become the nil ObjectID). It does not delegate to the driver's decoder, which also accepts extended JSON (`{"$oid": …}`) and copies any 12-byte input verbatim; both are errors here.

- **`ObjectID.SetValue` treats an empty input as "clear".** `nil`, `""`, an empty byte slice, and a nil `*primitive.ObjectID` all set the nil ObjectID. A value with no string form at all (a struct, a `*string`) is still an error, so that an unconvertible input is not mistaken for an empty one.

- **`Original()` reads the baseline, and it is the *stored* value, not the previous one.** It answers "what does the database still hold", which is what an update needs to address the old row or clean up whatever the old value pointed at. Because the baseline only moves at construction, at unmarshal, and at `Reset` (above), calling `Set` twice does not make `Original()` return the intermediate value — it still returns what was loaded.

- **Mutating methods take a pointer receiver; readers take a value receiver.** `Set`/`Clear`/`Reset`/`SetValue`/`Pointer` are pointer methods; `Value`/`IsChanged`/`NotChanged`/`IsZero`/`GetValue` are value methods. Store these wrappers as addressable fields, not in a map, or `Set` won't stick.

## `Slice` is a baseline and a current list, and its change lists are computed

- **`Slice[T]` holds `current` and `original`, exactly as the scalars do.** `Added()`, `Deleted()`, and `Unchanged()` compare the two on every call. Nothing is recorded as the list is edited, so the lists always describe the difference from what was loaded, however many writes came between. rosetta's `delta.Slice`, which this replaces, kept `Added` and `Deleted` as fields and rebuilt them against the previous write.
- **A reorder is a change, with nothing added or deleted.** `IsChanged` compares the two lists in order, because the stored value differs. The change lists count duplicates one pair at a time, so `[a]` to `[a, a]` adds one `a`.
- **No method hands out or keeps the caller's memory.** `NewSlice`, `Set`, `SetValue`, and both decoders copy what they are given, and `Value`, `Original`, `GetValue`, and the change lists return copies that are never nil. `Pointer` is the one deliberate exception, for a caller that edits in place. `SetIndex` writes to a fresh copy for the same reason: a struct that holds a Slice and is copied by assignment (Emissary's `Stream.CopyFrom`) shares the list with its source.
- **An empty Slice encodes as `[]`, never `null`,** in JSON and BSON, including the zero value. With `bson:",omitempty"` the driver calls `IsZero` and leaves an empty Slice out of the document.
- **`SetValue` converts as loosely as rosetta's `convert` allows, and refuses only a value that is not a list.** The check runs before any state is written, so a refused write leaves the Slice untouched. Form widgets hand it a `*sliceof.String`, never a `[]T`; do not replace the conversion with a type assertion.
- **`SetIndex` grows the list to any index, by design.** The schema's `maxLength` is the limit (Emissary BUG-116); do not add one here.
- **`T` must be comparable at run time, not only at compile time.** The compiler accepts `Slice[any]`, but comparing two interface values that hold slices or maps panics. Use a concrete element type.

## `Slice` reads a legacy BSON document, temporarily

rosetta's `delta.Slice` had no BSON methods, so the driver stored it as `{values: [...]}`. `Slice.UnmarshalBSONValue` reads a BSON array, a null, or that document, and `MarshalBSONValue` always writes an array, so a legacy record heals when it is next saved.

- **The legacy document is `values` and nothing else.** Any other key, a second key, or a `values` that is not an array or null is an error. `decodeBSONLegacy` is the whole of it.
- **Both decoders check the length header and the null terminator themselves before the driver sees the bytes.** mongo-driver v1 panics on a document whose header disagrees with its data, and its `Elements()` accepts a document with no terminator, which the driver's own struct decoder refuses. `isSizedBSON` is that check; do not remove it. `FuzzSlice_UnmarshalBSONValue` and `FuzzSlice_UnmarshalBSONDocument` found both, and their inputs are kept in `testdata/fuzz`.
- **Remove the legacy branch when Emissary's plan says so.** The condition and the steps are in `emissary-specs/projects/DELTA-CONSOLIDATION.md`. Removing it means deleting `decodeBSONLegacy`, `legacySliceKey`, and the `TypeEmbeddedDocument` case, and turning the legacy tests into refusals.

## Every wrapped type must keep satisfying the BSON `*Value` interfaces

`Bool`, `ObjectID`, `Slice`, and `String` serialize through `MarshalBSONValue`/`UnmarshalBSONValue`, and none of them has a plain `MarshalBSON` to fall back on. Go satisfies those interfaces *structurally*, so a signature that drifts out of step with the driver's does not fail to compile at the method itself — the driver silently stops recognizing the type and encodes it with the default struct codec instead. Because every field here is unexported, that fallback writes `{}`: a stored `true` becomes an empty document, with no error on either side.

Each `*_test.go` therefore carries `var _ bson.ValueMarshaler = …` assertions, which turn that silent change into a build failure. They matter most for the mongo-driver v2 upgrade (BUG-122), where the interface signature itself changes from `(bsontype.Type, []byte, error)` to `(byte, []byte, error)`. Do not delete them, and add the same pair alongside any new wrapped type.
