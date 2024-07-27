package app

import (
	"context"
	"html"
	"regexp"
	"strings"

	"github.com/mattn/go-mastodon"
)

type MastodonConfig struct {
	// https://botsin.space
	Server string `yaml:"server,omitempty"`
	// Client key: kZoi323…
	ClientID string `yaml:"client_id,omitempty"`
	// Client secret: ose…
	ClientSecret string `yaml:"client_secret,omitempty"`
	// Application name: fedilpd
	ClientName string `yaml:"client_name,omitempty"`
	// Scopes: read write follow
	Scopes string `yaml:"scopes,omitempty"`
	// Application website: https://berlin.de/presse
	Website string `yaml:"website,omitempty"`
	// Redirect URI: urn:ietf:wg:oauth:2.0:oob
	RedirectURI string `yaml:"redirect_uri,omitempty"`
	// Your access token: Rdn…
	Token string `yaml:"token,omitempty"`
	//
	UserAgent string `yaml:"user_agent,omitempty"`
	//
	//
	//
	tagsRe *regexp.Regexp
}

const (
	UserAgent = "fediSimpleSearch/0.01 (2023-12-30)"
)

func (s *MastodonConfig) GetClient(log Log) *mastodon.Client {
	if s.UserAgent == "" {
		s.UserAgent = UserAgent
	}
	c := &mastodon.Client{
		Config: &mastodon.Config{
			Server:       s.Server,
			ClientID:     s.ClientID,
			ClientSecret: s.ClientSecret,
			AccessToken:  s.Token,
		},
		UserAgent: s.UserAgent,
	}
	return c
}

func (s *MastodonConfig) GetApp(log Log) {
	if s.Scopes == "" {
		s.Scopes = "read write"
	}
	app, err := mastodon.RegisterApp(context.Background(), &mastodon.AppConfig{
		Server:     s.Server,
		ClientName: s.ClientName,
		Scopes:     s.Scopes,
		Website:    s.Website,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Logf("client-id: %s\n", app.ClientID)
	log.Logf("client-secret: %s\n", app.ClientSecret)
}

func (s *MastodonConfig) CompileTags(tags []string) error {
	var err error
	s.tagsRe, err = regexp.Compile(`\b(` + strings.Join(tags, "|") + `)\b`)
	return err
}

func (s *MastodonConfig) Hashtag(text string) string {
	out := s.tagsRe.ReplaceAllString(html.UnescapeString(text), "#$1")
	return out
}
