package auction

import (
	"fmt"
	"math"
	"net/url"
	"strings"
)

// Country identifies a market by its ISO 3166-1 alpha-2 code.
type Country string

// ParseCountry converts a supplied country code into a Country. It checks the
// shape of the code, not whether the country exists.
func ParseCountry(s string) (Country, error) {
	if len(s) != 2 || !isASCIIUpper(s[0]) || !isASCIIUpper(s[1]) {
		return "", fmt.Errorf("%q is not an ISO 3166-1 alpha-2 code", s)
	}
	return Country(s), nil
}

// DeviceType names a kind of screen.
type DeviceType string

// The device types a publisher may offer.
const (
	Mobile  DeviceType = "mobile"
	Desktop DeviceType = "desktop"
	TV      DeviceType = "tv"
)

// ParseDeviceType converts a supplied device name into a DeviceType, accepting
// only [Mobile], [Desktop] and [TV], spelled exactly.
func ParseDeviceType(s string) (DeviceType, error) {
	switch device := DeviceType(s); device {
	case Mobile, Desktop, TV:
		return device, nil
	default:
		return "", fmt.Errorf("%q is not a known device type", s)
	}
}

// Price is a finite, non-negative amount. The exchange compares prices and
// never computes with them, so it needs no unit or currency. It does assume
// both sides use the same one.
type Price float64

// ParsePrice converts a supplied amount into a Price. It turns away NaN and
// infinity as well as negative amounts.
func ParsePrice(f float64) (Price, error) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, fmt.Errorf("%g is not a finite number", f)
	}
	if f < 0 {
		return 0, fmt.Errorf("%g is negative", f)
	}
	return Price(f), nil
}

// Category names a kind of content a publisher carries. The exchange does not
// own this vocabulary, so categories are matched byte for byte.
type Category string

// ParseCategory converts a supplied content category into a Category. It turns
// away a name that is empty or padded with whitespace.
func ParseCategory(s string) (Category, error) {
	if s == "" || strings.TrimSpace(s) != s {
		return "", fmt.Errorf("%q is empty or padded with whitespace", s)
	}
	return Category(s), nil
}

// PartnerUID names one demand partner. It appears in configuration, in the
// logs, and in the answer sent to a publisher. Nothing here checks that it is
// unique; whatever reads the configuration does.
type PartnerUID string

// ParsePartnerUID converts a configured identifier into a PartnerUID. It turns
// away an identifier that is empty or padded with whitespace; beyond that the
// exchange reads no meaning into its shape.
func ParsePartnerUID(s string) (PartnerUID, error) {
	if s == "" || strings.TrimSpace(s) != s {
		return "", fmt.Errorf("%q is empty or padded with whitespace", s)
	}
	return PartnerUID(s), nil
}

// Endpoint is the address a demand partner listens for bid requests on.
type Endpoint string

// ParseEndpoint converts a configured address into an Endpoint. It insists on
// an absolute http or https URL naming a host.
func ParseEndpoint(s string) (Endpoint, error) {
	address, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("%q is not a URL: %w", s, err)
	}
	if address.Host == "" || (address.Scheme != "http" && address.Scheme != "https") {
		return "", fmt.Errorf("%q is not an absolute http or https URL", s)
	}
	return Endpoint(s), nil
}

func isASCIIUpper(b byte) bool {
	return 'A' <= b && b <= 'Z'
}
