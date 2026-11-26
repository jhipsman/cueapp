package app.cue.tv

import android.app.Activity
import android.app.AlertDialog
import android.content.Intent
import android.graphics.Color
import android.graphics.drawable.GradientDrawable
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.util.TypedValue
import android.view.Gravity
import android.view.KeyEvent
import android.view.View
import android.view.ViewGroup
import android.webkit.CookieManager
import android.widget.FrameLayout
import android.widget.LinearLayout
import android.widget.TextView
import androidx.annotation.OptIn
import androidx.media3.common.MediaItem
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.ui.AspectRatioFrameLayout
import androidx.media3.ui.PlayerView
import org.json.JSONArray
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL

// Multiview: two or four live channels at once. The remote moves between
// them; the one it's on has the sound and a lit border. OK on one offers:
// watch it full screen, change its channel, take it out, two or four. Back
// goes back to the channel that was playing.
//
// Each channel is its own connection to the provider, and its own video
// decoder on the TV: a channel the provider or the TV won't add says so.
@OptIn(UnstableApi::class)
class MultiviewActivity : Activity() {
    private val main = Handler(Looper.getMainLooper())
    private var server = ""
    private var token = ""
    private var accent = 0xFF3DD6C6.toInt()
    private var channels = JSONArray()
    private var dp = 1f
    private var size = 2
    private val slots = arrayOf("", "", "", "") // channel ids
    private val tiles = arrayOfNulls<Tile>(4)
    private lateinit var grid: LinearLayout
    private var heard = 0

    private inner class Tile(val index: Int) {
        val root = FrameLayout(this@MultiviewActivity)
        val video = PlayerView(this@MultiviewActivity).apply {
            useController = false
            resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FIT
            setShutterBackgroundColor(Color.BLACK)
        }
        val label = TextView(this@MultiviewActivity)
        val note = TextView(this@MultiviewActivity)
        val frame = View(this@MultiviewActivity)
        var player: ExoPlayer? = null
        var channel = ""
        val alts = ArrayDeque<String>()
        var started = false

        init {
            root.setBackgroundColor(0xFF111216.toInt())
            root.isFocusable = true
            root.addView(video, FrameLayout.LayoutParams(MATCH, MATCH))
            note.setTextColor(0xFFC7C8CC.toInt())
            note.textSize(15f)
            note.gravity = Gravity.CENTER
            note.setPadding(px(24), 0, px(24), 0)
            root.addView(note, FrameLayout.LayoutParams(MATCH, MATCH))
            label.setTextColor(Color.WHITE)
            label.textSize(14f)
            label.typeface = Fonts.text(this@MultiviewActivity, 600)
            label.maxLines = 2
            label.setPadding(px(16), px(26), px(16), px(10))
            label.background = GradientDrawable(GradientDrawable.Orientation.BOTTOM_TOP, intArrayOf(0xCC000000.toInt(), 0x00000000))
            root.addView(label, FrameLayout.LayoutParams(MATCH, ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.BOTTOM))
            root.addView(frame, FrameLayout.LayoutParams(MATCH, MATCH))
            root.setOnFocusChangeListener { _, has -> if (has) hear(index) }
            root.setOnClickListener { tileMenu(index) }
        }

        fun show(id: String) {
            stop()
            channel = id
            val c = channelById(id)
            if (c == null) {
                label.text = ""
                note.text = "＋\nAdd a channel"
                return
            }
            val now = c.optString("now")
            label.text = (if (c.optString("num").isNotEmpty()) c.optString("num") + "  " else "") + c.optString("name") +
                (if (now.isNotEmpty()) "\n$now" else "")
            note.text = "Tuning in…"
            Thread {
                val answer = fetch("/api/live/play/" + java.net.URLEncoder.encode(id, "UTF-8").replace("+", "%20"))
                runOnUiThread {
                    if (isFinishing || channel != id) return@runOnUiThread
                    val direct = answer?.optString("direct").orEmpty()
                    val url = direct.ifEmpty { answer?.optString("url")?.takeIf { it.isNotEmpty() }?.let { server + it }.orEmpty() }
                    if (url.isEmpty()) {
                        note.text = "${c.optString("name")} won't play right now."
                        return@runOnUiThread
                    }
                    alts.clear()
                    answer?.optJSONArray("directAlts")?.let { l -> for (i in 0 until l.length()) l.optString(i).takeIf { it.isNotEmpty() }?.let { alts.addLast(it) } }
                    play(url)
                }
            }.start()
        }

        private fun play(url: String) {
            val http = DefaultHttpDataSource.Factory()
                .setAllowCrossProtocolRedirects(true)
                .setConnectTimeoutMs(15000)
                .setReadTimeoutMs(30000)
                .setUserAgent("CueTV/1")
            val p = ExoPlayer.Builder(this@MultiviewActivity)
                .setMediaSourceFactory(DefaultMediaSourceFactory(http))
                .build()
            player = p
            started = false
            video.player = p
            p.volume = if (heard == index) 1f else 0f
            p.addListener(object : Player.Listener {
                override fun onPlaybackStateChanged(state: Int) {
                    if (state == Player.STATE_READY) {
                        started = true
                        note.text = ""
                    }
                }

                override fun onPlayerError(error: PlaybackException) {
                    if (!started && alts.isNotEmpty()) {
                        p.setMediaItem(MediaItem.fromUri(alts.removeFirst()))
                        p.prepare()
                        return
                    }
                    if (error.errorCode == PlaybackException.ERROR_CODE_BEHIND_LIVE_WINDOW) {
                        p.seekToDefaultPosition()
                        p.prepare()
                        return
                    }
                    note.text = "This one won't play alongside the others: your provider or this TV may allow fewer at once."
                }
            })
            p.setMediaItem(MediaItem.fromUri(url))
            p.prepare()
            p.playWhenReady = true
        }

        fun stop() {
            video.player = null
            player?.release()
            player = null
        }

        fun mark(on: Boolean) {
            frame.background = GradientDrawable().apply {
                setColor(Color.TRANSPARENT)
                setStroke(px(if (on) 4 else 1), if (on) accent else 0x22FFFFFF)
            }
            player?.volume = if (on) 1f else 0f
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        dp = resources.displayMetrics.density
        server = intent.getStringExtra(EXTRA_SERVER).orEmpty()
        val info = runCatching { JSONObject(intent.getStringExtra(EXTRA_JSON).orEmpty()) }.getOrDefault(JSONObject())
        token = info.optString("token")
        channels = info.optJSONArray("channels") ?: JSONArray()
        runCatching { Color.parseColor(info.optString("accent")) }.getOrNull()?.let { accent = it }
        // The channel that was playing, and the next one in the list.
        val current = info.optString("id")
        slots[0] = current
        val i = (0 until channels.length()).indexOfFirst { channels.optJSONObject(it)?.optString("id") == current }
        if (channels.length() > 1) slots[1] = channels.optJSONObject((i + 1).mod(channels.length()))?.optString("id").orEmpty()

        val root = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setBackgroundColor(Color.BLACK)
        }
        root.addView(TextView(this).apply {
            text = "Multiview   ·   the channel you're on has the sound   ·   OK for more"
            setTextColor(0xFFC7C8CC.toInt())
            textSize(13f)
            typeface = Fonts.text(this@MultiviewActivity, 500)
            setPadding(px(24), px(12), px(24), px(10))
        })
        grid = LinearLayout(this).apply { orientation = LinearLayout.VERTICAL }
        root.addView(grid, LinearLayout.LayoutParams(MATCH, 0, 1f))
        setContentView(root)
        for (k in 0 until 4) tiles[k] = Tile(k)
        layoutTiles()
        for (k in 0 until size) tiles[k]?.show(slots[k])
        main.post { tiles[0]?.root?.requestFocus() }
    }

    // Two side by side, or four in a square.
    private fun layoutTiles() {
        grid.removeAllViews()
        tiles.forEach { (it?.root?.parent as? ViewGroup)?.removeView(it?.root) }
        val rows = if (size == 4) 2 else 1
        for (r in 0 until rows) {
            val row = LinearLayout(this).apply { orientation = LinearLayout.HORIZONTAL }
            for (c in 0 until 2) {
                val t = tiles[r * 2 + c] ?: continue
                row.addView(t.root, LinearLayout.LayoutParams(0, MATCH, 1f).apply { setMargins(px(3), px(3), px(3), px(3)) })
            }
            grid.addView(row, if (size == 4) LinearLayout.LayoutParams(MATCH, 0, 1f) else LinearLayout.LayoutParams(MATCH, MATCH))
        }
        if (size == 2) {
            // Two 16:9 pictures side by side, in the middle of the screen.
            grid.gravity = Gravity.CENTER_VERTICAL
            (grid.getChildAt(0)?.layoutParams as? LinearLayout.LayoutParams)?.height = (resources.displayMetrics.widthPixels / 2 * 9 / 16)
        }
        for (k in 0 until 4) tiles[k]?.mark(k == heard)
    }

    private fun hear(index: Int) {
        heard = index
        for (k in 0 until 4) tiles[k]?.mark(k == index)
    }

    private fun tileMenu(index: Int) {
        val t = tiles[index] ?: return
        if (t.channel.isEmpty() || channelById(t.channel) == null) {
            pickChannel(index)
            return
        }
        val items = mutableListOf<Pair<String, () -> Unit>>()
        items += "Watch it full screen" to { done(t.channel) }
        items += "Change channel" to { pickChannel(index) }
        items += "Take it out" to {
            slots[index] = ""
            t.show("")
        }
        items += (if (size == 2) "Show four" else "Show two") to { setSize(if (size == 2) 4 else 2) }
        AlertDialog.Builder(this)
            .setItems(items.map { it.first }.toTypedArray()) { _, w -> items[w].second() }
            .show()
    }

    private fun setSize(n: Int) {
        if (n == size) return
        if (n == 2) for (k in 2 until 4) tiles[k]?.stop()
        size = n
        if (heard >= n) heard = 0
        layoutTiles()
        if (n == 4) for (k in 2 until 4) tiles[k]?.show(slots[k])
        main.post { tiles[heard]?.root?.requestFocus() }
    }

    private fun pickChannel(index: Int) {
        val list = (0 until channels.length()).mapNotNull { channels.optJSONObject(it) }.filter { it.optString("id") !in slots }
        if (list.isEmpty()) return
        val names = list.map { c ->
            val now = c.optString("now")
            (if (c.optString("num").isNotEmpty()) c.optString("num") + "  " else "") + c.optString("name") + (if (now.isNotEmpty()) "  ·  $now" else "")
        }
        AlertDialog.Builder(this)
            .setTitle("Pick a channel")
            .setItems(names.toTypedArray()) { _, w ->
                slots[index] = list[w].optString("id")
                tiles[index]?.show(slots[index])
                hear(index)
            }
            .show()
    }

    private fun done(channel: String) {
        setResult(RESULT_OK, Intent().putExtra(RESULT_CHANNEL, channel))
        finish()
    }

    override fun dispatchKeyEvent(event: KeyEvent): Boolean {
        if (event.action == KeyEvent.ACTION_DOWN && event.keyCode == KeyEvent.KEYCODE_MENU) {
            tileMenu(heard)
            return true
        }
        return super.dispatchKeyEvent(event)
    }

    private fun channelById(id: String): JSONObject? =
        if (id.isEmpty()) null else (0 until channels.length()).mapNotNull { channels.optJSONObject(it) }.firstOrNull { it.optString("id") == id }

    // fetch GETs a Cue address as this TV (blocking: off the main thread).
    private fun fetch(path: String): JSONObject? = try {
        val h = URL(server + path).openConnection() as HttpURLConnection
        h.connectTimeout = 8000
        h.readTimeout = 15000
        h.setRequestProperty("User-Agent", "CueTV/1 (native)")
        CookieManager.getInstance().getCookie(server)?.let { h.setRequestProperty("Cookie", it) }
        if (token.isNotEmpty()) h.setRequestProperty("X-Cue-Profile", token)
        val ok = h.responseCode == 200
        val body = if (ok) h.inputStream.bufferedReader().use { it.readText() } else ""
        h.disconnect()
        if (ok) JSONObject(body) else null
    } catch (_: Exception) {
        null
    }

    override fun onStop() {
        super.onStop()
        // Connections and decoders back as soon as it's out of sight.
        tiles.forEach { it?.stop() }
        if (!isFinishing) finish()
    }

    override fun onDestroy() {
        main.removeCallbacksAndMessages(null)
        tiles.forEach { it?.stop() }
        super.onDestroy()
    }

    private fun px(v: Int) = (v * dp).toInt()
    private fun TextView.textSize(units: Float) = setTextSize(TypedValue.COMPLEX_UNIT_PX, units * dp)

    companion object {
        const val EXTRA_JSON = "json"
        const val EXTRA_SERVER = "server"
        const val RESULT_CHANNEL = "channel"
        private const val MATCH = ViewGroup.LayoutParams.MATCH_PARENT
    }
}
