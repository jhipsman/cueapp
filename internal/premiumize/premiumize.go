// Package premiumize talks to Premiumize.me, a cloud downloader: it fetches a
// torrent or an NZB on its own servers and hands the finished files back as
// plain HTTPS links. Mediarium uses it in place of its built-in torrent engine
// (and, if asked, its Usenet downloader): the release is sent to Premiumize,
// and once Premiumize has it the files are downloaded into the download's
// working folder like any other download.
//
// API reference: https://www.premiumize.me/api
package premiumize

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBase is the Premiumize API.
const DefaultBase = "https://www.premiumize.me/api"

// ErrBadKey is returned when Premiumize refuses the API key.
var ErrBadKey = errors.New("premiumize didn't accept the API key")

// ErrNotCached is returned by DirectDL when Premiumize doesn't already have
// the release in its cache.
var ErrNotCached = errors.New("premiumize doesn't have this release cached")

// Transfer states, as Premiumize names them.
const (
	StatusWaiting  = "waiting"
	StatusQueued   = "queued"
	StatusRunning  = "running"
	StatusFinished = "finished"
	StatusSeeding  = "seeding"
	StatusError    = "error"
	StatusTimeout  = "timeout"
	StatusBanned   = "banned"
	StatusDeleted  = "deleted"
)

// Client is a Premiumize API client for one API key.
type Client struct {
	key  string
	base string
	hc   *http.Client
}

// New returns a client for apiKey. base is the API address ("" for the real
// one; tests point it elsewhere).
func New(apiKey, base string) *Client {
	if base == "" {
		base = DefaultBase
	}
	return &Client{
		key:  strings.TrimSpace(apiKey),
		base: strings.TrimRight(base, "/"),
		hc:   &http.Client{Timeout: 60 * time.Second},
	}
}

// Account is what /account/info tells about the account.
type Account struct {
	CustomerID   string  `json:"customer_id"`
	PremiumUntil int64   `json:"premium_until"` // Unix seconds; 0 = not premium
	LimitUsed    float64 `json:"limit_used"`    // share of the fair-use limit used, 0 to 1
	SpaceUsed    float64 `json:"space_used"`    // bytes in the cloud
}

// Transfer is one job on Premiumize's download list.
type Transfer struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Message  string  `json:"message"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress"` // 0 to 1
	Src      string  `json:"src"`
	FolderID string  `json:"folder_id"`
	FileID   string  `json:"file_id"`
}

// Done says the transfer's files are ready.
func (t Transfer) Done() bool { return t.Status == StatusFinished || t.Status == StatusSeeding }

// Failed says the transfer ended without its files.
func (t Transfer) Failed() bool {
	switch t.Status {
	case StatusError, StatusTimeout, StatusBanned, StatusDeleted:
		return true
	}
	return false
}

// File is one finished file and where to download it from.
type File struct {
	Path string // relative path, with / between folders
	Size int64
	Link string
	// StreamLink is Premiumize's converted copy (MP4) when it made one, for
	// players that can't open the original (browsers, Apple TV with MKV).
	StreamLink string
}

// envelope is the part every answer shares.
type envelope struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func (e envelope) err() error {
	if e.Status == "success" {
		return nil
	}
	msg := strings.TrimSpace(e.Message)
	low := strings.ToLower(msg)
	if strings.Contains(low, "not logged in") || strings.Contains(low, "apikey") || strings.Contains(low, "api key") || strings.Contains(low, "customer_id") {
		return ErrBadKey
	}
	if msg == "" {
		msg = "an unknown error"
	}
	return fmt.Errorf("premiumize answered: %s", msg)
}

// call sends one API request and, when Premiumize says it succeeded, decodes
// the answer into out (nil to ignore it).
func (c *Client) call(ctx context.Context, method, path string, query url.Values, body io.Reader, contentType string, out any) error {
	if query == nil {
		query = url.Values{}
	}
	query.Set("apikey", c.key)
	req, err := http.NewRequestWithContext(ctx, method, c.base+path+"?"+query.Encode(), body)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		// The URL holds the API key: never let it reach a message or the log.
		var ue *url.Error
		if errors.As(err, &ue) {
			return fmt.Errorf("couldn't reach Premiumize: %w", ue.Err)
		}
		return fmt.Errorf("couldn't reach Premiumize: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("couldn't read Premiumize's answer: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrBadKey
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("premiumize answered with status %d", resp.StatusCode)
		}
		return fmt.Errorf("couldn't read Premiumize's answer: %w", err)
	}
	if err := env.err(); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("couldn't read Premiumize's answer: %w", err)
	}
	return nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.call(ctx, http.MethodGet, path, query, nil, "", out)
}

func (c *Client) postForm(ctx context.Context, path string, form url.Values, out any) error {
	return c.call(ctx, http.MethodPost, path, nil, strings.NewReader(form.Encode()), "application/x-www-form-urlencoded", out)
}

// AccountInfo checks the key and tells about the account.
func (c *Client) AccountInfo(ctx context.Context) (Account, error) {
	var a Account
	err := c.get(ctx, "/account/info", nil, &a)
	return a, err
}

// DirectDL asks for the files of a release Premiumize already has, without
// making a transfer. src is a magnet link or a link to a file. ErrNotCached
// means it has to be fetched first (CreateTransfer).
func (c *Client) DirectDL(ctx context.Context, src string) ([]File, error) {
	var out struct {
		Content []struct {
			Path       string      `json:"path"`
			Size       json.Number `json:"size"`
			Link       string      `json:"link"`
			StreamLink string      `json:"stream_link"`
		} `json:"content"`
	}
	if err := c.postForm(ctx, "/transfer/directdl", url.Values{"src": {src}}, &out); err != nil {
		if errors.Is(err, ErrBadKey) {
			return nil, err
		}
		// Premiumize answers an uncached release with an error status.
		return nil, fmt.Errorf("%w (%v)", ErrNotCached, err)
	}
	files := make([]File, 0, len(out.Content))
	for _, f := range out.Content {
		if f.Link == "" {
			continue
		}
		size, _ := f.Size.Int64()
		files = append(files, File{Path: f.Path, Size: size, Link: f.Link, StreamLink: f.StreamLink})
	}
	if len(files) == 0 {
		return nil, ErrNotCached
	}
	return files, nil
}

// CacheCheck reports, for each torrent info hash (or link), whether
// Premiumize already has it, so it can be streamed at once.
func (c *Client) CacheCheck(ctx context.Context, items []string) ([]bool, error) {
	if len(items) == 0 {
		return nil, nil
	}
	var out struct {
		Response []bool `json:"response"`
	}
	if err := c.get(ctx, "/cache/check", url.Values{"items[]": items}, &out); err != nil {
		return nil, err
	}
	if len(out.Response) != len(items) {
		return nil, fmt.Errorf("premiumize answered for %d of %d releases", len(out.Response), len(items))
	}
	return out.Response, nil
}

// CreateTransfer starts fetching src (a magnet link or a link to a .torrent
// or .nzb file) and returns the transfer's id.
func (c *Client) CreateTransfer(ctx context.Context, src string) (string, error) {
	var out struct {
		ID string `json:"id"`
	}
	if err := c.postForm(ctx, "/transfer/create", url.Values{"src": {src}}, &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", errors.New("premiumize didn't return a transfer id")
	}
	return out.ID, nil
}

// CreateTransferFile starts fetching an uploaded .torrent or .nzb file.
// name must end in .torrent or .nzb: Premiumize tells them apart by it.
func (c *Client) CreateTransferFile(ctx context.Context, name string, data []byte) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", name)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := c.call(ctx, http.MethodPost, "/transfer/create", nil, &buf, mw.FormDataContentType(), &out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", errors.New("premiumize didn't return a transfer id")
	}
	return out.ID, nil
}

// Transfer looks up one transfer. A transfer that is no longer on the list
// comes back with StatusDeleted.
func (c *Client) Transfer(ctx context.Context, id string) (Transfer, error) {
	var out struct {
		Transfers []Transfer `json:"transfers"`
	}
	if err := c.get(ctx, "/transfer/list", nil, &out); err != nil {
		return Transfer{}, err
	}
	for _, t := range out.Transfers {
		if t.ID == id {
			return t, nil
		}
	}
	return Transfer{ID: id, Status: StatusDeleted, Message: "it is no longer on your Premiumize transfer list"}, nil
}

// DeleteTransfer removes a transfer from the list (its files, if any, stay
// in the cloud until DeleteFolder or DeleteItem).
func (c *Client) DeleteTransfer(ctx context.Context, id string) error {
	return c.postForm(ctx, "/transfer/delete", url.Values{"id": {id}}, nil)
}

// DeleteFolder removes a folder and everything in it from the cloud.
func (c *Client) DeleteFolder(ctx context.Context, id string) error {
	return c.postForm(ctx, "/folder/delete", url.Values{"id": {id}}, nil)
}

// DeleteItem removes one file from the cloud.
func (c *Client) DeleteItem(ctx context.Context, id string) error {
	return c.postForm(ctx, "/item/delete", url.Values{"id": {id}}, nil)
}

type cloudEntry struct {
	ID   string      `json:"id"`
	Name string      `json:"name"`
	Type string      `json:"type"` // "file" or "folder"
	Size json.Number `json:"size"`
	Link string      `json:"link"`
}

// maxFolderDepth stops a walk of a strangely deep (or looping) folder tree.
const maxFolderDepth = 16

// Files lists every file of a finished transfer, from its folder (walked
// recursively, paths relative to it) or its single file.
func (c *Client) Files(ctx context.Context, t Transfer) ([]File, error) {
	if t.FolderID != "" {
		var files []File
		if err := c.walk(ctx, t.FolderID, "", 0, &files); err != nil {
			return nil, err
		}
		if len(files) > 0 || t.FileID == "" {
			return files, nil
		}
	}
	if t.FileID != "" {
		var e cloudEntry
		if err := c.get(ctx, "/item/details", url.Values{"id": {t.FileID}}, &e); err != nil {
			return nil, err
		}
		size, _ := e.Size.Int64()
		return []File{{Path: e.Name, Size: size, Link: e.Link}}, nil
	}
	return nil, errors.New("premiumize finished the transfer but didn't say where its files are")
}

func (c *Client) walk(ctx context.Context, folderID, prefix string, depth int, out *[]File) error {
	if depth > maxFolderDepth {
		return errors.New("the folder on Premiumize is nested too deeply")
	}
	var list struct {
		Content []cloudEntry `json:"content"`
	}
	if err := c.get(ctx, "/folder/list", url.Values{"id": {folderID}}, &list); err != nil {
		return err
	}
	for _, e := range list.Content {
		p := e.Name
		if prefix != "" {
			p = prefix + "/" + e.Name
		}
		switch e.Type {
		case "folder":
			if err := c.walk(ctx, e.ID, p, depth+1, out); err != nil {
				return err
			}
		default:
			if e.Link == "" {
				continue
			}
			size, _ := e.Size.Int64()
			*out = append(*out, File{Path: p, Size: size, Link: e.Link})
		}
	}
	return nil
}
