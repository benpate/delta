# delta — Notes for AI Agents

See [README.md](README.md) for what delta is. One file per type (`bool.go`, `string.go`, `objectId.go`, `slice.go`), each with a `_test.go` beside it. These are the rules that the code does not show.

## The baseline

- **`original` moves at construction, at a successful unmarshal, and at `Reset`. Nowhere else.** A record loaded from the database reads as `IsChanged() == false` until application code writes to it. Nothing calls `Reset` for you, so a record saved twice without one still compares against the value it was loaded with.
- **`Original()` is the stored value, not the previous one.** It answers "what does the database still hold", which an update needs to address the old row or clean up whatever the old value pointed at. Calling `Set` twice never makes it return the intermediate value.
- **`Set`, `Clear`, `SetValue`, and `SetIndex` write `current` only.** `Clear` is a change like any other, unless the baseline was already the zero value.
- **Mutating methods take a pointer receiver; readers take a value receiver.** Store a wrapper as an addressable struct field, not as a map value, or `Set` will not stick.

## Unmarshalling

- **Unmarshal sets `original` and `current` together, and only on success.** A failed decode must change neither. Every type's tests and `Fuzz*_Unmarshal*` targets check it; keep the rule for any new type.
- **Every JSON decoder, and both `Slice` decoders, decode into a local value and assign afterwards.** Decoding in place is unsafe wherever a decoder can write part of a value before it fails. The scalar BSON decoders still decode into `original` in place and then copy to `current`; their tests show that a failure leaves `original` alone.
- **A JSON `null` becomes the zero value for both fields, and discards a pending edit without an error.** That is `false`, `""`, the nil ObjectID, or the empty list. Decoding into a local zero value is what makes this so; decoding in place would leave the old value behind.

## `ObjectID`

- **`ObjectID` JSON is a hex string and nothing else.** It decodes only from 24 hex characters, an empty string, or `null` (the last two become the nil ObjectID). It does not use the driver's decoder, which also accepts extended JSON (`{"$oid": …}`) and copies any 12-byte input verbatim; both are errors here.
- **The BSON path does use the driver's decoder, and it is looser.** It accepts an ObjectID, a null, a hex string in either case, and a 12-character string, which it reads as raw bytes. `TestObjectID_UnmarshalBSONValue` records what is accepted.
- **`ObjectID.SetValue` treats an empty input as "clear".** `nil`, `""`, an empty byte slice, and a nil `*primitive.ObjectID` all set the nil ObjectID. A value with no string form (a struct, a `*string`) is an error, so that an unconvertible input is not mistaken for an empty one.

## `String` and `Bool`

- **`String.SetValue` formats numbers through rosetta's `convert.String`.** A whole-number float is written without decimals (`150`, since rosetta v0.46.0), and any other float is rounded to two (`33.33`). `TestString_SetValue` checks both, so a rosetta change fails here first.
- **`Bool.SetValue` and `String.SetValue` never return an error.** They convert whatever they are given, so a value that cannot be converted becomes the zero value.

## `Slice`

- **`Slice[T]` holds `current` and `original`, as the scalars do, and computes its change lists on every call.** Nothing is recorded as the list is edited, so `Added()`, `Deleted()`, and `Unchanged()` always describe the difference from the baseline, however many writes came between.
- **A reorder is a change, with nothing added or deleted.** `IsChanged` compares the two lists in order, because the stored value differs. The change lists count duplicates one pair at a time, so `[a]` to `[a, a]` adds one `a`.
- **No method hands out or keeps the caller's memory.** `NewSlice`, `Set`, `SetValue`, and both decoders copy what they are given. `Value`, `Original`, `GetValue`, and the change lists return copies that are never nil. `Pointer` is the one deliberate exception, for a caller that edits in place.
- **`SetIndex` writes to a fresh copy.** A struct that holds a Slice and is copied by assignment (Emissary's `Stream.CopyFrom`) shares the list with its source, so a write in place would change both.
- **An empty Slice encodes as `[]`, never `null`,** in JSON and BSON, including the zero value. With `bson:",omitempty"` the driver calls `IsZero` and leaves an empty Slice out of the document.
- **`SetValue` accepts whatever rosetta's `convert.SliceOfAnyOk` reads as a list, and that includes a bare scalar.** A `string`, `int`, or `bool` becomes a list of one. Form widgets hand it a `*sliceof.String`, never a `[]T`, so do not replace the conversion with a type assertion. The check runs before any state is written, so a refused value leaves the Slice untouched.
- **`SetValue` keeps a lossy conversion.** An item that cannot become a `T` is stored as the zero value, not refused. `TestSlice_SetValue`, `TestSlice_SetValueInt`, and `TestSlice_SetValueRefused` record the whole surface, including rosetta's oddities (an `int32` is refused, and `"4,5"` splits into two ints).
- **`SetIndex` grows the list to any index, by design.** The schema's `maxLength` is the limit (Emissary BUG-116); do not add one here.
- **`T` must be comparable at run time, not only at compile time.** The compiler accepts `Slice[any]`, but comparing two interface values that hold slices or maps panics. Use a concrete element type.

## `Slice` reads a legacy BSON document, temporarily

rosetta's `delta.Slice` had no BSON methods, so the driver stored it as `{values: [...]}`. `Slice.UnmarshalBSONValue` reads a BSON array, a null, or that document, and `MarshalBSONValue` always writes an array, so a legacy record heals when it is next saved.

- **The legacy document is `values` and nothing else.** Any other key, a second key, or a `values` that is not an array or null is an error. `decodeBSONLegacy` is the whole of it.
- **`isSizedBSON` checks the length header and the null terminator before the driver sees the bytes. Do not remove it.** mongo-driver v1 panics on a document whose header disagrees with its data, and its `Elements()` accepts a document with no terminator. `FuzzSlice_UnmarshalBSONValue` and `FuzzSlice_UnmarshalBSONDocument` found both, and their inputs are kept in `testdata/fuzz`.
- **Remove the legacy branch when Emissary's plan says so.** The condition and the steps are in `emissary-specs/projects/DELTA-CONSOLIDATION.md`. Removing it means deleting `decodeBSONLegacy`, `legacySliceKey`, and the `TypeEmbeddedDocument` case, and turning the legacy tests into refusals.

## Every type must keep satisfying the BSON `*Value` interfaces

- **A method signature that drifts from the driver's fails silently.** `Bool`, `ObjectID`, `Slice`, and `String` serialize through `MarshalBSONValue`/`UnmarshalBSONValue`, with no `MarshalBSON` to fall back on. Go satisfies those interfaces structurally, so a mismatch still compiles, and the driver encodes the type with its default struct codec. Every field is unexported, so that writes `{}` with no error on either side.
- **The `var _ bson.ValueMarshaler = …` assertions in each `_test.go` turn that into a build failure. Do not delete them.** They matter most for the mongo-driver v2 upgrade (BUG-122), where the signature changes from `(bsontype.Type, []byte, error)` to `(byte, []byte, error)`.

## Adding a type

- **Copy the shape of an existing type.** That means `current` and `original` fields, the same method set and receiver kinds, decoders that follow the Unmarshalling rules above, and the interface assertions in the test file.
- **Update the type lists in `doc.go`, `README.md`, and this file in the same change.**

## Tests and gates

- **The package is at 100% statement coverage, and each type has `Fuzz*` targets with seeds in `testdata/fuzz`.** A seed file there is a regression test for a defect the fuzzer found; do not delete one.
- **Nilaway runs over the test files in CI.** It cannot see that `slices.Clone` or `cloneSlice` returned a non-empty list, so assert the length (`require.Len`, `require.Greater`) before indexing one in a test.
