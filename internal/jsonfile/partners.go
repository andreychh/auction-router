package jsonfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/andreychh/auction-router/internal/auction"
)

// Partners reads the exchange's demand partners from a JSON file holding one
// array of objects, each naming a partner and the terms it buys on.
type Partners struct {
	path string
}

// NewPartners creates a source. The file at path is not touched until Load.
func NewPartners(path string) Partners {
	return Partners{path: path}
}

// Load reads the file and returns the partners it names, in the order it lists
// them. It reports every fault at once, each addressed to the place in the file
// that caused it.
//
// A file naming no partners is a fault: an exchange with nobody to sell to
// answers every auction with nobody found, and failing to start says so louder.
//
// Reading a local file cannot be interrupted, so ctx only turns away a caller
// that has already given up.
func (p Partners) Load(ctx context.Context) ([]auction.Partner, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p.path)
	if err != nil {
		return nil, fmt.Errorf("reading partners: %w", err)
	}

	var records []Partner
	if err := json.Unmarshal(data, &records); err != nil {
		return nil, fmt.Errorf("reading partners from %s: %w", p.path, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%s names no partners", p.path)
	}

	partners := make([]auction.Partner, 0, len(records))
	claimed := make(map[auction.PartnerUID]int, len(records))
	var problems []error

	for i, record := range records {
		at := fmt.Sprintf("partners[%d]", i)

		parsed, err := ParsePartner(at, record)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		if first, duplicate := claimed[parsed.UID]; duplicate {
			problems = append(
				problems,
				fmt.Errorf("%s.uid: uid is claimed twice, first by partners[%d]", at, first),
			)
			continue
		}

		claimed[parsed.UID] = i
		partners = append(partners, parsed)
	}

	if len(problems) > 0 {
		return nil, fmt.Errorf("reading partners from %s:\n%w", p.path, errors.Join(problems...))
	}
	return partners, nil
}
