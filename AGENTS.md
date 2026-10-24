# delta — Notes for AI Agents

- **The "original" baseline is captured at construction *and* at unmarshal — not at save.** `NewString(v)` sets `original = current = v`, and `UnmarshalJSON`/`UnmarshalBSONValue` reset `original` to the freshly-decoded value. So a record loaded from the database reads as `IsChanged() == false` until application code calls `Set`. There is no `Commit`/`Reset` to re-baseline after a save; re-load or re-construct the value if you need the baseline to advance.

- **Unmarshal decodes into `original`, then copies to `current`.** Both serializers (JSON, BSON) populate `original` first and mirror it into `current`. If you add a new wrapped type, follow the same pattern or change-tracking will be wrong from the first load.

- **A failed unmarshal must change neither value, so `ObjectID.UnmarshalJSON` decodes into a copy.** mongo-driver's `primitive.ObjectID.UnmarshalJSON` hex-decodes straight into the ID and stops at the first bad character, so decoding into `original` in place left a failed decode with a baseline half old bytes and half new. `Bool` and `String` cannot be partially written, so they still decode in place. Every type's tests and fuzz targets pin the rule; keep it for any new type.

- **A JSON `null` discards a pending edit without an error.** Decoding `null` leaves `original` alone, and the copy into `current` then throws away whatever `Set` wrote. The tests pin this for all three types.

- **mongo-driver copies any 12-byte JSON input straight into an ObjectID.** Its `UnmarshalJSON` checks `len == 12` before reading the input as JSON, so a 10-character JSON string like `"abcdefghij"` decodes to its own raw bytes, quotes included, rather than failing. `TestObjectID_JSON` pins it; it is the driver's behavior, not this package's.

- **`String.SetValue` formats numbers through rosetta's `convert.String`.** A whole-number float is written without decimals (`150`, since rosetta v0.46.0), and any other float is rounded to two (`33.33`). `TestString_SetValue` pins both, so a rosetta change fails here first.

- **`Original()` reads the baseline, and it is the *stored* value, not the previous one.** It answers "what does the database still hold", which is what an update needs to address the old row or clean up whatever the old value pointed at. Because the baseline only moves at construction and unmarshal (above), calling `Set` twice does not make `Original()` return the intermediate value — it still returns what was loaded.

- **Mutating methods take a pointer receiver; readers take a value receiver.** `Set`/`SetValue`/`Pointer` are pointer methods; `Value`/`IsChanged`/`NotChanged`/`IsZero`/`GetValue` are value methods. Store these wrappers as addressable fields, not in a map, or `Set` won't stick.

## Every wrapped type must keep satisfying the BSON `*Value` interfaces

`Bool`, `ObjectID`, and `String` serialize through `MarshalBSONValue`/`UnmarshalBSONValue`, and none of them has a plain `MarshalBSON` to fall back on. Go satisfies those interfaces *structurally*, so a signature that drifts out of step with the driver's does not fail to compile at the method itself — the driver silently stops recognizing the type and encodes it with the default struct codec instead. Because every field here is unexported, that fallback writes `{}`: a stored `true` becomes an empty document, with no error on either side.

Each `*_test.go` therefore carries `var _ bson.ValueMarshaler = …` assertions, which turn that silent change into a build failure. They matter most for the mongo-driver v2 upgrade (BUG-122), where the interface signature itself changes from `(bsontype.Type, []byte, error)` to `(byte, []byte, error)`. Do not delete them, and add the same pair alongside any new wrapped type.
