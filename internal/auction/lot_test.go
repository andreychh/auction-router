package auction_test

import (
	"testing"

	"github.com/andreychh/auction-router/internal/auction"
)

func TestLotMentionsAny(t *testing.T) {
	tests := []struct {
		name string
		own  []auction.Category
		list []auction.Category
		want bool
	}{
		{
			name: "the only category is on the list",
			own:  []auction.Category{"news"},
			list: []auction.Category{"news"},
			want: true,
		},
		{
			name: "no category of the lot's is on the list",
			own:  []auction.Category{"news", "sport"},
			list: []auction.Category{"gambling", "adult"},
			want: false,
		},
		{
			name: "the match is not the first of the lot's own",
			own:  []auction.Category{"news", "sport", "gambling"},
			list: []auction.Category{"gambling"},
			want: true,
		},
		{
			name: "the match is not the first on the list",
			own:  []auction.Category{"gambling"},
			list: []auction.Category{"adult", "games", "gambling"},
			want: true,
		},
		{
			name: "several categories overlap",
			own:  []auction.Category{"news", "gambling", "adult"},
			list: []auction.Category{"adult", "gambling"},
			want: true,
		},
		{
			name: "case has to agree exactly",
			own:  []auction.Category{"News"},
			list: []auction.Category{"news"},
			want: false,
		},
		{
			name: "a lot mentioning nothing matches no list",
			own:  nil,
			list: []auction.Category{"gambling"},
			want: false,
		},
		{
			name: "an empty list is matched by nothing",
			own:  []auction.Category{"news"},
			list: nil,
			want: false,
		},
		{
			name: "neither side mentions anything",
			own:  nil,
			list: nil,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lot := auction.NewLot("RU", auction.Mobile, 1.5, tt.own)
			if got := lot.MentionsAny(tt.list); got != tt.want {
				t.Errorf("lot mentioning %q, MentionsAny(%q) = %v, want %v",
					tt.own, tt.list, got, tt.want)
			}
		})
	}
}
