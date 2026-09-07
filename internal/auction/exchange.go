package auction

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrInvitePanicked reports an invitation whose delivery panicked rather than
// returned.
var ErrInvitePanicked = errors.New("inviting the partner panicked")

// Inviter carries an invitation to bid to one partner and reports nil once the
// partner has answered. Declining the lot is an answer, not a failure.
//
// It must return once ctx is done: the exchange waits for every invitation it
// sends out.
type Inviter interface {
	Invite(ctx context.Context, endpoint Endpoint, lot Lot) error
}

// Exchange holds auctions. One exchange serves every auction the service holds.
type Exchange struct {
	roster  Roster
	inviter Inviter
	budget  time.Duration
}

// NewExchange creates an exchange. The budget must be positive: a budget of
// zero expires before the first invitation is away.
func NewExchange(roster Roster, inviter Inviter, budget time.Duration) Exchange {
	return Exchange{roster: roster, inviter: inviter, budget: budget}
}

// Conduct holds the auction for lot and reports what came of it. A partner that
// answers late, answers badly or says nothing is left out of the count; one
// partner's silence must not take the auction down with it.
func (e Exchange) Conduct(ctx context.Context, lot Lot) Outcome {
	started := time.Now()
	matched, excluded := e.roster.Consider(lot)

	var unanswered []Failure
	if len(matched) > 0 {
		unanswered = e.canvass(ctx, matched, lot)
	}

	return NewOutcome(matched, excluded, unanswered, time.Since(started))
}

// canvass invites every partner at once and returns the ones that came to
// nothing. All of them share one deadline, so the round never outlasts the
// budget. A sooner deadline on ctx ends the round earlier.
func (e Exchange) canvass(ctx context.Context, partners []Partner, lot Lot) []Failure {
	ctx, cancel := context.WithTimeout(ctx, e.budget)
	defer cancel()

	answers := make([]error, len(partners))
	var waiting sync.WaitGroup
	for i, partner := range partners {
		waiting.Add(1)
		go func() {
			defer waiting.Done()
			answers[i] = e.invite(ctx, partner, lot)
		}()
	}
	waiting.Wait()

	var unanswered []Failure
	for i, err := range answers {
		if err != nil {
			unanswered = append(unanswered, Failure{Partner: partners[i], Cause: err})
		}
	}
	return unanswered
}

// invite delivers one invitation, turning a panic into an error. The server
// recovers only the goroutine it calls a handler on, never the ones that handler
// starts, so a panic here would end the process.
func (e Exchange) invite(ctx context.Context, partner Partner, lot Lot) (err error) {
	defer func() {
		if panicked := recover(); panicked != nil {
			err = fmt.Errorf("%w: %v", ErrInvitePanicked, panicked)
		}
	}()
	return e.inviter.Invite(ctx, partner.Endpoint, lot)
}
