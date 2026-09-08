package api

import (
	"errors"
	"fmt"
	"strings"

	"github.com/andreychh/auction-router/internal/auction"
)

// Status says whether a lot found any partner to hear it.
type Status string

// The values [Status] may take.
const (
	StatusOK            Status = "ok"
	StatusNoMatchedDSPs Status = "no_matched_dsps"
)

// ErrFieldsMissing reports a request that left out required fields, naming them
// all.
var ErrFieldsMissing = errors.New("required fields are missing")

// AuctionRequest is the wire form of a lot a publisher offers for sale, naming
// the fields as JSON spells them.
//
// Scalar fields are pointers, so a field left out can be told from one sent at
// its zero value. Slices need no pointer: nil already says absent.
type AuctionRequest struct {
	RequestID  *string  `json:"request_id"`
	Country    *string  `json:"country"`
	DeviceType *string  `json:"device_type"`
	BidFloor   *float64 `json:"bid_floor"`
	Categories []string `json:"categories"`
}

// ParseLot converts a publisher's request into the lot it describes. It refuses
// a request missing required fields on that alone; otherwise it collects every
// complaint about a value, prefixes each with the JSON field it concerns, and
// returns them joined.
//
// Where ParseLot reports no error, every required field was present, so a
// caller may dereference them without checking. The lot carries no request
// identifier: the exchange only echoes that one back and logs it.
func ParseLot(wire AuctionRequest) (auction.Lot, error) {
	var missing []string
	if wire.RequestID == nil {
		missing = append(missing, "request_id")
	}
	if wire.Country == nil {
		missing = append(missing, "country")
	}
	if wire.DeviceType == nil {
		missing = append(missing, "device_type")
	}
	if len(missing) > 0 {
		return auction.Lot{}, fmt.Errorf("%w: %s", ErrFieldsMissing, strings.Join(missing, ", "))
	}

	var problems []error
	country, err := auction.ParseCountry(*wire.Country)
	if err != nil {
		problems = append(problems, fmt.Errorf("country: %w", err))
	}
	deviceType, err := auction.ParseDeviceType(*wire.DeviceType)
	if err != nil {
		problems = append(problems, fmt.Errorf("device_type: %w", err))
	}
	// No floor named means no floor, which the exchange reads as zero.
	bidFloor, err := auction.ParsePrice(orZero(wire.BidFloor))
	if err != nil {
		problems = append(problems, fmt.Errorf("bid_floor: %w", err))
	}
	var categories []auction.Category
	for i, name := range wire.Categories {
		category, err := auction.ParseCategory(name)
		if err != nil {
			problems = append(problems, fmt.Errorf("categories[%d]: %w", i, err))
			continue
		}
		categories = append(categories, category)
	}
	if len(problems) > 0 {
		return auction.Lot{}, errors.Join(problems...)
	}

	return auction.NewLot(country, deviceType, bidFloor, categories), nil
}

// AuctionResponse is the wire form of a finished auction.
type AuctionResponse struct {
	RequestID   string   `json:"request_id"`
	Status      Status   `json:"status"`
	MatchedDSPs []string `json:"matched_dsps"`
	Sent        int      `json:"sent"`
	Succeeded   int      `json:"succeeded"`
	DurationMS  int64    `json:"duration_ms"`
}

// NewAuctionResponse renders an outcome as the response a publisher receives,
// echoing requestID back. An empty match becomes a JSON array, not null.
func NewAuctionResponse(requestID string, outcome auction.Outcome) AuctionResponse {
	matched := make([]string, 0, len(outcome.Matched))
	for _, partner := range outcome.Matched {
		matched = append(matched, string(partner.UID))
	}

	status := StatusOK
	if len(matched) == 0 {
		status = StatusNoMatchedDSPs
	}

	return AuctionResponse{
		RequestID:   requestID,
		Status:      status,
		MatchedDSPs: matched,
		Sent:        len(matched),
		Succeeded:   outcome.Answered(),
		DurationMS:  outcome.Elapsed.Milliseconds(),
	}
}

// orZero returns what p points at, or the zero value where p is nil.
func orZero[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// ErrorResponse is the answer to a refused request: what was wrong with it, and
// a separate complaint for each reason.
type ErrorResponse struct {
	Error   string   `json:"error"`
	Details []string `json:"details,omitempty"`
}
