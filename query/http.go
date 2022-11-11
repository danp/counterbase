package query

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/prometheus/common/model"
)

type Client struct {
	URL string
}

func (c *Client) Query(ctx context.Context, q string) (Matrix, error) {
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

	var sps []model.SamplePair
	for _, row := range resps.Rows {
		sp := model.SamplePair{
			Timestamp: model.Time(int64(row[0])),
			Value:     model.SampleValue(row[1]),
		}
		sp.Timestamp *= 1000 // we store seconds, not millis
		sps = append(sps, sp)
	}

	mat := model.Matrix{
		{Metric: model.Metric{"x": "y"}, Values: sps},
	}

	return mat, nil
}
