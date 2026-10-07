# Watch

Watch is the streaming side of Cue. Open it with **Watch** at the top of any page, or go to `/watch`. It looks and works like a streaming service: a banner, rows of artwork to scroll through, a search box, and a **Play** button on everything.

It needs [Premiumize](downloads.md#premiumize) (the player streams straight from Premiumize; nothing is downloaded to your server) and, for IMDb ratings, an optional OMDb key.

## What you can do

- **Browse.** Home has Continue Watching, My List, then trending, popular and genre rows of movies and shows. **Shows** and **Movies** at the top narrow it to one kind.
- **Search anything.** Type in the search box: every movie and show TMDB knows comes up, not only what's in your library.
- **Open a title.** Its page has the backdrop, description, IMDb and Rotten Tomatoes ratings (or TMDB's score), age rating, length, cast and genres, and **More like this**. A show has a season picker and its episodes, each with a thumbnail, description and length.
- **Press Play.** Cue picks the best version Premiumize can stream at once ([how it picks](downloads.md#play-streaming-from-premiumize)) and it starts. If one won't play, the next one is tried by itself.
- **Pick up where you stopped.** Where you are is saved every 15 seconds, per account. Play on a title you started says **Resume**; on a show it goes to the episode you're on, or the next one.
- **Next episode.** At the end of an episode a card offers the next one and plays it after 10 seconds.
- **My List.** The **+** on a title's page saves it to My List, its own row on Home and its own page. This is separate from your library: adding to My List downloads nothing.

The library manager (downloads, quality, settings) stays one click away: **Manage** in the top bar.

## Stream add-ons (Comet, Torrentio, MediaFusion)

Play can ask Stremio add-ons for streams before it searches your own indexers. An add-on like **Comet** (hosted for you on ElfHosted) has its own scrapers and, set up with your Premiumize key, answers with links that play straight away, sorted the way you chose on its page.

1. Open the add-on's configure page, for Comet [comet.elfhosted.com/configure](https://comet.elfhosted.com/configure).
2. Choose **Premiumize** as the debrid service and paste your Premiumize key. Pick your scrapers, resolutions, languages and sorting there.
3. Press its **Copy link** button: the link ends in `/manifest.json`.
4. In Cue, open **Settings > Downloading > Usenet and torrents**, and under **Cloud downloader** paste it into **Stream add-ons**, then press **Add**. Cue checks the add-on answers.

When you press Play:

- Every add-on is asked at once. Links come in the order of the list, then each add-on's own order, so the first add-on's best stream plays first. **Try another version** moves down the list.
- An add-on without a debrid service answers with torrents instead of links. Those are checked with Premiumize like Cue's own search results.
- If no add-on has a link, Cue searches your indexers as before. With add-ons, you don't even need indexers or a Premiumize key in Cue; the add-on uses its own.
- The player shows where a stream came from ("via Comet").

The add-on link holds your Premiumize key, so Cue stores it encrypted and only ever shows its address (like comet.elfhosted.com).

## IMDb ratings

TMDB doesn't have IMDb ratings, so Watch reads them from [OMDb](https://www.omdbapi.com). Get a free key at omdbapi.com/apikey.aspx, click the activation link in OMDb's email, and paste the key into **Settings > Info, lists and subtitles > IMDb ratings (OMDb)**. A free key allows 1,000 lookups a day; each title is looked up at most once a day.

Without a key, Watch shows TMDB's own score instead.

## For apps

Everything Watch shows comes from the API, so a TV app can show the same things. All need a signed-in account (an API key from Settings > Profile, sent as `X-API-Key`) with permission to play.

| | |
|---|---|
| `GET /api/watch/home` | the banner and the rows |
| `GET /api/watch/search?q=` | movies and shows matching the words |
| `GET /api/watch/movie/{tmdbId}` | a movie's page |
| `GET /api/watch/tv/{tmdbId}` | a show's page, with its seasons and where to resume |
| `GET /api/watch/tv/{tmdbId}/season/{n}` | a season's episodes |
| `GET /api/watch/tv/{tmdbId}/next?season=&episode=` | the episode after that one |
| `GET /api/play/tmdb/movie/{tmdbId}` | what to play for a movie |
| `GET /api/play/tmdb/tv/{tmdbId}/{season}/{episode}` | what to play for an episode |
| `GET /api/watch/progress/{kind}/{tmdbId}?season=&episode=` | where to resume |
| `PUT /api/watch/progress` | save where you are: `{"kind","tmdbId","season","episode","position","duration"}` in seconds |
| `DELETE /api/watch/progress/{kind}/{tmdbId}` | take a title off Continue Watching |
| `PUT` / `DELETE /api/watch/list/{kind}/{tmdbId}` | add to or remove from My List |

`kind` is `movie` or `tv`.
