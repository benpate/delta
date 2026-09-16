# delta — Notes for AI Agents

- **The "original" baseline is captured at construction *and* at unmarshal — not at save.** `NewString(v)` sets `original = current = v`, and `UnmarshalJSON`/`UnmarshalBSONValue` reset `original` to the freshly-decoded value. So a record loaded from the database reads as `IsChanged() == false` until application code calls `Set`. There is no `Commit`/`Reset` to re-baseline after a save; re-load or re-construct the value if you need the baseline to advance.

- **Unmarshal decodes into `original`, then copies to `current`.** Both serializers (JSON, BSON) populate `original` first and mirror it into `current`. If you add a new wrapped type, follow the same pattern or change-tracking will be wrong from the first load.

- **`Original()` reads the baseline, and it is the *stored* value, not the previous one.** It answers "what does the database still hold", which is what an update needs to address the old row or clean up whatever the old value pointed at. Because the baseline only moves at construction and unmarshal (above), calling `Set` twice does not make `Original()` return the intermediate value — it still returns what was loaded.

- **Mutating methods take a pointer receiver; readers take a value receiver.** `Set`/`SetValue`/`Pointer` are pointer methods; `Value`/`IsChanged`/`NotChanged`/`IsZero`/`GetValue` are value methods. Store these wrappers as addressable fields, not in a map, or `Set` won't stick.
