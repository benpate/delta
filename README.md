# 🔺 delta

[![Go Reference](https://pkg.go.dev/badge/github.com/benpate/delta.svg)](https://pkg.go.dev/github.com/benpate/delta)
[![Version](https://img.shields.io/github/v/release/benpate/delta?include_prereleases&style=flat-square&color=brightgreen)](https://github.com/benpate/delta/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/benpate/delta/go.yml?branch=main)](https://github.com/benpate/delta/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/benpate/delta?style=flat-square)](https://goreportcard.com/report/github.com/benpate/delta)
[![Codecov](https://img.shields.io/codecov/c/github/benpate/delta.svg?style=flat-square)](https://codecov.io/gh/benpate/delta)

## Change-Tracking Value Wrappers for Go

`delta` provides typed value wrappers (`Bool`, `String`, `ObjectID`) that remember their original value when loaded and can report whether they have changed since — and hand that original value back. This lets you tell which fields of a record were actually modified, and what they used to be: useful for writing minimal database updates, skipping no-op saves, or cleaning up whatever the previous value pointed at. Each wrapper marshals transparently to and from JSON and BSON, so it drops into existing data models without changing the wire format.

### Using delta

```go
// Wrap a value as it loads from the database.
name := delta.NewString("Sarah Connor")

name.IsChanged() // false — nothing has changed yet

name.Set("Sarah Reese")
name.IsChanged() // true — current value differs from the original
name.Value()     // "Sarah Reese"
name.Original()  // "Sarah Connor" — what the database still holds
```

The baseline is captured when the value is constructed *and* when it is unmarshaled, so a
record freshly loaded from the database reads as unchanged until your code calls `Set`.

## Pull Requests Welcome

I'm trying to make delta the best it can be, and your help is greatly appreciated. If you find a bug or have an idea for a new feature, please open an issue or submit a pull request. We're all in this together! 🔺
