package auction_test

import (
	"math"
	"testing"

	"github.com/andreychh/auction-router/internal/auction"
)

func TestParseCountry(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    auction.Country
		wantErr bool
	}{
		{name: "alpha-2 code", in: "RU", want: "RU"},
		{name: "code outside ISO is well formed", in: "XK", want: "XK"},
		{name: "lower case is not normalised", in: "ru", wantErr: true},
		{name: "mixed case is not normalised", in: "Ru", wantErr: true},
		{name: "alpha-3 code", in: "RUS", wantErr: true},
		{name: "one letter", in: "R", wantErr: true},
		{name: "empty", in: "", wantErr: true},
		{name: "digits", in: "12", wantErr: true},
		{name: "padded", in: " R", wantErr: true},
		{name: "two non-ASCII letters", in: "РУ", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := auction.ParseCountry(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseCountry(%q) error = %v, want error: %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseCountry(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseDeviceType(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    auction.DeviceType
		wantErr bool
	}{
		{name: "mobile", in: "mobile", want: auction.Mobile},
		{name: "desktop", in: "desktop", want: auction.Desktop},
		{name: "tv", in: "tv", want: auction.TV},
		{name: "capitalised is not normalised", in: "Mobile", wantErr: true},
		{name: "upper case is not normalised", in: "MOBILE", wantErr: true},
		{name: "padded is not trimmed", in: " mobile", wantErr: true},
		{name: "unknown kind of screen", in: "phone", wantErr: true},
		{name: "empty", in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := auction.ParseDeviceType(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseDeviceType(%q) error = %v, want error: %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseDeviceType(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParsePrice(t *testing.T) {
	tests := []struct {
		name    string
		in      float64
		want    auction.Price
		wantErr bool
	}{
		{name: "zero is a floor, not an absence of one", in: 0, want: 0},
		{name: "fractional amount", in: 1.5, want: 1.5},
		{name: "negative zero is zero", in: math.Copysign(0, -1), want: 0},
		{name: "negative amount", in: -0.01, wantErr: true},
		// NaN compares false against every threshold: a lot priced at NaN would be
		// turned away by every partner and no rule would explain why.
		{name: "NaN would empty the roster", in: math.NaN(), wantErr: true},
		// An infinity does the opposite, clearing every floor there is.
		{name: "infinity would clear every floor", in: math.Inf(1), wantErr: true},
		{name: "negative infinity", in: math.Inf(-1), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := auction.ParsePrice(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParsePrice(%g) error = %v, want error: %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParsePrice(%g) = %g, want %g", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseCategory(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    auction.Category
		wantErr bool
	}{
		{name: "name", in: "news", want: "news"},
		{name: "taxonomy identifier", in: "IAB1-2", want: "IAB1-2"},
		{name: "case is carried through untouched", in: "News", want: "News"},
		{name: "inner whitespace is part of the name", in: "news sport", want: "news sport"},
		{name: "empty", in: "", wantErr: true},
		{name: "padded on the left", in: " news", wantErr: true},
		{name: "padded on the right", in: "news ", wantErr: true},
		{name: "whitespace only", in: "\t", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := auction.ParseCategory(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseCategory(%q) error = %v, want error: %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseCategory(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParsePartnerUID(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    auction.PartnerUID
		wantErr bool
	}{
		{name: "identifier", in: "dsp-alpha", want: "dsp-alpha"},
		{name: "shape carries no meaning", in: "42 :: alpha", want: "42 :: alpha"},
		{name: "inner whitespace is part of the identifier", in: "dsp alpha", want: "dsp alpha"},
		{name: "empty", in: "", wantErr: true},
		{name: "padded on the left", in: " dsp-alpha", wantErr: true},
		{name: "padded on the right", in: "dsp-alpha ", wantErr: true},
		{name: "spaces only", in: "   ", wantErr: true},
		{name: "whitespace only", in: "\t\n", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := auction.ParsePartnerUID(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParsePartnerUID(%q) error = %v, want error: %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParsePartnerUID(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    auction.Endpoint
		wantErr bool
	}{
		{name: "http URL", in: "http://localhost:9001/bid", want: "http://localhost:9001/bid"},
		{name: "https URL", in: "https://dsp.example.com/bid", want: "https://dsp.example.com/bid"},
		{name: "no path", in: "http://localhost:9001", want: "http://localhost:9001"},
		{name: "query is carried through", in: "http://dsp.example.com/bid?v=2", want: "http://dsp.example.com/bid?v=2"},
		{name: "scheme left out", in: "localhost:9001/bid", wantErr: true},
		{name: "scheme-relative", in: "//localhost:9001/bid", wantErr: true},
		{name: "path only", in: "/bid", wantErr: true},
		{name: "scheme nothing can be sent over", in: "ftp://localhost:9001/bid", wantErr: true},
		{name: "no host", in: "http://", wantErr: true},
		{name: "opaque rather than absolute", in: "http:bid", wantErr: true},
		{name: "not a URL at all", in: "http://local\nhost/bid", wantErr: true},
		{name: "empty", in: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := auction.ParseEndpoint(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseEndpoint(%q) error = %v, want error: %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseEndpoint(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
