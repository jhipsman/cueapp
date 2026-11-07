package watch

import (
	"sort"
	"time"
)

// YearShow is one title in a profile's year: how long was spent on it.
type YearShow struct {
	Kind         string  `json:"kind"`
	TMDBID       int     `json:"tmdbId"`
	Title        string  `json:"title"`
	BackdropPath string  `json:"-"`
	Hours        float64 `json:"hours"`
	Episodes     int     `json:"episodes,omitempty"`
}

// YearStats is a profile's year in Cue, from where it got to in each movie
// and episode (Cue keeps one place per episode, so a rewatch counts once).
type YearStats struct {
	Year            int        `json:"year"`
	Hours           float64    `json:"hours"`
	Titles          int        `json:"titles"`
	Shows           int        `json:"shows"`
	Movies          int        `json:"movies"`
	Episodes        int        `json:"episodes"`
	Days            int        `json:"days"` // days something was watched
	Top             []YearShow `json:"top"`  // most watched shows, at most 5
	TopMovie        *YearShow  `json:"topMovie,omitempty"`
	BusiestDay      string     `json:"busiestDay,omitempty"` // 2006-01-02
	BusiestDayHours float64    `json:"busiestDayHours,omitempty"`
	BusiestMonth    string     `json:"busiestMonth,omitempty"` // "March"
	First           string     `json:"first,omitempty"`        // the first title of the year
}

// Year sums up what profileID watched in year.
func (r *Repo) Year(profileID int64, year int) (YearStats, error) {
	from := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	to := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
	ps, err := r.query(`SELECT `+progressCols+` FROM watch_progress WHERE profile_id = ? AND updated_at >= ? AND updated_at < ?
		ORDER BY updated_at`, profileID, from, to)
	if err != nil {
		return YearStats{Year: year}, err
	}
	return summariseYear(year, ps), nil
}

// watched is how long was spent on one movie or episode: where it got to,
// or all of it once finished.
func watched(p Progress) float64 {
	if p.Finished() {
		return p.Duration
	}
	return max(p.Position, 0)
}

func summariseYear(year int, ps []Progress) YearStats {
	st := YearStats{Year: year}
	type key struct {
		kind string
		id   int
	}
	titles := map[key]*YearShow{}
	days := map[string]float64{}
	months := map[time.Month]float64{}
	var order []key
	for _, p := range ps {
		sec := watched(p)
		if sec < 60 {
			continue // opened and closed again
		}
		k := key{p.Kind, p.TMDBID}
		t, ok := titles[k]
		if !ok {
			t = &YearShow{Kind: p.Kind, TMDBID: p.TMDBID, Title: p.Title, BackdropPath: p.BackdropPath}
			titles[k] = t
			order = append(order, k)
		}
		t.Hours += sec / 3600
		if p.Kind == KindTV {
			t.Episodes++
			st.Episodes++
		}
		if t.BackdropPath == "" {
			t.BackdropPath = p.BackdropPath
		}
		day := p.UpdatedAt.UTC().Format("2006-01-02")
		days[day] += sec / 3600
		months[p.UpdatedAt.UTC().Month()] += sec / 3600
		st.Hours += sec / 3600
	}
	if len(order) > 0 {
		st.First = titles[order[0]].Title
	}
	var shows []YearShow
	for _, k := range order {
		t := titles[k]
		st.Titles++
		if t.Kind == KindTV {
			st.Shows++
			shows = append(shows, *t)
		} else {
			st.Movies++
			if st.TopMovie == nil || t.Hours > st.TopMovie.Hours {
				m := *t
				st.TopMovie = &m
			}
		}
	}
	sort.SliceStable(shows, func(i, j int) bool { return shows[i].Hours > shows[j].Hours })
	st.Top = append([]YearShow{}, shows[:min(len(shows), 5)]...)
	st.Days = len(days)
	for d, h := range days {
		if h > st.BusiestDayHours || (h == st.BusiestDayHours && d < st.BusiestDay) {
			st.BusiestDay, st.BusiestDayHours = d, h
		}
	}
	var best float64
	for m, h := range months {
		if h > best {
			best, st.BusiestMonth = h, m.String()
		}
	}
	st.Hours = round1(st.Hours)
	st.BusiestDayHours = round1(st.BusiestDayHours)
	for i := range st.Top {
		st.Top[i].Hours = round1(st.Top[i].Hours)
	}
	if st.TopMovie != nil {
		st.TopMovie.Hours = round1(st.TopMovie.Hours)
	}
	return st
}

func round1(h float64) float64 { return float64(int(h*10+0.5)) / 10 }
