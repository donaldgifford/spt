package config

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors returned by the loader, validator, and duration parsers.
// Callers use errors.Is to discriminate; ValidationError aggregates them.
var (
	// ErrParse signals an HCL parse failure (bad syntax, unterminated
	// string, etc). Wrapped errors carry the HCL diagnostics, including
	// file path and line numbers.
	ErrParse = errors.New("config: parse")

	// ErrDecode signals a gohcl decode failure: the HCL body parsed, but
	// it could not be mapped onto the typed Config struct (unknown
	// block, wrong attribute type, missing labelled block, etc.).
	ErrDecode = errors.New("config: decode")

	// ErrInvalidDuration signals a malformed Go duration string (e.g.,
	// "15 minutes" instead of "15m").
	ErrInvalidDuration = errors.New("config: invalid duration")

	// ErrRequired signals that a required field was empty after the
	// file/env/flag layering completed.
	ErrRequired = errors.New("config: required field missing")

	// ErrOutOfRange signals a numeric field is outside its allowed range
	// (e.g., sampling ratio not in [0, 1]).
	ErrOutOfRange = errors.New("config: value out of range")

	// ErrReadFile signals that an HCL file path could not be read from
	// disk. Wrapped error is the underlying *os.PathError.
	ErrReadFile = errors.New("config: read file")
)

// ValidationError aggregates every problem found while validating a
// Config so the user gets one report listing all issues instead of
// playing whack-a-mole one error at a time.
type ValidationError struct {
	Problems []FieldProblem
}

// FieldProblem identifies a single validation failure. Field is the
// dotted path (e.g., "postgres.dsn"); Err is the sentinel
// (ErrRequired, ErrOutOfRange, ErrInvalidDuration).
type FieldProblem struct {
	Field string
	Err   error
}

// Error formats the aggregated problems as one multiline string.
func (v *ValidationError) Error() string {
	if v == nil || len(v.Problems) == 0 {
		return "config: validation failed"
	}
	var b strings.Builder
	b.WriteString("config: validation failed:")
	for _, p := range v.Problems {
		b.WriteString("\n  - ")
		b.WriteString(p.Field)
		b.WriteString(": ")
		b.WriteString(p.Err.Error())
	}
	return b.String()
}

// Is reports whether any of the aggregated problems wraps target. This
// lets callers do errors.Is(err, config.ErrRequired) on the aggregate.
func (v *ValidationError) Is(target error) bool {
	if v == nil {
		return false
	}
	for _, p := range v.Problems {
		if errors.Is(p.Err, target) {
			return true
		}
	}
	return false
}

// add appends a new problem. If err is nil the call is a no-op.
func (v *ValidationError) add(field string, err error) {
	if err == nil {
		return
	}
	v.Problems = append(v.Problems, FieldProblem{Field: field, Err: err})
}

// addRequired appends an ErrRequired problem when value is empty.
func (v *ValidationError) addRequired(field, value string) {
	if value == "" {
		v.Problems = append(v.Problems, FieldProblem{
			Field: field,
			Err:   fmt.Errorf("%w", ErrRequired),
		})
	}
}

// asError returns nil when there are no problems, the *ValidationError
// itself otherwise. Callers use this to convert to a plain error return.
func (v *ValidationError) asError() error {
	if v == nil || len(v.Problems) == 0 {
		return nil
	}
	return v
}
