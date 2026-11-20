package app.cue.tv

import okhttp3.mockwebserver.Dispatcher
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.RecordedRequest
import org.json.JSONArray
import org.json.JSONObject
import java.time.Instant
import java.time.temporal.ChronoUnit

// A stand-in Cue server for the walk-through: answers what each screen asks
// for with believable data, shaped like the real server's. Anything it
// doesn't know gets a 404, as a server that's out of date would, so the
// screens' handling of a failed answer is tested too.
class FakeCue : Dispatcher() {
    val unknown = mutableListOf<String>()
    val asked: MutableList<String> = java.util.Collections.synchronizedList(mutableListOf())

    private val names = listOf("Night Harbor", "Paper Moons", "Signal Fire", "Glass Rivers", "Northbound", "Copper Sky",
        "Quiet Hours", "Last Light", "Marigold", "Orbit", "Dispatch", "The Valley")

    private fun card(i: Int, kind: String = if (i % 3 == 0) "tv" else "movie") = JSONObject()
        .put("kind", kind).put("tmdbId", 100 + i).put("title", names[i % names.size]).put("year", 2014 + i % 10)
        .put("overview", "A story told over one long summer in a small coastal town.")
        .put("rating", 7.4)

    private fun cards(n: Int, from: Int = 0, kind: String? = null) =
        JSONArray().apply { repeat(n) { put(if (kind == null) card(from + it) else card(from + it, kind)) } }

    private fun iso(hours: Double) = Instant.now().plus((hours * 3600).toLong(), ChronoUnit.SECONDS).toString()

    private fun row(key: String, title: String, items: JSONArray) = JSONObject().put("key", key).put("title", title).put("items", items)

    private fun home() = JSONObject()
        .put("hero", card(0, "movie").put("logoUrl", ""))
        .put("rows", JSONArray()
            .put(row("continue", "Continue watching", JSONArray().put(card(1, "tv").put("season", 2).put("episode", 4).put("progress", 0.4))))
            .put(row("trending-movies", "Trending movies", cards(10, 2, "movie")))
            .put(row("trending-tv", "Trending shows", cards(10, 4, "tv"))))
        .put("top10", JSONArray().put(JSONObject().put("key", "netflix").put("name", "Netflix")
            .put("items", cards(10, 1)).put("movies", cards(10, 1, "movie")).put("shows", cards(10, 1, "tv"))))

    private fun title(kind: String, id: Int) = card(id - 100, kind)
        .put("logoUrl", "").put("genres", JSONArray().put("Drama")).put("runtime", 112).put("tagline", "Some tides bring things back.")
        .put("seasons", if (kind == "tv") JSONArray().put(JSONObject().put("number", 1).put("name", "Season 1")) else JSONArray())
        .put("more", cards(8, 3))
        .put("people", JSONArray().put(JSONObject().put("id", 500).put("name", "Ana Reyes").put("character", "Captain Rhee")))

    private fun episodes() = JSONArray().apply {
        repeat(6) { put(JSONObject().put("season", 1).put("episode", it + 1).put("title", "Episode ${it + 1}").put("runtime", 48)) }
    }

    private fun channels() = JSONObject().put("configured", true)
        .put("categories", JSONArray().put(JSONObject().put("id", "news").put("name", "News")).put(JSONObject().put("id", "sports").put("name", "Sports")))
        .put("channels", JSONArray().apply {
            listOf("News 24", "World News", "Sports One", "Sports Two").forEachIndexed { i, n ->
                put(JSONObject().put("id", "c$i").put("num", i + 1).put("name", n).put("category", if (i < 2) "news" else "sports")
                    .put("now", JSONObject().put("title", "Live coverage").put("start", iso(-0.5)).put("stop", iso(0.5))))
            }
        })

    private fun team(name: String, short: String, abbr: String, score: String) =
        JSONObject().put("id", abbr).put("name", name).put("short", short).put("abbr", abbr).put("score", score).put("record", "9-2")

    private fun sports() = JSONObject().put("source", "espn")
        .put("big", JSONObject().put("id", "1").put("league", "nfl").put("leagueName", "NFL").put("state", "in").put("detail", "Q3 4:12")
            .put("start", iso(-1.5)).put("title", "Bills at Chiefs").put("onNow", true)
            .put("home", team("Kansas City Chiefs", "Chiefs", "KC", "17")).put("away", team("Buffalo Bills", "Bills", "BUF", "21"))
            .put("networks", JSONArray().put("CBS")))
        .put("now", JSONArray())
        .put("soon", JSONArray().put(JSONObject().put("id", "2").put("league", "nba").put("leagueName", "NBA").put("state", "pre")
            .put("start", iso(2.0)).put("title", "Lakers at Celtics")
            .put("home", team("Boston Celtics", "Celtics", "BOS", "")).put("away", team("Los Angeles Lakers", "Lakers", "LAL", ""))
            .put("networks", JSONArray().put("ESPN"))))
        .put("teams", JSONArray())

    private val services = listOf("netflix" to "Netflix", "prime" to "Prime Video", "max" to "Max")

    private fun browse() = JSONObject()
        .put("services", JSONArray().apply { services.forEach { (k, n) -> put(JSONObject().put("key", k).put("name", n)) } })
        .put("collections", JSONArray()
            .put(JSONObject().put("key", "acclaimed").put("title", "Critically acclaimed").put("blurb", "The best-rated films and shows."))
            .put(JSONObject().put("key", "family").put("title", "Family movie night").put("blurb", "Rated G or PG, and well liked.")))
        .put("franchises", JSONArray().put(JSONObject().put("id", 10).put("name", "Star Wars")))

    private fun ok(body: Any) = MockResponse().setResponseCode(200).setHeader("Content-Type", "application/json").setBody(body.toString())

    override fun dispatch(request: RecordedRequest): MockResponse {
        val path = request.requestUrl?.encodedPath ?: ""
        asked.add(request.method + " " + path)
        return when {
            path == "/api/auth/me" -> ok(JSONObject().put("username", "sam").put("role", "admin"))
            path == "/api/profiles" -> ok(JSONObject().put("profiles", JSONArray().put(JSONObject().put("id", 1).put("name", "Sam").put("avatar", "teal").put("main", true))))
            path == "/api/watch/home" -> ok(home())
            path.matches(Regex("/api/watch/tv/\\d+/season/\\d+")) -> ok(episodes())
            path.matches(Regex("/api/watch/(movie|tv)/\\d+")) -> {
                val (kind, id) = path.removePrefix("/api/watch/").split("/")
                ok(title(kind, id.toInt()))
            }
            path.startsWith("/api/watch/extras/") -> ok(JSONObject())
            path == "/api/watch/coming-soon" -> ok(cards(6, 5).also { a -> for (i in 0 until a.length()) a.getJSONObject(i).put("soon", true).put("outDate", "2026-12-0${i + 1}") })
            path == "/api/watch/this-week" -> ok(JSONArray())
            path == "/api/watch/search" -> ok(cards(9, 1))
            path == "/api/watch/genre" -> ok(cards(12, 2))
            path.startsWith("/api/watch/remote/") -> ok(JSONObject().put("q", "").put("seq", 0))
            path == "/api/watch/top" -> ok(JSONObject().put("title", "Top 10 titles").put("items", cards(10, 1))
                .put("services", JSONArray().apply { services.forEach { (k, n) -> put(JSONObject().put("key", k).put("name", n)) } })
                .put("genres", JSONArray().put("Comedy").put("Drama")).put("countries", JSONObject().put("US", "United States")))
            path == "/api/watch/browse" -> ok(browse())
            path == "/api/watch/ask" && request.method == "GET" -> ok(JSONObject().put("ready", true))
            path == "/api/watch/ask" -> ok(JSONObject().put("reply", "Here are a few easy laughs for tonight, all under two hours.")
                .put("heading", "Funny, under two hours").put("cards", cards(8, 1)))
            path.startsWith("/api/watch/browse/service/") -> ok(JSONObject().put("name", "Netflix").put("rows", JSONArray()
                .put(row("top", "Top 10 on Netflix today", cards(10, 1))).put(row("originals", "Netflix originals", cards(10, 3, "tv")))))
            path.startsWith("/api/watch/browse/collection/") || path.startsWith("/api/watch/browse/franchise/") ->
                ok(JSONObject().put("title", "Critically acclaimed").put("name", "Star Wars").put("items", cards(12, 2)))
            path.startsWith("/api/watch/person/") -> ok(JSONObject().put("credits", cards(6, 2)))
            path == "/api/live/channels" -> ok(channels())
            path == "/api/live/sports" -> ok(sports())
            path == "/api/parties" -> ok(JSONArray())
            path.startsWith("/api/live/") -> ok(JSONArray())
            path == "/api/play/crash" -> MockResponse().setResponseCode(204)
            else -> {
                synchronized(unknown) { unknown.add(request.method + " " + path) }
                MockResponse().setResponseCode(404).setBody("{\"error\":\"Not in the stand-in server.\"}")
            }
        }
    }
}
