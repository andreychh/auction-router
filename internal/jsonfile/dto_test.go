package jsonfile_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/andreychh/auction-router/internal/auction"
	"github.com/andreychh/auction-router/internal/jsonfile"
)

// samePartner reports whether two partners buy on the same terms. The three lists
// compare as sequences, so a term left unrestricted is one whether the list is nil
// or empty.
func samePartner(a, b auction.Partner) bool {
	return a.UID == b.UID &&
		a.Name == b.Name &&
		a.Endpoint == b.Endpoint &&
		a.Enabled == b.Enabled &&
		a.MinBidFloor == b.MinBidFloor &&
		slices.Equal(a.Countries, b.Countries) &&
		slices.Equal(a.DeviceTypes, b.DeviceTypes) &&
		slices.Equal(a.BlockedCategories, b.BlockedCategories)
}

// complete is an entry with every field filled, for a case to spoil one field of.
func complete() jsonfile.Partner {
	return jsonfile.Partner{
		UID:               new("dsp-alpha"),
		Name:              new("DSP Alpha"),
		Endpoint:          new("http://localhost:9001/bid"),
		IsEnabled:         new(true),
		Countries:         []string{"RU", "KZ"},
		DeviceTypes:       []string{"mobile", "desktop"},
		MinBidFloor:       new(0.5),
		BlockedCategories: []string{"gambling"},
	}
}

func TestParsePartner(t *testing.T) {
	tests := []struct {
		name string
		wire jsonfile.Partner
		want auction.Partner
	}{
		{
			name: "an entry with every field becomes the partner it describes",
			wire: complete(),
			want: auction.Partner{
				UID:               "dsp-alpha",
				Name:              "DSP Alpha",
				Endpoint:          "http://localhost:9001/bid",
				Enabled:           true,
				Countries:         []auction.Country{"RU", "KZ"},
				DeviceTypes:       []auction.DeviceType{auction.Mobile, auction.Desktop},
				MinBidFloor:       0.5,
				BlockedCategories: []auction.Category{"gambling"},
			},
		},
		{
			// Naming no country is not the same as ruling them all out, so the
			// lists an operator leaves out place no restriction at all.
			name: "lists left out place no restriction",
			wire: jsonfile.Partner{
				UID:       new("dsp-beta"),
				Name:      new("DSP Beta"),
				Endpoint:  new("https://beta.example/bid"),
				IsEnabled: new(true),
			},
			want: auction.Partner{
				UID:      "dsp-beta",
				Name:     "DSP Beta",
				Endpoint: "https://beta.example/bid",
				Enabled:  true,
			},
		},
		{
			name: "no floor named means a partner that buys at any price",
			wire: jsonfile.Partner{
				UID:       new("dsp-gamma"),
				Name:      new("DSP Gamma"),
				Endpoint:  new("http://gamma.example/bid"),
				IsEnabled: new(true),
				Countries: []string{"US"},
			},
			want: auction.Partner{
				UID:         "dsp-gamma",
				Name:        "DSP Gamma",
				Endpoint:    "http://gamma.example/bid",
				Enabled:     true,
				Countries:   []auction.Country{"US"},
				MinBidFloor: 0,
			},
		},
		{
			name: "a partner may be configured off",
			wire: jsonfile.Partner{
				UID:       new("dsp-delta"),
				Name:      new("DSP Delta"),
				Endpoint:  new("http://delta.example/bid"),
				IsEnabled: new(false),
			},
			want: auction.Partner{
				UID:      "dsp-delta",
				Name:     "DSP Delta",
				Endpoint: "http://delta.example/bid",
				Enabled:  false,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := jsonfile.ParsePartner("partners[0]", tt.wire)
			if err != nil {
				t.Fatalf("ParsePartner() returned %v, want the partner", err)
			}
			if !samePartner(got, tt.want) {
				t.Errorf("partner = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// A refused entry is answered with one complaint per thing wrong with it, each
// addressed to the entry's place in the file so that an operator knows which
// partner to go and look at.
func TestParsePartnerRefusals(t *testing.T) {
	// spoil returns a complete entry with one field replaced.
	spoil := func(change func(*jsonfile.Partner)) jsonfile.Partner {
		wire := complete()
		change(&wire)
		return wire
	}

	tests := []struct {
		name string
		wire jsonfile.Partner
		want []string
	}{
		{
			name: "an entry naming nothing is missing every required field",
			wire: jsonfile.Partner{},
			want: []string{"partners[2]: required fields are missing: uid, name, endpoint, is_enabled"},
		},
		{
			name: "one absent field is named alone",
			wire: spoil(func(p *jsonfile.Partner) { p.IsEnabled = nil }),
			want: []string{"partners[2]: required fields are missing: is_enabled"},
		},
		{
			// Judging a value the entry never wrote is not possible, so the absent
			// field is answered on its own and the bad endpoint goes unmentioned.
			name: "absent fields are answered before bad values",
			wire: spoil(func(p *jsonfile.Partner) {
				p.Name = nil
				p.Endpoint = new("not a url at all")
			}),
			want: []string{"partners[2]: required fields are missing: name"},
		},
		{
			name: "a blank uid",
			wire: spoil(func(p *jsonfile.Partner) { p.UID = new("  ") }),
			want: []string{"partners[2].uid: "},
		},
		{
			name: "an endpoint that names no host",
			wire: spoil(func(p *jsonfile.Partner) { p.Endpoint = new("/bid") }),
			want: []string{"partners[2].endpoint: "},
		},
		{
			name: "an endpoint the exchange cannot speak to",
			wire: spoil(func(p *jsonfile.Partner) { p.Endpoint = new("ftp://beta.example/bid") }),
			want: []string{"partners[2].endpoint: "},
		},
		{
			name: "a floor below zero",
			wire: spoil(func(p *jsonfile.Partner) { p.MinBidFloor = new(-1.0) }),
			want: []string{"partners[2].min_bid_floor: "},
		},
		{
			name: "a country outside the shape, named by its place in the list",
			wire: spoil(func(p *jsonfile.Partner) { p.Countries = []string{"RU", "RUS"} }),
			want: []string{"partners[2].countries[1]: "},
		},
		{
			name: "a device type nobody buys on, named by its place",
			wire: spoil(func(p *jsonfile.Partner) { p.DeviceTypes = []string{"fridge"} }),
			want: []string{"partners[2].device_types[0]: "},
		},
		{
			name: "a blank blocked category, named by its place",
			wire: spoil(func(p *jsonfile.Partner) { p.BlockedCategories = []string{"gambling", ""} }),
			want: []string{"partners[2].blocked_categories[1]: "},
		},
		{
			name: "every bad value is answered, not just the first",
			wire: spoil(func(p *jsonfile.Partner) {
				p.UID = new("")
				p.Endpoint = new("nowhere")
				p.MinBidFloor = new(-1.0)
				p.Countries = []string{"RUS"}
				p.DeviceTypes = []string{"fridge"}
				p.BlockedCategories = []string{" gambling"}
			}),
			want: []string{
				"partners[2].uid: ",
				"partners[2].endpoint: ",
				"partners[2].min_bid_floor: ",
				"partners[2].countries[0]: ",
				"partners[2].device_types[0]: ",
				"partners[2].blocked_categories[0]: ",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := jsonfile.ParsePartner("partners[2]", tt.wire)
			if err == nil {
				t.Fatal("ParsePartner() returned no error, want the entry refused")
			}
			got := strings.Split(err.Error(), "\n")
			if len(got) != len(tt.want) {
				t.Fatalf("%d complaints %q, want %d opening with %q",
					len(got), got, len(tt.want), tt.want)
			}
			for i, prefix := range tt.want {
				if !strings.HasPrefix(got[i], prefix) {
					t.Errorf("complaint %d is %q, want it to open with %q", i, got[i], prefix)
				}
			}
		})
	}
}
