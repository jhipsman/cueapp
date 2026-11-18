package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jhipsman/cueapp/internal/auth"
	"github.com/jhipsman/cueapp/internal/watch"
)

// Report a problem. Anyone watching can say what's wrong with a title in a
// couple of presses (no sound, subtitles, the picture, the wrong video, it
// keeps stopping). The owner gets an alert, sees the reports in Watch
// (Reports), fixes it in one press (this version is the wrong video: never
// again) or says why not, and the person who reported hears back.

var problemNames = map[string]string{
	"sound": "Sound", "subtitles": "Subtitles", "picture": "Picture", "wrong": "Wrong video", "buffering": "Keeps stopping", "other": "Something else",
}

// POST /api/reports {kind, tmdbId, season, episode, title, release, problem, note}
func (s *Server) handleAddReport(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	var req struct {
		Kind    string `json:"kind"`
		TMDBID  int    `json:"tmdbId"`
		Season  int    `json:"season"`
		Episode int    `json:"episode"`
		Title   string `json:"title"`
		Release string `json:"release"`
		Problem string `json:"problem"`
		Note    string `json:"note"`
	}
	if err := decodeJSON(r, &req); err != nil || (req.Kind != watch.KindMovie && req.Kind != watch.KindTV) || req.TMDBID <= 0 || !watch.Problems[req.Problem] {
		writeError(w, http.StatusBadRequest, "That isn't a problem Cue knows.")
		return
	}
	if req.Kind == watch.KindMovie {
		req.Season, req.Episode = 0, 0
	}
	rp, err := s.WatchRepo.AddReport(watch.Report{ProfileID: pid, Kind: req.Kind, TMDBID: req.TMDBID, Season: req.Season, Episode: req.Episode,
		Title: truncate(req.Title, 200), Release: truncate(req.Release, 500), Problem: req.Problem, Note: truncate(req.Note, 500)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't send that.")
		return
	}
	// The owner hears about it (alerts on, on their phone).
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		owners, _ := s.WatchRepo.OwnerProfiles()
		for _, o := range owners {
			if o == pid {
				continue
			}
			body := problemNames[rp.Problem]
			if rp.Note != "" {
				body += ": " + rp.Note
			}
			if ok, _ := s.WatchRepo.QueueAlert(o, fmt.Sprintf("report:%d", rp.ID), rp.Who+" reported a problem with "+rp.Title, body, "/watch/reports"); ok {
				s.wakeProfile(ctx, o)
			}
		}
	}()
	writeJSON(w, http.StatusOK, rp)
}

// isOwner says whether the signed-in account is an administrator.
func isOwner(r *http.Request) bool {
	u := auth.UserFromContext(r.Context())
	return u != nil && u.IsAdmin
}

// GET /api/reports: every report for the owner (open first); your own for
// everyone else.
func (s *Server) handleListReports(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	of := pid
	if isOwner(r) {
		of = 0
	}
	list, err := s.WatchRepo.Reports(of)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't read the reports.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reports": list, "open": s.WatchRepo.OpenReports(), "owner": isOwner(r)})
}

// POST /api/reports/{id}/resolve {status: fixed|wontfix, reply, wrongVideo}:
// the owner's answer. wrongVideo also marks the reported version the wrong
// video for the title (never used for it again) and throws away its clip.
func (s *Server) handleResolveReport(w http.ResponseWriter, r *http.Request) {
	if !isOwner(r) {
		writeError(w, http.StatusForbidden, "Only the owner answers reports.")
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "That isn't a report.")
		return
	}
	var req struct {
		Status     string `json:"status"`
		Reply      string `json:"reply"`
		WrongVideo bool   `json:"wrongVideo"`
	}
	if err := decodeJSON(r, &req); err != nil || (req.Status != "fixed" && req.Status != "wontfix") {
		writeError(w, http.StatusBadRequest, `A report is answered "fixed" or "wontfix".`)
		return
	}
	rp, err := s.WatchRepo.Report(id)
	if errors.Is(err, watch.ErrReportNotFound) {
		writeError(w, http.StatusNotFound, "That report doesn't exist any more.")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't read the report.")
		return
	}
	if req.WrongVideo && strings.TrimSpace(rp.Release) != "" {
		_ = s.WatchRepo.MarkWrong(rp.Kind, rp.TMDBID, rp.Season, rp.Episode, rp.Release)
		want := previewWant{kind: rp.Kind, tmdbID: rp.TMDBID, season: rp.Season, episode: rp.Episode}
		if rp.Kind == watch.KindTV && want.episode < 1 {
			want.season, want.episode = 1, 1
		}
		_ = os.Remove(filepath.Join(s.previewDir(), want.key()+".mp4"))
	}
	rp, err = s.WatchRepo.ResolveReport(id, req.Status, truncate(req.Reply, 500))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't save that.")
		return
	}
	// The reporter hears back.
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		head := rp.Title + ": fixed"
		if rp.Status == "wontfix" {
			head = rp.Title + ": couldn't be fixed"
		}
		body := rp.Reply
		if body == "" {
			body = "About the problem you reported (" + strings.ToLower(problemNames[rp.Problem]) + ")."
		}
		if ok, _ := s.WatchRepo.QueueAlert(rp.ProfileID, fmt.Sprintf("report-answer:%d:%d", rp.ID, rp.UpdatedAt.Unix()), head, body, fmt.Sprintf("/watch/%s/%d", rp.Kind, rp.TMDBID)); ok {
			s.wakeProfile(ctx, rp.ProfileID)
		}
	}()
	writeJSON(w, http.StatusOK, rp)
}

// GET /api/reports/news: answers to this profile's reports it hasn't seen
// (shown once, on Home).
func (s *Server) handleReportNews(w http.ResponseWriter, r *http.Request) {
	pid, ok := s.watchProfile(w, r)
	if !ok {
		return
	}
	news, err := s.WatchRepo.ReportNews(pid)
	if err != nil {
		news = []watch.Report{}
	}
	writeJSON(w, http.StatusOK, news)
}
