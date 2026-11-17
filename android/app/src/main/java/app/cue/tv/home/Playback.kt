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

    // The episode that comes next, offered full screen when one ends.
    data class UpNext(val showId: Int, val show: String, val backdrop: String?, val episode: Episode)
    var upNext by mutableStateOf<UpNext?>(null)
        private set

    // The next episode while its version is found: its picture and name fill
    // the screen (instead of the menus showing) until the player opens.
    // episode is null for the moment it's still being looked up.
    data class Starting(val showId: Int, val show: String, val backdrop: String?, val episode: Episode?, val failed: Boolean = false)
    var starting by mutableStateOf<Starting?>(null)
        private set
    private var startJob: kotlinx.coroutines.Job? = null

    fun cancelStarting() {
        startJob?.cancel()
        starting = null
        note = ""
    }

    fun playUpNext() {
        val u = upNext ?: return
        upNext = null
        starting = Starting(u.showId, u.show, u.backdrop, u.episode)
        playTitle("tv", u.showId, u.episode.season, u.episode.episode, 0.0, party = upNextParty)
    }

    fun dismissUpNext() {
        upNext = null
    }

    private data class Vod(val kind: String, val id: Int, val season: Int, val episode: Int, var option: Int = 1, var options: Int = 1, var silent: Int = 0, var party: String = "")

    private var vod: Vod? = null
    private var upNextParty = ""
    private var live: Channel? = null
    private var liveList: List<Channel> = emptyList()

    fun play(card: Tile) = playTitle(card.kind, card.tmdbId, card.season ?: 1, card.episode ?: 1)

    // party: a watch party's code (joined from Home, or carried on to the next episode).
    fun playTitle(kind: String, id: Int, season: Int, episode: Int, startSec: Double? = null, party: String = "") {
        val v = Vod(kind, id, if (kind == "tv") season else 0, if (kind == "tv") episode else 0, party = party)
        vod = v
        live = null
        startJob = scope.launch { openVod(v, startSec) }
    }

    fun clearNote() {
        note = ""
    }

    private suspend fun openVod(v: Vod, startSec: Double?) {
        note = if (v.option > 1) "Trying version ${v.option}…" else "Finding a version…"
        try {
            val path = if (v.kind == "tv") "/api/play/tmdb/tv/${v.id}/${v.season}/${v.episode}" else "/api/play/tmdb/movie/${v.id}"
            // In a party: the party's version, the same for everyone.
            val a = api.obj("$path?option=${v.option}" + (if (v.party.isNotEmpty()) "&party=${v.party}" else ""))
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
            if (startSec == null && v.party.isNotEmpty()) {
                // A party: start where it is now.
                start = runCatching { api.obj("/api/party/${v.party}").optDouble("pos", 0.0) }.getOrDefault(0.0)
            } else if (startSec == null) {
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
                .put("party", v.party)
            note = ""
            starting = null
            open(json.toString())
        } catch (e: kotlinx.coroutines.CancellationException) {
            throw e
        } catch (e: Exception) {
            note = e.message ?: "That didn't play."
            starting = starting?.copy(failed = true)
        }
    }

    // A live channel; list is the channels to move through with up and down.
    fun playLive(ch: Channel, list: List<Channel>) {
        live = ch
        liveList = list
        vod = null
        starting = null
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
        // Sent from a phone while something played: "kind|id|season|episode".
        if (reason == "remote-play") {
            message.split('|').takeIf { it.size == 4 }?.let { p ->
                val id = p[1].toIntOrNull() ?: return
                playTitle(p[0], id, p[2].toIntOrNull() ?: 0, p[3].toIntOrNull() ?: 0)
            }
            return
        }
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
            // Next pressed: straight on. The episode ended by itself: the Up
            // next screen, which counts down and plays it (or Back stays).
            // The party moved to another episode, or this TV moves to the
            // party's version: "CODE|id|season|episode" (and "|kind" for a film).
            "party" -> message.split('|').takeIf { it.size >= 4 }?.let { p ->
                playTitle(p.getOrNull(4) ?: "tv", p[1].toIntOrNull() ?: v.id, p[2].toIntOrNull() ?: 1, p[3].toIntOrNull() ?: 1, null, party = p[0])
            }
            "next" -> if (v.kind == "tv") {
                starting = Starting(v.id, "", null, null)
                startJob = scope.launch {
                    try {
                        val n = api.next(v.id, v.season, v.episode)
                        val show = runCatching { api.title("tv", v.id).card }.getOrNull()
                        starting = Starting(v.id, show?.title ?: "", show?.backdrop, n)
                        playTitle("tv", v.id, n.season, n.episode, 0.0, party = message)
                    } catch (e: kotlinx.coroutines.CancellationException) {
                        throw e
                    } catch (e: Exception) {
                        starting = null // the last episode
                    }
                }
            }
            "ended" -> if (v.kind == "tv") {
                upNextParty = message // a party carries on to the next episode
                // Covered while the next one is looked up, then the Up next screen.
                starting = Starting(v.id, "", null, null)
                startJob = scope.launch {
                    try {
                        val n = api.next(v.id, v.season, v.episode)
                        val show = runCatching { api.title("tv", v.id).card }.getOrNull()
                        upNext = UpNext(v.id, show?.title ?: "", show?.backdrop, n)
                    } catch (e: kotlinx.coroutines.CancellationException) {
                        throw e
                    } catch (e: Exception) {
                        // the last episode
                    } finally {
                        starting = null
                    }
                }
            }
            "other" -> nextVersion(v, position.toDouble(), "")
            // The wrong video for this title: never this version again; the
            // same number is now the next one.
            "wrong" -> startJob = scope.launch {
                note = "Got it: that version won't be used for this again. Trying the next one…"
                runCatching {
                    api.request("POST", "/api/play/wrong", JSONObject()
                        .put("kind", v.kind).put("tmdbId", v.id).put("season", v.season).put("episode", v.episode).put("release", message))
                }
                openVod(v, 0.0)
            }
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
            startJob = scope.launch {
                if (why.isNotEmpty()) note = why
                openVod(v, position)
            }
        } else {
            note = "None of the versions found would play."
            starting = starting?.copy(failed = true)
        }
    }

    private fun clock(ms: Long): String =
        java.text.DateFormat.getTimeInstance(java.text.DateFormat.SHORT).format(java.util.Date(ms))
}
