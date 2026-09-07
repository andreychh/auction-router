package auction_test

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/andreychh/auction-router/internal/auction"
)

// inviter answers invitations the way a test tells it to.
type inviter func(ctx context.Context, endpoint auction.Endpoint, lot auction.Lot) error

func (f inviter) Invite(ctx context.Context, endpoint auction.Endpoint, lot auction.Lot) error {
	return f(ctx, endpoint, lot)
}

// named builds a partner reachable at an address made from its own name, so that
// an inviter can tell one from another by the endpoint it is handed.
func named(uid auction.PartnerUID) auction.Partner {
	return auction.Partner{
		UID:      uid,
		Enabled:  true,
		Endpoint: auction.Endpoint("http://" + uid + "/bid"),
	}
}

func TestExchangeConduct(t *testing.T) {
	lot := auction.Lot{Country: "RU", DeviceType: auction.Mobile, BidFloor: 1.5}

	t.Run("invites only the matched", func(t *testing.T) {
		roster := auction.NewRoster([]auction.Partner{
			named("alpha"),
			{UID: "beta", Enabled: false, Endpoint: "http://beta/bid"},
			named("gamma"),
		})

		var mu sync.Mutex
		var invited []auction.Endpoint
		answer := inviter(func(_ context.Context, endpoint auction.Endpoint, _ auction.Lot) error {
			mu.Lock()
			defer mu.Unlock()
			invited = append(invited, endpoint)
			return nil
		})

		outcome := auction.NewExchange(roster, answer, time.Second).Conduct(context.Background(), lot)

		// Invitations go out at once, so the order they arrive in is nobody's
		// promise; only the set of them is.
		slices.Sort(invited)
		want := []auction.Endpoint{"http://alpha/bid", "http://gamma/bid"}
		if !slices.Equal(invited, want) {
			t.Errorf("invited %q, want %q", invited, want)
		}
		if got := uids(outcome.Matched); !slices.Equal(got, []auction.PartnerUID{"alpha", "gamma"}) {
			t.Errorf("Matched = %q, want [alpha gamma]", got)
		}
		if len(outcome.Excluded) != 1 || outcome.Excluded[0].Partner.UID != "beta" {
			t.Errorf("Excluded = %+v, want beta alone", outcome.Excluded)
		}
		if outcome.Answered() != 2 {
			t.Errorf("Answered() = %d, want 2", outcome.Answered())
		}
	})

	t.Run("invites nobody when nobody matched", func(t *testing.T) {
		roster := auction.NewRoster([]auction.Partner{
			{UID: "alpha", Enabled: false, Endpoint: "http://alpha/bid"},
		})
		answer := inviter(func(context.Context, auction.Endpoint, auction.Lot) error {
			t.Error("invited a partner pretargeting had turned away")
			return nil
		})

		outcome := auction.NewExchange(roster, answer, time.Second).Conduct(context.Background(), lot)

		if len(outcome.Matched) != 0 || outcome.Answered() != 0 {
			t.Errorf("Matched = %q, Answered() = %d, want none and 0",
				uids(outcome.Matched), outcome.Answered())
		}
		if outcome.Elapsed <= 0 {
			t.Errorf("Elapsed = %v, want the pretargeting it did to be timed", outcome.Elapsed)
		}
	})

	// Invitations must go out at once. Partners each holding for a while would
	// take as many times as long one after another as they do together.
	t.Run("invites in parallel", func(t *testing.T) {
		const held = 50 * time.Millisecond

		partners := []auction.Partner{named("alpha"), named("beta"), named("gamma"), named("delta")}

		var mu sync.Mutex
		var inside, peak int
		answer := inviter(func(context.Context, auction.Endpoint, auction.Lot) error {
			mu.Lock()
			inside++
			peak = max(peak, inside)
			mu.Unlock()

			time.Sleep(held)

			mu.Lock()
			inside--
			mu.Unlock()
			return nil
		})

		started := time.Now()
		outcome := auction.NewExchange(auction.NewRoster(partners), answer, time.Second).
			Conduct(context.Background(), lot)
		elapsed := time.Since(started)

		if peak != len(partners) {
			t.Errorf("at most %d invitations were in flight at once, want %d", peak, len(partners))
		}
		if want := time.Duration(len(partners)) * held; elapsed >= want {
			t.Errorf("took %v, want well under the %v of one invitation after another", elapsed, want)
		}
		if outcome.Answered() != len(partners) {
			t.Errorf("Answered() = %d, want %d", outcome.Answered(), len(partners))
		}
	})

	// One partner failing, and one taking the process down with it, must cost the
	// auction nothing but that partner.
	t.Run("survives a failure and a panic", func(t *testing.T) {
		broken := errors.New("502 from the partner")
		roster := auction.NewRoster([]auction.Partner{named("alpha"), named("beta"), named("gamma")})
		answer := inviter(func(_ context.Context, endpoint auction.Endpoint, _ auction.Lot) error {
			switch endpoint {
			case "http://alpha/bid":
				return broken
			case "http://beta/bid":
				panic("the partner's client blew up")
			}
			return nil
		})

		outcome := auction.NewExchange(roster, answer, time.Second).Conduct(context.Background(), lot)

		if outcome.Answered() != 1 {
			t.Errorf("Answered() = %d, want 1 — only gamma answered", outcome.Answered())
		}
		if len(outcome.Unanswered) != 2 {
			t.Fatalf("Unanswered = %+v, want alpha and beta", outcome.Unanswered)
		}
		// Unanswered follows the order the partners were invited in.
		if got := outcome.Unanswered[0].Partner.UID; got != "alpha" {
			t.Errorf("first unanswered is %q, want alpha", got)
		}
		if !errors.Is(outcome.Unanswered[0].Cause, broken) {
			t.Errorf("alpha's cause = %v, want the error it returned", outcome.Unanswered[0].Cause)
		}
		if got := outcome.Unanswered[1].Partner.UID; got != "beta" {
			t.Errorf("second unanswered is %q, want beta", got)
		}
		if !errors.Is(outcome.Unanswered[1].Cause, auction.ErrInvitePanicked) {
			t.Errorf("beta's cause = %v, want it to name the panic", outcome.Unanswered[1].Cause)
		}
	})

	// The budget is spent once, not once per partner: every invitation is handed
	// the same instant to finish by. Wall-clock timing cannot show this —
	// invitations that each started their own budget would run out at almost the
	// same moment anyway — so the test reads the deadline each of them was given.
	t.Run("hands every invitation one deadline", func(t *testing.T) {
		partners := []auction.Partner{named("alpha"), named("beta"), named("gamma"), named("delta")}

		var mu sync.Mutex
		var deadlines []time.Time
		answer := inviter(func(ctx context.Context, _ auction.Endpoint, _ auction.Lot) error {
			deadline, set := ctx.Deadline()
			if !set {
				t.Error("an invitation was given no deadline at all")
				return nil
			}
			mu.Lock()
			defer mu.Unlock()
			deadlines = append(deadlines, deadline)
			return nil
		})

		auction.NewExchange(auction.NewRoster(partners), answer, time.Second).
			Conduct(context.Background(), lot)

		if len(deadlines) != len(partners) {
			t.Fatalf("read %d deadlines, want %d", len(deadlines), len(partners))
		}
		for _, deadline := range deadlines[1:] {
			if !deadline.Equal(deadlines[0]) {
				t.Errorf("deadlines differ by %v, want every invitation to share one",
					deadline.Sub(deadlines[0]))
			}
		}
	})

	// The budget covers the round, so partners that never answer cost one budget
	// between them rather than one each.
	t.Run("spends one budget on the whole round", func(t *testing.T) {
		const budget = 100 * time.Millisecond

		partners := []auction.Partner{named("alpha"), named("beta"), named("gamma")}
		answer := inviter(func(ctx context.Context, _ auction.Endpoint, _ auction.Lot) error {
			<-ctx.Done()
			return ctx.Err()
		})

		started := time.Now()
		outcome := auction.NewExchange(auction.NewRoster(partners), answer, budget).
			Conduct(context.Background(), lot)
		elapsed := time.Since(started)

		if elapsed < budget {
			t.Errorf("took %v, want at least the %v budget", elapsed, budget)
		}
		if want := time.Duration(len(partners)) * budget; elapsed >= want {
			t.Errorf("took %v, want one budget rather than %d of them", elapsed, len(partners))
		}
		if len(outcome.Unanswered) != len(partners) {
			t.Fatalf("Unanswered = %+v, want all %d", outcome.Unanswered, len(partners))
		}
		for _, failure := range outcome.Unanswered {
			if !errors.Is(failure.Cause, context.DeadlineExceeded) {
				t.Errorf("%q's cause = %v, want the deadline", failure.Partner.UID, failure.Cause)
			}
		}
	})

	// Waiting out a budget for an answer nobody is left to read is waste, so a
	// caller's own deadline ends the round when it falls sooner.
	t.Run("stops at the caller's deadline", func(t *testing.T) {
		const (
			budget   = 2 * time.Second
			deadline = 50 * time.Millisecond
		)

		roster := auction.NewRoster([]auction.Partner{named("alpha")})
		answer := inviter(func(ctx context.Context, _ auction.Endpoint, _ auction.Lot) error {
			<-ctx.Done()
			return ctx.Err()
		})

		ctx, cancel := context.WithTimeout(context.Background(), deadline)
		defer cancel()

		started := time.Now()
		outcome := auction.NewExchange(roster, answer, budget).Conduct(ctx, lot)
		elapsed := time.Since(started)

		if elapsed >= budget {
			t.Errorf("took %v, want the %v deadline to beat the %v budget", elapsed, deadline, budget)
		}
		if len(outcome.Unanswered) != 1 {
			t.Fatalf("Unanswered = %+v, want alpha alone", outcome.Unanswered)
		}
	})

	// Conduct returns only once every invitation has come back, however it came
	// back. Nothing it started may outlive it.
	t.Run("leaves no goroutine behind", func(t *testing.T) {
		roster := auction.NewRoster([]auction.Partner{named("alpha"), named("beta"), named("gamma")})
		answer := inviter(func(ctx context.Context, endpoint auction.Endpoint, _ auction.Lot) error {
			switch endpoint {
			case "http://alpha/bid":
				<-ctx.Done()
				return ctx.Err()
			case "http://beta/bid":
				panic("the partner's client blew up")
			}
			return nil
		})

		before := runtime.NumGoroutine()
		auction.NewExchange(roster, answer, 50*time.Millisecond).Conduct(context.Background(), lot)

		// Goroutines from elsewhere may still be settling, so give the count a
		// moment to come back rather than reading it the instant Conduct returns.
		deadline := time.Now().Add(time.Second)
		for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		if after := runtime.NumGoroutine(); after > before {
			t.Errorf("%d goroutines before Conduct, %d after", before, after)
		}
	})
}
