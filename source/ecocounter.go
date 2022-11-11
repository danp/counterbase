package source

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danp/counterbase/directory"
	"github.com/danp/counterbase/source/internal/ecocounter"
	"github.com/danp/counterbase/submit"
)

type EcoCounterPrivateDomain struct {
	Name     string
	Username string
	Password string
	UserID   string
	DomainID string

	auth *ecocounter.EcoVisioAuth
}

type EcoCounterGetter struct {
	privateDomains map[string]EcoCounterPrivateDomain
}

func (g *EcoCounterGetter) AddPrivateDomain(dom EcoCounterPrivateDomain) error {
	if g.privateDomains == nil {
		g.privateDomains = make(map[string]EcoCounterPrivateDomain)
	}
	dom.auth = ecocounter.NewEcoVisioAuth(dom.Username, dom.Password)
	g.privateDomains[dom.Name] = dom
	return nil
}

func (g *EcoCounterGetter) Get(ctx context.Context, req GetRequest) ([]submit.Point, error) {
	loc, err := time.LoadLocation("America/Halifax")
	if err != nil {
		return nil, err
	}

	var dps []ecocounter.Datapoint

	switch req.URL.Host {
	case "public":
		var cl ecocounter.Client
		dps, err = cl.GetDatapoints(strings.TrimPrefix(req.URL.Path, "/"), req.After, time.Now(), ecocounter.ResolutionHour)
	case "private":
		parts := strings.Split(strings.TrimPrefix(req.URL.Path, "/"), "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("bad path in URL %q", req.URL)
		}
		domainName, id := parts[0], parts[1]

		domain, ok := g.privateDomains[domainName]
		if !ok {
			log.Printf("no private auth for URL %q domain %q available, skipping", req.URL, domainName)
			return nil, nil
		}

		q := ecocounter.EcoVisioQuerier{
			Auth:     domain.auth,
			UserID:   domain.UserID,
			DomainID: domain.DomainID,
			FlowIDs:  []string{id},
		}

		dps, err = q.Query(req.After, time.Now(), ecocounter.ResolutionHour)
	default:
		log.Println("not handling url", req.URL, "yet")
		return nil, nil
	}

	sps := make([]submit.Point, 0, len(dps))
	for _, dp := range dps {
		t, err := time.ParseInLocation("2006-01-02 15:04:05", dp.Time, loc)
		if err != nil {
			return nil, err
		}
		if !t.After(req.After) {
			continue
		}

		sps = append(sps, submit.Point{
			Time:       t.Unix(),
			Resolution: submit.ResolutionHour,
			Value:      float64(dp.Count),
		})
	}
	return sps, err
}

func (g *EcoCounterGetter) Counters(ctx context.Context, id string) ([]directory.Counter, error) {
	resp, err := http.Get("https://www.eco-visio.net/api/aladdin/1.0.0/pbl/publicwebpageplus/" + id + "?withNull=true")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if status := resp.StatusCode; status != 200 {
		return nil, fmt.Errorf("bad status %d", status)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var body []struct {
		IDPDC int
		Debut string
		Lat   float64
		Lon   float64
		Nom   string
	}

	if err := json.Unmarshal(b, &body); err != nil {
		return nil, err
	}

	var counters []directory.Counter
	for _, bc := range body {
		var c directory.Counter

		c.ID = "counter-" + strconv.Itoa(bc.IDPDC)
		c.Location = directory.Location{Lon: bc.Lon, Lat: bc.Lat}
		c.Name = bc.Nom

		start, err := time.Parse("01/02/2006", bc.Debut)
		if err != nil {
			return nil, err
		}

		c.ServiceRanges = append(c.ServiceRanges, directory.ServiceRange{Start: directory.SD(start)})

		counters = append(counters, c)
	}
	return counters, nil
}
