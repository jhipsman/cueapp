package app.cue.tv.home

import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.setValue
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.launch
import kotlinx.coroutines.withTimeoutOrNull
import org.json.JSONArray
import org.json.JSONObject

// Playing from the app's own screens, in PlayerActivity (the native player
// that plays every format): it finds the version to play, where to carry
// on from and the skip times, opens the player, and acts on how it ended
// (the next episode, another version, another channel), as Watch's page
// does in the web view.
class Playback(
    private val api: CueApi,
    private val scope: CoroutineScope,
    private val open: (json: String) -> Unit,
) {
    var note by mutableStateOf("") // shown over the screens while a version is found
        private set

    private data class Vod(val kind: String, val id: Int, val season: Int, val episode: Int, var option: Int = 1, var options: Int = 1, var silent: Int = 0)

    private var vod: Vod? = null
    private var live: Channel? = null
    private var liveList: List<Channel> = emptyList()

    fun play(card: Tile) = playTitle(card.kind, card.tmdbId, card.season ?: 1, card.episode ?: 1)

    fun playTitle(kind: String, id: Int, season: Int, episode: Int, startSec: Double? = null) {
        val v = Vod(kind, id, if (kind == "tv") season else 0, if (kind == "tv") episode else 0)
        vod = v
        live = null
        scope.launch { openVod(v, startSec) }
    }

    fun clearNote() {
        note = ""
    }

    private suspend fun openVod(v: Vod, startSec: Double?) {
        note = if (v.option > 1) "Trying version ${v.option}…" else "Finding a version…"
        try {
            val path = if (v.kind == "tv") "/api/play/tmdb/tv/${v.id}/${v.season}/${v.episode}" else "/api/play/tmdb/movie/${v.id}"
            val a = api.obj("$path?option=${v.option}")
            v.options = maxOf(a.optInt("options", 1), 1)
            if (a.optString("youtube").isNotEmpty()) {
                if (v.option < v.options) {
                    v.option++
                    openVod(v, startSec)
                } else {
                    note = "Only a YouTube version was found, which plays in the web version of Cue."
                }
                return
            }
            var start = startSec ?: 0.0
            if (startSec == null) {
                val p = runCatching { api.obj("/api/watch/progress/${v.kind}/${v.id}?season=${v.season}&episode=${v.episode}") }.getOrNull()
                if (p != null && !p.optBoolean("finished") && p.optDouble("position", 0.0) > 30) start = p.optDouble("position")
            }
            val skips = withTimeoutOrNull(2000) {
                runCatching { api.obj("/api/watch/skips/${v.kind}/${v.id}?season=${v.season}&episode=${v.episode}&duration=0") }.getOrNull()
            } ?: JSONObject()
            val backdrop = withTimeoutOrNull(1500) { runCatching { api.title(v.kind, v.id).card.backdrop }.getOrNull() } ?: ""
            val label = if (v.kind == "tv") "S${v.season}:E${v.episode}" else ""
            val json = JSONObject()
                .put("url", a.optString("url"))
                .put("title", a.optString("title"))
                .put("subtitle", listOf(label, a.optString("quality"), a.optString("source").takeIf { it.isNotEmpty() }?.let { "via $it" } ?: "")
                    .filter { it.isNotEmpty() }.joinToString(" · "))
                .put("startSec", start)
                .put("hasOther", v.option < v.options)
                .put("kind", v.kind)
                .put("tmdbId", v.id)
                .put("season", v.season)
                .put("episode", v.episode)
                .put("token", api.profileToken)
                .put("accent", api.accent)
                .put("introStart", skips.optJSONObject("intro")?.optDouble("start", 0.0) ?: 0.0)
                .put("introEnd", skips.optJSONObject("intro")?.optDouble("end", 0.0) ?: 0.0)
                .put("recapStart", skips.optJSONObject("recap")?.optDouble("start", 0.0) ?: 0.0)
                .put("recapEnd", skips.optJSONObject("recap")?.optDouble("end", 0.0) ?: 0.0)
                .put("creditsStart", skips.optDouble("creditsStart", 0.0))
                .put("creditsFromEnd", skips.optDouble("creditsFromEnd", 0.0))
                .put("backdrop", backdrop)
                .put("thumbs", a.optString("thumbs"))
                .put("release", a.optString("release"))
            note = ""
            open(json.toString())
        } catch (e: Exception) {
            note = e.message ?: "That didn't play."
        }
    }

    // A live channel; list is the channels to move through with up and down.
    fun playLive(ch: Channel, list: List<Channel>) {
        live = ch
        liveList = list
        vod = null
        scope.launch {
            note = "Tuning in…"
            try {
                val p = api.livePlay(ch.id)
                val direct = p.optString("direct")
                val url = direct.ifEmpty { api.server + p.optString("url") }
                val channels = JSONArray()
                list.take(400).forEach { c ->
                    channels.put(JSONObject().put("id", c.id).put("num", if (c.num > 0) "${c.num}" else "").put("name", c.name)
                        .put("now", c.now?.title ?: "").put("until", c.now?.let { clock(it.stop) } ?: ""))
                }
                val json = JSONObject()
                    .put("url", url)
                    .put("alts", p.optJSONArray("directAlts") ?: JSONArray())
                    .put("title", (if (ch.num > 0) "${ch.num}  " else "") + ch.name)
                    .put("subtitle", ch.now?.let { "${it.title} · until ${clock(it.stop)}" } ?: "")
                    .put("live", true)
                    .put("id", ch.id)
                    .put("channels", channels)
                    .put("token", api.profileToken)
                    .put("accent", api.accent)
                note = ""
                open(json.toString())
            } catch (e: Exception) {
                note = e.message ?: "This channel won't play."
            }
        }
    }

    // How the player ended: reason (ended, next, other, nosound, error, back,
    // channel-up, channel-down, channel), where it was, and a message.
    fun ended(reason: String, position: Long, message: String) {
        live?.let { ch ->
            val list = liveList
            val i = list.indexOfFirst { it.id == ch.id }
            when (reason) {
                "channel-up", "channel-down" -> if (list.size > 1) {
                    val n = list[((if (i < 0) 0 else i) + (if (reason == "channel-up") 1 else -1) + list.size) % list.size]
                    playLive(n, list)
                }
                "channel" -> list.firstOrNull { it.id == message }?.let { playLive(it, list) }
                "error" -> note = "This channel won't play.${if (message.isNotEmpty()) " $message" else ""}"
            }
            return
        }
        val v = vod ?: return
        when (reason) {
            "ended", "next" -> if (v.kind == "tv") scope.launch {
                try {
                    val n = api.next(v.id, v.season, v.episode)
                    playTitle("tv", v.id, n.season, n.episode, 0.0)
                } catch (e: Exception) {
                    // the last episode
                }
            }
            "other" -> nextVersion(v, position.toDouble(), "")
            "nosound" -> if (v.silent < 2) {
                v.silent++
                nextVersion(v, position.toDouble(), "Version ${v.option} had no sound on this TV.")
            } else {
                note = "The versions found have sound this TV can't play. Try again later for others."
            }
            "error" -> nextVersion(v, position.toDouble(), "Version ${v.option} wouldn't play.")
        }
    }

    private fun nextVersion(v: Vod, position: Double, why: String) {
        if (v.option < v.options) {
            v.option++
            scope.launch {
                if (why.isNotEmpty()) note = why
                openVod(v, position)
            }
        } else {
            note = "None of the versions found would play."
        }
    }

    private fun clock(ms: Long): String =
        java.text.DateFormat.getTimeInstance(java.text.DateFormat.SHORT).format(java.util.Date(ms))
}
