package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/anacrolix/torrent/metainfo"

	"github.com/rdborg/mediarium/internal/indexers"
	"github.com/rdborg/mediarium/internal/premiumize"
	"github.com/rdborg/mediarium/internal/queue"
	"github.com/rdborg/mediarium/internal/settings"
	"github.com/rdborg/mediarium/internal/speed"
)

// Premiumize.me as a downloader.
//
// With an API key saved (Settings > Downloading > Usenet and torrents, under Cloud downloader), the releases
// it is chosen for (torrents by default, Usenet too if asked) are not fetched
// by the built-in engines. They are sent to Premiumize, which downloads them on
// its own servers (or already has them cached), and the finished files are then
// downloaded from Premiumize over HTTPS into the item's working folder. From
// there repair, unpacking and import carry on as for any other download.
//
// Nothing torrent-related runs on this machine for such a download: no peers,
// no seeding, and so no need for the VPN.

const (
	premiumizeUseTorrents = "torrents"
	premiumizeUseUsenet   = "usenet"
	premiumizeUseBoth     = "both"

	// premiumizePoll is how often a transfer's progress is checked.
	premiumizePoll = 10 * time.Second
	// premiumizeCloudShare is the part of the progress bar the transfer on
	// Premiumize's side fills; the download from Premiumize fills the rest.
	premiumizeCloudShare = 50.0
)

var premiumizePollEvery = premiumizePoll // tests shorten it

// premiumizeClient is the client for the saved key, or nil without one.
func (s *Server) premiumizeClient() *premiumize.Client {
	key, err := s.Settings.Get(settings.KeyPremiumizeAPIKey)
	if err != nil || strings.TrimSpace(key) == "" {
		return nil
	}
	return premiumize.New(key, s.premiumizeBase)
}

func (s *Server) premiumizeUseFor() string {
	v, _ := s.Settings.Get(settings.KeyPremiumizeUseFor)
	switch v {
	case premiumizeUseUsenet, premiumizeUseBoth:
		return v
	}
	return premiumizeUseTorrents
}

// usePremiumize says a release of this protocol is downloaded through
// Premiumize.
func (s *Server) usePremiumize(protocol indexers.Protocol) bool {
	if s.premiumizeClient() == nil {
		return false
	}
	use := s.premiumizeUseFor()
	if protocol == indexers.ProtocolTorrent {
		return use == premiumizeUseTorrents || use == premiumizeUseBoth
	}
	return use == premiumizeUseUsenet || use == premiumizeUseBoth
}

// premiumizeSource is what is sent to Premiumize for one release: a link it
// can fetch itself, or a file's bytes to upload. magnet, when known, is used
// to ask whether Premiumize already has the torrent cached.
type premiumizeSource struct {
	src      string
	fileName string
	data     []byte
	magnet   string
}

// premiumizeSourceFor turns a grab's download URL into what Premiumize is
// given. Indexer links are fetched here (they may need the indexer's session
// or key, which must not be handed to Premiumize).
func (s *Server) premiumizeSourceFor(ctx context.Context, downloadURL string, protocol indexers.Protocol, workDir string) (premiumizeSource, error) {
	if protocol == indexers.ProtocolTorrent {
		magnet, torrentFile, err := s.torrentSource(ctx, downloadURL, workDir)
		if err != nil {
			return premiumizeSource{}, fmt.Errorf("couldn't get the torrent: %w", err)
		}
		if magnet != "" {
			return premiumizeSource{src: magnet, magnet: magnet}, nil
		}
		data, err := os.ReadFile(torrentFile)
		_ = os.Remove(torrentFile) // only the downloaded files belong in the folder
		if err != nil {
			return premiumizeSource{}, fmt.Errorf("couldn't read the .torrent file: %w", err)
		}
		mi, err := metainfo.Load(bytes.NewReader(data))
		if err != nil {
			return premiumizeSource{}, badRelease(fmt.Errorf("couldn't read the .torrent file: %w", err))
		}
		// The file itself is uploaded (a private tracker's passkey stays in
		// it); the magnet is only for asking whether it is cached.
		src := premiumizeSource{fileName: "release.torrent", data: data}
		if m, err := mi.MagnetV2(); err == nil {
			src.magnet = m.String()
		}
		return src, nil
	}
	if isMagnetURI(downloadURL) {
		return premiumizeSource{}, errors.New("this is a torrent (a magnet link), but its indexer is set up as a Usenet indexer. Open Settings > Indexers & Search, remove it and add it again on the Torrent tab")
	}
	data, err := s.fetchRelease(ctx, downloadURL)
	if err != nil {
		return premiumizeSource{}, fmt.Errorf("couldn't get the NZB file: %w", err)
	}
	if looksLikeTorrentFile(data) {
		return premiumizeSource{}, errors.New("this is a .torrent file, but its indexer is set up as a Usenet indexer. Open Settings > Indexers & Search, remove it and add it again on the Torrent tab")
	}
	return premiumizeSource{fileName: "release.nzb", data: data}, nil
}

// downloadPremiumize fetches a release through Premiumize into workDir.
func (s *Server) downloadPremiumize(ctx context.Context, queueID int64, downloadURL string, protocol indexers.Protocol, workDir string) error {
	pm := s.premiumizeClient()
	if pm == nil {
		return errors.New("premiumize isn't set up any more. Add your API key again in Settings > Downloading > Usenet and torrents, under Cloud downloader")
	}
	if protocol == indexers.ProtocolTorrent && !s.torrentsEnabled() {
		return errTorrentsDisabled
	}
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		return fmt.Errorf("couldn't create the download folder: %w", err)
	}
	src, err := s.premiumizeSourceFor(ctx, downloadURL, protocol, workDir)
	if err != nil {
		return err
	}
	progress := newProgressSaver(func(pct float64) error { return s.QueueRepo.SetProgress(queueID, pct) })
	defer progress.Flush()

	// A torrent Premiumize already has is ready at once, with no transfer.
	var files []premiumize.File
	if src.magnet != "" {
		if cached, err := pm.DirectDL(ctx, src.magnet); err == nil {
			files = cached
			s.queueEvent(queueID, "premiumize", queue.LevelInfo, "Premiumize already has this release. Downloading it from Premiumize.")
		} else if errors.Is(err, premiumize.ErrBadKey) {
			return premiumizeKeyError()
		}
	}

	var transfer premiumize.Transfer
	if files == nil {
		transfer, err = s.premiumizeTransfer(ctx, pm, queueID, src, progress)
		if err != nil {
			return err
		}
		// However the rest ends (done, failed, paused), the transfer and its
		// cloud copy go: a torrent comes back from Premiumize's cache on a
		// retry, and the files already here are carried on from.
		defer s.premiumizeCleanup(pm, transfer)
		if files, err = pm.Files(ctx, transfer); err != nil {
			return fmt.Errorf("couldn't list the files on Premiumize: %w", err)
		}
		s.queueEvent(queueID, "premiumize", queue.LevelInfo, "Premiumize has the release. Downloading it from Premiumize.")
	}
	if len(files) == 0 {
		return badRelease(errors.New("premiumize finished the release but it has no files"))
	}

	err = premiumize.Fetch(ctx, files, workDir, premiumize.FetchOptions{
		Wait: speed.Wait,
		Progress: func(done, total int64) {
			reportPercent(progress, premiumizeCloudShare+(100-premiumizeCloudShare)*float64(done)/float64(max(total, 1)))
		},
	})
	return err
}

// premiumizeTransfer sends the release to Premiumize and waits until it has
// it. A pause or stop takes the transfer off Premiumize's list again.
func (s *Server) premiumizeTransfer(ctx context.Context, pm *premiumize.Client, queueID int64, src premiumizeSource, progress *progressSaver) (premiumize.Transfer, error) {
	var (
		id  string
		err error
	)
	if src.data != nil {
		id, err = pm.CreateTransferFile(ctx, src.fileName, src.data)
	} else {
		id, err = pm.CreateTransfer(ctx, src.src)
	}
	if errors.Is(err, premiumize.ErrBadKey) {
		return premiumize.Transfer{}, premiumizeKeyError()
	}
	if err != nil {
		return premiumize.Transfer{}, fmt.Errorf("couldn't send the release to Premiumize: %w", err)
	}
	s.queueEvent(queueID, "premiumize", queue.LevelInfo, "Sent to Premiumize. Waiting for Premiumize to download it.")

	tick := time.NewTicker(premiumizePollEvery)
	defer tick.Stop()
	failures := 0
	for {
		t, err := pm.Transfer(ctx, id)
		switch {
		case err == nil:
			failures = 0
			if t.Done() {
				return t, nil
			}
			if t.Failed() {
				s.premiumizeCleanup(pm, t)
				reason := strings.TrimSpace(t.Message)
				if reason == "" {
					reason = t.Status
				}
				failure := fmt.Errorf("premiumize couldn't download it (%s)", reason)
				if t.Status == premiumize.StatusDeleted {
					return t, failure // removed by hand: not the release's fault
				}
				return t, badRelease(failure)
			}
			reportPercent(progress, premiumizeCloudShare*min(max(t.Progress, 0), 1))
		case errors.Is(err, premiumize.ErrBadKey):
			return t, premiumizeKeyError()
		case ctx.Err() == nil:
			// A hiccup reaching Premiumize: keep waiting, but not forever.
			failures++
			if failures >= 30 {
				return t, fmt.Errorf("couldn't check the transfer on Premiumize: %w", err)
			}
		}
		select {
		case <-ctx.Done():
			// Paused, stopped or removed: don't leave the job running there.
			cctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			if err := pm.DeleteTransfer(cctx, id); err != nil {
				slog.Info("premiumize: remove transfer after stop", "queueId", queueID, "err", err)
			}
			cancel()
			return premiumize.Transfer{}, ctx.Err()
		case <-tick.C:
		}
	}
}

// premiumizeCleanup takes a finished or failed transfer off Premiumize's list
// and removes the files it left in the cloud, which count against the
// account's storage.
func (s *Server) premiumizeCleanup(pm *premiumize.Client, t premiumize.Transfer) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := pm.DeleteTransfer(ctx, t.ID); err != nil {
		slog.Info("premiumize: remove transfer", "id", t.ID, "err", err)
	}
	switch {
	case t.FolderID != "":
		if err := pm.DeleteFolder(ctx, t.FolderID); err != nil {
			slog.Info("premiumize: remove cloud folder", "id", t.FolderID, "err", err)
		}
	case t.FileID != "":
		if err := pm.DeleteItem(ctx, t.FileID); err != nil {
			slog.Info("premiumize: remove cloud file", "id", t.FileID, "err", err)
		}
	}
}

func premiumizeKeyError() error {
	return errors.New("premiumize didn't accept your API key. Check it in Settings > Downloading > Usenet and torrents, under Cloud downloader")
}

// reportPercent hands a percentage to a progressSaver, which takes bytes.
func reportPercent(p *progressSaver, pct float64) {
	p.Report(int64(pct*100), 100*100)
}

// premiumizeState is what the settings page shows.
type premiumizeState struct {
	Set          bool    `json:"set"`
	UseFor       string  `json:"useFor"`
	CustomerID   string  `json:"customerId,omitempty"`
	PremiumUntil int64   `json:"premiumUntil,omitempty"` // Unix seconds
	LimitUsed    float64 `json:"limitUsed,omitempty"`    // 0 to 1
	Error        string  `json:"error,omitempty"`        // the saved key no longer works
}

// GET /api/settings/premiumize: whether a key is saved, what it is used for
// and, when Premiumize answers, the account it belongs to.
func (s *Server) handleGetPremiumize(w http.ResponseWriter, r *http.Request) {
	state := premiumizeState{UseFor: s.premiumizeUseFor()}
	if pm := s.premiumizeClient(); pm != nil {
		state.Set = true
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		acct, err := pm.AccountInfo(ctx)
		switch {
		case errors.Is(err, premiumize.ErrBadKey):
			state.Error = "Premiumize no longer accepts the saved API key."
		case err != nil:
			state.Error = "Premiumize couldn't be reached just now."
		default:
			state.CustomerID, state.PremiumUntil, state.LimitUsed = acct.CustomerID, acct.PremiumUntil, acct.LimitUsed
		}
	}
	writeJSON(w, http.StatusOK, state)
}

// PUT /api/settings/premiumize {"apiKey": "...", "useFor": "torrents"}: an
// API key is checked with Premiumize and saved (encrypted); an empty one
// removes it. Leaving apiKey out keeps the saved key and only changes useFor.
func (s *Server) handlePutPremiumize(w http.ResponseWriter, r *http.Request) {
	var req struct {
		APIKey *string `json:"apiKey"`
		UseFor string  `json:"useFor"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "That request couldn't be read.")
		return
	}
	switch req.UseFor {
	case "", premiumizeUseTorrents, premiumizeUseUsenet, premiumizeUseBoth:
	default:
		writeError(w, http.StatusBadRequest, `"Use Premiumize for" must be torrents, usenet or both.`)
		return
	}
	state := premiumizeState{}
	if req.APIKey != nil {
		key := strings.TrimSpace(*req.APIKey)
		if len(key) > 256 || strings.ContainsAny(key, " \r\n\t") {
			writeError(w, http.StatusBadRequest, "That doesn't look like a Premiumize API key. Copy it again from premiumize.me/account.")
			return
		}
		if key != "" {
			ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			defer cancel()
			acct, err := premiumize.New(key, s.premiumizeBase).AccountInfo(ctx)
			if errors.Is(err, premiumize.ErrBadKey) {
				writeError(w, http.StatusBadRequest, "Premiumize didn't accept that API key. Copy it again from premiumize.me/account.")
				return
			}
			if err != nil {
				slog.Warn("premiumize: check API key", "err", err)
				writeError(w, http.StatusBadGateway, "Couldn't reach Premiumize to check the key. Try again in a moment.")
				return
			}
			state.CustomerID, state.PremiumUntil, state.LimitUsed = acct.CustomerID, acct.PremiumUntil, acct.LimitUsed
		}
		if err := s.Settings.Set(settings.KeyPremiumizeAPIKey, key, true); err != nil {
			writeError(w, http.StatusInternalServerError, "The API key couldn't be saved.")
			return
		}
	}
	if req.UseFor != "" {
		if err := s.Settings.Set(settings.KeyPremiumizeUseFor, req.UseFor, false); err != nil {
			writeError(w, http.StatusInternalServerError, "The setting couldn't be saved.")
			return
		}
	}
	state.Set = s.premiumizeClient() != nil
	state.UseFor = s.premiumizeUseFor()
	writeJSON(w, http.StatusOK, state)
}
