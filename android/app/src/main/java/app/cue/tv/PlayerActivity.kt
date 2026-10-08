package app.cue.tv

import android.app.Activity
import android.content.Intent
import android.content.res.ColorStateList
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.graphics.drawable.StateListDrawable
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.util.TypedValue
import android.view.Gravity
import android.view.KeyEvent
import android.view.View
import android.view.ViewGroup
import android.view.WindowManager
import android.webkit.CookieManager
import android.widget.FrameLayout
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.ProgressBar
import android.widget.TextView
import androidx.annotation.OptIn
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.exoplayer.DefaultRenderersFactory
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.ui.AspectRatioFrameLayout
import androidx.media3.ui.PlayerView
import androidx.media3.ui.TrackSelectionDialogBuilder
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL

// The player: ExoPlayer, full screen, with Cue's own controls on top. It
// plays MKV and MP4, H.264, HEVC and AV1 as far as the TV can, and sends
// Dolby and DTS sound to the TV or receiver as it is when they take it. It
// resumes where Watch said, saves where you are to Cue every 15 seconds and
// on the way out, and tells Watch how it ended (the end, Next episode, an
// error, or Back) so Watch can play the next episode or another version.
// For a live channel ("live": true) it saves nothing, shows LIVE instead of
// the time bar, and up and down change channel (Watch picks the next one).
//
// The remote, with the controls hidden: OK pauses, left and right skip 10
// seconds, up or down show the controls. With them showing, the arrows move
// between the time bar and the buttons, and Back hides them.
@OptIn(UnstableApi::class)
class PlayerActivity : Activity() {
    private lateinit var player: ExoPlayer
    private lateinit var info: JSONObject
    private var server = ""
    private val main = Handler(Looper.getMainLooper())
    private var finished = false
    private var live = false
    private var catchup = false // a past show from the provider's recordings: seekable, nothing saved
    private var isShow = false

    private lateinit var controls: View
    private lateinit var timeBar: TimeBar
    private lateinit var timeLeft: TextView
    private lateinit var playPause: TextView
    private lateinit var spinner: ProgressBar
    private lateinit var pauseBadge: ImageView
    private var nextButton: TextView? = null
    private var nextOffered = false

    private val dp by lazy { resources.displayMetrics.density }
    private fun px(v: Int) = (v * dp).toInt()
    private val teal by lazy { getColor(R.color.cue_teal) }

    private val saver = object : Runnable {
        override fun run() {
            if (player.isPlaying) saveProgress()
            main.postDelayed(this, SAVE_EVERY_MS)
        }
    }
    private val ticker = object : Runnable {
        override fun run() {
            refreshTime()
            main.postDelayed(this, 300)
        }
    }
    private val hider = Runnable { if (player.isPlaying) hideControls() }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        info = JSONObject(intent.getStringExtra(EXTRA_JSON) ?: "{}")
        server = intent.getStringExtra(EXTRA_SERVER) ?: ""
        live = info.optBoolean("live")
        catchup = info.optBoolean("catchup")
        isShow = info.optString("kind") == "tv"

        val renderers = DefaultRenderersFactory(this)
            .setEnableDecoderFallback(true)
            .setExtensionRendererMode(DefaultRenderersFactory.EXTENSION_RENDERER_MODE_PREFER)
        // Debrid links often redirect, sometimes from http to https.
        val http = DefaultHttpDataSource.Factory()
            .setAllowCrossProtocolRedirects(true)
            .setConnectTimeoutMs(15000)
            .setReadTimeoutMs(30000)
            .setUserAgent("CueTV/1")
        player = ExoPlayer.Builder(this, renderers)
            .setMediaSourceFactory(DefaultMediaSourceFactory(http))
            .setSeekBackIncrementMs(10_000)
            .setSeekForwardIncrementMs(10_000)
            .build()
        player.trackSelectionParameters = player.trackSelectionParameters.buildUpon()
            .setPreferredAudioLanguage("en")
            .setPreferredTextLanguage(null)
            .build()

        val video = PlayerView(this).apply {
            player = this@PlayerActivity.player
            useController = false
            resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FIT
            setShutterBackgroundColor(Color.BLACK)
            setKeepContentOnPlayerReset(true)
        }
        spinner = ProgressBar(this).apply {
            isIndeterminate = true
            indeterminateTintList = ColorStateList.valueOf(teal)
        }
        pauseBadge = ImageView(this).apply {
            setImageResource(R.drawable.ic_cue_pause)
            setPadding(px(22), px(22), px(22), px(22))
            background = GradientDrawable().apply {
                shape = GradientDrawable.OVAL
                setColor(0x88000000.toInt())
            }
            visibility = View.GONE
        }
        controls = buildControls()

        val root = FrameLayout(this).apply {
            setBackgroundColor(Color.BLACK)
            addView(video, FrameLayout.LayoutParams(MATCH, MATCH))
            addView(spinner, FrameLayout.LayoutParams(px(64), px(64), Gravity.CENTER))
            addView(pauseBadge, FrameLayout.LayoutParams(px(110), px(110), Gravity.CENTER))
            addView(controls, FrameLayout.LayoutParams(MATCH, MATCH))
        }
        setContentView(root)

        player.addListener(object : Player.Listener {
            override fun onPlaybackStateChanged(state: Int) {
                spinner.visibility = if (state == Player.STATE_BUFFERING) View.VISIBLE else View.GONE
                if (state == Player.STATE_ENDED) {
                    if (live) end("error", "The channel stopped.") else end("ended")
                }
            }

            override fun onIsPlayingChanged(isPlaying: Boolean) {
                playPause.setCompoundDrawablesRelativeWithIntrinsicBounds(
                    if (isPlaying) R.drawable.ic_cue_pause else R.drawable.ic_cue_play, 0, 0, 0,
                )
                val paused = !isPlaying && player.playbackState == Player.STATE_READY
                pauseBadge.visibility = if (paused) View.VISIBLE else View.GONE
                if (paused) showControls() else scheduleHide()
            }

            override fun onPlayerError(error: PlaybackException) {
                if (live && error.errorCode == PlaybackException.ERROR_CODE_BEHIND_LIVE_WINDOW) {
                    player.seekToDefaultPosition() // fell behind: back to live
                    player.prepare()
                    return
                }
                end("error", error.errorCodeName)
            }
        })

        player.setMediaItem(MediaItem.fromUri(info.optString("url")))
        val start = (info.optDouble("startSec", 0.0) * 1000).toLong()
        if (start > 0 && !live) player.seekTo(start)
        player.prepare()
        player.playWhenReady = true
        if (!live && !catchup) main.postDelayed(saver, SAVE_EVERY_MS)
        main.post(ticker)
        showControls()
    }

    // ---- The controls ----

    private fun buildControls(): View {
        val title = TextView(this).apply {
            text = info.optString("title")
            setTextColor(Color.WHITE)
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 26f)
            typeface = Typeface.create("sans-serif-medium", Typeface.BOLD)
            setShadowLayer(8f, 0f, 2f, Color.BLACK)
        }
        val sub = TextView(this).apply {
            text = info.optString("subtitle")
            setTextColor(0xFFC7C8CC.toInt())
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 15f)
            setPadding(0, px(4), 0, 0)
            visibility = if (text.isNullOrEmpty()) View.GONE else View.VISIBLE
        }
        val top = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(px(48), px(32), px(48), px(56))
            background = GradientDrawable(GradientDrawable.Orientation.TOP_BOTTOM, intArrayOf(0xCC000000.toInt(), 0x00000000))
            addView(title)
            addView(sub)
        }

        timeBar = TimeBar(this).apply {
            onSeek = { to ->
                player.seekTo(to)
                showControls()
            }
        }
        timeLeft = TextView(this).apply {
            setTextColor(Color.WHITE)
            setTextSize(TypedValue.COMPLEX_UNIT_SP, 15f)
            typeface = Typeface.MONOSPACE
        }
        val timeRow = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
            if (live) {
                addView(liveBadge())
            } else {
                addView(timeBar, LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f))
                addView(timeLeft, LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT).apply { marginStart = px(12) })
            }
        }

        playPause = pill("", R.drawable.ic_cue_pause) { togglePlay() }
        val buttons = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
            setPadding(0, px(14), 0, 0)
            addView(playPause)
            if (!live) {
                addView(pill("10", R.drawable.ic_cue_replay) { seekBy(-10_000) })
                addView(pill("10", R.drawable.ic_cue_forward) { seekBy(10_000) })
            }
            addView(View(context), LinearLayout.LayoutParams(0, 1, 1f))
            if (isShow && !live && !catchup) {
                nextButton = pill("Next episode", R.drawable.ic_cue_next) { end("next") }.also { addView(it) }
            }
            addView(pill("Audio", R.drawable.ic_cue_audio) { chooseTrack(C.TRACK_TYPE_AUDIO, "Audio") })
            addView(pill("Subtitles", R.drawable.ic_cue_subtitles) { chooseTrack(C.TRACK_TYPE_TEXT, "Subtitles") })
        }
        val bottom = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(px(36), px(80), px(36), px(28))
            background = GradientDrawable(GradientDrawable.Orientation.BOTTOM_TOP, intArrayOf(0xE6000000.toInt(), 0x00000000))
            addView(timeRow)
            addView(buttons)
        }

        return FrameLayout(this).apply {
            addView(top, FrameLayout.LayoutParams(MATCH, ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.TOP))
            addView(bottom, FrameLayout.LayoutParams(MATCH, ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.BOTTOM))
        }
    }

    private fun liveBadge(): View = TextView(this).apply {
        text = "●  LIVE"
        setTextColor(Color.WHITE)
        setTextSize(TypedValue.COMPLEX_UNIT_SP, 14f)
        typeface = Typeface.DEFAULT_BOLD
        letterSpacing = 0.08f
        setPadding(px(12), px(5), px(12), px(5))
        background = GradientDrawable().apply {
            cornerRadius = px(4).toFloat()
            setColor(0x33000000)
            setStroke(px(2), teal)
        }
    }

    // pill is a button in the bottom row: white text and icon, turning into
    // a white pill with dark text when the remote is on it.
    private fun pill(label: String, icon: Int, onClick: () -> Unit): TextView = TextView(this).apply {
        text = label
        textSize = 16f
        typeface = Typeface.create("sans-serif-medium", Typeface.NORMAL)
        val colors = ColorStateList(
            arrayOf(intArrayOf(android.R.attr.state_focused), intArrayOf()),
            intArrayOf(0xFF0B0C0F.toInt(), Color.WHITE),
        )
        setTextColor(colors)
        compoundDrawableTintList = colors
        setCompoundDrawablesRelativeWithIntrinsicBounds(icon, 0, 0, 0)
        compoundDrawablePadding = if (label.isEmpty()) 0 else px(8)
        gravity = Gravity.CENTER
        setPadding(px(16), px(10), px(if (label.isEmpty()) 16 else 20), px(10))
        background = StateListDrawable().apply {
            addState(intArrayOf(android.R.attr.state_focused), GradientDrawable().apply {
                cornerRadius = px(24).toFloat()
                setColor(Color.WHITE)
            })
            addState(intArrayOf(), GradientDrawable().apply {
                cornerRadius = px(24).toFloat()
                setColor(Color.TRANSPARENT)
            })
        }
        isFocusable = true
        isFocusableInTouchMode = true
        setOnClickListener { onClick() }
        layoutParams = LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, px(48)).apply { marginEnd = px(8) }
    }

    private fun chooseTrack(type: Int, title: String) {
        main.removeCallbacks(hider)
        TrackSelectionDialogBuilder(this, title, player, type)
            .setTheme(android.R.style.Theme_DeviceDefault_Dialog_Alert)
            .setShowDisableOption(type == C.TRACK_TYPE_TEXT)
            .build()
            .show()
    }

    private fun refreshTime() {
        if (live) return
        val dur = player.duration.let { if (it == C.TIME_UNSET) 0L else it }
        val pos = player.currentPosition
        timeBar.update(pos, dur, player.bufferedPosition)
        timeLeft.text = if (dur > 0) clock(dur - pos) else ""
        // Near the end of an episode, offer the next one.
        val next = nextButton
        if (next != null && !nextOffered && dur > 0 && dur - pos < 45_000 && player.isPlaying) {
            nextOffered = true
            showControls()
            next.requestFocus()
        }
    }

    private fun clock(ms: Long): String {
        val s = (ms / 1000).coerceAtLeast(0)
        val h = s / 3600
        val m = (s % 3600) / 60
        val sec = s % 60
        return if (h > 0) "%d:%02d:%02d".format(h, m, sec) else "%d:%02d".format(m, sec)
    }

    private fun controlsShowing() = controls.visibility == View.VISIBLE

    private fun showControls(focus: View? = null) {
        if (!controlsShowing()) {
            controls.alpha = 0f
            controls.visibility = View.VISIBLE
            controls.animate().alpha(1f).setDuration(180).start()
        }
        val target = focus ?: currentFocus?.takeIf { it.isShown } ?: (if (live) playPause else timeBar)
        if (!target.isFocused) target.requestFocus()
        scheduleHide()
    }

    private fun hideControls() {
        main.removeCallbacks(hider)
        controls.animate().alpha(0f).setDuration(250).withEndAction { controls.visibility = View.GONE }.start()
    }

    private fun scheduleHide() {
        main.removeCallbacks(hider)
        main.postDelayed(hider, 4500)
    }

    private fun togglePlay() {
        if (player.isPlaying) player.pause() else player.play()
    }

    private fun seekBy(ms: Long) {
        if (live) return
        val dur = player.duration.let { if (it == C.TIME_UNSET) Long.MAX_VALUE else it }
        player.seekTo((player.currentPosition + ms).coerceIn(0, dur))
        refreshTime()
    }

    override fun dispatchKeyEvent(event: KeyEvent): Boolean {
        val code = event.keyCode
        if (code == KeyEvent.KEYCODE_BACK) {
            if (event.action == KeyEvent.ACTION_UP) {
                if (controlsShowing() && player.isPlaying) hideControls() else end("back")
            }
            return true
        }
        if (event.action == KeyEvent.ACTION_DOWN) {
            // Keys that do the same with the controls showing or not.
            when (code) {
                KeyEvent.KEYCODE_MEDIA_PLAY_PAUSE, KeyEvent.KEYCODE_MEDIA_PLAY, KeyEvent.KEYCODE_MEDIA_PAUSE -> {
                    togglePlay(); showControls(); return true
                }
                KeyEvent.KEYCODE_MEDIA_FAST_FORWARD -> { seekBy(10_000); showControls(timeBar); return true }
                KeyEvent.KEYCODE_MEDIA_REWIND -> { seekBy(-10_000); showControls(timeBar); return true }
            }
            if (live) {
                when (code) {
                    KeyEvent.KEYCODE_CHANNEL_UP, KeyEvent.KEYCODE_PAGE_UP -> { end("channel-up"); return true }
                    KeyEvent.KEYCODE_CHANNEL_DOWN, KeyEvent.KEYCODE_PAGE_DOWN -> { end("channel-down"); return true }
                }
            }
            if (!controlsShowing()) {
                when (code) {
                    KeyEvent.KEYCODE_DPAD_CENTER, KeyEvent.KEYCODE_ENTER -> {
                        if (!live) togglePlay()
                        showControls(playPause)
                        return true
                    }
                    KeyEvent.KEYCODE_DPAD_LEFT -> { if (live) showControls() else { seekBy(-10_000); showControls(timeBar) }; return true }
                    KeyEvent.KEYCODE_DPAD_RIGHT -> { if (live) showControls() else { seekBy(10_000); showControls(timeBar) }; return true }
                    KeyEvent.KEYCODE_DPAD_UP -> { if (live) end("channel-up") else showControls(); return true }
                    KeyEvent.KEYCODE_DPAD_DOWN -> { if (live) end("channel-down") else showControls(); return true }
                }
            } else {
                scheduleHide()
            }
        }
        return super.dispatchKeyEvent(event)
    }

    // saveProgress tells Cue where this profile is in the title, the same way
    // Watch in a browser does.
    private fun saveProgress() {
        if (live || catchup) return
        val pos = player.currentPosition / 1000
        val dur = player.duration.let { if (it == C.TIME_UNSET) 0 else it / 1000 }
        if (pos < 5 || dur <= 0 || server.isEmpty()) return
        val body = JSONObject()
            .put("kind", info.optString("kind"))
            .put("tmdbId", info.optInt("tmdbId"))
            .put("season", info.optInt("season"))
            .put("episode", info.optInt("episode"))
            .put("position", pos)
            .put("duration", dur)
            .toString()
        val cookie = CookieManager.getInstance().getCookie(server)
        val token = info.optString("token")
        Thread {
            try {
                val c = URL("$server/api/watch/progress").openConnection() as HttpURLConnection
                c.requestMethod = "PUT"
                c.doOutput = true
                c.connectTimeout = 8000
                c.readTimeout = 8000
                c.setRequestProperty("Content-Type", "application/json")
                if (cookie != null) c.setRequestProperty("Cookie", cookie)
                if (token.isNotEmpty()) c.setRequestProperty("X-Cue-Profile", token)
                c.outputStream.use { it.write(body.toByteArray()) }
                c.responseCode
                c.disconnect()
            } catch (_: Exception) {
                // A missed save is made up by the next one.
            }
        }.start()
    }

    private fun end(reason: String, message: String = "") {
        if (finished) return
        finished = true
        saveProgress()
        val dur = player.duration.let { if (it == C.TIME_UNSET) 0 else it / 1000 }
        setResult(
            RESULT_OK,
            Intent()
                .putExtra(RESULT_REASON, reason)
                .putExtra(RESULT_POSITION, player.currentPosition / 1000)
                .putExtra(RESULT_DURATION, dur)
                .putExtra(RESULT_MESSAGE, message),
        )
        finish()
    }

    override fun onStop() {
        super.onStop()
        if (!finished) {
            saveProgress()
            player.pause()
        }
    }

    override fun onDestroy() {
        main.removeCallbacksAndMessages(null)
        player.release()
        super.onDestroy()
    }

    companion object {
        const val EXTRA_JSON = "json"
        const val EXTRA_SERVER = "server"
        const val RESULT_REASON = "reason"
        const val RESULT_POSITION = "position"
        const val RESULT_DURATION = "duration"
        const val RESULT_MESSAGE = "message"
        private const val SAVE_EVERY_MS = 15_000L
        private const val MATCH = ViewGroup.LayoutParams.MATCH_PARENT
    }
}
