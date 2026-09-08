package jsonfile

import (
	"errors"
	"fmt"
	"strings"

	"github.com/andreychh/auction-router/internal/auction"
	"github.com/andreychh/auction-router/internal/parsing"
)

// Partner is the wire form of one entry of the file, naming the fields as JSON
// spells them.
//
// Scalars are pointers, so a field left out can be told from one written at its
// zero value. uid, name, endpoint and is_enabled have to be written. The rest
// default to placing no restriction, a floor left out to zero.
type Partner struct {
	UID               *string  `json:"uid"`
	Name              *string  `json:"name"`
	Endpoint          *string  `json:"endpoint"`
	IsEnabled         *bool    `json:"is_enabled"`
	Countries         []string `json:"countries"`
	DeviceTypes       []string `json:"device_types"`
	MinBidFloor       *float64 `json:"min_bid_floor"`
	BlockedCategories []string `json:"blocked_categories"`
}

// ParsePartner converts one entry of the file into a partner. It refuses an
// entry missing required fields on that alone; otherwise it collects every
// complaint about a value and returns them joined.
//
// Every complaint opens with at, the entry's place in the file, so an operator
// is told which partner to go and look at.
func ParsePartner(at string, wire Partner) (auction.Partner, error) {
	var missing []string
	if wire.UID == nil {
		missing = append(missing, "uid")
	}
	if wire.Name == nil {
		missing = append(missing, "name")
	}
	if wire.Endpoint == nil {
		missing = append(missing, "endpoint")
	}
	if wire.IsEnabled == nil {
		missing = append(missing, "is_enabled")
	}
	if len(missing) > 0 {
		return auction.Partner{}, fmt.Errorf(
			"%s: required fields are missing: %s",
			at,
			strings.Join(missing, ", "),
		)
	}

	var problems []error
	uid, err := auction.ParsePartnerUID(*wire.UID)
	if err != nil {
		problems = append(problems, fmt.Errorf("%s.uid: %w", at, err))
	}
	endpoint, err := auction.ParseEndpoint(*wire.Endpoint)
	if err != nil {
		problems = append(problems, fmt.Errorf("%s.endpoint: %w", at, err))
	}

	// No threshold named means none, so the partner hears about a lot at any floor.
	minBidFloor, err := auction.ParsePrice(parsing.OrZero(wire.MinBidFloor))
	if err != nil {
		problems = append(problems, fmt.Errorf("%s.min_bid_floor: %w", at, err))
	}

	countries, err := parsing.ParseSlice(at+".countries", wire.Countries, auction.ParseCountry)
	if err != nil {
		problems = append(problems, err)
	}

	deviceTypes, err := parsing.ParseSlice(at+".device_types", wire.DeviceTypes, auction.ParseDeviceType)
	if err != nil {
		problems = append(problems, err)
	}

	blocked, err := parsing.ParseSlice(at+".blocked_categories", wire.BlockedCategories, auction.ParseCategory)
	if err != nil {
		problems = append(problems, err)
	}

	if len(problems) > 0 {
		return auction.Partner{}, errors.Join(problems...)
	}

	return auction.NewPartner(uid, *wire.Name, endpoint, *wire.IsEnabled,
		countries, deviceTypes, minBidFloor, blocked), nil
}
