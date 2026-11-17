package api

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jhipsman/cueapp/internal/auth"
)

// Watch parties, as the old Netflix Party did it: one person starts a party
// from the player and sends its link; whoever opens it (signed in to this
// Cue) plays the same title from the same moment. Anyone's play, pause or
// jump happens for everyone, with a line in the chat saying who did it, and
// there's a chat and emoji reactions at the side. Each viewer plays from
// their own stream; only the position is shared.
//
// Updates reach the players as Server-Sent Events (GET .../events), and
// everything a player does is a plain POST. Parties live in memory: they
// end when everyone has left for a while, or when Cue restarts.

const (
	partyCodeLen    = 8
	partyMaxParties = 100
	partyMaxMembers = 20
	partyMaxMsgs    = 150 // kept for whoever joins later
	partyIdle       = 6 * time.Hour
	partyGone       = 45 * time.Second // a member not heard from (no open stream) for this long has left
	partyChatMax    = 300              // characters
	partyBeat       = 20 * time.Second
)

var partyAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

// partyEmoji are the reactions a viewer can send.
var partyEmoji = map[string]bool{"😂": true, "😮": true, "😍": true, "😢": true, "👏": true, "🔥": true, "😱": true, "❤️": true}

type partyMember struct {
	Client  string `json:"-"`
	Profile int64  `json:"profile"`
	User    int64  `json:"-"`
	Name    string `json:"name"`
	Avatar  string `json:"avatar"`
	Host    bool   `json:"host"`
	streams int
	seen    time.Time
}

type partyMsg struct {
	ID     int64  `json:"id"`
	Kind   string `json:"kind"` // chat, event (someone paused...), reaction
	Name   string `json:"name,omitempty"`
	Avatar string `json:"avatar,omitempty"`
	Text   string `json:"text"`
	At     int64  `json:"at"` // Unix milliseconds
}

// partyTitle is what the party is watching.
type partyTitle struct {
	Kind    string `json:"kind"`
	TMDBID  int    `json:"tmdbId"`
	Season  int    `json:"season,omitempty"`
	Episode int    `json:"episode,omitempty"`
	Title   string `json:"title"`
	Option  int    `json:"option,omitempty"` // the version the starter plays, so others pick the same
}

type party struct {
	mu      sync.Mutex
	code    string
	title   partyTitle
	playing bool
	pos     float64   // seconds into the title at posAt
	posAt   time.Time // when pos was true
	by      string    // who made the last change
	members map[string]*partyMember
	msgs    []partyMsg
	nextID  int64
	subs    map[chan []byte]string // open event streams, by client
	active  time.Time
}

var parties = struct {
	sync.Mutex
	byCode map[string]*party
}{byCode: map[string]*party{}}

func newPartyCode() (string, error) {
	var b strings.Builder
	for range partyCodeLen {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(partyAlphabet))))
		if err != nil {
			return "", err
		}
		b.WriteByte(partyAlphabet[n.Int64()])
	}
	return b.String(), nil
}

// now is the party's position at this moment.
func (p *party) now() float64 {
	if !p.playing {
		return p.pos
	}
	return p.pos + time.Since(p.posAt).Seconds()
}

type partySnapshot struct {
	Code    string         `json:"code"`
	Title   partyTitle     `json:"title"`
	Playing bool           `json:"playing"`
	Pos     float64        `json:"pos"`
	By      string         `json:"by,omitempty"`
	Members []*partyMember `json:"members"`
}

func (p *party) snapshot() partySnapshot {
	s := partySnapshot{Code: p.code, Title: p.title, Playing: p.playing, Pos: math.Round(p.now()*100) / 100, By: p.by, Members: []*partyMember{}}
	for _, m := range p.members {
		s.Members = append(s.Members, m)
	}
	return s
}

// send writes an event to every open stream. Called with p.mu held.
func (p *party) send(event string, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		return
	}
	frame := []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, body))
	for ch := range p.subs {
		select {
		case ch <- frame:
		default: // a stream that can't keep up misses this one; the next state catches it up
		}
	}
}

// say adds a line to the chat (and sends it). Called with p.mu held.
func (p *party) say(kind, name, avatar, text string) {
	p.nextID++
	m := partyMsg{ID: p.nextID, Kind: kind, Name: name, Avatar: avatar, Text: text, At: time.Now().UnixMilli()}
	if kind != "reaction" {
		p.msgs = append(p.msgs, m)
		if len(p.msgs) > partyMaxMsgs {
			p.msgs = p.msgs[len(p.msgs)-partyMaxMsgs:]
		}
	}
	p.send("msg", m)
}

// pruneParties drops parties nobody has touched for a while. Called with
// parties locked.
func pruneParties() {
	for code, p := range parties.byCode {
		p.mu.Lock()
		idle := time.Since(p.active) > partyIdle || (len(p.members) == 0 && time.Since(p.active) > 10*time.Minute)
		p.mu.Unlock()
		if idle {
			delete(parties.byCode, code)
		}
	}
}

func findParty(w http.ResponseWriter, r *http.Request) *party {
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	parties.Lock()
	p := parties.byCode[code]
	parties.Unlock()
	if p == nil {
		writeError(w, http.StatusNotFound, "That watch party has ended, or the link isn't right.")
		return nil
	}
	return p
}

// member is the caller's place in the party (by the client id it joined
// with), and only for the account that joined with it.
func (p *party) member(r *http.Request, client string) *partyMember {
	u := auth.UserFromContext(r.Context())
	m := p.members[client]
	if m == nil || u == nil || m.User != u.ID {
		return nil
	}
	return m
}

func clock(sec float64) string {
	s := int(sec)
	if s < 0 {
		s = 0
	}
	if s >= 3600 {
		return fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

type partyClientReq struct {
	Client string `json:"client"`
}

// POST /api/party {title..., "pos", "playing", "client"}: a new party for
// what the caller is watching, with the caller as its first member.
func (s *Server) handleStartParty(w http.ResponseWriter, r *http.Request) {
	prof, ok := s.activeProfile(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "Pick a profile first.")
		return
	}
	var req struct {
		partyTitle
		Pos     float64 `json:"pos"`
		Playing bool    `json:"playing"`
		Client  string  `json:"client"`
	}
	if err := decodeJSON(r, &req); err != nil || (req.Kind != "movie" && req.Kind != "tv") || req.TMDBID <= 0 || !validClient(req.Client) {
		writeError(w, http.StatusBadRequest, "That isn't something a party can watch.")
		return
	}
	code, err := newPartyCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Couldn't start a party. Try again.")
		return
	}
	req.Title = truncate(req.Title, 200)
	u := auth.UserFromContext(r.Context())
	p := &party{
		code: code, title: req.partyTitle, playing: req.Playing, pos: math.Max(0, req.Pos), posAt: time.Now(),
		members: map[string]*partyMember{}, subs: map[chan []byte]string{}, active: time.Now(),
	}
	p.members[req.Client] = &partyMember{Client: req.Client, Profile: prof.ID, User: u.ID, Name: prof.Name, Avatar: prof.Avatar, Host: true, seen: time.Now()}
	p.say("event", prof.Name, prof.Avatar, prof.Name+" started the party")
	parties.Lock()
	pruneParties()
	if len(parties.byCode) >= partyMaxParties {
		parties.Unlock()
		writeError(w, http.StatusServiceUnavailable, "There are too many watch parties going right now. Try again later.")
		return
	}
	parties.byCode[code] = p
	parties.Unlock()
	p.mu.Lock()
	snap := p.snapshot()
	p.mu.Unlock()
	writeJSON(w, http.StatusOK, snap)
}

func validClient(c string) bool {
	if len(c) < 8 || len(c) > 64 {
		return false
	}
	for _, r := range c {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
			return false
		}
	}
	return true
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// GET /api/party/{code}: what the party is watching, where it is, and who's
// there (the join page asks this first).
func (s *Server) handleGetParty(w http.ResponseWriter, r *http.Request) {
	p := findParty(w, r)
	if p == nil {
		return
	}
	p.mu.Lock()
	snap := p.snapshot()
	p.mu.Unlock()
	writeJSON(w, http.StatusOK, snap)
}

// POST /api/party/{code}/join {"client"}: joins as the active profile;
// answers the party and its chat so far.
func (s *Server) handleJoinParty(w http.ResponseWriter, r *http.Request) {
	p := findParty(w, r)
	if p == nil {
		return
	}
	prof, ok := s.activeProfile(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "Pick a profile first.")
		return
	}
	var req partyClientReq
	if err := decodeJSON(r, &req); err != nil || !validClient(req.Client) {
		writeError(w, http.StatusBadRequest, "Couldn't read that request.")
		return
	}
	u := auth.UserFromContext(r.Context())
	p.mu.Lock()
	defer p.mu.Unlock()
	if m := p.members[req.Client]; m != nil && m.User != u.ID {
		writeError(w, http.StatusConflict, "Reload the page and try again.")
		return
	}
	if p.members[req.Client] == nil {
		if len(p.members) >= partyMaxMembers {
			writeError(w, http.StatusConflict, "This party is full.")
			return
		}
		p.members[req.Client] = &partyMember{Client: req.Client, Profile: prof.ID, User: u.ID, Name: prof.Name, Avatar: prof.Avatar, seen: time.Now()}
		p.say("event", prof.Name, prof.Avatar, prof.Name+" joined")
		p.send("state", p.snapshot())
	}
	p.members[req.Client].seen = time.Now()
	p.active = time.Now()
	writeJSON(w, http.StatusOK, map[string]any{"party": p.snapshot(), "msgs": p.msgs})
}

// GET /api/party/{code}/events?client=: the party's updates as they
// happen (Server-Sent Events): "state" (where everyone should be, and who's
// there) and "msg" (chat, who did what, reactions).
func (s *Server) handlePartyEvents(w http.ResponseWriter, r *http.Request) {
	p := findParty(w, r)
	if p == nil {
		return
	}
	client := r.URL.Query().Get("client")
	p.mu.Lock()
	m := p.member(r, client)
	if m == nil {
		p.mu.Unlock()
		writeError(w, http.StatusForbidden, "Join the party first.")
		return
	}
	ch := make(chan []byte, 32)
	p.subs[ch] = client
	m.streams++
	m.seen = time.Now()
	p.mu.Unlock()

	fl, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	p.mu.Lock()
	first, _ := json.Marshal(p.snapshot())
	p.mu.Unlock()
	fmt.Fprintf(w, "retry: 2000\nevent: state\ndata: %s\n\n", first)
	if fl != nil {
		fl.Flush()
	}

	beat := time.NewTicker(partyBeat)
	defer beat.Stop()
	defer func() {
		p.mu.Lock()
		delete(p.subs, ch)
		if m := p.members[client]; m != nil {
			m.streams--
			m.seen = time.Now()
		}
		p.mu.Unlock()
		// Gone for good unless it comes back (a reload, a dropped
		// connection) within partyGone.
		time.AfterFunc(partyGone, func() { p.dropIfGone(client) })
	}()
	for {
		select {
		case <-r.Context().Done():
			return
		case frame := <-ch:
			if _, err := w.Write(frame); err != nil {
				return
			}
		case <-beat.C:
			if _, err := w.Write([]byte(": still here\n\n")); err != nil {
				return
			}
		}
		if fl != nil {
			fl.Flush()
		}
	}
}

// dropIfGone removes a member whose player went away and didn't come back.
func (p *party) dropIfGone(client string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	m := p.members[client]
	if m == nil || m.streams > 0 || time.Since(m.seen) < partyGone {
		return
	}
	delete(p.members, client)
	p.say("event", m.Name, m.Avatar, m.Name+" left")
	if m.Host {
		for _, other := range p.members {
			other.Host = true // the party carries on; someone else is shown as its host
			break
		}
	}
	p.send("state", p.snapshot())
}

// POST /api/party/{code}/state {"client","action","pos", title for
// "episode"}: play, pause, seek or a new episode, for everyone.
func (s *Server) handlePartyState(w http.ResponseWriter, r *http.Request) {
	p := findParty(w, r)
	if p == nil {
		return
	}
	var req struct {
		Client  string      `json:"client"`
		Action  string      `json:"action"` // play, pause, seek, episode, sync
		Pos     float64     `json:"pos"`
		Title   *partyTitle `json:"title"`
		Playing *bool       `json:"playing"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that request.")
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	m := p.member(r, req.Client)
	if m == nil {
		writeError(w, http.StatusForbidden, "Join the party first.")
		return
	}
	m.seen = time.Now()
	p.active = time.Now()
	pos := math.Max(0, req.Pos)
	switch req.Action {
	case "play":
		p.playing, p.pos, p.posAt = true, pos, time.Now()
		p.say("event", m.Name, m.Avatar, m.Name+" pressed play")
	case "pause":
		p.playing, p.pos, p.posAt = false, pos, time.Now()
		p.say("event", m.Name, m.Avatar, m.Name+" paused")
	case "seek":
		from := p.now()
		p.pos, p.posAt = pos, time.Now()
		if math.Abs(pos-from) >= 3 {
			p.say("event", m.Name, m.Avatar, fmt.Sprintf("%s jumped to %s", m.Name, clock(pos)))
		}
	case "episode":
		if req.Title == nil || req.Title.Kind != "tv" || req.Title.TMDBID != p.title.TMDBID {
			writeError(w, http.StatusBadRequest, "That isn't an episode of what the party is watching.")
			return
		}
		t := *req.Title
		t.Title = truncate(t.Title, 200)
		p.title = t
		p.playing, p.pos, p.posAt = true, 0, time.Now()
		p.say("event", m.Name, m.Avatar, fmt.Sprintf("%s started S%d:E%d", m.Name, t.Season, t.Episode))
	default:
		writeError(w, http.StatusBadRequest, "That isn't something a party can do.")
		return
	}
	p.by = req.Client
	p.send("state", p.snapshot())
	writeJSON(w, http.StatusOK, p.snapshot())
}

// POST /api/party/{code}/chat {"client","text"}.
func (s *Server) handlePartyChat(w http.ResponseWriter, r *http.Request) {
	p := findParty(w, r)
	if p == nil {
		return
	}
	var req struct {
		Client string `json:"client"`
		Text   string `json:"text"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Couldn't read that request.")
		return
	}
	text := truncate(req.Text, partyChatMax)
	if text == "" {
		writeError(w, http.StatusBadRequest, "Type a message first.")
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	m := p.member(r, req.Client)
	if m == nil {
		writeError(w, http.StatusForbidden, "Join the party first.")
		return
	}
	m.seen = time.Now()
	p.active = time.Now()
	p.say("chat", m.Name, m.Avatar, text)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// POST /api/party/{code}/react {"client","emoji"}: a reaction that floats
// up everyone's screen (not kept in the chat).
func (s *Server) handlePartyReact(w http.ResponseWriter, r *http.Request) {
	p := findParty(w, r)
	if p == nil {
		return
	}
	var req struct {
		Client string `json:"client"`
		Emoji  string `json:"emoji"`
	}
	if err := decodeJSON(r, &req); err != nil || !partyEmoji[req.Emoji] {
		writeError(w, http.StatusBadRequest, "That isn't a reaction Cue knows.")
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	m := p.member(r, req.Client)
	if m == nil {
		writeError(w, http.StatusForbidden, "Join the party first.")
		return
	}
	m.seen = time.Now()
	p.say("reaction", m.Name, m.Avatar, req.Emoji)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// POST /api/party/{code}/leave {"client"}: leaving straight away (the
// player closed), rather than after a while.
func (s *Server) handleLeaveParty(w http.ResponseWriter, r *http.Request) {
	p := findParty(w, r)
	if p == nil {
		return
	}
	var req partyClientReq
	_ = decodeJSON(r, &req)
	p.mu.Lock()
	m := p.member(r, req.Client)
	if m != nil {
		m.seen = time.Time{}
		m.streams = 0
	}
	p.mu.Unlock()
	if m != nil {
		p.dropIfGone(req.Client)
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// GET /api/parties: the watch parties going on now, for Home's "Join"
// (anyone signed in to this Cue can see and join them: on a TV, with the
// remote, there's no link to open).
func (s *Server) handleListParties(w http.ResponseWriter, r *http.Request) {
	type row struct {
		Code    string        `json:"code"`
		Title   partyTitle    `json:"title"`
		Host    *partyMember  `json:"host,omitempty"`
		Members int           `json:"members"`
		Faces   []partyMember `json:"faces"`
		Playing bool          `json:"playing"`
	}
	out := []row{}
	parties.Lock()
	pruneParties()
	list := make([]*party, 0, len(parties.byCode))
	for _, p := range parties.byCode {
		list = append(list, p)
	}
	parties.Unlock()
	for _, p := range list {
		p.mu.Lock()
		if len(p.members) > 0 {
			rw := row{Code: p.code, Title: p.title, Members: len(p.members), Playing: p.playing, Faces: []partyMember{}}
			for _, m := range p.members {
				if m.Host {
					h := *m
					rw.Host = &h
				}
				if len(rw.Faces) < 5 {
					rw.Faces = append(rw.Faces, *m)
				}
			}
			out = append(out, rw)
		}
		p.mu.Unlock()
	}
	writeJSON(w, http.StatusOK, out)
}
