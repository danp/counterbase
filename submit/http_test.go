package submit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/danp/counterbase/submit"
	"github.com/google/go-cmp/cmp"
)

func TestHandler(t *testing.T) {
	fs := &fakeSubmitter{}

	he := &submit.Handler{
		Submitter: fs,
	}

	srv := httptest.NewServer(he)
	defer srv.Close()

	req := submit.Request{
		ID:          "first",
		DirectionID: "one",
		Points: []submit.Point{
			{Time: 1, Value: 2, Resolution: submit.ResolutionHour},
		},
	}

	reqb, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := srv.Client().Post(srv.URL, "application/json", bytes.NewReader(reqb))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusNoContent; got != want {
		t.Fatalf("got status %d, want %d", got, want)
	}

	if d := cmp.Diff([]submit.Request{req}, fs.submits); d != "" {
		t.Error(d)
	}
}

func TestHandlerBadData(t *testing.T) {
	fs := &fakeSubmitter{}

	he := &submit.Handler{
		Submitter: fs,
	}

	srv := httptest.NewServer(he)
	defer srv.Close()

	resp, err := srv.Client().Post(srv.URL, "application/json", strings.NewReader("no"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusBadRequest; got != want {
		t.Fatalf("got status %d, want %d", got, want)
	}

	if len(fs.submits) != 0 {
		t.Errorf("got %d submits, wanted 0", len(fs.submits))
	}
}

func TestHandlerSubmitError(t *testing.T) {
	fs := &fakeSubmitter{err: errors.New("boom")}

	he := &submit.Handler{
		Submitter: fs,
	}

	srv := httptest.NewServer(he)
	defer srv.Close()

	req := submit.Request{
		ID:          "first",
		DirectionID: "one",
		Points: []submit.Point{
			{Time: 1, Value: 2, Resolution: submit.ResolutionHour},
		},
	}

	reqb, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	resp, err := srv.Client().Post(srv.URL, "application/json", bytes.NewReader(reqb))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if got, want := resp.StatusCode, http.StatusInternalServerError; got != want {
		t.Fatalf("got status %d, want %d", got, want)
	}
}

func TestClient(t *testing.T) {
	var gotReq submit.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Content-Type"), "application/json"; got != want {
			t.Errorf("got Content-Type %q, want %q", got, want)
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}

		if err := json.Unmarshal(b, &gotReq); err != nil {
			t.Error(err)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	cl := &submit.Client{
		URL: srv.URL,
	}

	req := submit.Request{
		ID:          "first",
		DirectionID: "one",
		Points: []submit.Point{
			{Time: 1, Value: 2, Resolution: submit.ResolutionHour},
		},
	}

	if err := cl.Submit(context.Background(), req); err != nil {
		t.Fatal(err)
	}

	if d := cmp.Diff(req, gotReq); d != "" {
		t.Error(d)
	}
}

type fakeSubmitter struct {
	err     error
	submits []submit.Request
}

func (f *fakeSubmitter) Submit(ctx context.Context, req submit.Request) error {
	f.submits = append(f.submits, req)
	return f.err
}
