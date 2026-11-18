package app.cue.tv.home

import android.content.Context
import android.webkit.CookieManager
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL
import java.net.URLEncoder

// Talking to Cue for the app's own screens. Signing in is a cookie kept in
// the system's CookieManager, the same one the web view and the player use,
// so all three are signed in together. The profile this TV is on goes with
// every request (X-Cue-Profile), as in Watch.
class CueApi(ctx: Context, val server: String) {
    private val prefs = ctx.getSharedPreferences("cue", Context.MODE_PRIVATE)

    var profileToken: String
        get() = prefs.getString("profileToken", "") ?: ""
        set(v) = prefs.edit().putString("profileToken", v).apply()
    var profileName: String
        get() = prefs.getString("profileName", "") ?: ""
        set(v) = prefs.edit().putString("profileName", v).apply()
    var profileId: Int // the profile this TV is on
        get() = prefs.getInt("profileId", 0)
        set(v) = prefs.edit().putInt("profileId", v).apply()
    var profileAvatar: String // "teal" or "teal/fox": its color, then its picture
        get() = prefs.getString("profileAvatar", "") ?: ""
        set(v) = prefs.edit().putString("profileAvatar", v).apply()
    var accent: String
        get() = prefs.getString("accent", "") ?: ""
        set(v) = prefs.edit().putString("accent", v).apply()
    var themeBackground: String // midnight, black, slate or tinted
        get() = prefs.getString("themeBackground", "") ?: ""
        set(v) = prefs.edit().putString("themeBackground", v).apply()
    var themeGlow: String // off, soft or strong
        get() = prefs.getString("themeGlow", "") ?: ""
        set(v) = prefs.edit().putString("themeGlow", v).apply()

    private fun keepTheme(theme: JSONObject?) {
        accent = theme?.optString("accent") ?: ""
        themeBackground = theme?.optString("background") ?: ""
        themeGlow = theme?.optString("glow") ?: ""
    }

    /** The profile's theme as it is now (it may have been changed on the web). */
    suspend fun refreshTheme() {
        val a = obj("/api/profiles").optJSONArray("profiles") ?: return
        for (i in 0 until a.length()) {
            val p = a.getJSONObject(i)
            if (p.optInt("id") == profileId) keepTheme(p.optJSONObject("theme"))
        }
    }

    class Problem(val code: Int, message: String) : Exception(message)

    val cookie: String get() = CookieManager.getInstance().getCookie(server) ?: ""

    suspend fun request(method: String, path: String, body: JSONObject? = null): String = withContext(Dispatchers.IO) {
        val c = try {
            URL(server + path).openConnection() as HttpURLConnection
        } catch (e: Exception) {
            throw Problem(0, "That server address doesn't look right.")
        }
        try {
            c.requestMethod = method
            c.connectTimeout = 15000
            c.readTimeout = 30000
            c.setRequestProperty("User-Agent", "CueTV/1 (native)")
            c.setRequestProperty("Accept", "application/json")
            cookie.takeIf { it.isNotEmpty() }?.let { c.setRequestProperty("Cookie", it) }
            profileToken.takeIf { it.isNotEmpty() }?.let { c.setRequestProperty("X-Cue-Profile", it) }
            if (body != null) {
                c.doOutput = true
                c.setRequestProperty("Content-Type", "application/json")
                c.outputStream.use { it.write(body.toString().toByteArray()) }
            }
            val code = try {
                c.responseCode
            } catch (e: Exception) {
                throw Problem(0, "Couldn't reach Cue at $server.")
            }
            // Keep any cookie Cue sets (signing in), for the web view and player too.
            val cm = CookieManager.getInstance()
            c.headerFields.forEach { (k, v) ->
                if (k != null && k.equals("Set-Cookie", ignoreCase = true)) v.forEach { cm.setCookie(server, it) }
            }
            cm.flush()
            val text = (if (code in 200..299) c.inputStream else c.errorStream)?.bufferedReader()?.use { it.readText() } ?: ""
            if (code !in 200..299) {
                val msg = try { JSONObject(text).optString("error") } catch (e: Exception) { "" }
                throw Problem(code, msg.ifEmpty { "Cue answered $code." })
            }
            text
        } finally {
            c.disconnect()
        }
    }

    suspend fun obj(path: String): JSONObject = JSONObject(request("GET", path).ifEmpty { "{}" })
    suspend fun arr(path: String): JSONArray = JSONArray(request("GET", path).ifEmpty { "[]" })

    fun q(s: String): String = URLEncoder.encode(s, "UTF-8")

    // ---- Signing in and profiles ----

    suspend fun signedIn(): Boolean = try {
        obj("/api/auth/me"); true
    } catch (e: Problem) {
        if (e.code == 401 || e.code == 403) false else throw e
    }

    suspend fun signIn(user: String, password: String) {
        val answer = request("POST", "/api/auth/login", JSONObject().put("username", user).put("password", password))
        // Two-step sign-in is on: the TV signs in with the phone instead.
        if (runCatching { JSONObject(answer).optBoolean("twoStep") }.getOrDefault(false)) {
            throw Problem(400, "This account uses two-step sign-in. Choose \"Sign in with your phone\" and scan the code instead.")
        }
    }

    /** A code for signing this TV in from a phone: (code, token). */
    suspend fun tvLinkStart(): Pair<String, String> =
        JSONObject(request("POST", "/api/auth/tv-link")).let { it.optString("code") to it.optString("token") }

    /** True once a phone has confirmed the code: this TV is then signed in. */
    suspend fun tvLinkDone(token: String): Boolean = obj("/api/auth/tv-link/$token").optBoolean("ok")

    suspend fun profiles(): List<Profile> {
        val o = obj("/api/profiles")
        val a = o.optJSONArray("profiles") ?: JSONArray()
        return (0 until a.length()).map { Profile.from(a.getJSONObject(it)) }
    }

    suspend fun choose(p: Profile, pin: String = "") {
        val o = JSONObject(request("POST", "/api/profiles/${p.id}/select", JSONObject().put("pin", pin)))
        profileToken = o.optString("token")
        val prof = o.optJSONObject("profile")
        profileId = p.id
        profileName = prof?.optString("name") ?: p.name
        profileAvatar = prof?.optString("avatar")?.ifEmpty { null } ?: p.avatar
        keepTheme(prof?.optJSONObject("theme"))
    }

    // Managing the account's profiles (from its main profile; the server checks).
    suspend fun addProfile(name: String, avatar: String) {
        request("POST", "/api/profiles", JSONObject().put("name", name).put("avatar", avatar))
    }

    suspend fun updateProfile(id: Int, name: String, avatar: String) {
        request("PUT", "/api/profiles/$id", JSONObject().put("name", name).put("avatar", avatar))
        if (id == profileId) {
            profileName = name
            profileAvatar = avatar
        }
    }

    suspend fun removeProfile(id: Int) {
        request("DELETE", "/api/profiles/$id")
    }

    fun forgetProfile() {
        profileToken = ""
    }

    /** Signs this TV out of its account (another person can then sign in). */
    suspend fun signOut() {
        runCatching { request("POST", "/api/auth/logout") }
        profileToken = ""
        profileName = ""
        withContext(Dispatchers.Main) {
            CookieManager.getInstance().removeAllCookies(null)
            CookieManager.getInstance().flush()
        }
    }

    // ---- Movies and shows ----

    suspend fun home(): Home = Home.from(obj("/api/watch/home"))
    suspend fun search(q: String): List<Tile> = arr("/api/watch/search?q=${q(q)}").cards()
    /** A genre's most popular films or shows (Search's genre tabs). */
    suspend fun genre(kind: String, id: Int): List<Tile> = arr("/api/watch/genre?kind=$kind&genre=$id").cards()
    /** What a phone has typed for this TV's search (code from the QR), and its count. */
    suspend fun remoteSearch(code: String): Pair<String, Int> = obj("/api/watch/remote/$code").let { it.optString("q") to it.optInt("seq") }
    suspend fun title(kind: String, id: Int): Title = Title.from(obj("/api/watch/$kind/$id"))
    /** What a person has been in (their name, and the titles, most popular first). */
    suspend fun personCredits(id: Int): List<Tile> = (obj("/api/watch/person/$id").optJSONArray("credits") ?: JSONArray()).cards()
    suspend fun season(id: Int, n: Int): List<Episode> {
        val a = arr("/api/watch/tv/$id/season/$n")
        return (0 until a.length()).map { Episode.from(a.getJSONObject(it)) }
    }
    suspend fun next(id: Int, season: Int, episode: Int): Episode =
        Episode.from(obj("/api/watch/tv/$id/next?season=$season&episode=$episode"))
    suspend fun setInList(kind: String, id: Int, on: Boolean) {
        request(if (on) "PUT" else "DELETE", "/api/watch/list/$kind/$id")
    }

    /** This profile's thumb on a title: "up", "down" (Not for me: off Home) or "". */
    suspend fun setThumb(kind: String, id: Int, thumb: String) {
        request("PUT", "/api/watch/thumb/$kind/$id", JSONObject().put("thumb", thumb))
    }

    /** Coming soon: films not out to stream yet, soonest first. */
    suspend fun comingSoon(): List<Tile> = arr("/api/watch/coming-soon").cards()

    /** This week: the next episodes of shows this profile watches, with the day each airs. */
    suspend fun thisWeek(): List<Pair<Tile, String>> {
        val a = arr("/api/watch/this-week")
        return (0 until a.length()).map { val o = a.getJSONObject(it); Tile.from(o) to o.optString("airDate") }
    }

    /** Tells this profile when a title is out to stream (or stops). */
    suspend fun setRemind(kind: String, id: Int, on: Boolean) {
        request(if (on) "PUT" else "DELETE", "/api/watch/remind/$kind/$id")
    }

    /** Takes a title off Continue Watching (where you were is kept). */
    suspend fun hideFromContinue(kind: String, id: Int) {
        request("DELETE", "/api/watch/continue/$kind/$id")
    }

    // ---- Live TV ----

    suspend fun channels(): LiveChannels = LiveChannels.from(obj("/api/live/channels"))
    /** The Sports tab: ESPN's games (none when Cue reads them from the TV guide instead). */
    suspend fun sportsBoard(): SportsBoard {
        val o = obj("/api/live/sports")
        if (o.optString("source") != "espn") return SportsBoard(null, emptyList(), emptyList())
        fun side(t: JSONObject?) = GameSide(t?.optString("short")?.ifEmpty { t.optString("name") }.orEmpty(), t?.str("logo"), t?.optString("score").orEmpty(), t?.optString("record").orEmpty())
        fun game(g: JSONObject): GameRow {
            val live = g.optString("state") == "in"
            val ch = g.optJSONObject("channel")
            val where = ch?.optString("name")?.takeIf { it.isNotEmpty() } ?: g.str("streaming")?.let { "On $it" }
                ?: g.optJSONArray("networks")?.optString(0)?.takeIf { it.isNotEmpty() }?.let { "On $it" } ?: ""
            return GameRow(g.optString("league") + "-" + g.optString("id"), g.optString("leagueName") + (g.str("note")?.let { " · $it" } ?: ""),
                side(g.optJSONObject("away")), side(g.optJSONObject("home")),
                if (live) g.optString("detail") else shortTime(g.optString("start")), live, g.optBoolean("favorite"),
                ch?.optString("id")?.takeIf { it.isNotEmpty() }, where)
        }
        fun list(k: String) = (o.optJSONArray(k) ?: JSONArray()).let { a -> (0 until a.length()).map { game(a.getJSONObject(it)) } }
        val big = o.optJSONObject("big")?.let { game(it) }
        return SportsBoard(big, list("now").filter { it.key != big?.key }, list("soon").filter { it.key != big?.key })
    }

    /** Sports: the channels with games on now or soon (the next big game first), and that game. */
    suspend fun sports(): SportsInfo {
        val o = obj("/api/live/sports")
        val ids = mutableListOf<String>()
        o.optJSONObject("big")?.optJSONObject("channel")?.optString("id")?.takeIf { it.isNotEmpty() }?.let { ids.add(it) }
        for (k in listOf("now", "soon")) {
            val a = o.optJSONArray(k) ?: continue
            for (i in 0 until a.length()) a.getJSONObject(i).optJSONObject("channel")?.optString("id")?.takeIf { it.isNotEmpty() }?.let { if (it !in ids) ids.add(it) }
        }
        val b = o.optJSONObject("big") ?: return SportsInfo(ids, null)
        val live = b.optBoolean("onNow")
        val where = b.optJSONObject("channel")?.optString("name")?.takeIf { it.isNotEmpty() } ?: b.str("streaming")?.let { "on $it" } ?: ""
        if (o.optString("source") != "espn") {
            val title = b.optJSONObject("show")?.optString("title").orEmpty()
            return SportsInfo(ids, if (title.isEmpty()) null else BigGame(
                tag = if (live) "LIVE NOW" else "NEXT BIG GAME", away = null, home = null, title = title, detail = where, live = live))
        }
        fun side(t: JSONObject?) = t?.let { GameSide(it.optString("short").ifEmpty { it.optString("name") }, it.str("logo"), it.optString("score"), it.optString("record")) }
        val tag = (if (live) "LIVE" else if (b.optBoolean("favorite")) "YOUR TEAM" else "NEXT BIG GAME") + " · " + b.optString("leagueName") +
            (b.str("note")?.let { " · $it" } ?: "")
        val detail = listOf(if (live) b.optString("detail") else shortTime(b.optString("start")), where).filter { it.isNotEmpty() }.joinToString(" · ")
        return SportsInfo(ids, BigGame(tag, side(b.optJSONObject("away")), side(b.optJSONObject("home")), b.optString("title"), detail, live))
    }
    suspend fun livePlay(id: String): JSONObject = obj("/api/live/play/${URLEncoder.encode(id, "UTF-8").replace("+", "%20")}")
    suspend fun setFavorite(id: String, on: Boolean) {
        request(if (on) "PUT" else "DELETE", "/api/live/favorites/${URLEncoder.encode(id, "UTF-8").replace("+", "%20")}")
    }
}

fun JSONArray.cards(): List<Tile> = (0 until length()).map { Tile.from(getJSONObject(it)) }

private fun JSONObject.str(k: String): String? = optString(k).takeIf { has(k) && !isNull(k) && it.isNotEmpty() }
private fun JSONObject.int(k: String): Int? = if (has(k) && !isNull(k)) optInt(k) else null
private fun JSONObject.dbl(k: String): Double? = if (has(k) && !isNull(k)) optDouble(k) else null

data class Profile(val id: Int, val name: String, val avatar: String, val hasPin: Boolean, val accent: String?, val main: Boolean = false) {
    companion object {
        fun from(o: JSONObject) = Profile(o.optInt("id"), o.optString("name"), o.optString("avatar"), o.optBoolean("hasPin"),
            o.optJSONObject("theme")?.str("accent"), o.optBoolean("main"))
    }
}

data class Tile(
    val kind: String, val tmdbId: Int, val title: String, val year: Int?, val overview: String?,
    val poster: String?, val backdrop: String?, val rating: Double?, val progress: Double?,
    val season: Int?, val episode: Int?, val episodeTitle: String?, val isNew: Boolean,
    val logo: String? = null,
    // Horror, anime or adult: kept off the screensaver (the server gives
    // these no preview clip either).
    val quiet: Boolean = false,
    // Not out to stream yet (Coming soon), the day it streams when known,
    // and whether a title this profile asked about is out now.
    val soon: Boolean = false,
    val inTheaters: Boolean = false,
    val outDate: String? = null,
    val outNow: Boolean = false,
    val reminding: Boolean = false,
    // Continue Watching, away from the show a while: what happened last time.
    val recap: String? = null,
) {
    val key get() = "$kind-$tmdbId-${season ?: 0}-${episode ?: 0}"
    // Being watched (Continue Watching, new episodes): its page offers Resume.
    val resumes get() = progress != null || episode != null
    val episodeLine get() = if (season != null && episode != null) "S$season:E$episode" + (episodeTitle?.let { " · $it" } ?: "") else ""

    companion object {
        fun from(o: JSONObject) = Tile(
            o.optString("kind"), o.optInt("tmdbId"), o.optString("title"), o.int("year"), o.str("overview"),
            o.str("posterUrl"), o.str("backdropUrl"), o.dbl("rating"), o.dbl("progress"),
            o.int("season"), o.int("episode"), o.str("episodeTitle"), o.optBoolean("new"),
            // Only the Home banner and title pages carry it ("" there: none).
            if (o.has("logoUrl") || o.has("genres")) o.optString("logoUrl") else null,
            quiet = o.optBoolean("quiet"),
            soon = o.optBoolean("soon"),
            inTheaters = o.optBoolean("inTheaters"),
            outDate = o.str("outDate"),
            outNow = o.optBoolean("outNow"),
            reminding = o.optBoolean("reminding"),
            recap = o.str("recap"),
        )
    }
}

data class Shelf(val key: String, val title: String, val items: List<Tile>)

data class Home(val hero: Tile?, val rows: List<Shelf>, val top10: List<TopList> = emptyList()) {
    companion object {
        fun from(o: JSONObject): Home {
            val rows = o.optJSONArray("rows") ?: JSONArray()
            return Home(
                o.optJSONObject("hero")?.let { Tile.from(it) },
                (0 until rows.length()).map {
                    val r = rows.getJSONObject(it)
                    Shelf(r.optString("key"), r.optString("title"), (r.optJSONArray("items") ?: JSONArray()).cards())
                },
                topLists(o),
            )
        }
    }
}

data class Season(val number: Int, val name: String)

// Scores from IMDb, Rotten Tomatoes and Metacritic (when Cue has an OMDb key).
data class Ratings(val imdb: String?, val rottenTomatoes: String?, val metacritic: String?) {
    companion object {
        fun from(o: JSONObject?): Ratings? {
            o ?: return null
            val r = Ratings(o.str("imdb"), o.str("rottenTomatoes"), o.str("metacritic"))
            return if (r.imdb == null && r.rottenTomatoes == null && r.metacritic == null) null else r
        }
    }
}

data class Title(
    val card: Tile, val tagline: String?, val runtime: Int?, val genres: List<String>, val certification: String?,
    val seasons: List<Season>, val inList: Boolean, val resumeSeason: Int?, val resumeEpisode: Int?, val resumePosition: Double?,
    val more: List<Tile>, val directors: List<String> = emptyList(), val ratings: Ratings? = null,
    // Not out to stream yet: when it's coming, shown instead of Play.
    val notOut: String? = null,
    // This profile's thumb on it: "up", "down" or "".
    val thumb: String = "",
    // This profile asked to be told when it's out.
    val reminding: Boolean = false,
    // In theaters now (it still plays): "In theaters now · streaming March 4th".
    val theaterNote: String? = null,
    // The film series it's part of: its name and films in order.
    val seriesName: String? = null,
    val series: List<Tile> = emptyList(),
    // The cast with photos: picking one shows what else they're in.
    val people: List<CastPerson> = emptyList(),
) {
    companion object {
        fun from(o: JSONObject): Title {
            val g = o.optJSONArray("genres") ?: JSONArray()
            val s = o.optJSONArray("seasons") ?: JSONArray()
            return Title(
                Tile.from(o), o.str("tagline"), o.int("runtime"), (0 until g.length()).map { g.getString(it) }, o.str("certification"),
                (0 until s.length()).map { s.getJSONObject(it).let { x -> Season(x.optInt("number"), x.optString("name")) } },
                o.optBoolean("inList"), o.int("resumeSeason"), o.int("resumeEpisode"), o.dbl("resumePosition"),
                (o.optJSONArray("more") ?: JSONArray()).cards(),
                (o.optJSONArray("directors") ?: JSONArray()).let { d -> (0 until d.length()).map { d.getString(it) } },
                Ratings.from(o.optJSONObject("ratings")),
                o.str("notOut"),
                o.optString("thumb"),
                o.optBoolean("reminding"),
                o.str("theaterNote"),
                o.optJSONObject("collection")?.optString("name")?.takeIf { it.isNotEmpty() },
                o.optJSONObject("collection")?.optJSONArray("parts")?.cards() ?: emptyList(),
                (o.optJSONArray("people") ?: JSONArray()).let { a ->
                    (0 until a.length()).map { a.getJSONObject(it) }.map { CastPerson(it.optInt("id"), it.optString("name"), it.str("character"), it.str("photo")) }
                },
            )
        }
    }
}

data class GameSide(val name: String, val logo: String?, val score: String, val record: String)
data class BigGame(val tag: String, val away: GameSide?, val home: GameSide?, val title: String, val detail: String, val live: Boolean)
data class SportsInfo(val channels: List<String>, val big: BigGame?)

/** One game on the Sports tab: the sides, when or the clock, and where it's on (a channel id to play, or a service). */
data class GameRow(
    val key: String, val league: String, val away: GameSide, val home: GameSide, val detail: String,
    val live: Boolean, val favorite: Boolean, val channelId: String?, val where: String,
)
data class SportsBoard(val big: GameRow?, val now: List<GameRow>, val soon: List<GameRow>)

/** "Sun 1:00 PM" from an ISO time, in the TV's time zone. */
fun shortTime(iso: String): String = runCatching {
    // The server's times are UTC ("2026-11-22T18:00:00Z"); minSdk 23 has no
    // pattern letter for an offset, so the Z is matched as text.
    val f = java.text.SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss", java.util.Locale.US).apply { timeZone = java.util.TimeZone.getTimeZone("UTC") }
    val d = f.parse(iso.take(19)) ?: return iso
    java.text.SimpleDateFormat("EEE h:mm a", java.util.Locale.US).format(d)
}.getOrDefault(iso)

data class CastPerson(val id: Int, val name: String, val character: String?, val photo: String?)

data class Episode(
    val season: Int, val episode: Int, val title: String, val overview: String?, val still: String?,
    val runtime: Int?, val progress: Double?, val finished: Boolean,
) {
    companion object {
        fun from(o: JSONObject) = Episode(o.optInt("season"), o.optInt("episode"), o.optString("title"), o.str("overview"),
            o.str("stillUrl"), o.int("runtime"), o.dbl("progress"), o.optBoolean("finished"))
    }
}

data class Show(val title: String, val start: Long, val stop: Long) {
    fun progress(now: Long = System.currentTimeMillis()): Float =
        if (stop > start) ((now - start).toFloat() / (stop - start)).coerceIn(0f, 1f) else 0f

    companion object {
        fun from(o: JSONObject?): Show? {
            o ?: return null
            return Show(o.optString("title"), parseTime(o.optString("start")), parseTime(o.optString("stop")))
        }

        // An RFC 3339 time ("2026-10-11T17:00:00Z", "...+02:00"), on every
        // Android version (java.time needs Android 8).
        fun parseTime(s: String): Long = try {
            var t = s.replace(Regex("""\.\d+"""), "")
            t = if (t.endsWith("Z")) t.dropLast(1) + "+0000" else t.replace(Regex("""([+-]\d\d):(\d\d)$"""), "$1$2")
            java.text.SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ssZ", java.util.Locale.US).parse(t)?.time ?: 0L
        } catch (e: Exception) {
            0L
        }
    }
}

data class Channel(
    val id: String, val num: Int, val name: String, val category: String, val logo: String?,
    val favorite: Boolean, val now: Show?, val next: Show?,
) {
    companion object {
        fun from(o: JSONObject) = Channel(o.optString("id"), o.optInt("num"), o.optString("name"), o.optString("category"),
            o.str("logo"), o.optBoolean("favorite"), Show.from(o.optJSONObject("now")), Show.from(o.optJSONObject("next")))
    }
}

data class LiveChannels(val configured: Boolean, val groups: List<Pair<String, String>>, val channels: List<Channel>) {
    companion object {
        fun from(o: JSONObject): LiveChannels {
            val c = o.optJSONArray("categories") ?: JSONArray()
            val ch = o.optJSONArray("channels") ?: JSONArray()
            return LiveChannels(
                o.optBoolean("configured"),
                (0 until c.length()).map { c.getJSONObject(it).let { x -> x.optString("id") to x.optString("name") } },
                (0 until ch.length()).map { Channel.from(ch.getJSONObject(it)) },
            )
        }
    }
}
