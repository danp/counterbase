package ecocounter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"
)

type EcoVisioQuerier struct {
	Auth             *EcoVisioAuth
	UserID, DomainID string
	FlowIDs          []string
}

func (q EcoVisioQuerier) Query(begin, end time.Time, resolution Resolution) ([]Datapoint, error) {
	var ress string
	switch resolution {
	case ResolutionDay:
		ress = "day"
	case ResolutionHour:
		ress = "hour"
	}
	if ress == "" {
		return nil, nil
	}

	// our end is inclusive, API's is not
	end = end.AddDate(0, 0, 1)

	begins, ends := begin.Format("2006-01-02"), end.Format("2006-01-02")
	bu := "https://www.eco-visio.net/api/aladdin/1.0.0/domain/" + q.DomainID + "/user/" + q.UserID + "/query/from/" + begins + "%2000:00/to/" + ends + "%2000:00/by/" + ress

	var reqs struct {
		Flows []int `json:"flows"`
	}
	for _, fid := range q.FlowIDs {
		idi, err := strconv.Atoi(fid)
		if err != nil {
			return nil, err
		}
		reqs.Flows = append(reqs.Flows, idi)
	}
	reqb, err := json.Marshal(reqs)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", bu, bytes.NewReader(reqb))
	if err != nil {
		return nil, err
	}

	tok, err := q.Auth.Token()
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:80.0) Gecko/20100101 Firefox/80.0")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", "https://www.eco-visio.net")
	req.Header.Set("DNT", "1")
	req.Header.Set("Referer", "https://www.eco-visio.net/v5/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading query response for %s: %w", q.FlowIDs, err)
	}

	if resp.StatusCode/100 != 2 {
		if len(b) > 100 {
			b = b[:100]
		}
		return nil, fmt.Errorf("bad status %d querying %s: %s", resp.StatusCode, q.FlowIDs, b)
	}

	var resps map[string]struct {
		Countdata [][]interface{}
	}
	if err := json.Unmarshal(b, &resps); err != nil {
		return nil, fmt.Errorf("unmarshaling query response for %s: %w", q.FlowIDs, err)
	}

	// Sum up all the flows we got, by time period.
	dps := make(map[string]Datapoint)
	for _, re := range resps {
		for _, rd := range re.Countdata {
			t := rd[0].(string)
			dp := dps[t]
			dp.Time = t
			dp.Count += int(rd[1].(float64))
			dps[t] = dp
		}
	}

	ds := make([]Datapoint, 0, len(dps))
	for _, dp := range dps {
		ds = append(ds, dp)
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i].Time < ds[j].Time })

	return ds, nil
}

type EcoVisioAuth struct {
	username, password string

	// should support expiry, etc
	tokenCh chan string
}

func NewEcoVisioAuth(username, password string) *EcoVisioAuth {
	ch := make(chan string, 1)
	ch <- ""
	return &EcoVisioAuth{
		username: username,
		password: password,
		tokenCh:  ch,
	}
}

func (a *EcoVisioAuth) Token() (string, error) {
	tok := <-a.tokenCh
	if tok == "" {
		t, err := a.auth()
		if err != nil {
			a.tokenCh <- ""
			return "", err
		}
		tok = t
	}
	a.tokenCh <- tok
	return tok, nil
}

func (a *EcoVisioAuth) auth() (string, error) {
	reqs := struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}{
		Login:    a.username,
		Password: a.password,
	}
	reqb, err := json.Marshal(reqs)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://www.eco-visio.net/api/aladdin/1.0.0/connect", bytes.NewReader(reqb))
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:80.0) Gecko/20100101 Firefox/80.0")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", "https://www.eco-visio.net")
	req.Header.Set("DNT", "1")
	req.Header.Set("Referer", "https://www.eco-visio.net/v5/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading eco visio auth response: %w", err)
	}

	if resp.StatusCode/100 != 2 {
		if len(b) > 100 {
			b = b[:100]
		}
		return "", fmt.Errorf("bad status %d for eco visio auth: %s", resp.StatusCode, b)
	}

	var resps struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(b, &resps); err != nil {
		return "", fmt.Errorf("decoding eco visio auth response: %w", err)
	}

	return resps.AccessToken, nil
}
