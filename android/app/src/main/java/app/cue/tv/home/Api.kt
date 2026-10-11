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
    var accent: String
        get() = prefs.getString("accent", "") ?: ""
        set(v) = prefs.edit().putString("accent", v).apply()

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
        request("POST", "/api/auth/login", JSONObject().put("username", user).put("password", password))
    }

    suspend fun profiles(): List<Profile> {
        val o = obj("/api/profiles")
        val a = o.optJSONArray("profiles") ?: JSONArray()
        return (0 until a.length()).map { Profile.from(a.getJSONObject(it)) }
    }

    suspend fun choose(p: Profile, pin: String = "") {
        val o = JSONObject(request("POST", "/api/profiles/${p.id}/select", JSONObject().put("pin", pin)))
        profileToken = o.optString("token")
        val prof = o.optJSONObject("profile")
        profileName = prof?.optString("name") ?: p.name
        accent = prof?.optJSONObject("theme")?.optString("accent") ?: ""
    }

    fun forgetProfile() {
        profileToken = ""
    }

    // ---- Movies and shows ----

    suspend fun home(): Home = Home.from(obj("/api/watch/home"))
    suspend fun search(q: String): List<Tile> = arr("/api/watch/search?q=${q(q)}").cards()
    suspend fun title(kind: String, id: Int): Title = Title.from(obj("/api/watch/$kind/$id"))
    suspend fun season(id: Int, n: Int): List<Episode> {
        val a = arr("/api/watch/tv/$id/season/$n")
        return (0 until a.length()).map { Episode.from(a.getJSONObject(it)) }
    }
    suspend fun next(id: Int, season: Int, episode: Int): Episode =
        Episode.from(obj("/api/watch/tv/$id/next?season=$season&episode=$episode"))
    suspend fun setInList(kind: String, id: Int, on: Boolean) {
        request(if (on) "PUT" else "DELETE", "/api/watch/list/$kind/$id")
    }

    // ---- Live TV ----

    suspend fun channels(): LiveChannels = LiveChannels.from(obj("/api/live/channels"))
    suspend fun livePlay(id: String): JSONObject = obj("/api/live/play/${URLEncoder.encode(id, "UTF-8").replace("+", "%20")}")
    suspend fun setFavorite(id: String, on: Boolean) {
        request(if (on) "PUT" else "DELETE", "/api/live/favorites/${URLEncoder.encode(id, "UTF-8").replace("+", "%20")}")
    }
}

fun JSONArray.cards(): List<Tile> = (0 until length()).map { Tile.from(getJSONObject(it)) }

private fun JSONObject.str(k: String): String? = optString(k).takeIf { has(k) && !isNull(k) && it.isNotEmpty() }
private fun JSONObject.int(k: String): Int? = if (has(k) && !isNull(k)) optInt(k) else null
private fun JSONObject.dbl(k: String): Double? = if (has(k) && !isNull(k)) optDouble(k) else null

data class Profile(val id: Int, val name: String, val avatar: String, val hasPin: Boolean, val accent: String?) {
    companion object {
        fun from(o: JSONObject) = Profile(o.optInt("id"), o.optString("name"), o.optString("avatar"), o.optBoolean("hasPin"),
            o.optJSONObject("theme")?.str("accent"))
    }
}

data class Tile(
    val kind: String, val tmdbId: Int, val title: String, val year: Int?, val overview: String?,
    val poster: String?, val backdrop: String?, val rating: Double?, val progress: Double?,
    val season: Int?, val episode: Int?, val episodeTitle: String?, val isNew: Boolean,
) {
    val key get() = "$kind-$tmdbId-${season ?: 0}-${episode ?: 0}"
    // Being watched: opening it carries on playing.
    val resumes get() = progress != null || episode != null
    val episodeLine get() = if (season != null && episode != null) "S$season:E$episode" + (episodeTitle?.let { " · $it" } ?: "") else ""

    companion object {
        fun from(o: JSONObject) = Tile(
            o.optString("kind"), o.optInt("tmdbId"), o.optString("title"), o.int("year"), o.str("overview"),
            o.str("posterUrl"), o.str("backdropUrl"), o.dbl("rating"), o.dbl("progress"),
            o.int("season"), o.int("episode"), o.str("episodeTitle"), o.optBoolean("new"),
        )
    }
}

data class Shelf(val key: String, val title: String, val items: List<Tile>)

data class Home(val hero: Tile?, val rows: List<Shelf>) {
    companion object {
        fun from(o: JSONObject): Home {
            val rows = o.optJSONArray("rows") ?: JSONArray()
            return Home(
                o.optJSONObject("hero")?.let { Tile.from(it) },
                (0 until rows.length()).map {
                    val r = rows.getJSONObject(it)
                    Shelf(r.optString("key"), r.optString("title"), (r.optJSONArray("items") ?: JSONArray()).cards())
                },
            )
        }
    }
}

data class Season(val number: Int, val name: String)

data class Title(
    val card: Tile, val tagline: String?, val runtime: Int?, val genres: List<String>, val certification: String?,
    val seasons: List<Season>, val inList: Boolean, val resumeSeason: Int?, val resumeEpisode: Int?, val resumePosition: Double?,
    val more: List<Tile>,
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
            )
        }
    }
}

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
