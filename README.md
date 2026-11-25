# 🔺 delta

[![Go Reference](https://pkg.go.dev/badge/github.com/benpate/delta.svg)](https://pkg.go.dev/github.com/benpate/delta)
[![Version](https://img.shields.io/github/v/release/benpate/delta?include_prereleases&style=flat-square&color=brightgreen)](https://github.com/benpate/delta/releases)
[![Build Status](https://img.shields.io/github/actions/workflow/status/benpate/delta/go.yml?branch=main)](https://github.com/benpate/delta/actions/workflows/go.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/benpate/delta?style=flat-square)](https://goreportcard.com/report/github.com/benpate/delta)
[![Codecov](https://img.shields.io/codecov/c/github/benpate/delta.svg?style=flat-square)](https://codecov.io/gh/benpate/delta)

## Change-Tracking Value Wrappers for Go

`delta` provides typed value wrappers that remember the value they were loaded with, report whether it has changed since, and hand the original back. This lets you tell which fields of a record were modified, and what they used to be. That is useful for writing minimal database updates, skipping no-op saves, or cleaning up whatever the previous value pointed at.

| Type | Wraps |
|---|---|
| `Bool` | `bool` |
| `String` | `string` |
| `ObjectID` | a MongoDB `primitive.ObjectID` |
| `Slice[T]` | a list of any comparable type |

Each wrapper marshals to and from JSON and BSON as its plain value, so it drops into an existing data model without changing the stored format.

### Using delta

```go
// Wrap a value as it loads from the database.
name := delta.NewString("Sarah Connor")

name.IsChanged() // false, because nothing has changed yet

name.Set("Sarah Reese")
name.IsChanged() // true, because the current value differs from the original
name.Value()     // "Sarah Reese"
name.Original()  // "Sarah Connor", which the database still holds

// After you save the record, make the saved value the new baseline.
name.Reset()
name.IsChanged() // false
name.Original()  // "Sarah Reese"
```

The baseline is captured when a value is constructed and when it is unmarshaled, so a record freshly loaded from the database reads as unchanged until your code writes to it. Nothing moves the baseline at a save, so call `Reset()` on each value once the save has succeeded.

`Clear()` sets a value to the zero value for its type, and counts as a change like any other `Set`.

### Track a list

`Slice[T]` reports what was added and deleted since the list was loaded.

```go
targets := delta.NewSlice("mastodon", "bluesky")

targets.Set([]string{"bluesky", "threads"})

targets.IsChanged() // true
targets.Added()     // ["threads"]
targets.Deleted()   // ["mastodon"]
targets.Unchanged() // ["bluesky"]
targets.Original()  // ["mastodon", "bluesky"]
```

A change of order alone counts as a change, with nothing added or deleted.

### Use a wrapper in a record

A wrapper is an ordinary struct field. It is stored as its plain value, and it loads with its baseline already set.

```go
type Person struct {
	Name delta.String        `json:"name" bson:"name"`
	Tags delta.Slice[string] `json:"tags" bson:"tags"`
}

// Stored as {"name": "Sarah Connor", "tags": ["resistance"]}
```

A `Slice` also reads the `{values: [...]}` document that the older `rosetta/delta` package stored, and writes it back as a plain array.

### Work with rosetta schemas

Every wrapper implements the `GetValue` and `SetValue` methods that [rosetta](https://github.com/benpate/rosetta)'s `schema` package uses, so a schema can read and write a wrapped field as a single value. `SetValue` converts loosely: an `ObjectID` accepts a hex string, and a `Slice` accepts any list that rosetta can convert. A `Slice` can also be addressed by index.

## No Warranty

This software is provided as-is, without any warranty of any kind. Use it at your own risk.

## Pull Requests Welcome

I'm trying to make delta the best it can be, and your help is greatly appreciated. If you find a bug or have an idea for a new feature, please open an issue or submit a pull request. We're all in this together! 🔺
