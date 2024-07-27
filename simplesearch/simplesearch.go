package simplesearch

import (
	"encoding/json"
	"net/http"

	"github.com/Jeffail/gabs/v2"
)

type SimpleSearchConfig struct {
	client    *http.Client
	UserAgent string
}

func (s *SimpleSearchConfig) FetchJson(url string) (*gabs.Container, error) {
	var err error

	if s.client == nil {
		s.client = http.DefaultClient
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", s.UserAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()

	gj, err := gabs.ParseJSONDecoder(dec)
	if err != nil {
		return nil, err
	}

	return gj, err
}
