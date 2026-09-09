package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net/http"

	"github.com/andreychh/auction-router/internal/auction"
	"github.com/andreychh/auction-router/internal/parsing"
)

// maxRequestBodySize bounds the JSON a publisher may post. A lot runs to a few
// hundred bytes, so anything near this bound is a broken or hostile caller.
const maxRequestBodySize = 64 << 10

// mediaTypeJSON is the only form a lot may be posted in, and the form every
// answer is sent in.
const mediaTypeJSON = "application/json"

// Auctioneer holds an auction for a lot and reports what came of it. Finding no
// buyer is an outcome, not a failure, so there is no error to return.
type Auctioneer interface {
	Conduct(ctx context.Context, lot auction.Lot) auction.Outcome
}

// AuctionHandler serves the endpoint a publisher offers a lot to.
type AuctionHandler struct {
	auctioneer Auctioneer
	logger     *slog.Logger
}

// NewAuctionHandler creates a handler.
func NewAuctionHandler(auctioneer Auctioneer, logger *slog.Logger) *AuctionHandler {
	return &AuctionHandler{auctioneer: auctioneer, logger: logger}
}

// ServeHTTP implements [http.Handler].
func (h *AuctionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.checkMediaType(r.Header.Get("Content-Type")); err != nil {
		h.refuse(ctx, w, http.StatusUnsupportedMediaType, "", ErrorResponse{
			Error:   "request body must be declared as " + mediaTypeJSON,
			Details: []string{err.Error()},
		})
		return
	}

	var wire AuctionRequest
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodySize)).Decode(&wire)
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			h.refuse(ctx, w, http.StatusRequestEntityTooLarge, parsing.OrZero(wire.RequestID),
				ErrorResponse{Error: "request body is too large"})
			return
		}
		h.refuse(ctx, w, http.StatusBadRequest, parsing.OrZero(wire.RequestID), ErrorResponse{
			Error:   "request body could not be read as an auction request",
			Details: []string{err.Error()},
		})
		return
	}

	lot, err := ParseLot(wire)
	if err != nil {
		h.refuse(ctx, w, http.StatusBadRequest, parsing.OrZero(wire.RequestID), ErrorResponse{
			Error:   "request does not describe a lot the exchange can sell",
			Details: h.problems(err),
		})
		return
	}

	// ParseLot reported no error, so request_id was present.
	requestID := *wire.RequestID

	outcome := h.auctioneer.Conduct(ctx, lot)

	h.logger.InfoContext(ctx, "auction held",
		"request_id", requestID,
		"country", string(lot.Country),
		"device_type", string(lot.DeviceType),
		"matched", len(outcome.Matched),
		"filtered", len(outcome.Excluded),
		"succeeded", outcome.Answered(),
		"duration_ms", outcome.Elapsed.Milliseconds(),
	)

	for _, exclusion := range outcome.Excluded {
		h.logger.DebugContext(ctx, "partner passed over",
			"request_id", requestID,
			"partner", string(exclusion.Partner.UID),
			"term", string(exclusion.Term),
		)
	}

	for _, failure := range outcome.Unanswered {
		h.logger.WarnContext(ctx, "partner did not answer",
			"request_id", requestID,
			"partner", string(failure.Partner.UID),
			"error", failure.Cause,
		)
	}

	h.reply(ctx, w, http.StatusOK, NewAuctionResponse(requestID, outcome))
}

// refuse turns a request away, logging the same complaint it answers with.
// requestID is empty where the request broke before it could be read.
func (h *AuctionHandler) refuse(
	ctx context.Context,
	w http.ResponseWriter,
	status int,
	requestID string,
	payload ErrorResponse,
) {
	h.logger.WarnContext(ctx, "request refused",
		"request_id", requestID,
		"status", status,
		"error", payload.Error,
		"details", payload.Details,
	)
	h.reply(ctx, w, status, payload)
}

// reply sends payload as the whole of the response, encoding before it writes
// the status: a payload that will not render becomes a 500, not a truncated
// body under a 200.
func (h *AuctionHandler) reply(ctx context.Context, w http.ResponseWriter, status int, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		h.logger.ErrorContext(ctx, "response could not be encoded", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", mediaTypeJSON)
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		h.logger.WarnContext(ctx, "response could not be delivered", "error", err)
	}
}

// checkMediaType reports whether a body declared by header may be read as
// JSON. Parameters are disregarded, an undeclared body refused.
func (*AuctionHandler) checkMediaType(header string) error {
	if header == "" {
		return errors.New("Content-Type is absent")
	}
	media, _, err := mime.ParseMediaType(header)
	if err != nil {
		return fmt.Errorf("Content-Type %q cannot be read: %w", header, err)
	}
	if media != mediaTypeJSON {
		return fmt.Errorf("Content-Type is %q", media)
	}
	return nil
}

// problems flattens err into one message per complaint it carries, however many
// levels errors were joined at, so that a publisher reads its mistakes as
// separate items instead of one string with newlines buried in it.
func (h *AuctionHandler) problems(err error) []string {
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return []string{err.Error()}
	}

	var flattened []string
	for _, nested := range joined.Unwrap() {
		flattened = append(flattened, h.problems(nested)...)
	}
	return flattened
}
