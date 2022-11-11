package source

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/danp/counterbase/directory"
	"github.com/danp/counterbase/submit"
	"golang.org/x/exp/slices"
)

type HalifaxTransit struct {
	fo   sync.Once
	err  error
	data map[string]halifaxTransitRoute
}

func (h *HalifaxTransit) Get(ctx context.Context, req GetRequest) ([]submit.Point, error) {
	h.fo.Do(func() {
		h.data = make(map[string]halifaxTransitRoute)
		h.err = h.fetch(ctx)
	})
	if h.err != nil {
		return nil, h.err
	}

	var out []submit.Point
	for _, pt := range h.data[req.URL.Opaque].points {
		if !pt.day.After(req.After) {
			continue
		}

		out = append(out, submit.Point{
			Time:       pt.day.Unix(),
			Resolution: submit.ResolutionDay,
			Value:      float64(pt.count),
		})
	}

	return out, nil
}

func (h *HalifaxTransit) Counters(ctx context.Context) ([]directory.Counter, error) {
	h.fo.Do(func() {
		h.data = make(map[string]halifaxTransitRoute)
		h.err = h.fetch(ctx)
	})
	if h.err != nil {
		return nil, h.err
	}

	var lastDay time.Time
	for _, rt := range h.data {
		if day := rt.points[len(rt.points)-1].day; day.After(lastDay) {
			lastDay = day
		}
	}

	var counters []directory.Counter
	for _, rt := range h.data {
		firstDay := rt.points[0].day

		var c directory.Counter
		c.ID = rt.number
		c.Mode = "bus"
		c.Name = rt.name
		c.Directions = []directory.Direction{
			{ID: "non", Name: "nondirectional", Source: directory.Source{URL: "hfxtransit:" + rt.number}},
		}

		sr := directory.ServiceRange{Start: directory.SD(firstDay)}
		if lpd := rt.points[len(rt.points)-1].day; lastDay.Sub(lpd) > 7*24*time.Hour {
			sr.End = directory.SD(lpd)
		}

		c.ServiceRanges = append(c.ServiceRanges, sr)
		counters = append(counters, c)
	}
	return counters, nil
}

func (h *HalifaxTransit) fetch(ctx context.Context) error {
	loc, err := time.LoadLocation("America/Halifax")
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://opendata.arcgis.com/datasets/a0ece3efdc7144d69cb1881b90cd93fe_0.csv", nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status %d", resp.StatusCode)
	}

	cr := csv.NewReader(resp.Body)
	hdrr, err := cr.Read()
	if err != nil {
		return err
	}

	if slices.Index(hdrr, "Route_Date") < 0 {
		return fmt.Errorf("Route_Date not in columns: %v", hdrr)
	}

	hdr := make(map[string]int)
	for i, h := range hdrr {
		hdr[h] = i
	}

	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		rd := rec[hdr["Route_Date"]]
		rdf := strings.Fields(rd)
		rdt, err := time.ParseInLocation("2006/01/02", rdf[0], loc)
		if err != nil {
			return fmt.Errorf("invalid date %q", rdf[0])
		}

		var pt halifaxTransitPoint
		pt.count, err = strconv.Atoi(rec[hdr["Ridership_Total"]])
		if err != nil {
			return err
		}
		pt.day = rdt

		rt := halifaxTransitRoute{
			number: strings.ToLower(rec[hdr["Route_Number"]]),
			name:   rec[hdr["Route_Name"]],
		}

		if d, ok := h.data[rt.number]; ok {
			d.points = append(d.points, pt)
			h.data[rt.number] = d
		} else {
			rt.points = []halifaxTransitPoint{pt}
			h.data[rt.number] = rt
		}
	}

	for k := range h.data {
		sort.Slice(h.data[k].points, func(i, j int) bool { return h.data[k].points[i].day.Before(h.data[k].points[j].day) })
	}

	return nil
}

type halifaxTransitRoute struct {
	number string
	name   string

	points []halifaxTransitPoint
}

type halifaxTransitPoint struct {
	day   time.Time
	count int
}
