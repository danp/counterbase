package source

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/danp/counterbase/directory"
	"github.com/danp/counterbase/query"
	"github.com/danp/counterbase/submit"
	"golang.org/x/exp/slices"
)

type Directory interface {
	Counters(context.Context) ([]directory.Counter, error)
}

type Querier interface {
	Query(ctx context.Context, q string) ([]query.Point, error)
}

type Crawler struct {
	Directory Directory
	Querier   Querier
	Submitter submit.Submitter

	getters map[string]Getter
}

type GetRequest struct {
	URL   *url.URL
	After time.Time
}

type Getter interface {
	Get(ctx context.Context, req GetRequest) ([]submit.Point, error)
}

func (c *Crawler) AddGetter(scheme string, getter Getter) {
	if c.getters == nil {
		c.getters = make(map[string]Getter)
	}
	c.getters[scheme] = getter
}

func (c *Crawler) Run(ctx context.Context) error {
	counters, err := c.Directory.Counters(ctx)
	if err != nil {
		return err
	}

	var getErrs []error
	for _, ctr := range counters {
		if !ctr.IsActive() {
			continue
		}

		for _, dir := range ctr.Directions {
			dsurl, err := url.Parse(dir.Source.URL)
			if err != nil {
				return err
			}

			gtr, ok := c.getters[dsurl.Scheme]
			if !ok {
				return fmt.Errorf("no getter for active counter %q direction %q source URL %q", ctr.ID, dir.ID, dir.Source.URL)
			}

			after := ctr.ServiceRanges[len(ctr.ServiceRanges)-1].Start.Add(-1 * time.Minute)
			latest, err := c.Querier.Query(ctx, "select time, value from latest_counter_data where counter_id='"+ctr.ID+"' and direction_id='"+dir.ID+"'")
			if err != nil {
				return err
			}
			if len(latest) > 0 {
				after = latest[0].Time
				if slices.Contains(ctr.Tags, "backdate1d") {
					after = after.AddDate(0, 0, -1)
					log.Println("backdating", ctr.ID, "request to", after.Format(time.RFC3339))
				}
			}

			pts, err := gtr.Get(ctx, GetRequest{URL: dsurl, After: after})
			if err != nil {
				getErrs = append(getErrs, fmt.Errorf("Get for %v %v after %v: %w", ctr.ID, dir, after, err))
				continue
			}

			req := submit.Request{
				ID:          ctr.ID,
				DirectionID: dir.ID,
				Points:      pts,
			}

			if err := c.Submitter.Submit(ctx, req); err != nil {
				return err
			}
		}
	}

	return errors.Join(getErrs...)
}
