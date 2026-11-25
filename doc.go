// Package delta wraps a few value types so that each one remembers the value it was
// loaded with, and can report whether it has changed since.
//
// Bool, String, ObjectID, and the generic Slice each hold a current value and an
// original value. The original is captured when the wrapper is constructed and again
// whenever it is unmarshaled from JSON or BSON, so a record freshly read from the
// database reads as unchanged until application code calls Set, Clear, or SetValue.
// Original returns that baseline, which is the value the database still holds, and
// Reset moves the baseline to the current value once a caller has saved it.
//
// Slice tracks a list of any comparable type. Its Added, Deleted, and Unchanged methods
// compare the current list with the original, and a change of order alone is a change.
//
// Each wrapper marshals as its plain value rather than as a sub-document, so it drops
// into an existing model without changing the stored format. A failed unmarshal
// changes neither value. GetValue and SetValue let rosetta's schema package read and
// write a wrapper as a single value, and Slice can also be addressed by index.
package delta
