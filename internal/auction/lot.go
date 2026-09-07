package auction

import "slices"

// Lot is one advertising opportunity offered for sale: a single impression,
// described well enough for a partner to judge whether it wants to bid.
type Lot struct {
	// Country is where the impression will be shown: the user's market, not the
	// publisher's own.
	Country Country

	DeviceType DeviceType

	// BidFloor is the least the publisher will take for the impression.
	BidFloor Price

	// Categories name the content the impression will sit among. Empty means the
	// publisher disclosed nothing.
	Categories []Category
}

// NewLot creates a lot.
func NewLot(country Country, deviceType DeviceType, bidFloor Price, categories []Category) Lot {
	return Lot{
		Country:    country,
		DeviceType: deviceType,
		BidFloor:   bidFloor,
		Categories: categories,
	}
}

// MentionsAny reports whether the lot mentions any of the given categories.
// Either side being empty makes it false.
func (l Lot) MentionsAny(categories []Category) bool {
	for _, mentioned := range l.Categories {
		if slices.Contains(categories, mentioned) {
			return true
		}
	}
	return false
}
