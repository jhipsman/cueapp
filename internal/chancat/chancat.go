// Package chancat sorts a provider's live channel list into a catalog that
// reads like a TV's: each channel's plain name ("US ★ ESPN FHD ◉" is
// "ESPN"), its country, what it's about (News, Sports, Kids…), how good
// its picture is, and which channels are the same channel twice (HD and
// FHD, a backup). Divider entries some lists carry ("##### UK BACKUP
// #####") are marked so they can be left out.
//
// It sorts by the channel itself, not by the provider's groups, which
// differ from one provider to the next.
package chancat

import (
	"sort"
	"strings"
	"unicode"
)

// Info is what a channel's name and group say about it.
type Info struct {
	Clean   string // the channel's plain name
	Country string // "US", "UK", "CA"…; "" when the list doesn't say
	Topic   string // one of Topics' keys
	Quality int    // higher is the better pick of two copies of a channel
	Divider bool   // not a channel: a heading the provider put in the list
}

// Topics are what a channel can be about, in the order a picker lists them.
var Topics = []struct{ Key, Name string }{
	{"general", "General"},
	{"news", "News"},
	{"sports", "Sports"},
	{"kids", "Kids"},
	{"movies", "Movies"},
	{"docs", "Documentary"},
	{"music", "Music"},
	{"local", "Local"},
	{"247", "24/7"},
	{"faith", "Faith"},
	{"events", "Events"},
	{"adult", "Adult"},
}

// countries maps the codes and names lists use to one code, and the code
// to the country's name.
var countryCodes = map[string]string{
	"US": "US", "USA": "US", "UNITED STATES": "US", "AMERICA": "US",
	"UK": "UK", "GB": "UK", "UNITED KINGDOM": "UK", "ENGLAND": "UK", "BRITAIN": "UK",
	"CA": "CA", "CAN": "CA", "CANADA": "CA",
	"AU": "AU", "AUS": "AU", "AUSTRALIA": "AU",
	"NZ": "NZ", "NEW ZEALAND": "NZ",
	"IE": "IE", "IRL": "IE", "IRELAND": "IE",
	"DE": "DE", "GER": "DE", "GERMANY": "DE", "DEUTSCHLAND": "DE",
	"FR": "FR", "FRA": "FR", "FRANCE": "FR",
	"ES": "ES", "SPA": "ES", "SPAIN": "ES", "ESPAÑA": "ES",
	"IT": "IT", "ITA": "IT", "ITALY": "IT", "ITALIA": "IT",
	"PT": "PT", "POR": "PT", "PORTUGAL": "PT",
	"BR": "BR", "BRA": "BR", "BRAZIL": "BR", "BRASIL": "BR",
	"MX": "MX", "MEX": "MX", "MEXICO": "MX",
	"LAT": "LAT", "LATINO": "LAT", "LATAM": "LAT", "LATIN": "LAT",
	"NL": "NL", "NED": "NL", "NETHERLANDS": "NL", "HOLLAND": "NL",
	"BE": "BE", "BEL": "BE", "BELGIUM": "BE",
	"PL": "PL", "POL": "PL", "POLAND": "PL", "POLSKA": "PL",
	"TR": "TR", "TUR": "TR", "TURKEY": "TR", "TÜRKIYE": "TR",
	"AR": "AR", "ARABIC": "AR", "ARAB": "AR",
	"IN": "IN", "IND": "IN", "INDIA": "IN", "HINDI": "IN",
	"PK": "PK", "PAKISTAN": "PK",
	"EXYU": "EXYU", "EX-YU": "EXYU", "YU": "EXYU",
	"GR": "GR", "GREECE": "GR",
	"SE": "SE", "SWE": "SE", "SWEDEN": "SE",
	"NO": "NO", "NOR": "NO", "NORWAY": "NO",
	"DK": "DK", "DEN": "DK", "DENMARK": "DK",
	"FI": "FI", "FIN": "FI", "FINLAND": "FI",
	"RU": "RU", "RUS": "RU", "RUSSIA": "RU",
	"RO": "RO", "ROMANIA": "RO",
	"AL": "AL", "ALBANIA": "AL",
	"PH": "PH", "PHILIPPINES": "PH",
	"ZA": "ZA", "SOUTH AFRICA": "ZA",
	"AF": "AFR", "AFR": "AFR", "AFRICA": "AFR",
	"CARIB": "CARIB", "CARIBBEAN": "CARIB",
}

// CountryNames are the countries' names for a picker.
var CountryNames = map[string]string{
	"US": "United States", "UK": "United Kingdom", "CA": "Canada", "AU": "Australia", "NZ": "New Zealand",
	"IE": "Ireland", "DE": "Germany", "FR": "France", "ES": "Spain", "IT": "Italy", "PT": "Portugal",
	"BR": "Brazil", "MX": "Mexico", "LAT": "Latin America", "NL": "Netherlands", "BE": "Belgium",
	"PL": "Poland", "TR": "Turkey", "AR": "Arabic", "IN": "India", "PK": "Pakistan", "EXYU": "Ex-Yugoslavia",
	"GR": "Greece", "SE": "Sweden", "NO": "Norway", "DK": "Denmark", "FI": "Finland", "RU": "Russia",
	"RO": "Romania", "AL": "Albania", "PH": "Philippines", "ZA": "South Africa", "AFR": "Africa", "CARIB": "Caribbean",
}

// country is the code a prefix stands for ("" when it isn't one).
func country(s string) string {
	s = strings.ToUpper(strings.Trim(strings.TrimSpace(s), "[]()|:★-#*• "))
	return countryCodes[s]
}

// prefix splits a leading country off a name or group: "US ★ ESPN",
// "US: ESPN", "|US| ESPN", "[US] ESPN", "US - ESPN", "USA ESPN" (a group's
// "UK | NEWS", "USA SPORTS", "UNITED KINGDOM - NEWS").
//
// A space alone is only taken as the end of a country in a group's name: a
// channel's ("USA Network", "UK Gold") is the channel's own.
func prefix(s string, group bool) (code, rest string) {
	t := strings.TrimSpace(s)
	t = strings.TrimLeft(t, "★•*#|[( ")
	// Up to a separator.
	if i := strings.IndexAny(t, "|:])★•-"); i > 0 && i <= 16 {
		if c := country(t[:i]); c != "" {
			return c, strings.TrimLeft(t[i:], "|:])★•- ")
		}
	}
	// "USA SPORTS", "UK NEWS": a code then a word.
	if f := strings.Fields(t); group && len(f) > 1 {
		if c := country(f[0]); c != "" && len(f[0]) <= 4 && strings.ToUpper(f[0]) == f[0] {
			return c, strings.Join(f[1:], " ")
		}
		if len(f) > 2 {
			if c := country(f[0] + " " + f[1]); c != "" {
				return c, strings.Join(f[2:], " ")
			}
		}
	}
	return "", s
}

var qualityWords = map[string]int{
	"FHD": 4, "1080": 4, "1080P": 4, "HD": 3, "720": 3, "720P": 3, "HQ": 3,
	"4K": 2, "UHD": 2, "2160P": 2, "HEVC": 2, "H265": 2, "H.265": 2,
	"SD": 1, "LQ": 1, "LOW": 1, "RAW": 2, "50FPS": 3, "60FPS": 3,
}

// junkWords are decorations that say nothing about the channel.
var junkWords = map[string]bool{"VIP": true, "PREMIUM": true, "ᴴᴰ": true, "ᴿᴬᵂ": true, "ⁱᴾ": true, "4KHDR": true}

var backupWords = []string{"backup", "back up", "alt", "(b)", "[b]", "multi"}

// IsDivider is whether a list entry is a heading rather than a channel:
// "##### UK BACKUP #####", "----- SPORTS -----", "=== NEWS ===", "•••".
func IsDivider(name string) bool {
	t := strings.TrimSpace(name)
	if t == "" {
		return true
	}
	letters := 0
	for _, r := range t {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			letters++
		}
	}
	if letters == 0 {
		return true
	}
	for _, mark := range []string{"###", "***", "===", "---", "▬▬", "•••", "~~~", "___", "+++", "▀▀", "▄▄", "░░", "◆◆", "◇◇", "»»", "««"} {
		if strings.HasPrefix(t, mark) || strings.HasSuffix(t, mark) {
			return true
		}
	}
	return false
}

// Read is what a channel's name and group say about it.
func Read(name, group string) Info {
	in := Info{Divider: IsDivider(name), Quality: 2}
	gc, groupRest := prefix(group, true)
	nc, rest := prefix(name, false)
	in.Country = nc
	if in.Country == "" {
		in.Country = gc
	}
	in.Clean, in.Quality = clean(rest)
	if in.Clean == "" {
		in.Clean = strings.TrimSpace(name)
	}
	low := strings.ToLower(name)
	for _, b := range backupWords {
		if strings.Contains(low, b) {
			in.Quality--
			break
		}
	}
	in.Topic = topic(strings.ToLower(in.Clean), strings.ToLower(groupRest))
	return in
}

// clean drops symbols, brackets of qualifiers and quality words: "ESPN
// FHD ◉" is "ESPN", "BBC One (HD) [UK]" is "BBC One"; and says how good the
// picture is from the words it dropped.
func clean(s string) (string, int) {
	s = strings.Map(func(r rune) rune {
		switch {
		case r == '◉' || r == '●' || r == '★' || r == '☆' || r == '•' || r == '|' || r == '#' || r == '✪' || r == '♦' || r == '◆':
			return ' '
		case r >= 0x1D2C && r <= 0x1DBF: // superscript letters ("ᴴᴰ")
			return -1
		case r >= 0x2070 && r <= 0x209F: // superscript digits
			return -1
		}
		return r
	}, s)
	quality := 0
	var out []string
	for _, f := range strings.Fields(s) {
		bare := strings.ToUpper(strings.Trim(f, "()[]{}:-"))
		if q, ok := qualityWords[bare]; ok {
			quality = max(quality, q)
			continue
		}
		if junkWords[bare] || bare == "" {
			continue
		}
		// "[UK]" or "(US)" at the end: the country again.
		if (strings.HasPrefix(f, "[") || strings.HasPrefix(f, "(")) && country(bare) != "" {
			continue
		}
		out = append(out, strings.Trim(f, "-:"))
	}
	if quality == 0 {
		quality = 2
	}
	// Words left empty by trimming ("-" alone).
	var keep []string
	for _, w := range out {
		if w != "" {
			keep = append(keep, w)
		}
	}
	return strings.Join(keep, " "), quality
}

type rule struct {
	topic string
	words []string
}

// Checked in order: the first that fits wins ("ESPN News" is sports, as a
// viewer thinks of it; "Disney Junior" kids before "Disney" anything else).
var rules = []rule{
	{"adult", []string{"adult", "xxx", "18+", "playboy", "hustler", "brazzers", "for adults"}},
	{"events", []string{"ppv", "pay per view", "event", "live event"}},
	{"kids", []string{"kids", "kid's", "cartoon", "nick jr", "nickelodeon", "nicktoons", " nick ", "disney junior", "disney jr", "disney channel", "disney xd", "boomerang", "baby", "junior", "cbeebies", "cbbc", "citv", "pbs kids", "teen", "toon", "jim jam", "universal kids", "discovery family", "duck tv", "babytv", "tiny pop", "pop max"}},
	{"sports", []string{"sport", "espn", "fs1", "fs2", "nfl", "nba", "nhl", "mlb", "wnba", "golf", "tennis", "f1 ", "formula 1", "racing", "bein", "dazn", "tsn", "acc network", "sec network", "big ten", " btn ", "redzone", "red zone", "league pass", "ufc", "wwe", "boxing", "cricket", "rugby", "football", "soccer", "premier league", "laliga", "la liga", "bundesliga", "serie a", "ligue 1", "motogp", "nascar", "fight", "olympic", "stadium", "willow", "eurosport", "supersport", "arena sport", "polsat sport", "fox deportes", "tudn", "marquee", " msg ", "yes network", "bally", "nesn", "sny", "altitude", "root sports", "spectrum sportsnet", "center ice", "extra innings", "fanduel", "tennis channel", "outdoor channel", "sportsman"}},
	{"news", []string{"news", "cnn", "msnbc", "cnbc", "bloomberg", "al jazeera", "aljazeera", "euronews", "france 24", " dw ", "newsmax", "newsnation", "weather", "c-span", "cspan", "bbc world", "sky news", "gb news", "fox business", "abc news", "cbs news", "nbc news", "cheddar", "i24", " rt ", "trt world", "cgtn", "nhk world", "headline"}},
	{"movies", []string{"movie", "cinema", "film", "hbo", "showtime", "starz", "cinemax", " tmc", "epix", "mgm+", "mgm", "tcm", "sky cinema", "hallmark movies", "lifetime movie", "flix", "sony max", " action", "thriller", "comedy central"}},
	{"docs", []string{"discovery", "history", "nat geo", "national geographic", "natgeo", "documentar", "docu", "animal planet", "science", "smithsonian", "investigation", " id ", "crime", "travel", "planet", "nature", "military", "curiosity", "magnolia", "hgtv", "food network", "cooking"}},
	{"music", []string{"music", "mtv", "vh1", "cmt", "bet jams", "bet soul", "radio", "hits", "jukebox", "stingray", "concert", "kerrang", "clubland", "4music", "trace"}},
	{"faith", []string{"religio", "church", "god tv", "ewtn", "tbn", "daystar", "quran", "islam", "christian", "gospel", "catholic", "worship", "bible", "peace tv", "sky angel", "hope channel"}},
	{"247", []string{"24/7", "24-7", "24x7", "247 "}},
	{"local", []string{"local", "regional", "region", "affiliate", "metro", " pbs", "pbs ", "wcbs", "wnbc", "wabc", "kcbs", "knbc", "kabc"}},
}

// topic is what a channel is about, from its group first (the provider's
// "UK | NEWS") then its name.
func topic(name, group string) string {
	for _, src := range []string{" " + group + " ", " " + name + " "} {
		if strings.TrimSpace(src) == "" {
			continue
		}
		for _, r := range rules {
			for _, w := range r.words {
				if strings.Contains(src, w) {
					return r.topic
				}
			}
		}
	}
	return "general"
}

// Key is what two copies of a channel share: the country and the plain
// name, without case or spacing ("US|espn").
func Key(in Info) string {
	f := strings.Fields(strings.ToLower(in.Clean))
	return in.Country + "|" + strings.Join(f, " ")
}

// Copy is one channel of the list, as Fold needs it.
type Copy struct {
	ID      string
	Num     int
	Info    Info
	HasEPG  bool
	Picture bool // has a logo
}

// Fold finds channels that are the same channel twice: for each channel
// that's a copy, the one to show instead (its "main"). The main is the
// best picture, then one with a guide, then a logo, then the lowest number.
// Dividers are left out.
func Fold(list []Copy) (mainOf map[string]string, copies map[string][]string) {
	groups := map[string][]int{}
	for i, c := range list {
		if c.Info.Divider {
			continue
		}
		k := Key(c.Info)
		groups[k] = append(groups[k], i)
	}
	mainOf = map[string]string{}
	copies = map[string][]string{}
	for _, idx := range groups {
		if len(idx) < 2 {
			continue
		}
		sort.SliceStable(idx, func(a, b int) bool {
			x, y := list[idx[a]], list[idx[b]]
			if x.Info.Quality != y.Info.Quality {
				return x.Info.Quality > y.Info.Quality
			}
			if x.HasEPG != y.HasEPG {
				return x.HasEPG
			}
			if x.Picture != y.Picture {
				return x.Picture
			}
			if (x.Num > 0) != (y.Num > 0) {
				return x.Num > 0
			}
			return x.Num < y.Num
		})
		main := list[idx[0]].ID
		for _, i := range idx[1:] {
			mainOf[list[i].ID] = main
			copies[main] = append(copies[main], list[i].ID)
		}
	}
	return mainOf, copies
}
