package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/jhipsman/cueapp/internal/auth"
	"github.com/jhipsman/cueapp/internal/push"
	"github.com/jhipsman/cueapp/internal/settings"
	"github.com/jhipsman/cueapp/internal/watch"
)

// Phone alerts (web push). A phone or computer turns alerts on for its
// profile (Settings > Notifications in Watch): its browser subscribes with
// Cue's VAPID key and Cue keeps the subscription. Every hour Cue looks for
// what to tell each such profile: a new episode of a show it's watching
// (aired in the last three days), and a title it asked to be reminded about
// that's out now. It keeps the alert and sends the device a push with no
// text; the device's worker (public/push-sw.js) asks for the alert and shows
// it. Each thing is told once per profile.

const (
	pushEvery     = time.Hour
	pushFirst     = 2 * time.Minute
	pushNewWithin = 3 // days: episodes aired longer ago aren't news
)

var pushKey struct {
	sync.Mutex
	k *push.Key
}

// vapidKey is Cue's push key, made and kept (encrypted) the first time.
func (s *Server) vapidKey() (*push.Key, error) {
	pushKey.Lock()
	defer pushKey.Unlock()
	if pushKey.k != nil {
		return pushKey.k, nil
	}
	if v, err := s.Settings.Get(settings.KeyPushVAPID); err == nil && v != "" {
		if k, err := push.ParseKey(v); err == nil {
			pushKey.k = k
			return k, nil
		}
	}
	k, err := push.NewKey()
	if err != nil {
		return nil, err
	}
	if err := s.Settings.Set(settings.KeyPushVAPID, k.String(), true); err != nil {
		return nil, err
	}
	pushKey.k = k
	return k, nil
}

// GET /api/push/key: the key a browser subscribes with.
func (s *Server) handlePushKey(w http.ResponseWriter, r *http.Request) {
	k, err := s.vapidKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't get alerts ready.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"key": k.Public()})
}

type pushSubReq struct {
	Endpoint string `json:"endpoint"`
}

// POST /api/push/subscribe {"endpoint"}: alerts on, on this device, for the
// active profile.
func (s *Server) handlePushSubscribe(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	var req pushSubReq
	if err := decodeJSON(r, &req); err != nil || !push.AllowedEndpoint(req.Endpoint) || len(req.Endpoint) > 1000 {
		writeError(w, http.StatusBadRequest, "That browser's alerts can't be used.")
		return
	}
	// Push services want to know who sends (the VAPID subject): Cue's
	// own https address, as this device reached it.
	origin := r.Header.Get("Origin")
	if u, err := url.Parse(origin); err != nil || u.Scheme != "https" || u.Host == "" {
		origin = "mailto:alerts@cue.invalid"
	}
	if err := s.WatchRepo.AddPushSub(pid, req.Endpoint, origin); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't turn alerts on.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/push/unsubscribe {"endpoint"}: alerts off on this device.
func (s *Server) handlePushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	var req pushSubReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that.")
		return
	}
	if pid, ok := s.WatchRepo.PushSubProfile(req.Endpoint); ok && s.ownProfile(r, pid) {
		_ = s.WatchRepo.RemovePushSub(req.Endpoint)
	}
	w.WriteHeader(http.StatusNoContent)
}

// ownProfile says whether pid is one of the signed-in account's profiles.
func (s *Server) ownProfile(r *http.Request, pid int64) bool {
	u := auth.UserFromContext(r.Context())
	if u == nil {
		return false
	}
	_, err := s.WatchRepo.Profile(u.ID, pid)
	return err == nil
}

// POST /api/push/pending {"endpoint"}: the alerts waiting for this device
// (its worker asks when a push wakes it).
func (s *Server) handlePushPending(w http.ResponseWriter, r *http.Request) {
	var req pushSubReq
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that.")
		return
	}
	pid, ok := s.WatchRepo.PushSubProfile(req.Endpoint)
	if !ok || !s.ownProfile(r, pid) {
		writeJSON(w, http.StatusOK, []watch.Alert{})
		return
	}
	alerts, err := s.WatchRepo.TakeAlerts(pid)
	if err != nil || alerts == nil {
		alerts = []watch.Alert{}
	}
	writeJSON(w, http.StatusOK, alerts)
}

// POST /api/push/test: an alert to this profile's devices now, to see that
// alerts arrive.
func (s *Server) handlePushTest(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	key := fmt.Sprintf("test:%d", time.Now().UnixNano())
	if _, err := s.WatchRepo.QueueAlert(pid, key, "Alerts are on", "Cue will tell you here when a new episode is out.", "/watch"); err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't send a test.")
		return
	}
	if n := s.wakeProfile(r.Context(), pid); n == 0 {
		writeError(w, http.StatusNotFound, "No device has alerts on for this profile.")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

var pushClient = &http.Client{Timeout: 15 * time.Second}

// wakeProfile pushes to every device of a profile, forgetting those the push
// service says are gone; it reports how many it reached.
func (s *Server) wakeProfile(ctx context.Context, pid int64) int {
	k, err := s.vapidKey()
	if err != nil {
		return 0
	}
	subs, err := s.WatchRepo.PushSubs()
	if err != nil {
		return 0
	}
	n := 0
	for _, sub := range subs {
		if sub.ProfileID != pid {
			continue
		}
		err := k.Send(ctx, pushClient, sub.Endpoint, sub.Origin)
		switch {
		case errors.Is(err, push.ErrGone):
			_ = s.WatchRepo.RemovePushSub(sub.Endpoint)
		case err != nil:
			slog.Info("alerts: push", "err", err)
		default:
			n++
		}
	}
	return n
}

// StartPush starts the hourly look for what to tell; the returned function
// stops it.
func (s *Server) StartPush() (stop func()) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		t := time.NewTimer(pushFirst)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
			s.pushRound(ctx)
			t.Reset(pushEvery)
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

// pushRound queues and sends what each profile with alerts on should hear.
func (s *Server) pushRound(ctx context.Context) {
	subs, err := s.WatchRepo.PushSubs()
	if err != nil || len(subs) == 0 {
		return
	}
	profiles := map[int64]bool{}
	for _, sub := range subs {
		profiles[sub.ProfileID] = true
	}
	for pid := range profiles {
		if ctx.Err() != nil {
			return
		}
		rctx, cancel := context.WithTimeout(ctx, time.Minute)
		if s.queueNews(rctx, pid) > 0 {
			s.wakeProfile(rctx, pid)
		}
		cancel()
	}
}

// queueNews keeps alerts for a profile's news (new episodes of what it's
// watching, reminded titles now out) and reports how many are new.
func (s *Server) queueNews(ctx context.Context, pid int64) int {
	n := 0
	tell := func(key, title, body, link string) {
		if ok, _ := s.WatchRepo.QueueAlert(pid, key, title, body, link); ok {
			n++
		}
	}
	tm := s.TMDB()
	if tm == nil || !tm.HasAPIKey() {
		return 0
	}
	// A favorite team's game about to start.
	s.kickoffAlerts(ctx, pid, tell)
	// Friday's look at the week ahead on this profile's shows.
	if key, title, body, ok := s.weekAhead(ctx, pid, time.Now()); ok {
		tell(key, title, body, "/watch/coming-soon?show=tv")
	}
	cont := s.continueWatching(ctx, pid)
	cands := append([]watchCard{}, cont...)
	cands = append(cands, s.newEpisodes(ctx, pid, cont)...)
	// Day one: what's just come out, sent to Premiumize ahead of time
	// (play_prepare.go), in the background so the alerts don't wait.
	var wants []prepareWant
	for _, c := range cands {
		if c.Kind != watch.KindTV || !c.New || c.Episode < 1 {
			continue
		}
		key := fmt.Sprintf("ep:%d:%d:%d", c.TMDBID, c.Season, c.Episode)
		eps, err := tm.GetSeasonCached(ctx, c.TMDBID, c.Season)
		if err != nil {
			continue
		}
		told := s.WatchRepo.AlreadyTold(pid, key)
		for _, e := range eps {
			// Out in the last two days: ready ahead of time (told or not).
			if e.Episode == c.Episode && recentlyAired(e.AirDate, 2) {
				wants = append(wants, prepareWant{tmdbID: c.TMDBID, season: c.Season, episode: c.Episode, link: fmt.Sprintf("/watch/tv/%d", c.TMDBID)})
			}
			if !told && e.Episode == c.Episode && recentlyAired(e.AirDate, pushNewWithin) {
				body := fmt.Sprintf("S%d:E%d", c.Season, c.Episode)
				if c.EpisodeTitle != "" {
					body += " · " + c.EpisodeTitle
				}
				tell(key, "New episode of "+c.Title, body, fmt.Sprintf("/watch/tv/%d", c.TMDBID))
			}
		}
	}
	out, _ := s.remindedCards(ctx, pid)
	for _, c := range out {
		if c.Kind == watch.KindMovie {
			wants = append(wants, prepareWant{tmdbID: c.TMDBID, link: fmt.Sprintf("/watch/movie/%d", c.TMDBID)})
		}
	}
	if len(wants) > 0 {
		go func() {
			pctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			defer cancel()
			s.prepareNews(pctx, pid, wants)
		}()
	}
	for _, c := range out {
		tell(fmt.Sprintf("out:%s:%d", c.Kind, c.TMDBID), c.Title+" is out", "You asked to be told: it's ready to watch on Cue.",
			fmt.Sprintf("/watch/%s/%d", c.Kind, c.TMDBID))
	}
	return n
}
