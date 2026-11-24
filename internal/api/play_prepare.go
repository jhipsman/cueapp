package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/premiumize"
)

// Day-one releases. Play streams only what Premiumize already has in its
// cloud, so on the day something comes out, when versions of it have just
// appeared, there's often nothing ready yet ("Premiumize doesn't have any
// of them ready to stream yet"). Get it ready sends the best of those
// versions to Premiumize to fetch (a popular new release usually takes a
// few minutes), keeps an eye on it, and once it's there Play finds it like
// anything else; whoever asked hears on their phone that it's ready.
//
// The hourly alert round does the same by itself for the new episodes of
// what a profile is watching and the films it asked to be reminded of
// (prepareNews), so on release day they're ready before anyone sits down.

const (
	// prepareTries: how many versions are kept to send, one after another
	// if Premiumize can't fetch one.
	prepareTries = 3
	// prepareFor: how long a fetch is watched before it's given up on.
	prepareFor = 3 * time.Hour
)

// prepareEvery: how often a fetch's progress is read (tests shorten it).
var prepareEvery = 15 * time.Second

type prepareJob struct {
	mu       sync.Mutex
	key      string
	label    string
	link     string // the title's page, for the alert
	state    string // "preparing", "ready", "failed"
	progress float64
	message  string
	release  string
	hash     string
	started  time.Time
	pids     map[int64]bool // who to tell when it's ready
	left     []playCandidate
}

type prepareStatus struct {
	State    string  `json:"state"` // "none", "preparing", "ready", "failed"
	Progress float64 `json:"progress,omitempty"`
	Message  string  `json:"message,omitempty"`
	Release  string  `json:"release,omitempty"`
}

func (j *prepareJob) status() prepareStatus {
	j.mu.Lock()
	defer j.mu.Unlock()
	return prepareStatus{State: j.state, Progress: j.progress, Message: j.message, Release: j.release}
}

var (
	preparesMu sync.Mutex
	prepares   = map[string]*prepareJob{} // playTarget.key -> its fetch
	// preparedHashes: torrents a fetch finished, counted as ready to stream
	// even before Premiumize's cache check says so.
	preparedHashes sync.Map
)

// prepared says a fetch of this torrent finished.
func (s *Server) prepared(hash string) bool {
	_, ok := preparedHashes.Load(strings.ToLower(hash))
	return ok
}

func prepareJobFor(key string) *prepareJob {
	preparesMu.Lock()
	defer preparesMu.Unlock()
	return prepares[key]
}

// startPrepare sends the best version of t that isn't in Premiumize's cloud
// yet to be fetched, unless one is on its way already. pid, when not 0, is
// told when it's ready.
func (s *Server) startPrepare(ctx context.Context, t playTarget, pid int64, link string) (prepareStatus, error) {
	if j := prepareJobFor(t.key); j != nil {
		j.mu.Lock()
		running := j.state != "failed"
		if running && pid != 0 {
			j.pids[pid] = true
		}
		j.mu.Unlock()
		if running {
			return j.status(), nil
		}
	}
	pm := s.premiumizeClient()
	if pm == nil {
		return prepareStatus{}, fmt.Errorf("getting something ready needs Premiumize: add your API key in Settings > Downloading")
	}
	entry, err := s.findStreamable(ctx, pm, t)
	if err != nil {
		return prepareStatus{}, err
	}
	if len(entry.candidates) > 0 {
		// It's in Premiumize's cloud already: Play has it.
		playCache.put(t.key, entry)
		return prepareStatus{State: "ready"}, nil
	}
	if len(entry.uncached) == 0 {
		msg := entry.nothing
		if msg == "" {
			msg = "No versions of " + t.label + " were found yet."
		}
		return prepareStatus{State: "none", Message: msg}, nil
	}
	j := &prepareJob{key: t.key, label: t.label, link: link, state: "preparing", started: time.Now(), pids: map[int64]bool{}, left: entry.uncached}
	if pid != 0 {
		j.pids[pid] = true
	}
	id, err := s.sendNext(ctx, pm, j)
	if err != nil {
		return prepareStatus{}, err
	}
	preparesMu.Lock()
	prepares[t.key] = j
	preparesMu.Unlock()
	slog.Info("play: getting it ready", "title", t.label, "release", j.release)
	go s.watchPrepare(pm, j, id)
	return j.status(), nil
}

// sendNext sends the next version still to try; its transfer id.
func (s *Server) sendNext(ctx context.Context, pm *premiumize.Client, j *prepareJob) (string, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	var lastErr error
	for len(j.left) > 0 {
		c := j.left[0]
		j.left = j.left[1:]
		magnet := "magnet:?xt=urn:btih:" + c.Hash + "&dn=" + url.QueryEscape(c.Release)
		id, err := pm.CreateTransfer(ctx, magnet)
		if err != nil {
			lastErr = err
			continue
		}
		j.release, j.hash, j.progress, j.state, j.message = c.Release, strings.ToLower(c.Hash), 0, "preparing", ""
		return id, nil
	}
	j.state = "failed"
	if lastErr != nil {
		j.message = "Premiumize wouldn't fetch it: " + lastErr.Error()
		return "", lastErr
	}
	j.message = "Premiumize couldn't fetch any of the versions found. Try again later."
	return "", fmt.Errorf("%s", j.message)
}

// watchPrepare follows a fetch to its end, moving on to the next version if
// one fails, and tells whoever asked once it's ready.
func (s *Server) watchPrepare(pm *premiumize.Client, j *prepareJob, id string) {
	ctx, cancel := context.WithTimeout(context.Background(), prepareFor)
	defer cancel()
	tick := time.NewTicker(prepareEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			j.mu.Lock()
			j.state, j.message = "failed", "Premiumize was still fetching it after three hours. Try again later."
			j.mu.Unlock()
			return
		case <-tick.C:
		}
		tr, err := pm.Transfer(ctx, id)
		if err != nil {
			continue // a hiccup: read it again next time
		}
		switch {
		case tr.Done():
			j.mu.Lock()
			j.state, j.progress = "ready", 1
			preparedHashes.Store(j.hash, true)
			pids := make([]int64, 0, len(j.pids))
			for pid := range j.pids {
				pids = append(pids, pid)
			}
			j.mu.Unlock()
			playCache.del(j.key) // Play looks again, and finds it
			slog.Info("play: ready", "title", j.label, "release", j.release, "took", time.Since(j.started).Round(time.Second))
			s.tellReady(ctx, j, pids)
			return
		case tr.Failed():
			next, err := s.sendNext(ctx, pm, j)
			if err != nil {
				slog.Info("play: couldn't get it ready", "title", j.label, "err", err)
				return
			}
			id = next
		default:
			j.mu.Lock()
			j.progress = tr.Progress
			j.mu.Unlock()
		}
	}
}

// tellReady tells each profile that asked (or was waiting on it) that it's
// ready to watch.
func (s *Server) tellReady(ctx context.Context, j *prepareJob, pids []int64) {
	for _, pid := range pids {
		if ok, _ := s.WatchRepo.QueueAlert(pid, "ready:"+j.key, j.label+" is ready to watch", "It's in your Premiumize cloud now: press Play.", j.link); ok {
			s.wakeProfile(ctx, pid)
		}
	}
}

// prepareTarget reads the title a prepare request is for.
func (s *Server) prepareTarget(r *http.Request) (playTarget, string, int, string) {
	tmdbID, err := strconv.Atoi(r.PathValue("tmdbId"))
	if err != nil || tmdbID <= 0 {
		return playTarget{}, "", http.StatusBadRequest, "That isn't a valid title."
	}
	if r.PathValue("season") == "" {
		t, status, msg := s.tmdbMovieTarget(r.Context(), tmdbID)
		return t, fmt.Sprintf("/watch/movie/%d", tmdbID), status, msg
	}
	season, err1 := strconv.Atoi(r.PathValue("season"))
	episode, err2 := strconv.Atoi(r.PathValue("episode"))
	if err1 != nil || err2 != nil || season < 0 || episode < 1 {
		return playTarget{}, "", http.StatusBadRequest, "That isn't a valid episode."
	}
	t, status, msg := s.tmdbEpisodeTarget(r.Context(), tmdbID, season, episode)
	return t, fmt.Sprintf("/watch/tv/%d", tmdbID), status, msg
}

// POST /api/play/prepare/movie/{tmdbId}, POST /api/play/prepare/tv/{tmdbId}/{season}/{episode}:
// get it ready (see above). GET the same: how it's going ({state: none |
// preparing | ready | failed, progress, message, release}).
func (s *Server) handlePrepare(w http.ResponseWriter, r *http.Request) {
	kind := "movie"
	if r.PathValue("season") != "" {
		kind = "tv"
	}
	if id, _ := strconv.Atoi(r.PathValue("tmdbId")); s.ageBlocked(w, r, kind, id) {
		return
	}
	t, link, status, msg := s.prepareTarget(r)
	if status != 0 {
		writeError(w, status, msg)
		return
	}
	if r.Method == http.MethodGet {
		if j := prepareJobFor(t.key); j != nil {
			writeJSON(w, http.StatusOK, j.status())
			return
		}
		writeJSON(w, http.StatusOK, prepareStatus{State: "none"})
		return
	}
	var pid int64
	if p, ok := s.activeProfile(r); ok {
		pid = p.ID
	}
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	st, err := s.startPrepare(ctx, t, pid, link)
	if err != nil {
		writeError(w, http.StatusBadGateway, "Couldn't get it ready: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, st)
}

// prepareNews, in the hourly alert round: new episodes of what pid is
// watching (aired in the last two days) and reminded films now out, sent to
// Premiumize ahead of time when they aren't there yet. A few per round.
func (s *Server) prepareNews(ctx context.Context, pid int64, targets []prepareWant) {
	if s.premiumizeClient() == nil {
		return
	}
	sent := 0
	for _, want := range targets {
		if sent >= 4 || ctx.Err() != nil {
			return
		}
		var (
			t      playTarget
			status int
		)
		if want.season == 0 && want.episode == 0 {
			t, status, _ = s.tmdbMovieTarget(ctx, want.tmdbID)
		} else {
			t, status, _ = s.tmdbEpisodeTarget(ctx, want.tmdbID, want.season, want.episode)
		}
		if status != 0 {
			continue
		}
		if e, ok := playCache.get(t.key); ok && len(e.candidates) > 0 {
			continue // playable already
		}
		st, err := s.startPrepare(ctx, t, pid, want.link)
		if err == nil && st.State == "preparing" {
			sent++
		}
	}
}

type prepareWant struct {
	tmdbID, season, episode int
	link                    string
}
