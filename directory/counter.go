package directory

import (
	"time"
)

type Counter struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	ServiceRanges []ServiceRange `json:"service_ranges"`
	Mode          string         `json:"mode"`
	Location      Location       `json:"location"`
	Directions    []Direction    `json:"directions"`
	Notes         []Note         `json:"notes,omitempty"`
	Tags          []string       `json:"tags,omitempty"`
}

func (c Counter) IsActive() bool {
	return len(c.ServiceRanges) > 0 && c.ServiceRanges[len(c.ServiceRanges)-1].End.IsZero()
}

type ServiceDate struct {
	time.Time
}

func SD(t time.Time) ServiceDate {
	return ServiceDate{Time: t}
}

func (s *ServiceDate) UnmarshalJSON(b []byte) error {
	if string(b) == "null" || string(b) == `""` {
		return nil
	}
	t, err := time.Parse("\"2006-01-02\"", string(b))
	if err != nil {
		return err
	}
	s.Time = t
	return nil
}

func (s *ServiceDate) MarshalJSON() ([]byte, error) {
	if s.IsZero() {
		return []byte("null"), nil
	}
	return []byte(s.Format("\"2006-01-02\"")), nil
}

type ServiceRange struct {
	Start ServiceDate `json:"start,omitempty"`
	End   ServiceDate `json:"end,omitempty"`
}

type Location struct {
	Lon  float64 `json:"lon,omitempty"`
	Lat  float64 `json:"lat,omitempty"`
	Text string  `json:"text,omitempty"`
}

type Direction struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Source Source `json:"source"`
}

type Note struct {
	Text string `json:"text"`
}

type Source struct {
	URL string `json:"url"`
}
