package jsonfile_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/andreychh/auction-router/internal/auction"
	"github.com/andreychh/auction-router/internal/jsonfile"
)

// written puts content in a file of the test's own and returns its path. The
// directory goes away with the test, whether it passed or not.
func written(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "partners.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing the file the test reads: %v", err)
	}
	return path
}

// uids reduces the loaded partners to the names an assertion can read.
func uids(partners []auction.Partner) []auction.PartnerUID {
	names := make([]auction.PartnerUID, 0, len(partners))
	for _, partner := range partners {
		names = append(names, partner.UID)
	}
	return names
}

func TestPartnersLoad(t *testing.T) {
	const three = `[
		{"uid":"dsp-alpha","name":"DSP Alpha","endpoint":"http://alpha.example/bid",
		 "is_enabled":true,"countries":["RU","KZ"],"min_bid_floor":0.5},
		{"uid":"dsp-beta","name":"DSP Beta","endpoint":"http://beta.example/bid",
		 "is_enabled":false},
		{"uid":"dsp-gamma","name":"DSP Gamma","endpoint":"https://gamma.example/bid",
		 "is_enabled":true,"device_types":["mobile","tv"]}
	]`

	t.Run("a file naming partners returns them in the order it lists them", func(t *testing.T) {
		got, err := jsonfile.NewPartners(written(t, three)).Load(context.Background())
		if err != nil {
			t.Fatalf("Load() returned %v, want the partners", err)
		}

		want := []auction.PartnerUID{"dsp-alpha", "dsp-beta", "dsp-gamma"}
		if !slices.Equal(uids(got), want) {
			t.Errorf("loaded %q, want %q", uids(got), want)
		}
		// A partner switched off is still loaded: keeping it out is pretargeting's
		// business, not the file's.
		if got[1].Enabled {
			t.Error("dsp-beta was loaded switched on, want it as the file wrote it")
		}
		if got[0].MinBidFloor != 0.5 {
			t.Errorf("dsp-alpha's floor = %v, want 0.5", got[0].MinBidFloor)
		}
	})

	// The check on the caller's context comes before the file is opened, so a path
	// that leads nowhere still answers with the cancellation rather than with a
	// complaint about the file.
	t.Run("a caller that has already given up is turned away", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := jsonfile.NewPartners("/nowhere/partners.json").Load(ctx)

		if !errors.Is(err, context.Canceled) {
			t.Errorf("Load() returned %v, want the cancellation", err)
		}
	})

	// Every fault is answered together and addressed to the place that caused it:
	// an operator repairing configuration is not made to fix one line per restart.
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "a file that is not there",
			content: "",
			want:    []string{"reading partners", "no such file"},
		},
		{
			name:    "a file that is not JSON at all",
			content: `partners: []`,
			want:    []string{"partners.json", "invalid character"},
		},
		{
			name:    "a file holding something other than a list",
			content: `{"uid":"dsp-alpha"}`,
			want:    []string{"partners.json", "cannot unmarshal object"},
		},
		{
			name:    "a file naming no partners",
			content: `[]`,
			want:    []string{"partners.json names no partners"},
		},
		{
			name: "two partners claiming one uid, naming both places",
			content: `[
				{"uid":"dsp-alpha","name":"A","endpoint":"http://a.example/bid","is_enabled":true},
				{"uid":"dsp-beta","name":"B","endpoint":"http://b.example/bid","is_enabled":true},
				{"uid":"dsp-alpha","name":"C","endpoint":"http://c.example/bid","is_enabled":true}
			]`,
			want: []string{"partners[2].uid: uid is claimed twice, first by partners[0]"},
		},
		{
			name: "faults in two entries at once",
			content: `[
				{"uid":"dsp-alpha","name":"A","endpoint":"nowhere","is_enabled":true},
				{"uid":"dsp-beta","name":"B","endpoint":"http://b.example/bid"},
				{"uid":"dsp-gamma","name":"C","endpoint":"http://c.example/bid","is_enabled":true,
				 "countries":["RUS"]}
			]`,
			want: []string{
				"partners[0].endpoint: ",
				"partners[1]: required fields are missing: is_enabled",
				"partners[2].countries[0]: ",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "partners.json")
			if tt.content != "" {
				path = written(t, tt.content)
			}

			got, err := jsonfile.NewPartners(path).Load(context.Background())

			if err == nil {
				t.Fatalf("Load() returned %d partners, want the file refused", len(got))
			}
			if got != nil {
				t.Errorf("Load() returned %q alongside the fault, want nothing", uids(got))
			}
			for _, want := range tt.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the fault reads\n\t%v\nwant it to mention %q", err, want)
				}
			}
		})
	}
}
