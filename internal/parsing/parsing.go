// Package parsing turns permissive wire values into strict domain ones: what
// reading a request and reading a configuration file have in common. It knows
// nothing of what any one field means.
package parsing

import (
	"errors"
	"fmt"
)

// OrZero returns what p points at, or the zero value where p is nil.
func OrZero[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// ParseSlice parses every entry of raw, naming the entry behind a complaint as
// field[i]. A list is taken whole or not at all: where any entry fails it
// reports every complaint together and parses nothing. A list left out or left
// empty parses to nil.
func ParseSlice[T any](field string, raw []string, parse func(string) (T, error)) ([]T, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	values := make([]T, 0, len(raw))
	var problems []error
	for i, entry := range raw {
		value, err := parse(entry)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s[%d]: %w", field, i, err))
			continue
		}
		values = append(values, value)
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	return values, nil
}
