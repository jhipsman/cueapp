package api

import (
	"context"
	"errors"
	"time"

	"github.com/rdborg/mediarium/internal/indexers"
)

// How long Play waits for the indexers. Sites behind Cloudflare (through
// FlareSolverr) can take most of a minute the first time; Play doesn't wait
// for them. It goes ahead with what has come back after playSearchWait, or,
// when nothing has, waits up to playSearchMax for the first answers. Every
// search still runs to its end in the background, so FlareSolverr's pass is
// kept and the next search is quick.
var (
	playSearchWait = 10 * time.Second
	playSearchMax  = 30 * time.Second
)

const playSearchBackground = 90 * time.Second

var errSearchSlow = errors.New("it was still searching when Play went ahead (it keeps going in the background, so it's quicker next time)")

// playSearch asks every enabled indexer for t, each on its own, and returns
// what has come back by the deadline (see playSearchWait).
func (s *Server) playSearch(ctx context.Context, t playTarget) ([]indexers.Outcome, error) {
	instances, err := s.IndexerRepo.List()
	if err != nil {
		return nil, err
	}
	type query struct {
		q    string
		cats []int
	}
	var queries []query
	if t.movie != nil {
		queries = []query{{t.movie.Title, movieCategory}}
	} else {
		queries = []query{{tvSearchQuery(t.series.Title, t.season, t.episode), tvCategory}}
		for _, q := range s.extraTVQueries(*t.series, t.season, t.episode) {
			queries = append(queries, query{q, tvCategory})
		}
	}
	var enabled []indexers.Instance
	for _, inst := range instances {
		if inst.Enabled {
			enabled = append(enabled, inst)
		}
	}
	total := len(enabled) * len(queries)
	if total == 0 {
		return nil, nil
	}
	ch := make(chan indexers.Outcome, total)
	bg := context.WithoutCancel(ctx) // a slow site finishes even after Play went ahead
	for _, inst := range enabled {
		for _, q := range queries {
			go func(inst indexers.Instance, q query) {
				sctx, cancel := context.WithTimeout(bg, playSearchBackground)
				defer cancel()
				out := indexers.SearchAll(sctx, []indexers.Instance{inst}, q.q, q.cats)
				if len(out) == 0 {
					out = []indexers.Outcome{{IndexerName: inst.Name}}
				}
				ch <- out[0]
			}(inst, q)
		}
	}

	var got []indexers.Outcome
	answered := map[string]bool{}
	results := 0
	wait := time.NewTimer(playSearchWait)
	defer wait.Stop()
	limit := time.NewTimer(playSearchMax)
	defer limit.Stop()
	waitDone := false
collect:
	for len(got) < total {
		select {
		case o := <-ch:
			got = append(got, o)
			answered[o.IndexerName] = true
			results += len(o.Results)
			if waitDone && results > 0 {
				break collect
			}
		case <-wait.C:
			waitDone = true
			if results > 0 {
				break collect
			}
		case <-limit.C:
			break collect
		case <-ctx.Done():
			break collect
		}
	}
	for _, inst := range enabled {
		if !answered[inst.Name] {
			got = append(got, indexers.Outcome{IndexerName: inst.Name, Err: errSearchSlow})
		}
	}
	return got, nil
}
