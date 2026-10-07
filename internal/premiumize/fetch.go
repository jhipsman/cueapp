package premiumize

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FetchOptions are the hooks Fetch reports through.
type FetchOptions struct {
	// Progress is told how many bytes of the total are in the folder so far.
	Progress func(done, total int64)
	// Wait, when set, is called before each chunk is written and may hold the
	// download back (the shared speed limit).
	Wait func(ctx context.Context, n int) error
	// Client downloads the files; nil uses one with no overall timeout (files
	// can be many GB) but a limit on waiting for each answer.
	Client *http.Client
}

// SafePath turns a path Premiumize gave into one inside dir, or reports that
// it would land outside it.
func SafePath(dir, rel string) (string, error) {
	rel = strings.ReplaceAll(rel, "\\", "/")
	for _, part := range strings.Split(rel, "/") {
		if part == ".." {
			return "", fmt.Errorf("premiumize gave an unsafe file name %q", rel)
		}
	}
	clean := path.Clean("/" + rel)
	if clean == "/" {
		return "", fmt.Errorf("premiumize gave a file with no name")
	}
	clean = strings.TrimPrefix(clean, "/")
	for _, part := range strings.Split(clean, "/") {
		if part == ".." || part == "." || part == "" {
			return "", fmt.Errorf("premiumize gave an unsafe file name %q", rel)
		}
	}
	full := filepath.Join(dir, filepath.FromSlash(clean))
	if r, err := filepath.Rel(dir, full); err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("premiumize gave an unsafe file name %q", rel)
	}
	return full, nil
}

func defaultFetchClient() *http.Client {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = 60 * time.Second
	return &http.Client{Transport: tr}
}

// Fetch downloads files into dir, keeping their folders. A file already
// there at its full size is skipped and a shorter one is carried on from
// where it stopped, so a paused or interrupted download picks up again.
func Fetch(ctx context.Context, files []File, dir string, opt FetchOptions) error {
	if opt.Client == nil {
		opt.Client = defaultFetchClient()
	}
	var total int64
	targets := make([]string, len(files))
	for i, f := range files {
		p, err := SafePath(dir, f.Path)
		if err != nil {
			return err
		}
		targets[i] = p
		total += f.Size
	}
	var done int64
	report := func() {
		if opt.Progress != nil {
			opt.Progress(done, total)
		}
	}
	report()
	for i, f := range files {
		err := fetchOne(ctx, opt, f, targets[i], func(delta int64) {
			done += delta
			report()
		})
		if err != nil {
			return fmt.Errorf("downloading %s from Premiumize: %w", path.Base(f.Path), err)
		}
	}
	return nil
}

// fetchOne downloads one file to target, reporting the bytes added.
func fetchOne(ctx context.Context, opt FetchOptions, f File, target string, add func(int64)) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	var have int64
	if st, err := os.Stat(target); err == nil && st.Mode().IsRegular() {
		have = st.Size()
	}
	if f.Size > 0 && have == f.Size {
		add(have)
		return nil
	}
	if f.Size > 0 && have > f.Size {
		have = 0 // not the same file: start again
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.Link, nil)
	if err != nil {
		return errors.New("premiumize gave an invalid download link")
	}
	if have > 0 {
		req.Header.Set("Range", "bytes="+strconv.FormatInt(have, 10)+"-")
	}
	resp, err := opt.Client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		// The link carries a token: keep it out of the message.
		return errors.New("couldn't reach Premiumize's download server")
	}
	defer resp.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY
	switch {
	case have > 0 && resp.StatusCode == http.StatusPartialContent:
		flags |= os.O_APPEND
		add(have)
	case resp.StatusCode == http.StatusOK:
		flags |= os.O_TRUNC
		have = 0
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable && have > 0 && f.Size <= 0:
		add(have) // the size wasn't known and the file is already whole
		return nil
	default:
		return fmt.Errorf("the download server answered with status %d", resp.StatusCode)
	}

	out, err := os.OpenFile(target, flags, 0o644)
	if err != nil {
		return err
	}
	written := have
	buf := make([]byte, 256<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if opt.Wait != nil {
				if err := opt.Wait(ctx, n); err != nil {
					out.Close()
					return err
				}
			}
			if _, err := out.Write(buf[:n]); err != nil {
				out.Close()
				return err
			}
			written += int64(n)
			add(int64(n))
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			out.Close()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("the download was cut off: %w", rerr)
		}
	}
	if err := out.Close(); err != nil {
		return err
	}
	if f.Size > 0 && written != f.Size {
		return fmt.Errorf("got %d of %d bytes", written, f.Size)
	}
	return nil
}
