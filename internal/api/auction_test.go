package api_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andreychh/auction-router/internal/api"
	"github.com/andreychh/auction-router/internal/auction"
)

// auctioneer holds the auction a test tells it to.
type auctioneer func(ctx context.Context, lot auction.Lot) auction.Outcome

func (f auctioneer) Conduct(ctx context.Context, lot auction.Lot) auction.Outcome {
	return f(ctx, lot)
}

// unwanted fails the test if it is offered a lot at all, which is what a refused
// request must never reach.
func unwanted(t *testing.T) auctioneer {
	return func(context.Context, auction.Lot) auction.Outcome {
		t.Error("held an auction for a request the handler should have refused")
		return auction.Outcome{}
	}
}

// serve offers r to a handler backed by a and returns everything written back.
// The logger discards, since no case here reads the log.
func serve(a api.Auctioneer, r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	api.NewAuctionHandler(a, slog.New(slog.DiscardHandler)).ServeHTTP(w, r)
	return w
}

// request builds the POST a publisher makes, declared as the JSON it is.
func request(body string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/auction", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	return r
}

// declared builds the same POST under a media type of the test's choosing,
// where an empty one means the header is left off altogether.
func declared(body, mediaType string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "/auction", strings.NewReader(body))
	if mediaType != "" {
		r.Header.Set("Content-Type", mediaType)
	}
	return r
}

// refusal reads the answer to a refused request.
func refusal(t *testing.T, w *httptest.ResponseRecorder) api.ErrorResponse {
	t.Helper()
	var got api.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("answer %q does not read as a refusal: %v", w.Body, err)
	}
	return got
}

func TestAuctionHandlerServeHTTP(t *testing.T) {
	const lot = `{"request_id":"r-1","country":"RU","device_type":"mobile",` +
		`"bid_floor":1.5,"categories":["news"]}`

	t.Run("an auction with buyers is answered with what came of it", func(t *testing.T) {
		held := auctioneer(func(context.Context, auction.Lot) auction.Outcome {
			return auction.NewOutcome(
				matched("alpha", "beta"),
				nil,
				make([]auction.Failure, 1),
				42*time.Millisecond,
			)
		})

		w := serve(held, request(lot))

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if got := w.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		want := `{"request_id":"r-1","status":"ok","matched_dsps":["alpha","beta"],` +
			`"sent":2,"succeeded":1,"duration_ms":42}`
		if got := w.Body.String(); got != want {
			t.Errorf("answered\n\t%s\nwant\n\t%s", got, want)
		}
	})

	// The exchange still ran, so every counter it earned is reported; the status
	// is what says the lot went nowhere.
	t.Run("a lot that suited nobody is still an answer, not a failure", func(t *testing.T) {
		held := auctioneer(func(context.Context, auction.Lot) auction.Outcome {
			return auction.NewOutcome(nil, make([]auction.Exclusion, 3), nil, time.Millisecond)
		})

		w := serve(held, request(lot))

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d — nobody to sell to is not a failure", w.Code, http.StatusOK)
		}
		want := `{"request_id":"r-1","status":"no_matched_dsps","matched_dsps":[],` +
			`"sent":0,"succeeded":0,"duration_ms":1}`
		if got := w.Body.String(); got != want {
			t.Errorf("answered\n\t%s\nwant\n\t%s", got, want)
		}
	})

	// Parsing is proved on its own elsewhere; what only the handler can show is
	// that the lot it holds an auction for is the one the JSON described.
	t.Run("the lot offered is the one the request described", func(t *testing.T) {
		var offered auction.Lot
		held := auctioneer(func(_ context.Context, got auction.Lot) auction.Outcome {
			offered = got
			return auction.Outcome{}
		})

		serve(held, request(lot))

		want := auction.Lot{
			Country:    "RU",
			DeviceType: auction.Mobile,
			BidFloor:   1.5,
			Categories: []auction.Category{"news"},
		}
		if !sameLot(offered, want) {
			t.Errorf("offered %+v, want %+v", offered, want)
		}
	})

	// The exchange gives every invitation the request's own context, so a
	// publisher that hangs up ends the round instead of leaving partners to be
	// canvassed for an answer nobody will read.
	t.Run("the request's context reaches the auctioneer", func(t *testing.T) {
		type marker struct{}

		var reached bool
		held := auctioneer(func(ctx context.Context, _ auction.Lot) auction.Outcome {
			reached = ctx.Value(marker{}) == "the request's own"
			return auction.Outcome{}
		})

		r := request(lot)
		r = r.WithContext(context.WithValue(r.Context(), marker{}, "the request's own"))
		serve(held, r)

		if !reached {
			t.Error("the auctioneer was handed a context of its own, want the request's")
		}
	})

	// A body is read only once it says what it is. A client that mislabels what
	// it sends is misconfigured, and reading it anyway would make the label a
	// decoration.
	t.Run("a body not declared as JSON is refused unread", func(t *testing.T) {
		tests := []struct {
			name      string
			mediaType string
		}{
			{name: "another media type entirely", mediaType: "text/plain"},
			{name: "the form curl -d sends by default", mediaType: "application/x-www-form-urlencoded"},
			{name: "no declaration at all", mediaType: ""},
			{name: "a declaration that does not parse", mediaType: "application/json; charset"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				w := serve(unwanted(t), declared(lot, tt.mediaType))

				if w.Code != http.StatusUnsupportedMediaType {
					t.Errorf("status = %d, want %d", w.Code, http.StatusUnsupportedMediaType)
				}
				if got := refusal(t, w); got.Error == "" {
					t.Error("the refusal says nothing about what was wrong")
				}
			})
		}
	})

	// The parameters a media type carries say how to read a body, not what is in
	// it, so naming a charset must not cost a publisher its auction.
	t.Run("JSON declared with parameters is read all the same", func(t *testing.T) {
		held := auctioneer(func(context.Context, auction.Lot) auction.Outcome {
			return auction.NewOutcome(matched("alpha"), nil, nil, time.Millisecond)
		})

		w := serve(held, declared(lot, "application/json; charset=utf-8"))

		if w.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("a body that is not JSON is refused unread", func(t *testing.T) {
		w := serve(unwanted(t), request(`{"request_id":`))

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		if got := w.Header().Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want a refusal in JSON too", got)
		}
		if got := refusal(t, w); got.Error == "" {
			t.Error("the refusal says nothing about what was wrong")
		}
	})

	t.Run("a request missing fields is refused, naming them", func(t *testing.T) {
		w := serve(unwanted(t), request(`{"bid_floor":1.5}`))

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		got := refusal(t, w)
		if len(got.Details) != 1 {
			t.Fatalf("details = %q, want the one complaint naming the absent fields", got.Details)
		}
		for _, field := range []string{"request_id", "country", "device_type"} {
			if !strings.Contains(got.Details[0], field) {
				t.Errorf("details = %q, want %s named among the absent", got.Details[0], field)
			}
		}
	})

	// Every complaint arrives as its own item. Joined errors print as one string
	// with newlines in it, which a publisher would have to split apart itself.
	// The complaints about the list arrive joined already, so this is also where
	// a nested join has to be flattened rather than rendered.
	t.Run("a request of bad values is answered with each complaint apart", func(t *testing.T) {
		w := serve(unwanted(t), request(
			`{"request_id":"r-1","country":"russia","device_type":"fridge","bid_floor":-1,`+
				`"categories":[""," news"]}`,
		))

		if w.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
		}
		got := refusal(t, w)
		if len(got.Details) != 5 {
			t.Fatalf("details = %q, want one item for each of the five bad values", got.Details)
		}
		for _, complaint := range got.Details {
			if strings.Contains(complaint, "\n") {
				t.Errorf("complaint %q carries newlines, want the complaints apart", complaint)
			}
		}
	})

	// A lot runs to a few hundred bytes, so a body this size is a broken or
	// hostile caller and is turned away before it is read to the end.
	t.Run("a body far larger than any lot is refused as too large", func(t *testing.T) {
		bloated := `{"request_id":"r-1","junk":"` + strings.Repeat("A", 1<<20) + `"}`

		w := serve(unwanted(t), request(bloated))

		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
		}
		if got := refusal(t, w); got.Error == "" {
			t.Error("the refusal says nothing about what was wrong")
		}
	})
}
