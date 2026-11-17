package app.cue.tv

import android.content.Context
import android.os.Build
import android.os.Handler
import android.os.Looper
import android.provider.Settings
import android.webkit.CookieManager
import org.json.JSONObject
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.HttpURLConnection
import java.net.URL
import java.util.UUID

/**
 * Send to TV (Cue's side is screens.go): while the app is open, this TV keeps
 * a line to Cue so a phone signed in to the same Cue can see it, send it
 * something to play ("play") and work as its remote ("control"), and says
 * what it's playing ([report]). Events reach the main thread: a "play" goes
 * to the player when one is open (it hands over to Home), else to Home.
 */
object ScreenLink {
    /** Home: play this title ({kind, tmdbId, season, episode, from}). */
    @Volatile var onPlay: ((JSONObject) -> Unit)? = null
    /** The open player: a remote button ({action, pos}), or a new title to play. */
    @Volatile var onControl: ((JSONObject) -> Unit)? = null
    @Volatile var onPlayInPlayer: ((JSONObject) -> Unit)? = null

    private val main = Handler(Looper.getMainLooper())
    @Volatile private var running = false
    @Volatile private var conn: HttpURLConnection? = null
    private var server = ""
    private var token: () -> String = { "" }
    var id: String = ""
        private set
    private var name = "TV"

    fun start(ctx: Context, server: String, token: () -> String) {
        if (running) return
        val prefs = ctx.getSharedPreferences("cue", Context.MODE_PRIVATE)
        id = prefs.getString("screenId", null) ?: ("tv-" + UUID.randomUUID().toString().replace("-", "").take(20)).also {
            prefs.edit().putString("screenId", it).apply()
        }
        name = runCatching { Settings.Global.getString(ctx.contentResolver, "device_name") }.getOrNull()?.takeIf { it.isNotBlank() }
            ?: Build.MODEL ?: "TV"
        this.server = server.trimEnd('/')
        this.token = token
        running = true
        Thread { listen() }.apply { isDaemon = true }.start()
    }

    fun stop() {
        running = false
        Thread { runCatching { conn?.disconnect() } }.start()
    }

    private fun open(method: String, path: String, timeout: Int): HttpURLConnection {
        val c = URL("$server$path").openConnection() as HttpURLConnection
        c.requestMethod = method
        c.connectTimeout = 8000
        c.readTimeout = timeout
        CookieManager.getInstance().getCookie(server)?.let { c.setRequestProperty("Cookie", it) }
        token().takeIf { it.isNotEmpty() }?.let { c.setRequestProperty("X-Cue-Profile", it) }
        return c
    }

    private fun listen() {
        while (running) {
            try {
                val q = "id=$id&name=" + java.net.URLEncoder.encode(name, "UTF-8")
                val c = open("GET", "/api/screens/listen?$q", 60_000)
                conn = c
                if (c.responseCode == 200) {
                    BufferedReader(InputStreamReader(c.inputStream)).use { r ->
                        var event = ""
                        while (running) {
                            val line = r.readLine() ?: break
                            when {
                                line.startsWith("event: ") -> event = line.removePrefix("event: ")
                                line.startsWith("data: ") -> {
                                    val data = runCatching { JSONObject(line.removePrefix("data: ")) }.getOrNull() ?: continue
                                    val ev = event
                                    main.post { dispatch(ev, data) }
                                }
                            }
                        }
                    }
                }
                c.disconnect()
            } catch (_: Exception) {
                // Dropped (or not signed in yet): try again shortly.
            }
            if (running) Thread.sleep(4000)
        }
    }

    private fun dispatch(event: String, data: JSONObject) {
        when (event) {
            "play" -> (onPlayInPlayer ?: onPlay)?.invoke(data)
            "control" -> onControl?.invoke(data)
        }
    }

    /** Tells Cue what this TV is showing (an empty kind: nothing). Any thread. */
    fun report(state: JSONObject) {
        if (!running || id.isEmpty()) return
        Thread {
            runCatching {
                val c = open("POST", "/api/screens/$id/state", 8000)
                c.doOutput = true
                c.setRequestProperty("Content-Type", "application/json")
                c.outputStream.use { it.write(state.toString().toByteArray()) }
                c.responseCode
                c.disconnect()
            }
        }.start()
    }
}
