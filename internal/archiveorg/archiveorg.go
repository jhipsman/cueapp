// Package archiveorg searches the Internet Archive (archive.org) for videos:
// old TV, films and recordings people have uploaded, which the usual places
// often don't have. Videos play straight from archive.org.
package archiveorg

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// Client talks to archive.org.
type Client struct {
	base string
	hc   *http.Client
}

// DefaultBase is archive.org's address (tests point it elsewhere).
var DefaultBase = "https://archive.org"

// New returns a client; base is "" for DefaultBase.
func New(base string) *Client {
	if base == "" {
		base = DefaultBase
	}
	return &Client{base: strings.TrimRight(base, "/"), hc: &http.Client{Timeout: 15 * time.Second}}
}

// Item is one upload (a film, an episode, or a whole season).
type Item struct {
	ID    string
	Title string
}

// Video is one video file in an item.
type Video struct {
	Item     string
	Title    string // the item's title
	Name     string // the file's name, with any folders
	URL      string
	Size     int64
	Length   time.Duration
	Original bool // the uploaded file, not a copy archive.org made
}

func (c *Client) getJSON(ctx context.Context, u string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Cue (family streaming app)")
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("couldn't reach archive.org: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("archive.org answered with status %d", resp.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(out)
}

// Search finds video items whose title has the phrase, most downloaded
// first.
func (c *Client) Search(ctx context.Context, phrase string, rows int) ([]Item, error) {
	return c.Query(ctx, fmt.Sprintf(`title:(%s)`, Phrase(phrase)), rows)
}

// Phrase quotes words for a query.
func Phrase(s string) string {
	return `"` + strings.ReplaceAll(strings.TrimSpace(s), `"`, "") + `"`
}

// Query finds video items matching q (archive.org's search syntax, like
// title:("Fraggle Rock") AND "Scared Silly"), most downloaded first.
func (c *Client) Query(ctx context.Context, query string, rows int) ([]Item, error) {
	q := url.Values{
		"q":      {query + " AND mediatype:(movies)"},
		"fl[]":   {"identifier", "title"},
		"sort[]": {"downloads desc"},
		"rows":   {strconv.Itoa(rows)},
		"output": {"json"},
	}
	var out struct {
		Response struct {
			Docs []struct {
				ID    string          `json:"identifier"`
				Title json.RawMessage `json:"title"`
			} `json:"docs"`
		} `json:"response"`
	}
	if err := c.getJSON(ctx, c.base+"/advancedsearch.php?"+q.Encode(), &out); err != nil {
		return nil, err
	}
	items := make([]Item, 0, len(out.Response.Docs))
	for _, d := range out.Response.Docs {
		items = append(items, Item{ID: d.ID, Title: oneString(d.Title)})
	}
	return items, nil
}

// oneString reads a field archive.org sends as a string or a list of them.
func oneString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var list []string
	if json.Unmarshal(raw, &list) == nil && len(list) > 0 {
		return list[0]
	}
	return ""
}

var playable = map[string]bool{".mp4": true, ".m4v": true, ".webm": true, ".mkv": true, ".avi": true, ".mov": true, ".mpg": true, ".mpeg": true, ".ogv": true}

// Videos lists an item's video files.
func (c *Client) Videos(ctx context.Context, item Item) ([]Video, error) {
	var meta struct {
		Metadata struct {
			Title json.RawMessage `json:"title"`
		} `json:"metadata"`
		Files []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
			Format string `json:"format"`
			Size   string `json:"size"`
			Length string `json:"length"`
		} `json:"files"`
	}
	if err := c.getJSON(ctx, c.base+"/metadata/"+url.PathEscape(item.ID), &meta); err != nil {
		return nil, err
	}
	title := item.Title
	if t := oneString(meta.Metadata.Title); t != "" {
		title = t
	}
	var out []Video
	for _, f := range meta.Files {
		ext := strings.ToLower(path.Ext(f.Name))
		if !playable[ext] || strings.Contains(strings.ToLower(f.Name), "sample") {
			continue
		}
		size, _ := strconv.ParseInt(f.Size, 10, 64)
		segs := strings.Split(f.Name, "/")
		for i := range segs {
			segs[i] = url.PathEscape(segs[i])
		}
		out = append(out, Video{
			Item: item.ID, Title: title, Name: f.Name, Size: size, Length: parseLength(f.Length),
			URL:      c.base + "/download/" + url.PathEscape(item.ID) + "/" + strings.Join(segs, "/"),
			Original: f.Source == "original",
		})
	}
	return out, nil
}

// parseLength reads "1234.56" seconds or "MM:SS" / "HH:MM:SS".
func parseLength(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if !strings.Contains(s, ":") {
		f, _ := strconv.ParseFloat(s, 64)
		return time.Duration(f * float64(time.Second))
	}
	var total float64
	for _, p := range strings.Split(s, ":") {
		f, _ := strconv.ParseFloat(p, 64)
		total = total*60 + f
	}
	return time.Duration(total * float64(time.Second))
}
