package app.cue.tv

import android.os.Handler
import android.os.Looper
import android.webkit.CookieManager
import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URL
import java.util.UUID

/**
 * A watch party, from the TV (Cue's side is party.go): joining, the party's
 * updates as they happen (Server-Sent Events, read on a background thread,
 * handed to the main thread), and sending what this TV does. Anyone's play,
 * pause or jump comes back as a "state"; chat lines, who-did-what and
 * reactions as a "msg".
 */
class PartyLink(
    private val server: String,
    private val token: String,
    code: String,
    private val onState: (JSONObject) -> Unit,
    private val onMsg: (JSONObject) -> Unit,
    private val onProblem: (String) -> Unit,
) {
    val client: String = "tv-" + UUID.randomUUID().toString().replace("-", "").take(20)
    @Volatile var code: String = code
        private set
    private val main = Handler(Looper.getMainLooper())
    @Volatile private var closed = false
    @Volatile private var conn: HttpURLConnection? = null

    private fun open(method: String, path: String, timeout: Int = 8000): HttpURLConnection {
        val c = URL("$server$path").openConnection() as HttpURLConnection
        c.requestMethod = method
        c.connectTimeout = 8000
        c.readTimeout = timeout
        CookieManager.getInstance().getCookie(server)?.let { c.setRequestProperty("Cookie", it) }
        if (token.isNotEmpty()) c.setRequestProperty("X-Cue-Profile", token)
        return c
    }

    private fun post(path: String, body: JSONObject): JSONObject? {
        val c = open("POST", path)
        c.doOutput = true
        c.setRequestProperty("Content-Type", "application/json")
        c.outputStream.use { it.write(body.toString().toByteArray()) }
        val ok = c.responseCode in 200..299
        val text = (if (ok) c.inputStream else c.errorStream)?.bufferedReader()?.use { it.readText() } ?: ""
        c.disconnect()
        val json = runCatching { JSONObject(text) }.getOrNull()
        if (!ok) throw IllegalStateException(json?.optString("error")?.takeIf { it.isNotEmpty() } ?: "Cue answered ${c.responseCode}.")
        return json
    }

    /** Starts a party for what's playing (this TV its first member), then listens like [start]. */
    fun create(title: JSONObject, posSec: Double, playing: Boolean, joined: (JSONObject) -> Unit) {
        Thread {
            try {
                val body = JSONObject(title.toString()).put("pos", posSec).put("playing", playing).put("client", client)
                code = post("/api/party", body)?.optString("code")?.takeIf { it.isNotEmpty() } ?: throw IllegalStateException("Couldn't start a party.")
            } catch (e: Exception) {
                main.post { onProblem(e.message ?: "Couldn't start a party.") }
                return@Thread
            }
            start(joined)
        }.start()
    }

    /** Joins, then listens until [close]. [joined] gets the party as it is. */
    fun start(joined: (JSONObject) -> Unit) {
        Thread {
            try {
                val r = post("/api/party/$code/join", JSONObject().put("client", client)) ?: JSONObject()
                val party = r.optJSONObject("party") ?: JSONObject()
                main.post { joined(party) }
                val msgs = r.optJSONArray("msgs")
                if (msgs != null) for (i in 0 until msgs.length()) msgs.optJSONObject(i)?.let { m -> main.post { onMsg(m) } }
            } catch (e: Exception) {
                main.post { onProblem(e.message ?: "Couldn't join the party.") }
                return@Thread
            }
            listen()
        }.start()
    }

    private fun listen() {
        while (!closed) {
            try {
                val c = open("GET", "/api/party/$code/events?client=$client", timeout = 60_000)
                conn = c
                if (c.responseCode == 404) {
                    main.post { onProblem("The watch party has ended.") }
                    return
                }
                BufferedReader(InputStreamReader(c.inputStream)).use { r ->
                    var event = ""
                    while (!closed) {
                        val line = r.readLine() ?: break
                        when {
                            line.startsWith("event: ") -> event = line.removePrefix("event: ")
                            line.startsWith("data: ") -> {
                                val data = runCatching { JSONObject(line.removePrefix("data: ")) }.getOrNull() ?: continue
                                if (event == "state") main.post { onState(data) } else if (event == "msg") main.post { onMsg(data) }
                            }
                        }
                    }
                }
            } catch (_: Exception) {
                // Dropped: try again in a moment.
            }
            if (!closed) Thread.sleep(2000)
        }
    }

    /** play, pause or seek, at [posSec]. */
    fun send(action: String, posSec: Double) = fire("/api/party/$code/state", JSONObject().put("client", client).put("action", action).put("pos", posSec))

    /** This TV moved on to another episode: the party follows. Call off the main thread. */
    fun sendEpisode(title: JSONObject) {
        post("/api/party/$code/state", JSONObject().put("client", client).put("action", "episode").put("pos", 0).put("title", title))
    }

    /** The party's version (its release name) of what [playPath] plays, so the TV that started the party can move to it. Call off the main thread. */
    fun partyRelease(playPath: String): String {
        val c = open("GET", "$playPath?option=1&party=$code", timeout = 30_000)
        val text = if (c.responseCode in 200..299) c.inputStream.bufferedReader().use { it.readText() } else ""
        c.disconnect()
        return runCatching { JSONObject(text).optString("release") }.getOrDefault("")
    }

    fun react(emoji: String) = fire("/api/party/$code/react", JSONObject().put("client", client).put("emoji", emoji))

    private fun fire(path: String, body: JSONObject) {
        Thread { runCatching { post(path, body) } }.start()
    }

    fun close() {
        if (closed) return
        closed = true
        Thread {
            runCatching { post("/api/party/$code/leave", JSONObject().put("client", client)) }
            runCatching { conn?.disconnect() }
        }.start()
    }

    companion object {
        val REACTIONS = listOf("😂", "😮", "😍", "😢", "👏", "🔥", "😱", "❤️")
    }
}
