package query

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	URL string
}

func (c *Client) Query(ctx context.Context, q string) ([]Point, error) {
	u, err := url.Parse(c.URL)
	if err != nil {
		return nil, err
	}

	uq := u.Query()
	uq.Set("sql", q)
	u.RawQuery = uq.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got bad status %d", resp.StatusCode)
	}

	var resps struct {
		Rows [][]float64
	}
	if err := json.Unmarshal(b, &resps); err != nil {
		return nil, err
	}

	var pts []Point
	for _, row := range resps.Rows {
		p := Point{
			Time:  time.Unix(int64(row[0]), 0),
			Value: row[1],
		}
		pts = append(pts, p)
	}

	return pts, nil
}
