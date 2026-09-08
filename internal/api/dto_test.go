package api_test

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/andreychh/auction-router/internal/api"
	"github.com/andreychh/auction-router/internal/auction"
)

// sameLot reports whether two lots describe the same opportunity. Categories
// compare as a sequence, so a lot mentioning none is one whether the list is nil
// or empty.
func sameLot(a, b auction.Lot) bool {
	return a.Country == b.Country &&
		a.DeviceType == b.DeviceType &&
		a.BidFloor == b.BidFloor &&
		slices.Equal(a.Categories, b.Categories)
}

func TestParseLot(t *testing.T) {
	tests := []struct {
		name string
		wire api.AuctionRequest
		want auction.Lot
	}{
		{
			name: "a complete request becomes the lot it describes",
			wire: api.AuctionRequest{
				RequestID:  new("r-1"),
				Country:    new("RU"),
				DeviceType: new("mobile"),
				BidFloor:   new(1.5),
				Categories: []string{"news", "sport"},
			},
			want: auction.Lot{
				Country:    "RU",
				DeviceType: auction.Mobile,
				BidFloor:   1.5,
				Categories: []auction.Category{"news", "sport"},
			},
		},
		{
			name: "no floor named means a floor of zero",
			wire: api.AuctionRequest{
				RequestID:  new("r-1"),
				Country:    new("US"),
				DeviceType: new("desktop"),
			},
			want: auction.Lot{Country: "US", DeviceType: auction.Desktop, BidFloor: 0},
		},
		{
			name: "an empty category list is a lot mentioning none",
			wire: api.AuctionRequest{
				RequestID:  new("r-1"),
				Country:    new("DE"),
				DeviceType: new("tv"),
				BidFloor:   new(2.0),
				Categories: []string{},
			},
			want: auction.Lot{Country: "DE", DeviceType: auction.TV, BidFloor: 2.0},
		},
		{
			name: "a blank identifier is accepted",
			wire: api.AuctionRequest{
				RequestID:  new(""),
				Country:    new("RU"),
				DeviceType: new("mobile"),
			},
			want: auction.Lot{Country: "RU", DeviceType: auction.Mobile},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := api.ParseLot(tt.wire)
			if err != nil {
				t.Fatalf("ParseLot() returned %v, want the lot", err)
			}
			if !sameLot(got, tt.want) {
				t.Errorf("lot = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// A refused request is answered with one complaint per thing wrong with it. The
// cases give the prefix each complaint has to open with, leaving the wording of
// the complaint itself to the package that judged the value.
func TestParseLotRefusals(t *testing.T) {
	tests := []struct {
		name string
		wire api.AuctionRequest
		want []string
	}{
		{
			name: "a request naming nothing is missing every required field",
			wire: api.AuctionRequest{},
			want: []string{"required fields are missing: request_id, country, device_type"},
		},
		{
			name: "one absent field is named alone",
			wire: api.AuctionRequest{RequestID: new("r-1"), DeviceType: new("mobile")},
			want: []string{"required fields are missing: country"},
		},
		{
			// Judging a value the request never sent is not possible, so the
			// absent field is answered on its own and device_type goes unmentioned.
			name: "absent fields are answered before bad values",
			wire: api.AuctionRequest{
				RequestID:  new("r-1"),
				DeviceType: new("fridge"),
			},
			want: []string{"required fields are missing: country"},
		},
		{
			name: "a country that is not a two-letter code",
			wire: api.AuctionRequest{
				RequestID:  new("r-1"),
				Country:    new("russia"),
				DeviceType: new("mobile"),
			},
			want: []string{"country: "},
		},
		{
			name: "a device type nobody buys on",
			wire: api.AuctionRequest{
				RequestID:  new("r-1"),
				Country:    new("RU"),
				DeviceType: new("fridge"),
			},
			want: []string{"device_type: "},
		},
		{
			name: "a floor below zero",
			wire: api.AuctionRequest{
				RequestID:  new("r-1"),
				Country:    new("RU"),
				DeviceType: new("mobile"),
				BidFloor:   new(-1.0),
			},
			want: []string{"bid_floor: "},
		},
		{
			// A JSON null inside the array arrives as an empty string, which is
			// the same case as one sent empty outright.
			name: "a blank category is named by its place in the list",
			wire: api.AuctionRequest{
				RequestID:  new("r-1"),
				Country:    new("RU"),
				DeviceType: new("mobile"),
				Categories: []string{"news", ""},
			},
			want: []string{"categories[1]: "},
		},
		{
			name: "every bad value is answered, not just the first",
			wire: api.AuctionRequest{
				RequestID:  new("r-1"),
				Country:    new("russia"),
				DeviceType: new("fridge"),
				BidFloor:   new(-1.0),
				Categories: []string{" news"},
			},
			want: []string{"country: ", "device_type: ", "bid_floor: ", "categories[0]: "},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := api.ParseLot(tt.wire)
			if err == nil {
				t.Fatal("ParseLot() returned no error, want the request refused")
			}
			// Joined complaints print one per line, which is how the handler
			// hands them to a publisher as separate items.
			got := strings.Split(err.Error(), "\n")
			if len(got) != len(tt.want) {
				t.Fatalf("%d complaints %q, want %d opening with %q",
					len(got), got, len(tt.want), tt.want)
			}
			for i, prefix := range tt.want {
				if !strings.HasPrefix(got[i], prefix) {
					t.Errorf("complaint %d is %q, want it to open with %q", i, got[i], prefix)
				}
			}
		})
	}
}

// matched builds the partners an outcome matched from their names alone, since
// the response reads nothing else about them.
func matched(uids ...auction.PartnerUID) []auction.Partner {
	partners := make([]auction.Partner, 0, len(uids))
	for _, uid := range uids {
		partners = append(partners, auction.Partner{UID: uid})
	}
	return partners
}

func TestNewAuctionResponse(t *testing.T) {
	tests := []struct {
		name    string
		outcome auction.Outcome
		want    api.AuctionResponse
	}{
		{
			name: "one of the invited partners said nothing",
			outcome: auction.NewOutcome(
				matched("alpha", "beta", "gamma"),
				nil,
				make([]auction.Failure, 1),
				42*time.Millisecond,
			),
			want: api.AuctionResponse{
				RequestID:   "r-1",
				Status:      api.StatusOK,
				MatchedDSPs: []string{"alpha", "beta", "gamma"},
				Sent:        3,
				Succeeded:   2,
				DurationMS:  42,
			},
		},
		{
			// Every counter still stands at the number it earned; only the status
			// says the lot went nowhere.
			name: "the lot suited no partner at all",
			outcome: auction.NewOutcome(
				nil,
				make([]auction.Exclusion, 4),
				nil,
				time.Millisecond,
			),
			want: api.AuctionResponse{
				RequestID:   "r-1",
				Status:      api.StatusNoMatchedDSPs,
				MatchedDSPs: []string{},
				Sent:        0,
				Succeeded:   0,
				DurationMS:  1,
			},
		},
		{
			name: "an auction shorter than a millisecond is reported as none",
			outcome: auction.NewOutcome(
				matched("alpha"),
				nil,
				nil,
				999*time.Microsecond,
			),
			want: api.AuctionResponse{
				RequestID:   "r-1",
				Status:      api.StatusOK,
				MatchedDSPs: []string{"alpha"},
				Sent:        1,
				Succeeded:   1,
				DurationMS:  0,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := api.NewAuctionResponse("r-1", tt.outcome)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewAuctionResponse() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// The wire form is the whole of what a publisher sees, so the cases pin it byte
// for byte: field names, their order, and which of them a response leaves out.
func TestResponsesRenderAsJSON(t *testing.T) {
	tests := []struct {
		name    string
		payload any
		want    string
	}{
		{
			name: "a finished auction",
			payload: api.NewAuctionResponse("r-1", auction.NewOutcome(
				matched("alpha", "beta"), nil, nil, 42*time.Millisecond,
			)),
			want: `{"request_id":"r-1","status":"ok",` +
				`"matched_dsps":["alpha","beta"],"sent":2,"succeeded":2,"duration_ms":42}`,
		},
		{
			// An empty match is an empty array. A publisher iterating the field
			// must not have to test it for null first.
			name:    "an auction nobody was invited to",
			payload: api.NewAuctionResponse("r-1", auction.Outcome{}),
			want: `{"request_id":"r-1","status":"no_matched_dsps",` +
				`"matched_dsps":[],"sent":0,"succeeded":0,"duration_ms":0}`,
		},
		{
			name: "a refusal naming what was wrong",
			payload: api.ErrorResponse{
				Error:   "request does not describe a lot the exchange can sell",
				Details: []string{`country: "russia" is not an ISO 3166-1 alpha-2 code`},
			},
			want: `{"error":"request does not describe a lot the exchange can sell",` +
				`"details":["country: \"russia\" is not an ISO 3166-1 alpha-2 code"]}`,
		},
		{
			name:    "a refusal with nothing to add leaves the details out",
			payload: api.ErrorResponse{Error: "request body is too large"},
			want:    `{"error":"request body is too large"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := json.Marshal(tt.payload)
			if err != nil {
				t.Fatalf("Marshal() returned %v, want the wire form", err)
			}
			if string(body) != tt.want {
				t.Errorf("rendered\n\t%s\nwant\n\t%s", body, tt.want)
			}
		})
	}
}
