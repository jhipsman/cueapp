package app.cue.tv

import android.webkit.CookieManager
import app.cue.tv.home.Show
import org.json.JSONArray
import java.net.HttpURLConnection
import java.net.URL

// Live TV reminders on the TV: the shows this profile asked to be told about
// (set here, on the web or on a phone). Checked every half minute by the
// app's screens and by the player: a show starting says so, and one set to
// "Switch to it" changes to its channel once it has started.
object LiveReminders {
    data class Due(val channelId: String, val title: String, val channel: String, val switch: Boolean, val started: Boolean) {
        val key get() = "$channelId|$title"
    }

    // What's been said or done for each reminder on this TV (until the app restarts).
    private val announced = HashSet<String>()
    private val switched = HashSet<String>()

    /** The reminder to act on now, if any (blocking: call it off the main thread). */
    fun check(server: String, profileToken: String): Due? {
        val list = runCatching { fetch(server, profileToken) }.getOrNull() ?: return null
        val now = System.currentTimeMillis()
        for (i in 0 until list.length()) {
            val o = list.optJSONObject(i) ?: continue
            val start = Show.parseTime(o.optString("start"))
            val stop = Show.parseTime(o.optString("stop"))
            if (start == 0L || now < start - 60_000 || now >= stop || now - start > 30 * 60_000) continue
            val d = Due(o.optString("channelId"), o.optString("title"), o.optString("channel"), o.optBoolean("switch"), now >= start)
            synchronized(this) {
                if (d.switch && d.started && d.key !in switched) {
                    switched += d.key
                    announced += d.key
                    return d
                }
                if (d.key !in announced) {
                    announced += d.key
                    return d
                }
            }
        }
        return null
    }

    private fun fetch(server: String, token: String): JSONArray {
        val c = URL("$server/api/live/reminders").openConnection() as HttpURLConnection
        try {
            c.connectTimeout = 8000
            c.readTimeout = 10000
            CookieManager.getInstance().getCookie(server)?.let { c.setRequestProperty("Cookie", it) }
            if (token.isNotEmpty()) c.setRequestProperty("X-Cue-Profile", token)
            if (c.responseCode != 200) return JSONArray()
            return JSONArray(c.inputStream.bufferedReader().use { it.readText() })
        } finally {
            c.disconnect()
        }
    }

    /** What to say for a reminder that isn't switching. */
    fun words(d: Due): String =
        (if (d.started) "Starting now: " else "Starting in a minute: ") + d.title + (if (d.channel.isNotEmpty()) " on ${d.channel}" else "")
}
