// Package youtube searches YouTube (the Data API, with the owner's key) for
// full episodes and films, to play in YouTube's own embedded player.
package youtube

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ErrBadKey is returned when Google refuses the API key.
var ErrBadKey = errors.New("google didn't accept the YouTube API key")

// Client talks to the YouTube Data API.
type Client struct {
	key  string
	base string
	hc   *http.Client
}

// New returns a client for key; base is "" for Google itself.
func New(key, base string) *Client {
	if base == "" {
		base = "https://www.googleapis.com/youtube/v3"
	}
	return &Client{key: key, base: strings.TrimRight(base, "/"), hc: &http.Client{Timeout: 12 * time.Second}}
}

// Video is one search result with what's needed to choose it.
type Video struct {
	ID         string
	Title      string
	Channel    string
	Length     time.Duration
	Embeddable bool
	// Language is the video's spoken language, when its uploader set it
	// ("en", "ru"); "" when not.
	Language string
}

func (c *Client) get(ctx context.Context, endpoint string, q url.Values, out any) error {
	q.Set("key", c.key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/"+endpoint+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) { // the address holds the key
			return fmt.Errorf("couldn't reach YouTube: %w", ue.Err)
		}
		return fmt.Errorf("couldn't reach YouTube: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode == http.StatusBadRequest && strings.Contains(string(body), "keyInvalid") ||
		resp.StatusCode == http.StatusForbidden && strings.Contains(string(body), "accessNotConfigured") {
		return ErrBadKey
	}
	if resp.StatusCode != http.StatusOK {
		if strings.Contains(string(body), "quotaExceeded") {
			return errors.New("the YouTube key's searches for today are used up")
		}
		return fmt.Errorf("YouTube answered with status %d", resp.StatusCode)
	}
	return json.Unmarshal(body, out)
}

// Search finds videos for q, with their lengths and whether they may be
// played outside YouTube.
func (c *Client) Search(ctx context.Context, q string, max int) ([]Video, error) {
	var found struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
			Snippet struct {
				Title   string `json:"title"`
				Channel string `json:"channelTitle"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := c.get(ctx, "search", url.Values{"part": {"snippet"}, "type": {"video"}, "q": {q}, "maxResults": {strconv.Itoa(max)}, "safeSearch": {"none"}}, &found); err != nil {
		return nil, err
	}
	if len(found.Items) == 0 {
		return nil, nil
	}
	ids := make([]string, 0, len(found.Items))
	out := make([]Video, 0, len(found.Items))
	for _, it := range found.Items {
		if it.ID.VideoID == "" {
			continue
		}
		ids = append(ids, it.ID.VideoID)
		out = append(out, Video{ID: it.ID.VideoID, Title: htmlUnescape(it.Snippet.Title), Channel: htmlUnescape(it.Snippet.Channel)})
	}
	var details struct {
		Items []struct {
			ID             string `json:"id"`
			ContentDetails struct {
				Duration string `json:"duration"`
			} `json:"contentDetails"`
			Status struct {
				Embeddable bool `json:"embeddable"`
			} `json:"status"`
			Snippet struct {
				AudioLanguage string `json:"defaultAudioLanguage"`
				Language      string `json:"defaultLanguage"`
			} `json:"snippet"`
		} `json:"items"`
	}
	if err := c.get(ctx, "videos", url.Values{"part": {"contentDetails,status,snippet"}, "id": {strings.Join(ids, ",")}}, &details); err != nil {
		return nil, err
	}
	byID := map[string]int{}
	for i, v := range out {
		byID[v.ID] = i
	}
	for _, d := range details.Items {
		if i, ok := byID[d.ID]; ok {
			out[i].Length = parseISODuration(d.ContentDetails.Duration)
			out[i].Embeddable = d.Status.Embeddable
			out[i].Language = d.Snippet.AudioLanguage
			if out[i].Language == "" {
				out[i].Language = d.Snippet.Language
			}
		}
	}
	return out, nil
}

var isoDuration = regexp.MustCompile(`^P(?:(\d+)D)?T?(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?$`)

// parseISODuration reads YouTube's "PT1H2M3S".
func parseISODuration(s string) time.Duration {
	m := isoDuration.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	n := func(i int) time.Duration { v, _ := strconv.Atoi(m[i]); return time.Duration(v) }
	return n(1)*24*time.Hour + n(2)*time.Hour + n(3)*time.Minute + n(4)*time.Second
}

var htmlEntities = strings.NewReplacer("&amp;", "&", "&#39;", "'", "&quot;", `"`, "&lt;", "<", "&gt;", ">")

func htmlUnescape(s string) string { return htmlEntities.Replace(s) }

// Check asks Google about one well-known video, to see the key works (it
// costs 1 of the day's 10,000 units; a search costs 100).
func (c *Client) Check(ctx context.Context) error {
	var out struct {
		Items []json.RawMessage `json:"items"`
	}
	return c.get(ctx, "videos", url.Values{"part": {"id"}, "id": {"jNQXAC9IVRw"}}, &out)
}
