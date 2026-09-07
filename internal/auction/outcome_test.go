package auction_test

import (
	"testing"

	"github.com/andreychh/auction-router/internal/auction"
)

// Answered reads nothing but the length of the two lists, so the cases give
// their sizes and leave the entries at their zero values.
func TestOutcomeAnswered(t *testing.T) {
	tests := []struct {
		name       string
		matched    int
		unanswered int
		want       int
	}{
		{name: "every invited partner answered", matched: 3, unanswered: 0, want: 3},
		{name: "one of three said nothing", matched: 3, unanswered: 1, want: 2},
		{name: "none of the invited answered", matched: 3, unanswered: 3, want: 0},
		{name: "nobody was invited", matched: 0, unanswered: 0, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outcome := auction.NewOutcome(
				make([]auction.Partner, tt.matched),
				nil,
				make([]auction.Failure, tt.unanswered),
				0,
			)
			if got := outcome.Answered(); got != tt.want {
				t.Errorf("Answered() = %d, want %d; %d invited, %d said nothing",
					got, tt.want, tt.matched, tt.unanswered)
			}
		})
	}
}
