package query_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/danp/counterbase/query"
	"github.com/google/go-cmp/cmp"
)

func TestClientQuery(t *testing.T) {
	const q = `select time, sum(value) from counter_data where counter_id='south-park' and time >= 1616727600 and time < 1616814000 group by time`

	const resp = `{"database": "data", "query_name": null, "rows": [[1616727600, 1], [1616731200, 2]], "truncated": false, "columns": ["time", "sum(value)"], "query": {"sql": "select time, sum(value) from counter_data where counter_id='south-park' and time >= 1616727600 and time < 1616814000 group by time", "params": {}}, "private": false, "allow_execute_sql": true, "query_ms": 3.7816818803548813}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("sql"), q; got != want {
			t.Errorf("got query:\n%s\nwant:\n%s", got, want)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}))
	defer srv.Close()

	cl := &query.Client{
		URL: srv.URL,
	}

	mat, err := cl.Query(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}

	want := []query.Point{
		{Time: time.Unix(1616727600, 0), Value: 1},
		{Time: time.Unix(1616731200, 0), Value: 2},
	}
	if d := cmp.Diff(want, mat); d != "" {
		t.Error(d)
	}
}
