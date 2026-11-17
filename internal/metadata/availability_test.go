package metadata

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAvailability(t *testing.T) {
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	film := func(release, dates string) *MovieDetail {
		var d MovieDetail
		if err := json.Unmarshal([]byte(`{"release_date":"`+release+`","release_dates":{"results":[`+dates+`]}}`), &d); err != nil {
			t.Fatal(err)
		}
		return &d
	}
	for _, c := range []struct {
		name          string
		d             *MovieDetail
		out           bool
		home, cinemas string
	}{
		{"in cinemas, digital date ahead", film("2026-11-07", `{"iso_3166_1":"US","release_dates":[{"type":3,"release_date":"2026-11-07T00:00:00.000Z"},{"type":4,"release_date":"2027-01-20T00:00:00.000Z"}]}`), false, "2027-01-20", "2026-11-07"},
		{"digital out", film("2026-08-01", `{"iso_3166_1":"US","release_dates":[{"type":3,"release_date":"2026-08-01T00:00:00.000Z"},{"type":4,"release_date":"2026-10-01T00:00:00.000Z"}]}`), true, "2026-10-01", "2026-08-01"},
		{"earliest country counts", film("2026-09-01", `{"iso_3166_1":"GB","release_dates":[{"type":4,"release_date":"2026-11-10T00:00:00.000Z"}]},{"iso_3166_1":"US","release_dates":[{"type":4,"release_date":"2026-12-01T00:00:00.000Z"}]}`), true, "2026-11-10", "2026-09-01"},
		{"announced only", film("2027-05-01", `{"iso_3166_1":"US","release_dates":[{"type":3,"release_date":"2027-05-01T00:00:00.000Z"}]}`), false, "", "2027-05-01"},
		{"old film with no home date", film("1999-03-31", `{"iso_3166_1":"US","release_dates":[{"type":3,"release_date":"1999-03-31T00:00:00.000Z"}]}`), true, "", "1999-03-31"},
		{"in cinemas a month, no home date", film("2026-10-15", ``), false, "", "2026-10-15"},
		{"nothing known", film("", ``), false, "", ""},
	} {
		out, home, cinemas := c.d.Availability(now)
		if out != c.out || home != c.home || cinemas != c.cinemas {
			t.Errorf("%s: got %v %q %q, want %v %q %q", c.name, out, home, cinemas, c.out, c.home, c.cinemas)
		}
	}
}
