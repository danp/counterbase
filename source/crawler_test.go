package source_test

import (
	"context"
	"testing"
	"time"

	"github.com/danp/counterbase/directory"
	"github.com/danp/counterbase/query"
	"github.com/danp/counterbase/source"
	"github.com/danp/counterbase/submit"
	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/common/model"
)

func TestCrawlerScheme(t *testing.T) {
	t.Parallel()

	now := time.Now()

	dir := fakeDirectory{
		C: []directory.Counter{
			{
				ID:   "test-1",
				Name: "Test counter",
				ServiceRanges: []directory.ServiceRange{
					{Start: directory.SD(now.Add(-5 * time.Hour))},
				},
				Directions: []directory.Direction{
					{
						ID:   "nb",
						Name: "northbound",
						Source: directory.Source{
							URL: "testscheme:1",
						},
					},
				},
				Mode: "cycling",
			},
		},
	}

	que := fakeQuerier{}

	get := &fakeGetter{
		P: []submit.Point{
			{Time: now.Add(-1 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 55},
		},
	}

	sub := &fakeSubmitter{}

	c := source.Crawler{
		Directory: dir,
		Querier:   que,
		Submitter: sub,
	}

	c.AddGetter("testscheme", get)

	if err := c.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []submit.Request{
		{
			ID:          "test-1",
			DirectionID: "nb",
			Points:      get.P,
		},
	}

	if d := cmp.Diff(want, sub.submits); d != "" {
		t.Error(d)
	}
}

func TestCrawlerUnknownScheme(t *testing.T) {
	t.Parallel()

	now := time.Now()

	dir := fakeDirectory{
		C: []directory.Counter{
			{
				ID:   "test-1",
				Name: "Test counter",
				ServiceRanges: []directory.ServiceRange{
					{Start: directory.SD(now.Add(-5 * time.Hour))},
				},
				Directions: []directory.Direction{
					{
						ID:   "nb",
						Name: "northbound",
						Source: directory.Source{
							URL: "unknown:1",
						},
					},
				},
				Mode: "cycling",
			},
		},
	}

	que := fakeQuerier{}

	sub := &fakeSubmitter{}

	c := source.Crawler{
		Directory: dir,
		Querier:   que,
		Submitter: sub,
	}

	if err := c.Run(context.Background()); err == nil {
		t.Fatal("wanted error")
	}
}

func TestCrawlerDefaultAfter(t *testing.T) {
	t.Parallel()

	now := time.Now()

	dir := fakeDirectory{
		C: []directory.Counter{
			{
				ID:   "test-1",
				Name: "Test counter",
				ServiceRanges: []directory.ServiceRange{
					{Start: directory.SD(now.Add(-5 * time.Hour))},
				},
				Directions: []directory.Direction{
					{
						ID:   "nb",
						Name: "northbound",
						Source: directory.Source{
							URL: "testscheme:1",
						},
					},
				},
				Mode: "cycling",
			},
		},
	}

	que := fakeQuerier{}

	get := &fakeGetter{
		P: []submit.Point{
			{Time: now.Add(-6 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 55},
			{Time: now.Add(-5 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 56},
			{Time: now.Add(-4 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 57},
		},
	}

	sub := &fakeSubmitter{}

	c := source.Crawler{
		Directory: dir,
		Querier:   que,
		Submitter: sub,
	}

	c.AddGetter("testscheme", get)

	if err := c.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []submit.Request{
		{
			ID:          "test-1",
			DirectionID: "nb",
			Points: []submit.Point{
				{Time: now.Add(-5 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 56},
				{Time: now.Add(-4 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 57},
			},
		},
	}

	if d := cmp.Diff(want, sub.submits); d != "" {
		t.Error(d)
	}
}

func TestCrawlerAfterLatest(t *testing.T) {
	t.Parallel()

	now := time.Now()

	dir := fakeDirectory{
		C: []directory.Counter{
			{
				ID:   "test-1",
				Name: "Test counter",
				ServiceRanges: []directory.ServiceRange{
					{Start: directory.SD(now.Add(-30 * time.Hour))},
				},
				Directions: []directory.Direction{
					{
						ID:   "nb",
						Name: "northbound",
						Source: directory.Source{
							URL: "testscheme:1",
						},
					},
				},
				Mode: "cycling",
			},
		},
	}

	que := fakeQuerier{
		P: map[string]query.Matrix{
			"select time, value from latest_counter_data where counter_id='test-1' and direction_id='nb'": {
				{Metric: model.Metric{"x": "y"}, Values: []model.SamplePair{{Timestamp: model.TimeFromUnix(now.Add(-5 * time.Hour).Unix()), Value: 3}}},
			},
		},
	}

	get := &fakeGetter{
		P: []submit.Point{
			{Time: now.Add(-6 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 55},
			{Time: now.Add(-5 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 56},
			{Time: now.Add(-4 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 57},
		},
	}

	sub := &fakeSubmitter{}

	c := source.Crawler{
		Directory: dir,
		Querier:   que,
		Submitter: sub,
	}

	c.AddGetter("testscheme", get)

	if err := c.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []submit.Request{
		{
			ID:          "test-1",
			DirectionID: "nb",
			Points: []submit.Point{
				{Time: now.Add(-4 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 57},
			},
		},
	}

	if d := cmp.Diff(want, sub.submits); d != "" {
		t.Error(d)
	}
}

func TestCrawlerIgnoresInactive(t *testing.T) {
	t.Parallel()

	now := time.Now()

	dir := fakeDirectory{
		C: []directory.Counter{
			{
				ID:   "test-1",
				Name: "Test counter",
				ServiceRanges: []directory.ServiceRange{
					{Start: directory.SD(now.Add(-30 * time.Hour)), End: directory.SD(now)},
				},
				Directions: []directory.Direction{
					{
						ID:   "nb",
						Name: "northbound",
						Source: directory.Source{
							URL: "testscheme:1",
						},
					},
				},
				Mode: "cycling",
			},
		},
	}

	que := fakeQuerier{}

	get := &fakeGetter{
		P: []submit.Point{
			{Time: now.Add(-6 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 55},
			{Time: now.Add(-5 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 56},
			{Time: now.Add(-4 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 57},
		},
	}

	sub := &fakeSubmitter{}

	c := source.Crawler{
		Directory: dir,
		Querier:   que,
		Submitter: sub,
	}

	c.AddGetter("testscheme", get)

	if err := c.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	var want []submit.Request
	if d := cmp.Diff(want, sub.submits); d != "" {
		t.Error(d)
	}
}

func TestCrawlerMultipleCounters(t *testing.T) {
	t.Parallel()

	now := time.Now()

	dir := fakeDirectory{
		C: []directory.Counter{
			{
				ID:   "cycling-1",
				Name: "Test cycling counter",
				ServiceRanges: []directory.ServiceRange{
					{Start: directory.SD(now.Add(-6 * time.Hour))},
				},
				Directions: []directory.Direction{
					{
						ID:   "nb",
						Name: "northbound",
						Source: directory.Source{
							URL: "testscheme:1",
						},
					},
				},
				Mode: "cycling",
			},
			{
				ID:   "cycling-2",
				Name: "Other cycling counter",
				ServiceRanges: []directory.ServiceRange{
					{Start: directory.SD(now.Add(-5 * time.Hour))},
				},
				Directions: []directory.Direction{
					{
						ID:   "sb",
						Name: "southbound",
						Source: directory.Source{
							URL: "testscheme:2",
						},
					},
				},
				Mode: "cycling",
			},
		},
	}

	que := fakeQuerier{}

	get := &fakeGetter{
		P: []submit.Point{
			{Time: now.Add(-6 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 55},
			{Time: now.Add(-5 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 56},
			{Time: now.Add(-4 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 57},
		},
	}

	sub := &fakeSubmitter{}

	c := source.Crawler{
		Directory: dir,
		Querier:   que,
		Submitter: sub,
	}

	c.AddGetter("testscheme", get)

	if err := c.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []submit.Request{
		{
			ID:          "cycling-1",
			DirectionID: "nb",
			Points: []submit.Point{
				{Time: now.Add(-6 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 55},
				{Time: now.Add(-5 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 56},
				{Time: now.Add(-4 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 57},
			},
		},
		{
			ID:          "cycling-2",
			DirectionID: "sb",
			Points: []submit.Point{
				{Time: now.Add(-5 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 56},
				{Time: now.Add(-4 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 57},
			},
		},
	}

	if d := cmp.Diff(want, sub.submits); d != "" {
		t.Error(d)
	}
}

func TestCrawlerMultipleDirections(t *testing.T) {
	t.Parallel()

	now := time.Now()

	dir := fakeDirectory{
		C: []directory.Counter{
			{
				ID:   "cycling-1",
				Name: "Test cycling counter",
				ServiceRanges: []directory.ServiceRange{
					{Start: directory.SD(now.Add(-6 * time.Hour))},
				},
				Directions: []directory.Direction{
					{
						ID:   "nb",
						Name: "northbound",
						Source: directory.Source{
							URL: "testscheme:1",
						},
					},
					{
						ID:   "sb",
						Name: "southbound",
						Source: directory.Source{
							URL: "testscheme:2",
						},
					},
				},
				Mode: "cycling",
			},
		},
	}

	que := fakeQuerier{}

	get := &fakeGetter{
		P: []submit.Point{
			{Time: now.Add(-6 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 55},
			{Time: now.Add(-5 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 56},
			{Time: now.Add(-4 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 57},
		},
	}

	sub := &fakeSubmitter{}

	c := source.Crawler{
		Directory: dir,
		Querier:   que,
		Submitter: sub,
	}

	c.AddGetter("testscheme", get)

	if err := c.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []submit.Request{
		{
			ID:          "cycling-1",
			DirectionID: "nb",
			Points: []submit.Point{
				{Time: now.Add(-6 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 55},
				{Time: now.Add(-5 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 56},
				{Time: now.Add(-4 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 57},
			},
		},
		{
			ID:          "cycling-1",
			DirectionID: "sb",
			Points: []submit.Point{
				{Time: now.Add(-6 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 55},
				{Time: now.Add(-5 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 56},
				{Time: now.Add(-4 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 57},
			},
		},
	}

	if d := cmp.Diff(want, sub.submits); d != "" {
		t.Error(d)
	}
}

func TestCrawlerMultipleSchemes(t *testing.T) {
	t.Parallel()

	now := time.Now()

	dir := fakeDirectory{
		C: []directory.Counter{
			{
				ID:   "cycling-1",
				Name: "Test cycling counter",
				ServiceRanges: []directory.ServiceRange{
					{Start: directory.SD(now.Add(-6 * time.Hour))},
				},
				Directions: []directory.Direction{
					{
						ID:   "nb",
						Name: "northbound",
						Source: directory.Source{
							URL: "testscheme:1",
						},
					},
				},
				Mode: "cycling",
			},
			{
				ID:   "cycling-2",
				Name: "Other cycling counter",
				ServiceRanges: []directory.ServiceRange{
					{Start: directory.SD(now.Add(-5 * time.Hour))},
				},
				Directions: []directory.Direction{
					{
						ID:   "sb",
						Name: "southbound",
						Source: directory.Source{
							URL: "otherscheme:2",
						},
					},
				},
				Mode: "cycling",
			},
		},
	}

	que := fakeQuerier{}

	get := &fakeGetter{
		P: []submit.Point{
			{Time: now.Add(-6 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 55},
			{Time: now.Add(-5 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 56},
			{Time: now.Add(-4 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 57},
		},
	}

	sub := &fakeSubmitter{}

	c := source.Crawler{
		Directory: dir,
		Querier:   que,
		Submitter: sub,
	}

	c.AddGetter("testscheme", get)
	c.AddGetter("otherscheme", get)

	if err := c.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	want := []submit.Request{
		{
			ID:          "cycling-1",
			DirectionID: "nb",
			Points: []submit.Point{
				{Time: now.Add(-6 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 55},
				{Time: now.Add(-5 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 56},
				{Time: now.Add(-4 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 57},
			},
		},
		{
			ID:          "cycling-2",
			DirectionID: "sb",
			Points: []submit.Point{
				{Time: now.Add(-5 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 56},
				{Time: now.Add(-4 * time.Hour).Unix(), Resolution: submit.ResolutionHour, Value: 57},
			},
		},
	}

	if d := cmp.Diff(want, sub.submits); d != "" {
		t.Error(d)
	}
}

type fakeDirectory struct {
	C []directory.Counter
}

func (f fakeDirectory) Counters(context.Context) ([]directory.Counter, error) {
	return f.C, nil
}

type fakeQuerier struct {
	P map[string]query.Matrix
}

func (f fakeQuerier) Query(ctx context.Context, q string) (query.Matrix, error) {
	return f.P[q], nil
}

type fakeGetter struct {
	P []submit.Point
}

func (f *fakeGetter) Get(ctx context.Context, req source.GetRequest) ([]submit.Point, error) {
	var out []submit.Point
	for _, p := range f.P {
		if time.Unix(p.Time, 0).After(req.After) {
			out = append(out, p)
		}
	}

	return out, nil
}

type fakeSubmitter struct {
	submits []submit.Request
}

func (f *fakeSubmitter) Submit(ctx context.Context, req submit.Request) error {
	f.submits = append(f.submits, req)
	return nil
}
