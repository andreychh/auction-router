package auction

// Roster is the partners the exchange trades by, in the order they were
// configured.
type Roster struct {
	partners []Partner
}

// NewRoster creates a roster.
func NewRoster(partners []Partner) Roster {
	return Roster{partners: partners}
}

// Consider splits the roster in two: the partners whose terms the lot meets,
// and an exclusion for each of the rest. Both keep roster order.
func (r Roster) Consider(lot Lot) (matched []Partner, excluded []Exclusion) {
	for _, partner := range r.partners {
		if term := partner.Consider(lot); term != TermNone {
			excluded = append(excluded, Exclusion{Partner: partner, Term: term})
			continue
		}
		matched = append(matched, partner)
	}
	return matched, excluded
}

// Exclusion is a partner kept out of a lot's auction by one of its own terms.
type Exclusion struct {
	Partner Partner
	Term    Term
}
