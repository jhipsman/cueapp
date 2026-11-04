package watch

import (
	"encoding/json"
	"fmt"
)

// StreamPrefs is how a profile wants versions picked when it presses Play,
// as in apps like Nuvio: the picture to aim for, how big a file may be, and
// what to prefer or avoid.
type StreamPrefs struct {
	// Quality is the picture to aim for: "4k", "1080p" or "720p". Better
	// than this is used only when nothing at it is found.
	Quality string `json:"quality"`
	// Size is how big a version may be, for a two-hour film (an episode or
	// a longer film in proportion): "smart" (the size streaming services
	// use for that picture: sharp, starts fast), "10", "20", "30", "50"
	// (gigabytes at most) or "any" (the add-on's order, often the biggest).
	Size string `json:"size"`
	// Remux: "avoid" (straight Blu-ray copies, 40-80 GB) or "allow".
	Remux string `json:"remux"`
	// HDR: "prefer" or "any".
	HDR string `json:"hdr"`
	// DolbyVision: "prefer", "allow" or "avoid" (for TVs that show it with
	// odd colours).
	DolbyVision string `json:"dolbyVision"`
	// Efficient: prefer HEVC and AV1 (the same picture in half the size).
	Efficient bool `json:"efficient"`
	// LightSound: prefer Dolby Digital Plus and Atmos over TrueHD and DTS-HD
	// (much bigger, and most TVs can't send them to speakers anyway).
	LightSound bool `json:"lightSound"`
}

// DefaultStreamPrefs: 4K at the size streaming services use, no Remux.
var DefaultStreamPrefs = StreamPrefs{Quality: "4k", Size: "smart", Remux: "avoid", HDR: "prefer", DolbyVision: "allow", Efficient: true, LightSound: true}

var (
	prefQualities = map[string]bool{"4k": true, "1080p": true, "720p": true}
	prefSizes     = map[string]bool{"smart": true, "10": true, "20": true, "30": true, "50": true, "any": true}
)

// Clean fills anything missing or unknown from DefaultStreamPrefs.
func (p StreamPrefs) Clean() StreamPrefs {
	d := DefaultStreamPrefs
	if !prefQualities[p.Quality] {
		p.Quality = d.Quality
	}
	if !prefSizes[p.Size] {
		p.Size = d.Size
	}
	if p.Remux != "avoid" && p.Remux != "allow" {
		p.Remux = d.Remux
	}
	if p.HDR != "prefer" && p.HDR != "any" {
		p.HDR = d.HDR
	}
	if p.DolbyVision != "prefer" && p.DolbyVision != "allow" && p.DolbyVision != "avoid" {
		p.DolbyVision = d.DolbyVision
	}
	return p
}

func parseStreamPrefs(s string) StreamPrefs {
	if s == "" {
		return DefaultStreamPrefs
	}
	var p StreamPrefs
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		return DefaultStreamPrefs
	}
	return p.Clean()
}

// SetStreamPrefs saves one of userID's profiles' streaming preferences.
func (r *Repo) SetStreamPrefs(userID, id int64, p StreamPrefs) (StreamPrefs, error) {
	p = p.Clean()
	b, _ := json.Marshal(p)
	res, err := r.db.Exec(`UPDATE profiles SET stream_prefs = ? WHERE id = ? AND user_id = ?`, string(b), id, userID)
	if err != nil {
		return StreamPrefs{}, fmt.Errorf("save streaming preferences: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return StreamPrefs{}, ErrProfileNotFound
	}
	return p, nil
}
