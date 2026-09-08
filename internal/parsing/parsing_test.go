package parsing_test

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/andreychh/auction-router/internal/parsing"
)

// accept parses any entry but "bad", which it refuses, so that a case chooses
// which entries fail by where it puts that word.
func accept(entry string) (string, error) {
	if entry == "bad" {
		return "", errors.New("refused")
	}
	return entry, nil
}

func TestOrZero(t *testing.T) {
	t.Run("a pointer to nothing is the zero value", func(t *testing.T) {
		if got := parsing.OrZero[float64](nil); got != 0 {
			t.Errorf("OrZero(nil) = %v, want 0", got)
		}
		if got := parsing.OrZero[string](nil); got != "" {
			t.Errorf("OrZero(nil) = %q, want the empty string", got)
		}
	})

	// A field written at its zero value and one left out mean the same thing to
	// everything downstream; telling them apart is the caller's business, not this
	// function's.
	t.Run("a pointer to the zero value is the zero value", func(t *testing.T) {
		if got := parsing.OrZero(new(0.0)); got != 0 {
			t.Errorf("OrZero(&0) = %v, want 0", got)
		}
	})

	t.Run("a pointer to a value is that value", func(t *testing.T) {
		if got := parsing.OrZero(new(1.5)); got != 1.5 {
			t.Errorf("OrZero(&1.5) = %v, want 1.5", got)
		}
		if got := parsing.OrZero(new("mobile")); got != "mobile" {
			t.Errorf("OrZero(&\"mobile\") = %q, want mobile", got)
		}
	})
}

func TestParseSlice(t *testing.T) {
	tests := []struct {
		name  string
		raw   []string
		want  []string
		fails []string
	}{
		{
			name: "a list left out parses to nothing",
			raw:  nil,
			want: nil,
		},
		{
			name: "a list left empty parses to nothing",
			raw:  []string{},
			want: nil,
		},
		{
			name: "every entry is parsed, in the order given",
			raw:  []string{"RU", "KZ", "US"},
			want: []string{"RU", "KZ", "US"},
		},
		{
			name:  "the entry that failed is named by its place",
			raw:   []string{"RU", "bad"},
			fails: []string{"countries[1]: "},
		},
		{
			name:  "every failing entry is named, not just the first",
			raw:   []string{"bad", "RU", "bad"},
			fails: []string{"countries[0]: ", "countries[2]: "},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsing.ParseSlice("countries", tt.raw, accept)

			if len(tt.fails) == 0 {
				if err != nil {
					t.Fatalf("ParseSlice() returned %v, want the parsed entries", err)
				}
				if !slices.Equal(got, tt.want) {
					t.Errorf("parsed %q, want %q", got, tt.want)
				}
				// slices.Equal reads nil and empty alike, so the promise of nil
				// needs saying separately.
				if tt.want == nil && got != nil {
					t.Errorf("parsed %#v, want nil", got)
				}
				return
			}

			if err == nil {
				t.Fatalf("ParseSlice() parsed %q, want the bad entries refused", got)
			}
			// A list is taken whole or not at all: the entries that did parse are
			// no use while their neighbours have to be fixed.
			if got != nil {
				t.Errorf("parsed %q alongside the complaints, want nothing", got)
			}
			complaints := strings.Split(err.Error(), "\n")
			if len(complaints) != len(tt.fails) {
				t.Fatalf("%d complaints %q, want %d opening with %q",
					len(complaints), complaints, len(tt.fails), tt.fails)
			}
			for i, prefix := range tt.fails {
				if !strings.HasPrefix(complaints[i], prefix) {
					t.Errorf("complaint %d is %q, want it to open with %q",
						i, complaints[i], prefix)
				}
			}
		})
	}
}

// The domain parsers ParseSlice is used with return named types of their own, so
// it must not be tied to parsing strings into strings.
func TestParseSliceParsesToAnyType(t *testing.T) {
	got, err := parsing.ParseSlice("floors", []string{"1", "2", "3"}, strconv.Atoi)
	if err != nil {
		t.Fatalf("ParseSlice() returned %v, want the numbers", err)
	}
	if want := []int{1, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("parsed %v, want %v", got, want)
	}
}
