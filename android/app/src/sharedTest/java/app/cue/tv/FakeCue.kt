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
    /** The server's own address, once started: cards then carry pictures from it. */
    var base = ""
    val unknown = mutableListOf<String>()
    val asked: MutableList<String> = java.util.Collections.synchronizedList(mutableListOf())

    private val names = listOf("Night Harbor", "Paper Moons", "Signal Fire", "Glass Rivers", "Northbound", "Copper Sky",
        "Quiet Hours", "Last Light", "Marigold", "Orbit", "Dispatch", "The Valley")

    private fun card(i: Int, kind: String = if (i % 3 == 0) "tv" else "movie") = JSONObject()
        .put("kind", kind).put("tmdbId", 100 + i).put("title", names[i % names.size]).put("year", 2014 + i % 10)
        .put("overview", "A story told over one long summer in a small coastal town.")
        .put("rating", 7.4)
        .apply {
            if (base.isNotEmpty()) {
                put("posterUrl", "$base/art/poster/${i % names.size}.png")
                put("backdropUrl", "$base/art/backdrop/${i % names.size}.png")
            }
        }

    // A picture for a card: soft light over a colour of its own, so the
    // screens look like they do with real artwork.
    private fun art(kind: String, i: Int): MockResponse {
        val (w, h) = if (kind == "poster") 300 to 450 else 640 to 360
        val bmp = android.graphics.Bitmap.createBitmap(w, h, android.graphics.Bitmap.Config.ARGB_8888)
        val c = android.graphics.Canvas(bmp)
        val hue = (i * 47f) % 360f
        val deep = android.graphics.Color.HSVToColor(floatArrayOf(hue, 0.55f, 0.32f))
        val bright = android.graphics.Color.HSVToColor(floatArrayOf((hue + 30f) % 360f, 0.45f, 0.78f))
        val p = android.graphics.Paint()
        p.shader = android.graphics.LinearGradient(0f, h.toFloat(), w.toFloat(), 0f, deep, bright, android.graphics.Shader.TileMode.CLAMP)
        c.drawRect(0f, 0f, w.toFloat(), h.toFloat(), p)
        p.shader = android.graphics.RadialGradient(w * 0.7f, h * 0.3f, w * 0.6f, 0x66FFFFFF, 0x00FFFFFF, android.graphics.Shader.TileMode.CLAMP)
        c.drawRect(0f, 0f, w.toFloat(), h.toFloat(), p)
        val out = okio.Buffer()
        bmp.compress(android.graphics.Bitmap.CompressFormat.PNG, 100, out.outputStream())
        return MockResponse().setHeader("Content-Type", "image/png").setBody(out)
    }

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

    // Shaped like the server's catalog: plain names, a country and topic
    // each; a copy of a channel and a divider, which the lists leave out.
    private fun channels() = JSONObject().put("configured", true)
        .put("categories", JSONArray().put(JSONObject().put("id", "news").put("name", "News")).put(JSONObject().put("id", "sports").put("name", "Sports")))
        .put("countries", JSONArray().put(JSONObject().put("code", "US").put("name", "United States").put("count", 6))
            .put(JSONObject().put("code", "UK").put("name", "United Kingdom").put("count", 1)))
        .put("homeCountry", "US")
        .put("lists", JSONArray().put(JSONObject().put("id", 1).put("name", "Evenings").put("items", JSONArray().put("c5").put("c2"))))
        .put("topics", JSONArray().apply {
            listOf("general" to "General", "news" to "News", "sports" to "Sports", "kids" to "Kids", "movies" to "Movies").forEach { (k, n) ->
                put(JSONObject().put("key", k).put("name", n))
            }
        })
        .put("channels", JSONArray().apply {
            data class Ch(val name: String, val country: String, val topic: String, val show: String, val copyOf: String = "", val divider: Boolean = false)
            listOf(
                Ch("News 24", "US", "news", "Evening Report"), Ch("World News", "US", "news", "The World Tonight"),
                Ch("Sports One", "US", "sports", "NFL Football: Bills at Chiefs"), Ch("Sports Two", "US", "sports", "College Basketball"),
                Ch("Cartoon Club", "US", "kids", "Saturday Toons"), Ch("Cinema One", "US", "movies", "Night Harbor"),
                Ch("BBC One", "UK", "general", "The Evening Report"),
                Ch("News 24", "US", "news", "Evening Report", copyOf = "c0"), Ch("##### US BACKUP #####", "US", "general", "", divider = true),
            ).forEachIndexed { i, c ->
                nowShows["c$i"] = c.show
                put(JSONObject().put("id", "c$i").put("num", i + 1).put("name", c.name).put("category", if (c.topic == "news") "news" else "sports")
                    .put("country", c.country).put("topic", c.topic).put("copyOf", c.copyOf).put("divider", c.divider)
                    .put("now", JSONObject().put("title", c.show).put("start", iso(-0.5)).put("stop", iso(0.5))))
            }
        })

    // Each channel's guide: what's on now (from the channel list), then two more.
    private fun guide(ids: List<String>) = JSONObject().put("programmes", JSONObject().apply {
        ids.filter { it.isNotBlank() }.forEachIndexed { i, id ->
            put(id, JSONArray()
                .put(JSONObject().put("title", "Earlier Edition").put("start", iso(-1.5)).put("stop", iso(-0.5)))
                .put(JSONObject().put("title", nowShows[id] ?: "Live").put("desc", "Tonight's top stories, with the weather and sport.").put("start", iso(-0.5)).put("stop", iso(0.5 + (i % 2) * 0.5)))
                .put(JSONObject().put("title", if (i % 2 == 0) "Late Night Live" else "The Week Ahead").put("start", iso(0.5 + (i % 2) * 0.5)).put("stop", iso(2.0)))
                .put(JSONObject().put("title", "Overnight").put("start", iso(2.0)).put("stop", iso(4.0))))
        }
    })

    private val nowShows = mutableMapOf<String, String>()

    private val colors = mapOf("KC" to "e31837", "BUF" to "00338d", "BOS" to "007a33", "LAL" to "552583")

    private fun team(name: String, short: String, abbr: String, score: String) =
        JSONObject().put("id", abbr).put("name", name).put("short", short).put("abbr", abbr).put("score", score).put("record", "9-2")
            .put("color", colors[abbr] ?: "")

    private fun sports() = JSONObject().put("source", "espn")
        .put("big", JSONObject().put("id", "1").put("league", "nfl").put("leagueName", "NFL").put("state", "in").put("detail", "Q3 4:12")
            .put("start", iso(-1.5)).put("title", "Bills at Chiefs").put("onNow", true)
            .put("home", team("Kansas City Chiefs", "Chiefs", "KC", "17")).put("away", team("Buffalo Bills", "Bills", "BUF", "21"))
            .put("networks", JSONArray().put("CBS"))
            .put("venue", "Arrowhead Stadium")
            .put("channel", JSONObject().put("id", "c2").put("num", 3).put("name", "Sports One")))
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
            path.startsWith("/art/") -> path.removePrefix("/art/").removeSuffix(".png").split("/").let { (kind, i) -> art(kind, i.toInt()) }
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
            path == "/api/watch/search" && request.requestUrl?.queryParameter("v") == "2" -> ok(JSONObject()
                .put("query", request.requestUrl?.queryParameter("q")).put("understood", "Comedy movies and shows")
                .put("sections", JSONArray()
                    .put(JSONObject().put("key", "intent").put("title", "Comedy movies and shows").put("items", cards(10, 2)))
                    .put(JSONObject().put("key", "person").put("title", "With Ana Reyes").put("items", cards(8, 5)))
                    .put(JSONObject().put("key", "titles").put("title", "Titles").put("items", cards(4, 7)))))
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
            path == "/api/live/picks" -> ok(JSONObject()
                .put("forYou", JSONArray().put(JSONObject().put("channel", JSONObject().put("id", "c2")).put("onNow", true)))
                .put("tonight", JSONArray().put(JSONObject().put("channel", JSONObject().put("id", "c5"))
                    .put("show", JSONObject().put("title", "Night Harbor").put("start", iso(2.0)).put("stop", iso(4.0))))))
            path == "/api/live/guide" -> ok(guide(request.requestUrl?.queryParameter("ids").orEmpty().split(",")))
            path == "/api/parties" -> ok(JSONArray())
            path.startsWith("/api/live/") -> ok(JSONArray())
            path == "/api/play/crash" -> MockResponse().setResponseCode(204)
            path == "/api/version" -> ok(JSONObject().put("version", "2.1.9"))
            path == "/api/system/crashes" -> ok(JSONObject().put("open", 1).put("items", JSONArray().put(JSONObject()
                .put("id", 7).put("app", "tv").put("count", 3).put("device", "Amazon AFTMM (Android 7)").put("version", "0.1.250")
                .put("headline", "java.lang.IllegalStateException: Release should only be called once").put("fixedAt", ""))))
            else -> {
                synchronized(unknown) { unknown.add(request.method + " " + path) }
                MockResponse().setResponseCode(404).setBody("{\"error\":\"Not in the stand-in server.\"}")
            }
        }
    }
}
