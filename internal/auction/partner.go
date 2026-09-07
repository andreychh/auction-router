package auction

import "slices"

// Partner is a demand-side platform the exchange may offer lots to, together
// with the standing terms on which it buys.
type Partner struct {
	UID PartnerUID

	// Name is what a person calls the partner. The exchange itself has no use for
	// it and identifies a partner by UID.
	Name string

	Endpoint Endpoint

	// Enabled says whether the exchange may offer lots to the partner at all.
	Enabled bool

	// Countries are the markets the partner buys in. Empty means every market:
	// naming no country is not the same as ruling them all out.
	Countries []Country

	// DeviceTypes are the kinds of screen the partner buys. Empty means every
	// kind.
	DeviceTypes []DeviceType

	// MinBidFloor is the least a lot's own floor may be for the partner to hear
	// about it.
	MinBidFloor Price

	// BlockedCategories are the kinds of content the partner will not appear
	// beside. A lot mentioning any of them is not offered to it.
	BlockedCategories []Category
}

// NewPartner creates a partner.
func NewPartner(
	uid PartnerUID,
	name string,
	endpoint Endpoint,
	enabled bool,
	countries []Country,
	deviceTypes []DeviceType,
	minBidFloor Price,
	blockedCategories []Category,
) Partner {
	return Partner{
		UID:               uid,
		Name:              name,
		Endpoint:          endpoint,
		Enabled:           enabled,
		Countries:         countries,
		DeviceTypes:       deviceTypes,
		MinBidFloor:       minBidFloor,
		BlockedCategories: blockedCategories,
	}
}

// Consider names the first of the partner's terms the lot fails, taking them in
// a fixed order: enabled, country, device type, floor, blocked content. A lot
// that meets them all gets [TermNone].
func (p Partner) Consider(lot Lot) Term {
	switch {
	case !p.Enabled:
		return TermEnabled
	case !p.buysIn(lot.Country):
		return TermCountry
	case !p.buysOn(lot.DeviceType):
		return TermDeviceType
	case lot.BidFloor < p.MinBidFloor:
		return TermBidFloor
	case lot.MentionsAny(p.BlockedCategories):
		return TermCategory
	}
	return TermNone
}

func (p Partner) buysIn(country Country) bool {
	return allows(p.Countries, country)
}

func (p Partner) buysOn(deviceType DeviceType) bool {
	return allows(p.DeviceTypes, deviceType)
}

// allows reports whether offered is among accepted, an empty list allowing
// everything.
func allows[T comparable](accepted []T, offered T) bool {
	return len(accepted) == 0 || slices.Contains(accepted, offered)
}

// Term names one of the conditions a partner places on the lots it will hear
// about.
type Term string

// The conditions a lot is weighed against. TermNone is the absence of one: the
// lot met them all.
const (
	TermNone       Term = ""
	TermEnabled    Term = "enabled"
	TermCountry    Term = "country"
	TermDeviceType Term = "device_type"
	TermBidFloor   Term = "bid_floor"
	TermCategory   Term = "category"
)
