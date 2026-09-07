package auction_test

import (
	"slices"
	"testing"

	"github.com/andreychh/auction-router/internal/auction"
)

// exclusion is an auction.Exclusion reduced to what the roster promises about
// it: which partner was kept out, and by which term.
type exclusion struct {
	partner auction.PartnerUID
	term    auction.Term
}

// uids reduces a list of partners to the names an assertion can read.
func uids(partners []auction.Partner) []auction.PartnerUID {
	names := make([]auction.PartnerUID, 0, len(partners))
	for _, partner := range partners {
		names = append(names, partner.UID)
	}
	return names
}

func TestRosterConsider(t *testing.T) {
	tests := []struct {
		name         string
		partners     []auction.Partner
		lot          auction.Lot
		wantMatched  []auction.PartnerUID
		wantExcluded []exclusion
	}{
		{
			// Each partner here fails a different term, so an exclusion paired
			// with the wrong partner shows up as the wrong term.
			name: "both lists keep roster order",
			partners: []auction.Partner{
				{UID: "alpha", Enabled: true},
				{UID: "beta", Enabled: false},
				{UID: "gamma", Enabled: true},
				{UID: "delta", Enabled: true, Countries: []auction.Country{"US"}},
				{UID: "epsilon", Enabled: true, MinBidFloor: 5.0},
			},
			lot:         auction.Lot{Country: "RU", DeviceType: auction.Mobile, BidFloor: 1.5},
			wantMatched: []auction.PartnerUID{"alpha", "gamma"},
			wantExcluded: []exclusion{
				{"beta", auction.TermEnabled},
				{"delta", auction.TermCountry},
				{"epsilon", auction.TermBidFloor},
			},
		},
		{
			name:         "an empty roster has nobody to offer the lot to",
			partners:     nil,
			lot:          auction.Lot{Country: "RU", DeviceType: auction.Mobile},
			wantMatched:  nil,
			wantExcluded: nil,
		},
		{
			name: "every partner suits the lot",
			partners: []auction.Partner{
				{UID: "alpha", Enabled: true},
				{UID: "beta", Enabled: true},
			},
			lot:          auction.Lot{Country: "RU", DeviceType: auction.Mobile},
			wantMatched:  []auction.PartnerUID{"alpha", "beta"},
			wantExcluded: nil,
		},
		{
			name: "no partner suits the lot",
			partners: []auction.Partner{
				{UID: "alpha", Enabled: false},
				{UID: "beta", Enabled: true, Countries: []auction.Country{"US"}},
			},
			lot:         auction.Lot{Country: "RU", DeviceType: auction.Mobile},
			wantMatched: nil,
			wantExcluded: []exclusion{
				{"alpha", auction.TermEnabled},
				{"beta", auction.TermCountry},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matched, excluded := auction.NewRoster(tt.partners).Consider(tt.lot)
			if got := uids(matched); !slices.Equal(got, tt.wantMatched) {
				t.Errorf("matched = %q, want %q", got, tt.wantMatched)
			}
			gotExcluded := make([]exclusion, 0, len(excluded))
			for _, e := range excluded {
				gotExcluded = append(gotExcluded, exclusion{e.Partner.UID, e.Term})
			}
			if !slices.Equal(gotExcluded, tt.wantExcluded) {
				t.Errorf("excluded = %v, want %v", gotExcluded, tt.wantExcluded)
			}
		})
	}
}
