// Package delta wraps a few value types so that each one remembers the value it was
// loaded with, and can report whether it has changed since.
//
// Bool, String, and ObjectID each hold a current value and an original value. The
// original is captured when the wrapper is constructed and again whenever it is
// unmarshaled from JSON or BSON, so a record freshly read from the database reads as
// unchanged until application code calls Set or SetValue. Original returns that
// baseline, which is the value the database still holds.
//
// Each wrapper marshals as its plain value rather than as a sub-document, so it drops
// into an existing model without changing the stored format. A failed unmarshal
// changes neither value. GetValue and SetValue let rosetta's schema package read and
// write a wrapper as a single value.
package delta
