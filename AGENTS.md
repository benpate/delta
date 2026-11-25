# delta — Notes for AI Agents

- **The "original" baseline is captured at construction *and* at unmarshal, and moves at a save only when the caller says so.** `NewString(v)` sets `original = current = v`, and `UnmarshalJSON`/`UnmarshalBSONValue` reset `original` to the freshly-decoded value. So a record loaded from the database reads as `IsChanged() == false` until application code calls `Set`. After a save, call `Reset()` to make the saved value the baseline. Nothing calls it for you, so a record that is saved twice without a `Reset` still compares against the value it was loaded with.

- **Unmarshal sets `original` and `current` together, and only on success.** The BSON decoders populate `original` first and mirror it into `current`. The JSON decoders decode into a local zero value and assign both fields afterwards, for two reasons: a JSON `null` then becomes the zero value for the type, and a decode that fails part-way cannot leave a corrupted baseline behind (the driver's `primitive.ObjectID.UnmarshalJSON` writes the valid prefix of a hex string before rejecting the rest). If you add a new wrapped type, follow the same pattern or change-tracking will be wrong from the first load. The `Fuzz*_Unmarshal*` targets check it.

- **`ObjectID` JSON is a hex string and nothing else.** It encodes as 24 lowercase hex characters, and decodes only from 24 hex characters, an empty string, or `null` (the last two become the nil ObjectID). It does not delegate to the driver's decoder, which also accepts extended JSON (`{"$oid": …}`) and copies any 12-byte input verbatim; both are errors here.

- **`ObjectID.SetValue` treats an empty input as "clear".** `nil`, `""`, an empty byte slice, and a nil `*primitive.ObjectID` all set the nil ObjectID. A value with no string form at all (a struct, a `*string`) is still an error, so that an unconvertible input is not mistaken for an empty one.

- **`Original()` reads the baseline, and it is the *stored* value, not the previous one.** It answers "what does the database still hold", which is what an update needs to address the old row or clean up whatever the old value pointed at. Because the baseline only moves at construction, at unmarshal, and at `Reset` (above), calling `Set` twice does not make `Original()` return the intermediate value — it still returns what was loaded.

- **Mutating methods take a pointer receiver; readers take a value receiver.** `Set`/`Clear`/`Reset`/`SetValue`/`Pointer` are pointer methods; `Value`/`IsChanged`/`NotChanged`/`IsZero`/`GetValue` are value methods. Store these wrappers as addressable fields, not in a map, or `Set` won't stick.

## Every wrapped type must keep satisfying the BSON `*Value` interfaces

`Bool`, `ObjectID`, and `String` serialize through `MarshalBSONValue`/`UnmarshalBSONValue`, and none of them has a plain `MarshalBSON` to fall back on. Go satisfies those interfaces *structurally*, so a signature that drifts out of step with the driver's does not fail to compile at the method itself — the driver silently stops recognizing the type and encodes it with the default struct codec instead. Because every field here is unexported, that fallback writes `{}`: a stored `true` becomes an empty document, with no error on either side.

Each `*_test.go` therefore carries `var _ bson.ValueMarshaler = …` assertions, which turn that silent change into a build failure. They matter most for the mongo-driver v2 upgrade (BUG-122), where the interface signature itself changes from `(bsontype.Type, []byte, error)` to `(byte, []byte, error)`. Do not delete them, and add the same pair alongside any new wrapped type.
