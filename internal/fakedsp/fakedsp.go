// Package fakedsp stands in for the demand partners an exchange would otherwise
// invite over the network. It sends nothing: the end of a partner's endpoint
// decides what that partner answers.
package fakedsp

import (
	"context"
	"errors"
	"strings"

	"github.com/andreychh/auction-router/internal/auction"
)

// The endings that answer with something other than a bid.
const (
	endingSlow  = "/slow"
	endingError = "/error"
)

// Client answers invitations without leaving the process.
type Client struct{}

// NewClient creates a client.
func NewClient() Client {
	return Client{}
}

// Invite answers as the end of endpoint says: /slow holds until the round runs
// out of time, /error answers with a failure, and anything else answers at
// once. The lot is never read.
//
// A round that cannot run out of time is one /slow never answers in.
func (c Client) Invite(ctx context.Context, endpoint auction.Endpoint, _ auction.Lot) error {
	switch {
	case strings.HasSuffix(string(endpoint), endingSlow):
		<-ctx.Done()
		return ctx.Err()
	case strings.HasSuffix(string(endpoint), endingError):
		return errors.New("the partner answered with an error")
	default:
		return nil
	}
}
