// Package iptv reads a live TV provider: an Xtream Codes account (a server
// address, a username and a password) for its channels, categories and logos,
// and its XMLTV guide for what's on.
package iptv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ErrBadLogin is returned when the provider refuses the username or password.
var ErrBadLogin = errors.New("the IPTV provider didn't accept the username or password")

// Account is an Xtream Codes login.
type Account struct {
	Server   string `json:"server"` // like http://line.example.com:8080
	Username string `json:"username"`
	Password string `json:"password"`
}

// Clean tidies what people paste: spaces, a trailing slash, a missing http://,
// or a whole player_api.php or get.php link.
func (a Account) Clean() Account {
	s := strings.TrimSpace(a.Server)
	if i := strings.Index(s, "/player_api.php"); i >= 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "/get.php"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimRight(s, "/")
	if s != "" && !strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://") {
		s = "http://" + s
	}
	return Account{Server: s, Username: strings.TrimSpace(a.Username), Password: strings.TrimSpace(a.Password)}
}

// Valid says the account has everything it needs.
func (a Account) Valid() bool {
	u, err := url.Parse(a.Server)
	return err == nil && u.Host != "" && a.Username != "" && a.Password != ""
}

// flex reads a JSON value that providers send as a string, a number or null.
type flex string

func (f *flex) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	switch {
	case s == "null":
		*f = ""
	case strings.HasPrefix(s, `"`):
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*f = flex(v)
	default:
		*f = flex(s)
	}
	return nil
}

// Status is the account's state as the provider reports it.
type Status struct {
	Active         bool      `json:"active"`
	Status         string    `json:"status"` // "Active", "Expired", "Banned"...
	Expires        time.Time `json:"expires,omitempty"`
	MaxConnections int       `json:"maxConnections,omitempty"`
	Formats        []string  `json:"formats,omitempty"` // "m3u8", "ts"
	// Timezone is the provider's own (catch-up addresses give times in it);
	// "" when it doesn't say.
	Timezone string `json:"timezone,omitempty"`
}

// Category is a group of channels.
type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Channel is one live channel.
type Channel struct {
	ID       string `json:"id"` // the provider's stream id
	Num      int    `json:"num"`
	Name     string `json:"name"`
	Logo     string `json:"-"` // the provider's logo address (served through Cue)
	Category string `json:"category"`
	EPGID    string `json:"-"` // the channel's id in the XMLTV guide
	// CatchupDays is how many days back the provider keeps the channel
	// (catch-up / timeshift); 0 when it doesn't.
	CatchupDays int `json:"catchupDays,omitempty"`
}

// Client talks to one Xtream Codes account.
type Client struct {
	acct Account
	hc   *http.Client
}

// New returns a client for acct (cleaned).
func New(acct Account, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{acct: acct.Clean(), hc: hc}
}

func (c *Client) api(ctx context.Context, action string, out any) error {
	q := url.Values{"username": {c.acct.Username}, "password": {c.acct.Password}}
	if action != "" {
		q.Set("action", action)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.acct.Server+"/player_api.php?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "Cue")
	resp, err := c.hc.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) { // the address holds the password
			return fmt.Errorf("couldn't reach the IPTV provider: %w", ue.Err)
		}
		return fmt.Errorf("couldn't reach the IPTV provider: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrBadLogin
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the IPTV provider answered with status %d", resp.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<20)).Decode(out); err != nil {
		return fmt.Errorf("couldn't read the IPTV provider's answer: %w", err)
	}
	return nil
}

// Status checks the login and reads the account's state.
func (c *Client) Status(ctx context.Context) (Status, error) {
	var out struct {
		UserInfo struct {
			Auth           flex     `json:"auth"`
			Status         flex     `json:"status"`
			ExpDate        flex     `json:"exp_date"`
			MaxConnections flex     `json:"max_connections"`
			Formats        []string `json:"allowed_output_formats"`
		} `json:"user_info"`
		ServerInfo struct {
			Timezone flex `json:"timezone"`
		} `json:"server_info"`
	}
	if err := c.api(ctx, "", &out); err != nil {
		return Status{}, err
	}
	ui := out.UserInfo
	if ui.Auth != "1" {
		return Status{}, ErrBadLogin
	}
	st := Status{Status: string(ui.Status), Formats: ui.Formats, Timezone: strings.TrimSpace(string(out.ServerInfo.Timezone))}
	st.Active = strings.EqualFold(st.Status, "active")
	if n, err := strconv.ParseInt(string(ui.ExpDate), 10, 64); err == nil && n > 0 {
		st.Expires = time.Unix(n, 0).UTC()
	}
	st.MaxConnections, _ = strconv.Atoi(string(ui.MaxConnections))
	return st, nil
}

// Categories lists the live channel groups, in the provider's order.
func (c *Client) Categories(ctx context.Context) ([]Category, error) {
	var raw []struct {
		ID   flex `json:"category_id"`
		Name flex `json:"category_name"`
	}
	if err := c.api(ctx, "get_live_categories", &raw); err != nil {
		return nil, err
	}
	out := make([]Category, 0, len(raw))
	for _, r := range raw {
		out = append(out, Category{ID: string(r.ID), Name: strings.TrimSpace(string(r.Name))})
	}
	return out, nil
}

// Channels lists every live channel, by its number.
func (c *Client) Channels(ctx context.Context) ([]Channel, error) {
	var raw []struct {
		Num      flex `json:"num"`
		Name     flex `json:"name"`
		Type     flex `json:"stream_type"`
		ID       flex `json:"stream_id"`
		Icon     flex `json:"stream_icon"`
		EPGID    flex `json:"epg_channel_id"`
		Category flex `json:"category_id"`
		Archive  flex `json:"tv_archive"`
		Days     flex `json:"tv_archive_duration"`
	}
	if err := c.api(ctx, "get_live_streams", &raw); err != nil {
		return nil, err
	}
	out := make([]Channel, 0, len(raw))
	for _, r := range raw {
		if r.ID == "" || (r.Type != "" && r.Type != "live" && r.Type != "created_live") {
			continue
		}
		n, _ := strconv.Atoi(string(r.Num))
		days := 0
		if r.Archive == "1" {
			days, _ = strconv.Atoi(string(r.Days))
			if days <= 0 {
				days = 1
			}
		}
		out = append(out, Channel{
			ID: string(r.ID), Num: n, Name: strings.TrimSpace(string(r.Name)), Logo: strings.TrimSpace(string(r.Icon)),
			Category: string(r.Category), EPGID: strings.TrimSpace(string(r.EPGID)), CatchupDays: days,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Num < out[j].Num })
	return out, nil
}

// StreamURL is where a channel plays, as HLS ("m3u8") or MPEG-TS ("ts").
// It holds the password: never log it or show it.
func (c *Client) StreamURL(id, format string) string {
	if format != "ts" {
		format = "m3u8"
	}
	return fmt.Sprintf("%s/live/%s/%s/%s.%s", c.acct.Server, url.PathEscape(c.acct.Username), url.PathEscape(c.acct.Password), url.PathEscape(id), format)
}

// CatchupURL is where a past show plays from the provider's recordings:
// start (in the provider's time zone, loc) and the minutes to play. Format
// as for StreamURL. It holds the password: never log it or show it.
func (c *Client) CatchupURL(id string, start time.Time, minutes int, format string, loc *time.Location) string {
	if format != "ts" {
		format = "m3u8"
	}
	if loc == nil {
		loc = time.UTC
	}
	return fmt.Sprintf("%s/timeshift/%s/%s/%d/%s/%s.%s", c.acct.Server, url.PathEscape(c.acct.Username), url.PathEscape(c.acct.Password),
		max(minutes, 1), start.In(loc).Format("2006-01-02:15-04"), url.PathEscape(id), format)
}

// CatchupPHPURL is the older way providers serve catch-up
// (/streaming/timeshift.php), as MPEG-TS; some offer only this one.
func (c *Client) CatchupPHPURL(id string, start time.Time, minutes int, loc *time.Location) string {
	if loc == nil {
		loc = time.UTC
	}
	q := url.Values{
		"username": {c.acct.Username}, "password": {c.acct.Password}, "stream": {id},
		"start": {start.In(loc).Format("2006-01-02:15-04")}, "duration": {strconv.Itoa(max(minutes, 1))},
	}
	return c.acct.Server + "/streaming/timeshift.php?" + q.Encode()
}

// GuideURL is the account's full XMLTV guide.
func (c *Client) GuideURL() string {
	q := url.Values{"username": {c.acct.Username}, "password": {c.acct.Password}}
	return c.acct.Server + "/xmltv.php?" + q.Encode()
}

// HTTP is the client's HTTP client, for the guide and logos.
func (c *Client) HTTP() *http.Client { return c.hc }
