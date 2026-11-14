// Package deezer reads Deezer's public music catalog (api.deezer.com): the
// charts, new releases, search, albums with their tracklists, and artists.
// It needs no account or key. Cue uses it only for what to show (names,
// cover art, track lengths and order), as the music section's catalog; the
// sound itself comes from the owner's own sources (Premiumize). Answers are
// kept a few hours, so browsing costs Deezer little.
package deezer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultBase is Deezer's public API.
const DefaultBase = "https://api.deezer.com"

const (
	keepFor  = 6 * time.Hour
	maxKept  = 2000 // answers kept at once
	maxBytes = 4 << 20
)

// Artist is one artist.
type Artist struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Picture string `json:"picture"` // a large square photo
	Fans    int    `json:"fans,omitempty"`
}

// Album is one album, EP or single. Tracks is filled only by Album().
type Album struct {
	ID       int64   `json:"id"`
	Title    string  `json:"title"`
	Cover    string  `json:"cover"`      // large
	CoverSm  string  `json:"coverSmall"` // for lists
	Artist   Artist  `json:"artist"`
	Released string  `json:"released,omitempty"` // YYYY-MM-DD
	Type     string  `json:"type,omitempty"`     // album, ep, single, compile
	Genre    string  `json:"genre,omitempty"`
	Explicit bool    `json:"explicit,omitempty"`
	Tracks   []Track `json:"tracks,omitempty"`
	Length   int     `json:"length,omitempty"` // seconds
}

// Year is the album's release year, or 0.
func (a Album) Year() int {
	if len(a.Released) >= 4 {
		y, _ := strconv.Atoi(a.Released[:4])
		return y
	}
	return 0
}

// Track is one song. AlbumID and AlbumTitle say where it's from.
type Track struct {
	ID         int64  `json:"id"`
	Title      string `json:"title"`
	Length     int    `json:"length"` // seconds
	Number     int    `json:"number,omitempty"`
	Disc       int    `json:"disc,omitempty"`
	Explicit   bool   `json:"explicit,omitempty"`
	Artist     Artist `json:"artist"`
	AlbumID    int64  `json:"albumId"`
	AlbumTitle string `json:"albumTitle"`
	Cover      string `json:"cover"` // the album's, for lists
}

// Home is what the music section opens on.
type Home struct {
	TopTracks   []Track  `json:"topTracks"`
	TopAlbums   []Album  `json:"topAlbums"`
	TopArtists  []Artist `json:"topArtists"`
	NewReleases []Album  `json:"newReleases"`
}

// Results is what a search found.
type Results struct {
	Tracks  []Track  `json:"tracks"`
	Albums  []Album  `json:"albums"`
	Artists []Artist `json:"artists"`
}

// ArtistPage is an artist with their most played songs and their albums.
type ArtistPage struct {
	Artist  Artist   `json:"artist"`
	Top     []Track  `json:"top"`
	Albums  []Album  `json:"albums"`
	Related []Artist `json:"related"`
}

// Client is a Deezer client.
type Client struct {
	base string
	hc   *http.Client

	mu   sync.Mutex
	kept map[string]keptAnswer
}

type keptAnswer struct {
	at   time.Time
	body []byte
}

// New returns a client; base "" is the real Deezer.
func New(base string) *Client {
	if base == "" {
		base = DefaultBase
	}
	return &Client{base: strings.TrimRight(base, "/"), hc: &http.Client{Timeout: 12 * time.Second}, kept: map[string]keptAnswer{}}
}

// ---- Deezer's own shapes ----

type dzArtist struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Picture       string `json:"picture_xl"`
	PictureMedium string `json:"picture_medium"`
	Fans          int    `json:"nb_fan"`
}

func (a dzArtist) artist() Artist {
	pic := a.Picture
	if pic == "" {
		pic = a.PictureMedium
	}
	return Artist{ID: a.ID, Name: a.Name, Picture: pic, Fans: a.Fans}
}

type dzAlbum struct {
	ID          int64    `json:"id"`
	Title       string   `json:"title"`
	CoverXL     string   `json:"cover_xl"`
	CoverBig    string   `json:"cover_big"`
	CoverMedium string   `json:"cover_medium"`
	ReleaseDate string   `json:"release_date"`
	RecordType  string   `json:"record_type"`
	Explicit    bool     `json:"explicit_lyrics"`
	Duration    int      `json:"duration"`
	Artist      dzArtist `json:"artist"`
	Genres      struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	} `json:"genres"`
}

func (a dzAlbum) album() Album {
	cover := a.CoverXL
	if cover == "" {
		cover = a.CoverBig
	}
	out := Album{ID: a.ID, Title: a.Title, Cover: cover, CoverSm: a.CoverMedium, Artist: a.Artist.artist(),
		Released: a.ReleaseDate, Type: a.RecordType, Explicit: a.Explicit, Length: a.Duration}
	if len(a.Genres.Data) > 0 {
		out.Genre = a.Genres.Data[0].Name
	}
	return out
}

type dzTrack struct {
	ID       int64    `json:"id"`
	Title    string   `json:"title"`
	Duration int      `json:"duration"`
	Position int      `json:"track_position"`
	Disc     int      `json:"disk_number"`
	Explicit bool     `json:"explicit_lyrics"`
	Readable *bool    `json:"readable"`
	Artist   dzArtist `json:"artist"`
	Album    dzAlbum  `json:"album"`
}

func (t dzTrack) track() Track {
	return Track{ID: t.ID, Title: t.Title, Length: t.Duration, Number: t.Position, Disc: t.Disc, Explicit: t.Explicit,
		Artist: t.Artist.artist(), AlbumID: t.Album.ID, AlbumTitle: t.Album.Title, Cover: t.Album.CoverMedium}
}

type list[T any] struct {
	Data []T `json:"data"`
}

// ---- Calls ----

// get reads path into out, from what was kept when it's fresh.
func (c *Client) get(ctx context.Context, path string, out any) error {
	c.mu.Lock()
	k, ok := c.kept[path]
	c.mu.Unlock()
	if ok && time.Since(k.at) < keepFor {
		return json.Unmarshal(k.body, out)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	res, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("couldn't reach the music catalog: %w", err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxBytes))
	if err != nil {
		return err
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("the music catalog answered %s", res.Status)
	}
	// Deezer reports its errors inside a 200.
	var e struct {
		Error *struct {
			Message string `json:"message"`
			Code    int    `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &e) == nil && e.Error != nil {
		if e.Error.Code == 800 {
			return ErrNotFound
		}
		return fmt.Errorf("the music catalog said: %s", e.Error.Message)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("the music catalog's answer: %w", err)
	}
	c.mu.Lock()
	if len(c.kept) >= maxKept {
		for p, a := range c.kept {
			if time.Since(a.at) > keepFor || len(c.kept) >= maxKept {
				delete(c.kept, p)
			}
		}
	}
	c.kept[path] = keptAnswer{at: time.Now(), body: body}
	c.mu.Unlock()
	return nil
}

// ErrNotFound is an album or artist Deezer doesn't have.
var ErrNotFound = fmt.Errorf("not in the music catalog")

// Home is the charts and the week's new releases.
func (c *Client) Home(ctx context.Context) (Home, error) {
	var chart struct {
		Tracks  list[dzTrack]  `json:"tracks"`
		Albums  list[dzAlbum]  `json:"albums"`
		Artists list[dzArtist] `json:"artists"`
	}
	if err := c.get(ctx, "/chart/0?limit=30", &chart); err != nil {
		return Home{}, err
	}
	h := Home{TopTracks: []Track{}, TopAlbums: []Album{}, TopArtists: []Artist{}, NewReleases: []Album{}}
	for _, t := range chart.Tracks.Data {
		h.TopTracks = append(h.TopTracks, t.track())
	}
	for _, a := range chart.Albums.Data {
		h.TopAlbums = append(h.TopAlbums, a.album())
	}
	for _, a := range chart.Artists.Data {
		h.TopArtists = append(h.TopArtists, a.artist())
	}
	var fresh list[dzAlbum]
	if err := c.get(ctx, "/editorial/0/releases?limit=30", &fresh); err == nil {
		for _, a := range fresh.Data {
			h.NewReleases = append(h.NewReleases, a.album())
		}
	}
	return h, nil
}

// Search looks for songs, albums and artists matching q.
func (c *Client) Search(ctx context.Context, q string) (Results, error) {
	q = strings.TrimSpace(q)
	r := Results{Tracks: []Track{}, Albums: []Album{}, Artists: []Artist{}}
	if q == "" {
		return r, nil
	}
	enc := url.QueryEscape(q)
	var (
		wg         sync.WaitGroup
		tracks     list[dzTrack]
		albums     list[dzAlbum]
		artists    list[dzArtist]
		e1, e2, e3 error
	)
	wg.Add(3)
	go func() { defer wg.Done(); e1 = c.get(ctx, "/search?limit=20&q="+enc, &tracks) }()
	go func() { defer wg.Done(); e2 = c.get(ctx, "/search/album?limit=20&q="+enc, &albums) }()
	go func() { defer wg.Done(); e3 = c.get(ctx, "/search/artist?limit=10&q="+enc, &artists) }()
	wg.Wait()
	if e1 != nil && e2 != nil && e3 != nil {
		return r, e1
	}
	for _, t := range tracks.Data {
		if t.Readable != nil && !*t.Readable {
			continue
		}
		r.Tracks = append(r.Tracks, t.track())
	}
	for _, a := range albums.Data {
		r.Albums = append(r.Albums, a.album())
	}
	for _, a := range artists.Data {
		r.Artists = append(r.Artists, a.artist())
	}
	return r, nil
}

// Album is one album with its tracklist, in order.
func (c *Client) Album(ctx context.Context, id int64) (Album, error) {
	var a dzAlbum
	if err := c.get(ctx, fmt.Sprintf("/album/%d", id), &a); err != nil {
		return Album{}, err
	}
	var tracks list[dzTrack]
	if err := c.get(ctx, fmt.Sprintf("/album/%d/tracks?limit=300", id), &tracks); err != nil {
		return Album{}, err
	}
	out := a.album()
	for i, t := range tracks.Data {
		tr := t.track()
		tr.AlbumID, tr.AlbumTitle, tr.Cover = out.ID, out.Title, out.CoverSm
		if tr.Number == 0 {
			tr.Number = i + 1
		}
		if tr.Disc == 0 {
			tr.Disc = 1
		}
		out.Tracks = append(out.Tracks, tr)
	}
	return out, nil
}

// Artist is an artist's page: their top songs, albums (newest first) and
// similar artists.
func (c *Client) Artist(ctx context.Context, id int64) (ArtistPage, error) {
	var a dzArtist
	if err := c.get(ctx, fmt.Sprintf("/artist/%d", id), &a); err != nil {
		return ArtistPage{}, err
	}
	p := ArtistPage{Artist: a.artist(), Top: []Track{}, Albums: []Album{}, Related: []Artist{}}
	var (
		wg      sync.WaitGroup
		top     list[dzTrack]
		albums  list[dzAlbum]
		related list[dzArtist]
	)
	wg.Add(3)
	go func() { defer wg.Done(); _ = c.get(ctx, fmt.Sprintf("/artist/%d/top?limit=10", id), &top) }()
	go func() { defer wg.Done(); _ = c.get(ctx, fmt.Sprintf("/artist/%d/albums?limit=100", id), &albums) }()
	go func() { defer wg.Done(); _ = c.get(ctx, fmt.Sprintf("/artist/%d/related?limit=12", id), &related) }()
	wg.Wait()
	for _, t := range top.Data {
		p.Top = append(p.Top, t.track())
	}
	for _, al := range albums.Data {
		x := al.album()
		x.Artist = p.Artist
		p.Albums = append(p.Albums, x)
	}
	for _, r := range related.Data {
		p.Related = append(p.Related, r.artist())
	}
	return p, nil
}
