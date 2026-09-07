package auction

import "time"

// Outcome is what the exchange has to show for a lot once its auction has run:
// who was worth asking, who would not hear of it, who was asked and said
// nothing, and how long the whole thing took.
type Outcome struct {
	// Matched are the partners pretargeting found the lot suited, in roster
	// order. Every one of them was invited to bid.
	Matched []Partner

	// Excluded holds one exclusion for every partner pretargeting turned away.
	Excluded []Exclusion

	// Unanswered holds one entry for every invited partner whose invitation came
	// to nothing. A partner that answered only to decline the lot is not among
	// them.
	Unanswered []Failure

	// Elapsed is how long the auction took, pretargeting included.
	Elapsed time.Duration
}

// NewOutcome creates an outcome.
func NewOutcome(
	matched []Partner,
	excluded []Exclusion,
	unanswered []Failure,
	elapsed time.Duration,
) Outcome {
	return Outcome{
		Matched:    matched,
		Excluded:   excluded,
		Unanswered: unanswered,
		Elapsed:    elapsed,
	}
}

// Answered counts the invited partners that answered inside the time allowed.
func (o Outcome) Answered() int {
	return len(o.Matched) - len(o.Unanswered)
}

// Failure is an invitation that came to nothing: which partner did not answer
// it, and what stood in the way.
type Failure struct {
	Partner Partner
	Cause   error
}
