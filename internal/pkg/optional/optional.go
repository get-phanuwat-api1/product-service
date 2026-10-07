// Package optional provides a JSON field type that can distinguish between
// three states: undefined (key absent), null, and a concrete value.
// It is used to implement PATCH semantics where only provided fields are overwritten.
package optional

import (
	"bytes"
	"encoding/json"
)

// Field represents a JSON field that may be undefined, null or hold a value.
type Field[T any] struct {
	// Set is true when the key was present in the JSON payload (even if null).
	Set bool
	// Null is true when the key was present and its value was JSON null.
	Null bool
	// Value holds the decoded value when Set && !Null.
	Value T
}

// Of returns a Field holding the given value.
func Of[T any](v T) Field[T] { return Field[T]{Set: true, Value: v} }

// Null returns a Field explicitly set to null.
func Null[T any]() Field[T] { return Field[T]{Set: true, Null: true} }

// Undefined returns a Field that was not provided.
func Undefined[T any]() Field[T] { return Field[T]{} }

// HasValue reports whether the field was provided with a non-null value.
func (f Field[T]) HasValue() bool { return f.Set && !f.Null }

// Ptr returns a pointer to the value, or nil when the field is null/undefined.
func (f Field[T]) Ptr() *T {
	if !f.HasValue() {
		return nil
	}
	v := f.Value
	return &v
}

// UnmarshalJSON is only invoked by encoding/json when the key is present,
// which is how "undefined" is distinguished from "null".
func (f *Field[T]) UnmarshalJSON(data []byte) error {
	f.Set = true
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		f.Null = true
		var zero T
		f.Value = zero
		return nil
	}
	f.Null = false
	return json.Unmarshal(data, &f.Value)
}

// MarshalJSON writes null for null/undefined fields, otherwise the value.
func (f Field[T]) MarshalJSON() ([]byte, error) {
	if !f.HasValue() {
		return []byte("null"), nil
	}
	return json.Marshal(f.Value)
}
