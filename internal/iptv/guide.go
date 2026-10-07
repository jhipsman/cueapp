package iptv

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// Programme is one show in the guide.
type Programme struct {
	Title string    `json:"title"`
	Desc  string    `json:"desc,omitempty"`
	Start time.Time `json:"start"`
	Stop  time.Time `json:"stop"`
}

// Guide is what's on, per guide channel id, each list in time order.
type Guide map[string][]Programme

// At is what's on channel at t, and what comes after it.
func (g Guide) At(channel string, t time.Time) (now, next *Programme) {
	list := g[channel]
	i := sort.Search(len(list), func(i int) bool { return list[i].Stop.After(t) })
	if i < len(list) && !list[i].Start.After(t) {
		now = &list[i]
		if i+1 < len(list) {
			next = &list[i+1]
		}
	} else if i < len(list) {
		next = &list[i]
	}
	return now, next
}

// Between is channel's programmes that overlap [from, to).
func (g Guide) Between(channel string, from, to time.Time) []Programme {
	var out []Programme
	for _, p := range g[channel] {
		if p.Stop.After(from) && p.Start.Before(to) {
			out = append(out, p)
		}
	}
	return out
}

// xmltvTime reads XMLTV's "20061231235959 +0100".
func xmltvTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"20060102150405 -0700", "20060102150405"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("bad XMLTV time %q", s)
}

// ParseGuide reads an XMLTV document, keeping the programmes of the wanted
// channels (all when wanted is nil) that overlap [from, to). Guides run to
// tens of megabytes, so it reads as it goes and keeps only those.
func ParseGuide(r io.Reader, wanted map[string]bool, from, to time.Time) (Guide, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = false
	dec.CharsetReader = func(_ string, in io.Reader) (io.Reader, error) { return in, nil }
	g := Guide{}
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if len(g) > 0 {
				break // a guide cut short still has its first part
			}
			return nil, fmt.Errorf("couldn't read the guide: %w", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "programme" {
			continue
		}
		var p struct {
			Start   string `xml:"start,attr"`
			Stop    string `xml:"stop,attr"`
			Channel string `xml:"channel,attr"`
			Title   string `xml:"title"`
			Desc    string `xml:"desc"`
		}
		if err := dec.DecodeElement(&p, &se); err != nil {
			continue
		}
		if wanted != nil && !wanted[p.Channel] {
			continue
		}
		start, err1 := xmltvTime(p.Start)
		stop, err2 := xmltvTime(p.Stop)
		if err1 != nil || err2 != nil || !stop.After(from) || !start.Before(to) {
			continue
		}
		desc := strings.TrimSpace(p.Desc)
		if len(desc) > 400 {
			desc = desc[:397] + "..."
		}
		g[p.Channel] = append(g[p.Channel], Programme{Title: strings.TrimSpace(p.Title), Desc: desc, Start: start, Stop: stop})
	}
	for k := range g {
		sort.Slice(g[k], func(i, j int) bool { return g[k][i].Start.Before(g[k][j].Start) })
	}
	return g, nil
}

// LoadGuide downloads and reads the account's XMLTV guide.
func (c *Client) LoadGuide(ctx context.Context, wanted map[string]bool, from, to time.Time) (Guide, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.GuideURL(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Cue")
	hc := *c.hc
	hc.Timeout = 5 * time.Minute // a big guide on a slow provider
	resp, err := hc.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			return nil, fmt.Errorf("couldn't download the guide: %w", ue.Err)
		}
		return nil, fmt.Errorf("couldn't download the guide: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the guide download answered with status %d", resp.StatusCode)
	}
	return ParseGuide(io.LimitReader(resp.Body, 512<<20), wanted, from, to)
}
