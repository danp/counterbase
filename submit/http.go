package submit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Request struct {
	ID          string  `json:"id"`
	DirectionID string  `json:"direction_id"`
	Points      []Point `json:"points"`
}

type Point struct {
	Time       int64      `json:"time"`
	Resolution Resolution `json:"resolution"`
	Value      float64    `json:"value"`
}

type Resolution int

const (
	ResolutionMinute Resolution = 1
	ResolutionHour   Resolution = 2
	ResolutionDay    Resolution = 3
)

type Submitter interface {
	Submit(context.Context, Request) error
}

type Handler struct {
	Submitter Submitter
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req Request

	b, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := json.Unmarshal(b, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := h.Submitter.Submit(r.Context(), req); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type Client struct {
	URL string
}

func (c *Client) Submit(ctx context.Context, req Request) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}

	resp, err := http.Post(c.URL, "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("bad status %d", resp.StatusCode)
	}

	return nil
}
