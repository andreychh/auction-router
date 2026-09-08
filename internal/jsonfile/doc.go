// Package jsonfile reads the exchange's demand partners from a JSON file on disk.
// It is the only place that knows partners live in a file at all: it takes a path
// and hands back domain values, so nothing downstream depends on where they came
// from.
package jsonfile
