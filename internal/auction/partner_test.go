package auction_test

import (
	"testing"

	"github.com/andreychh/auction-router/internal/auction"
)

// Partners are written as literals rather than built with NewPartner, so that a
// case states only the terms it is about. The zero partner accepts everything —
// an empty list means every value and a zero floor means any price — with the
// one exception of Enabled, which every passing case has to set.
func TestPartnerConsider(t *testing.T) {
	tests := []struct {
		name    string
		partner auction.Partner
		lot     auction.Lot
		want    auction.Term
	}{
		{
			name:    "a partner placing no terms hears about anything",
			partner: auction.Partner{Enabled: true},
			lot:     auction.Lot{Country: "RU", DeviceType: auction.Mobile, BidFloor: 1.5},
			want:    auction.TermNone,
		},
		{
			name: "every term met",
			partner: auction.Partner{
				Enabled:           true,
				Countries:         []auction.Country{"RU", "KZ"},
				DeviceTypes:       []auction.DeviceType{auction.Mobile, auction.Desktop},
				MinBidFloor:       0.5,
				BlockedCategories: []auction.Category{"gambling"},
			},
			lot: auction.Lot{
				Country:    "RU",
				DeviceType: auction.Mobile,
				BidFloor:   1.5,
				Categories: []auction.Category{"news", "sport"},
			},
			want: auction.TermNone,
		},

		{
			name:    "a partner switched off",
			partner: auction.Partner{Enabled: false},
			lot:     auction.Lot{Country: "RU", DeviceType: auction.Mobile},
			want:    auction.TermEnabled,
		},
		{
			name: "being switched off is named before a country not bought",
			partner: auction.Partner{
				Enabled:   false,
				Countries: []auction.Country{"US"},
			},
			lot:  auction.Lot{Country: "RU", DeviceType: auction.Mobile},
			want: auction.TermEnabled,
		},

		{
			name: "a country the partner does not buy",
			partner: auction.Partner{
				Enabled:   true,
				Countries: []auction.Country{"US", "DE"},
			},
			lot:  auction.Lot{Country: "RU", DeviceType: auction.Mobile},
			want: auction.TermCountry,
		},
		{
			name: "a country further down the list is bought all the same",
			partner: auction.Partner{
				Enabled:   true,
				Countries: []auction.Country{"US", "DE", "RU"},
			},
			lot:  auction.Lot{Country: "RU", DeviceType: auction.Mobile},
			want: auction.TermNone,
		},
		{
			name:    "naming no country means buying in every market",
			partner: auction.Partner{Enabled: true, Countries: nil},
			lot:     auction.Lot{Country: "RU", DeviceType: auction.Mobile},
			want:    auction.TermNone,
		},
		{
			name: "the country is named before a device type not bought",
			partner: auction.Partner{
				Enabled:     true,
				Countries:   []auction.Country{"US"},
				DeviceTypes: []auction.DeviceType{auction.Desktop},
			},
			lot:  auction.Lot{Country: "RU", DeviceType: auction.Mobile},
			want: auction.TermCountry,
		},

		{
			name: "a kind of screen the partner does not buy",
			partner: auction.Partner{
				Enabled:     true,
				DeviceTypes: []auction.DeviceType{auction.Desktop, auction.TV},
			},
			lot:  auction.Lot{Country: "RU", DeviceType: auction.Mobile},
			want: auction.TermDeviceType,
		},
		{
			name: "a device type further down the list is bought all the same",
			partner: auction.Partner{
				Enabled:     true,
				DeviceTypes: []auction.DeviceType{auction.Desktop, auction.TV, auction.Mobile},
			},
			lot:  auction.Lot{Country: "RU", DeviceType: auction.Mobile},
			want: auction.TermNone,
		},
		{
			name:    "naming no device type means buying every kind of screen",
			partner: auction.Partner{Enabled: true, DeviceTypes: nil},
			lot:     auction.Lot{Country: "RU", DeviceType: auction.TV},
			want:    auction.TermNone,
		},
		{
			name: "the device type is named before a floor below the minimum",
			partner: auction.Partner{
				Enabled:     true,
				DeviceTypes: []auction.DeviceType{auction.Desktop},
				MinBidFloor: 2.0,
			},
			lot:  auction.Lot{Country: "RU", DeviceType: auction.Mobile, BidFloor: 0.5},
			want: auction.TermDeviceType,
		},

		{
			name:    "a floor below the minimum",
			partner: auction.Partner{Enabled: true, MinBidFloor: 0.5},
			lot:     auction.Lot{Country: "RU", DeviceType: auction.Mobile, BidFloor: 0.49},
			want:    auction.TermBidFloor,
		},
		{
			name:    "a floor equal to the minimum is enough",
			partner: auction.Partner{Enabled: true, MinBidFloor: 0.5},
			lot:     auction.Lot{Country: "RU", DeviceType: auction.Mobile, BidFloor: 0.5},
			want:    auction.TermNone,
		},
		{
			name:    "a floor of zero is the least a lot can offer, not the most it can ask",
			partner: auction.Partner{Enabled: true, MinBidFloor: 0.1},
			lot:     auction.Lot{Country: "RU", DeviceType: auction.Mobile, BidFloor: 0},
			want:    auction.TermBidFloor,
		},
		{
			name: "the floor is named before a blocked category",
			partner: auction.Partner{
				Enabled:           true,
				MinBidFloor:       2.0,
				BlockedCategories: []auction.Category{"gambling"},
			},
			lot: auction.Lot{
				Country:    "RU",
				DeviceType: auction.Mobile,
				BidFloor:   0.5,
				Categories: []auction.Category{"gambling"},
			},
			want: auction.TermBidFloor,
		},

		{
			name: "a category the partner will not appear beside",
			partner: auction.Partner{
				Enabled:           true,
				BlockedCategories: []auction.Category{"gambling"},
			},
			lot: auction.Lot{
				Country:    "RU",
				DeviceType: auction.Mobile,
				Categories: []auction.Category{"gambling"},
			},
			want: auction.TermCategory,
		},
		{
			name: "one blocked category among several the lot mentions",
			partner: auction.Partner{
				Enabled:           true,
				BlockedCategories: []auction.Category{"adult", "gambling"},
			},
			lot: auction.Lot{
				Country:    "RU",
				DeviceType: auction.Mobile,
				Categories: []auction.Category{"news", "sport", "gambling"},
			},
			want: auction.TermCategory,
		},
		{
			name:    "blocking nothing lets every category through",
			partner: auction.Partner{Enabled: true, BlockedCategories: nil},
			lot: auction.Lot{
				Country:    "RU",
				DeviceType: auction.Mobile,
				Categories: []auction.Category{"gambling", "adult"},
			},
			want: auction.TermNone,
		},
		{
			name: "a lot disclosing no category is never blocked",
			partner: auction.Partner{
				Enabled:           true,
				BlockedCategories: []auction.Category{"gambling"},
			},
			lot:  auction.Lot{Country: "RU", DeviceType: auction.Mobile, Categories: nil},
			want: auction.TermNone,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.partner.Consider(tt.lot); got != tt.want {
				t.Errorf("Consider() = %q, want %q\n\tpartner: %+v\n\tlot:     %+v",
					got, tt.want, tt.partner, tt.lot)
			}
		})
	}
}
