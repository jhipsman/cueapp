// Package stremio asks Stremio add-ons (Comet, Torrentio, MediaFusion and
// the like) for streams. An add-on is a web address that answers
// /manifest.json and /stream/{type}/{id}.json, where id is an IMDb id
// ("tt0111161") or, for an episode, "tt0944947:1:2". Add-ons set up with a
// debrid service (such as Comet with a Premiumize key) answer with links that
// play straight away; others answer with torrent info hashes.
//
// Protocol: https://github.com/Stremio/stremio-addon-sdk/blob/master/docs/protocol.md
package stremio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/rdborg/mediarium/internal/netguard"
)

// Addon is one add-on, by its base address (what comes before /manifest.json).
type Addon struct {
	Base string
	hc   *http.Client
}

// New takes the address of an add-on as people copy it, usually ending in
// /manifest.json (a stremio:// link works too), and returns the add-on.
func New(raw string) (*Addon, error) {
	base, err := BaseURL(raw)
	if err != nil {
		return nil, err
	}
	return &Addon{Base: base, hc: netguard.Client(25 * time.Second)}, nil
}

// BaseURL turns an add-on link into its base address.
func BaseURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(strings.ToLower(s), "stremio://") {
		s = "https://" + s[len("stremio://"):]
	}
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, "/manifest.json")
	s = strings.TrimSuffix(s, "/configure")
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", errors.New("that isn't an add-on link: copy the link that ends in /manifest.json from the add-on's install page")
	}
	return s, nil
}

// Manifest is what an add-on says about itself.
type Manifest struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Resources   []any  `json:"resources"` // strings, or objects with a "name"
	Types       []string
}

// ServesStreams says the add-on answers stream requests.
func (m Manifest) ServesStreams() bool {
	for _, r := range m.Resources {
		switch v := r.(type) {
		case string:
			if v == "stream" {
				return true
			}
		case map[string]any:
			if v["name"] == "stream" {
				return true
			}
		}
	}
	return false
}

// Stream is one stream an add-on offers.
type Stream struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	InfoHash    string `json:"infoHash"`
	FileIdx     *int   `json:"fileIdx"`
	Hints       struct {
		Filename    string `json:"filename"`
		VideoSize   int64  `json:"videoSize"`
		NotWebReady bool   `json:"notWebReady"`
	} `json:"behaviorHints"`
}

// Label is the best name the add-on gives for what the stream is: the file
// name, else the first line of its title or description.
func (s Stream) Label() string {
	if s.Hints.Filename != "" {
		return s.Hints.Filename
	}
	for _, t := range []string{s.Title, s.Description, s.Name} {
		if line := strings.TrimSpace(strings.SplitN(t, "\n", 2)[0]); line != "" {
			return line
		}
	}
	return "stream"
}

func (a *Addon) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.Base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := a.hc.Do(req)
	if err != nil {
		// The address often carries a debrid key: keep it out of messages.
		return fmt.Errorf("couldn't reach the add-on: %w", netguard.CleanError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the add-on answered with status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out); err != nil {
		return fmt.Errorf("couldn't read the add-on's answer: %w", err)
	}
	return nil
}

// Manifest fetches the add-on's manifest.
func (a *Addon) Manifest(ctx context.Context) (Manifest, error) {
	var m Manifest
	err := a.get(ctx, "/manifest.json", &m)
	return m, err
}

// Streams asks for a movie's ("movie", "tt…") or an episode's ("series",
// "tt…:season:episode") streams, in the add-on's own order.
func (a *Addon) Streams(ctx context.Context, kind, id string) ([]Stream, error) {
	var out struct {
		Streams []Stream `json:"streams"`
	}
	if err := a.get(ctx, "/stream/"+url.PathEscape(kind)+"/"+url.PathEscape(id)+".json", &out); err != nil {
		return nil, err
	}
	return out.Streams, nil
}

// Name is a short name for the add-on's address, for messages ("comet.elfhosted.com").
func (a *Addon) Name() string {
	if u, err := url.Parse(a.Base); err == nil {
		return u.Hostname()
	}
	return "add-on"
}
