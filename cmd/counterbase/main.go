package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/danp/counterbase/directory"
	"github.com/danp/counterbase/query"
	"github.com/danp/counterbase/source"
	"github.com/danp/counterbase/submit"
	"github.com/peterbourgon/ff/v3/ffcli"
	"github.com/prometheus/common/model"
	_ "modernc.org/sqlite"
)

func main() {
	var (
		rootFlagSet = flag.NewFlagSet("counterbase", flag.ExitOnError)
	)

	dbg := databaseGetter{
		file: "data.db",
	}

	stg := storageGetter{
		getDB: dbg.get,
	}

	dg := directoryGetter{}

	sg := submitGetter{stg: stg.get}

	qg := queryGetter{}

	var (
		apiCmd      = newAPICmd(stg.get)
		crawlerCmd  = dg.addFlags(sg.addFlags(qg.addFlags(newCrawlerCmd(dg.get, sg.get, qg.get))))
		discoverCmd = newDiscoverCmd()
	)

	root := &ffcli.Command{
		ShortUsage: "counterbase [flags] <subcommand>",
		Subcommands: []*ffcli.Command{
			apiCmd,
			crawlerCmd,
			discoverCmd,
		},
		FlagSet: rootFlagSet,
		Exec: func(context.Context, []string) error {
			return flag.ErrHelp
		},
	}

	if err := root.ParseAndRun(context.Background(), os.Args[1:]); err != nil {
		if err != flag.ErrHelp {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

type commaSeparatedString struct {
	vals []string
}

func (c *commaSeparatedString) Set(s string) error {
	c.vals = strings.Split(s, ",")
	return nil
}

func (c *commaSeparatedString) String() string {
	return strings.Join(c.vals, ",")
}

type databaseGetter struct {
	file string
}

func (d databaseGetter) get(ctx context.Context) (*sql.DB, error) {
	return sql.Open("sqlite", d.file)
}

type storageGetter struct {
	getDB func(context.Context) (*sql.DB, error)
}

func (s storageGetter) get(ctx context.Context) (*dbStorage, error) {
	db, err := s.getDB(ctx)
	if err != nil {
		return nil, err
	}

	st := &dbStorage{db: db}
	return st, st.init(ctx)
}

type directoryGetter struct {
	directoryURL *string
}

func (g *directoryGetter) addFlags(cmd *ffcli.Command) *ffcli.Command {
	g.directoryURL = cmd.FlagSet.String("directory-url", "", "directory URL")
	return cmd
}

func (g *directoryGetter) get(ctx context.Context) (source.Directory, error) {
	var counters []directory.Counter

	if g.directoryURL == nil || *g.directoryURL == "" {
		return nil, fmt.Errorf("need -directory-url")
	}

	u, err := url.Parse(*g.directoryURL)
	if err != nil {
		return nil, fmt.Errorf("parsing -directory-url: %w", err)
	}

	var src string
	switch u.Scheme {
	case "file":
		src = u.Path

		f, err := os.Open(src)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		if err := json.NewDecoder(f).Decode(&counters); err != nil {
			return nil, err
		}
	case "http", "https":
		src = u.String()

		resp, err := http.Get(src)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("-directory-url: bad status %d", resp.StatusCode)
		}

		if err := json.NewDecoder(resp.Body).Decode(&counters); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("-directory-url: unsupported scheme %q", u.Scheme)
	}

	log.Println("loaded", len(counters), "counters from", src)

	dir := fakeDirectory{C: counters}
	return dir, nil
}

type submitGetter struct {
	submitURL *string
	stg       func(ctx context.Context) (*dbStorage, error)
}

func (q *submitGetter) addFlags(cmd *ffcli.Command) *ffcli.Command {
	q.submitURL = cmd.FlagSet.String("submit-url", "", "submit endpoint URL")
	return cmd
}

func (q *submitGetter) get(ctx context.Context) (submit.Submitter, error) {
	if *q.submitURL == "" {
		return nil, fmt.Errorf("need -submit-url")
	}

	su, err := url.Parse(*q.submitURL)
	if err != nil {
		return nil, err
	}

	switch su.Scheme {
	case "http", "https":
		cl := &submit.Client{
			URL: *q.submitURL,
		}
		return cl, nil
	case "sqlite":
		return q.stg(ctx)
	}

	return nil, fmt.Errorf("bad -submit-url")
}

type queryGetter struct {
	queryURL *string
}

func (q *queryGetter) addFlags(cmd *ffcli.Command) *ffcli.Command {
	q.queryURL = cmd.FlagSet.String("query-url", "", "query endpoint URL")
	return cmd
}

func (q *queryGetter) get(ctx context.Context) (source.Querier, error) {
	if q.queryURL != nil && *q.queryURL != "" {
		cl := &query.Client{
			URL: *q.queryURL,
		}
		return cl, nil
	}

	return nil, fmt.Errorf("need -query-url")
}

type fakeDirectory struct {
	C []directory.Counter
}

func (f fakeDirectory) Counters(context.Context) ([]directory.Counter, error) {
	return f.C, nil
}

type dbStorage struct {
	db *sql.DB
}

func (s dbStorage) init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "create table if not exists counter_data (counter_id text not null, direction_id text not null, time integer not null, resolution integer not null, value numeric not null, primary key(counter_id, direction_id, time)); create view if not exists latest_counter_data as with latest_times as (select counter_id, direction_id, max(time) as time from counter_data group by 1, 2) select counter_data.* from counter_data, latest_times where counter_data.counter_id=latest_times.counter_id and counter_data.direction_id=latest_times.direction_id and counter_data.time=latest_times.time")
	return err
}

func (s dbStorage) Query(ctx context.Context, q string) (query.Matrix, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sps []model.SamplePair
	for rows.Next() {
		var sp model.SamplePair
		if err := rows.Scan(&sp.Timestamp, &sp.Value); err != nil {
			return nil, err
		}
		sp.Timestamp *= 1000 // we store seconds, not millis
		sps = append(sps, sp)
	}

	mat := model.Matrix{
		{Metric: model.Metric{"x": "y"}, Values: sps},
	}

	return mat, rows.Err()
}

func (s dbStorage) Submit(ctx context.Context, req submit.Request) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var sum int
	var tmin, tmax int64
	for _, pt := range req.Points {
		if _, err := tx.ExecContext(ctx, "replace into counter_data (counter_id, direction_id, time, resolution, value) values (?, ?, ?, ?, ?)",
			req.ID, req.DirectionID, pt.Time, pt.Resolution, pt.Value,
		); err != nil {
			return fmt.Errorf("adding counter %q direction %q pt %v: %w", req.ID, req.DirectionID, pt, err)
		}
		sum += int(pt.Value)
		if tmin == 0 || pt.Time < tmin {
			tmin = pt.Time
		}
		if pt.Time > tmax {
			tmax = pt.Time
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if pl := len(req.Points); pl > 0 {
		log.Println(req.ID, req.DirectionID, "added", pl, "points for range", tmin, tmax, "with sum", sum)
	}

	return nil
}

func (s dbStorage) Close() error {
	return s.db.Close()
}
