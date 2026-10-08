package app.cue.tv

import android.app.Activity
import android.content.Intent
import android.content.res.ColorStateList
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.graphics.drawable.StateListDrawable
import android.net.Uri
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
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import androidx.annotation.OptIn
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.Tracks
import androidx.media3.common.VideoSize
import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.exoplayer.DefaultRenderersFactory
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.ui.AspectRatioFrameLayout
import androidx.media3.ui.CaptionStyleCompat
import androidx.media3.ui.SubtitleView
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
    // Other addresses for the same thing, tried in turn if one won't start
    // (providers serve catch-up in different ways).
    private val alts = ArrayDeque<String>()
    private var started = false

    // Catch-up: the show's start and end (ms since 1970), where in it the
    // stream began (providers start a catch-up stream at a whole minute and
    // it can't seek far by itself), and the channel, to ask Cue for the
    // stream from another minute.
    private var cuStart = 0L
    private var cuStop = 0L
    private var cuOffset = 0L
    private var cuChannel = ""
    private var pendingSeek = -1L
    private val restarter = Runnable { restartCatchupAt(pendingSeek) }
    private var isShow = false

    private lateinit var controls: View
    private lateinit var timeBar: TimeBar
    private lateinit var timeLeft: TextView
    private lateinit var playPause: TextView
    private lateinit var spinner: ProgressBar
    private lateinit var pauseBadge: ImageView
    private var nextButton: TextView? = null
    private var subtitle: TextView? = null
    // Whether Watch has another version to try, and whether the sound was
    // checked yet (a version whose sound this TV can't play moves on).
    private var hasOther = false
    private var soundChecked = false
    private var nextOffered = false

    // Skip intro / recap and the credits, in ms (0 = not known), from
    // TheIntroDB or learned from what the household skips.
    private var introStart = 0L
    private var introEnd = 0L
    private var recapStart = 0L
    private var recapEnd = 0L
    private var creditsStart = 0L
    private var creditsFromEnd = 0L
    private lateinit var skipButton: TextView
    private var skipTo = 0L
    // The viewer's jump forward in progress (several presses count as one).
    private var jumpFrom = -1L
    private var jumpTo = -1L
    private val jumpDone = Runnable { learnIntro() }
    private var skipsAsked = false
    // An English subtitle from OpenSubtitles added to the video (Watch
    // finds it when subtitles are on, or Subtitles here asks for one).
    private var externalSubs = false
    // Live TV: the channel list shown over the picture (OK or left).
    private var guide: View? = null
    private var guideList: LinearLayout? = null
    private val guideHider = Runnable { hideGuide() }
    private var subsAsking = false

    // Sizes follow the screen, not the TV's density or font-size setting
    // (Google TV boxes differ there): 1 unit is 1/960 of the screen's width.
    private val dp by lazy { screenUnit(this) }
    private fun px(v: Int) = (v * dp).toInt()
    private fun TextView.size(units: Float) = setTextSize(TypedValue.COMPLEX_UNIT_PX, units * dp)
    // The profile's color from Watch, or Cue's own.
    private val teal by lazy {
        runCatching { Color.parseColor(info.optString("accent")) }.getOrNull() ?: getColor(R.color.cue_teal)
    }

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
        if (catchup) {
            cuStart = info.optLong("showStart")
            cuStop = info.optLong("showStop")
            cuOffset = (info.optDouble("offsetSec", 0.0) * 1000).toLong()
            cuChannel = info.optString("id")
        }
        info.optJSONArray("alts")?.let { list ->
            for (i in 0 until list.length()) list.optString(i).takeIf { it.isNotEmpty() }?.let { alts.addLast(it) }
        }
        isShow = info.optString("kind") == "tv"
        fun ms(key: String) = (info.optDouble(key, 0.0) * 1000).toLong()
        introStart = ms("introStart"); introEnd = ms("introEnd")
        recapStart = ms("recapStart"); recapEnd = ms("recapEnd")
        creditsStart = ms("creditsStart"); creditsFromEnd = ms("creditsFromEnd")
        hasOther = info.optBoolean("hasOther")

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
            // Subtitles show from the start when they're on in Watch.
            .setPreferredTextLanguage(if (info.optBoolean("subsOn") && info.optString("subtitleUrl").isNotEmpty()) "en" else null)
            .build()

        val video = PlayerView(this).apply {
            player = this@PlayerActivity.player
            useController = false
            resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FIT
            setShutterBackgroundColor(Color.BLACK)
            setKeepContentOnPlayerReset(true)
            styleSubtitles(subtitleView)
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
            skipButton = buildSkipButton()
            val fb = buildFrameBox()
            frameBox = fb
            addView(fb, FrameLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.BOTTOM or Gravity.START).apply {
                bottomMargin = px(158)
            })
            buildGuide()?.let {
                guide = it
                addView(it, FrameLayout.LayoutParams(px(400), MATCH, Gravity.START))
            }
            addView(skipButton, FrameLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, px(52), Gravity.BOTTOM or Gravity.END).apply {
                marginEnd = px(48)
                bottomMargin = px(150)
            })
        }
        setContentView(root)

        player.addListener(object : Player.Listener {
            override fun onPlaybackStateChanged(state: Int) {
                if (state == Player.STATE_READY && !skipsAsked && !live && !catchup) {
                    skipsAsked = true
                    fetchSkips()
                }
                spinner.visibility = if (state == Player.STATE_BUFFERING) View.VISIBLE else View.GONE
                if (state == Player.STATE_ENDED) {
                    if (live) end("error", "The channel stopped.") else end("ended")
                }
            }

            override fun onIsPlayingChanged(isPlaying: Boolean) {
                if (isPlaying) started = true
                playPause.setCompoundDrawablesRelativeWithIntrinsicBounds(
                    if (isPlaying) R.drawable.ic_cue_pause else R.drawable.ic_cue_play, 0, 0, 0,
                )
                val paused = !isPlaying && player.playbackState == Player.STATE_READY
                pauseBadge.visibility = if (paused) View.VISIBLE else View.GONE
                if (paused) showControls() else scheduleHide()
            }

            override fun onTracksChanged(tracks: Tracks) {
                if (live || soundChecked) return
                val audio = tracks.groups.filter { it.type == C.TRACK_TYPE_AUDIO }
                if (audio.isEmpty()) return
                soundChecked = true
                if (audio.none { it.isSupported }) {
                    // The picture would play in silence.
                    if (hasOther) {
                        end("nosound")
                    } else {
                        Toast.makeText(this@PlayerActivity, "This version's sound can't play on this TV.", Toast.LENGTH_LONG).show()
                    }
                }
            }

            override fun onVideoSizeChanged(videoSize: VideoSize) {
                // Say what the picture really is: a version can be smaller
                // than its name says.
                val w = videoSize.width
                if (w <= 0) return
                // By width: a film cropped wide is 1920 x 800 and still 1080p.
                val label = when {
                    w >= 3200 -> "4K"
                    w >= 1800 -> "1080p"
                    w >= 1200 -> "720p"
                    else -> "SD (${videoSize.height}p)"
                }
                val base = info.optString("subtitle")
                subtitle?.apply {
                    text = listOf(base, "Picture $label").filter { it.isNotEmpty() }.joinToString("  ·  ")
                    visibility = View.VISIBLE
                }
            }

            override fun onPlayerError(error: PlaybackException) {
                if (!started && alts.isNotEmpty()) {
                    player.setMediaItem(MediaItem.fromUri(alts.removeFirst()))
                    player.prepare()
                    player.playWhenReady = true
                    return
                }
                if (live && error.errorCode == PlaybackException.ERROR_CODE_BEHIND_LIVE_WINDOW) {
                    player.seekToDefaultPosition() // fell behind: back to live
                    player.prepare()
                    return
                }
                end("error", error.errorCodeName)
            }
        })

        player.setMediaItem(mediaItem(info.optString("subtitleUrl")))
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
            size(22f)
            typeface = Typeface.create("sans-serif-medium", Typeface.BOLD)
            setShadowLayer(8f, 0f, 2f, Color.BLACK)
        }
        val sub = TextView(this).also { subtitle = it }.apply {
            text = info.optString("subtitle")
            setTextColor(0xFFC7C8CC.toInt())
            size(13f)
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
            setAccent(teal)
            onSeek = { to ->
                noteJump(this@PlayerActivity.position(), to)
                showFrame(to)
                seekToPosition(to)
                showControls()
            }
        }
        timeLeft = TextView(this).apply {
            setTextColor(Color.WHITE)
            size(13f)
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
                nextButton = pill("Next episode", R.drawable.ic_cue_next) {
                    learnCredits()
                    end("next")
                }.also { addView(it) }
            }
            if (hasOther && !live && !catchup) {
                addView(pill("Other version", R.drawable.ic_cue_versions) { end("other") })
            }
            addView(pill("Audio", R.drawable.ic_cue_audio) { chooseTrack(C.TRACK_TYPE_AUDIO, "Audio") })
            addView(pill("Subtitles", R.drawable.ic_cue_subtitles) { subtitlesPressed() })
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
        size(12f)
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
        size(14f)
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
        layoutParams = LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, px(42)).apply { marginEnd = px(8) }
    }

    private fun chooseTrack(type: Int, title: String) {
        main.removeCallbacks(hider)
        TrackSelectionDialogBuilder(this, title, player, type)
            .setTheme(android.R.style.Theme_DeviceDefault_Dialog_Alert)
            .setShowDisableOption(type == C.TRACK_TYPE_TEXT)
            .build()
            .show()
    }

    // Where in the title the picture is, and how long the title is: for
    // catch-up, the whole show, whatever minute the stream began at.
    private fun position(): Long = if (catchup) cuOffset + player.currentPosition else player.currentPosition
    private fun length(): Long =
        if (catchup && cuStop > cuStart) cuStop - cuStart else player.duration.let { if (it == C.TIME_UNSET) 0L else it }

    private fun refreshTime() {
        if (live) return
        val dur = length()
        val pos = if (pendingSeek >= 0) pendingSeek else position()
        timeBar.update(pos, dur, if (catchup) cuOffset + player.bufferedPosition else player.bufferedPosition)
        timeLeft.text = if (dur > 0) clock(dur - pos) else ""
        updateSkip(pos)
        // When the credits start (or near the end), offer the next episode.
        val next = nextButton
        val credits = when {
            creditsStart > 0 -> creditsStart
            creditsFromEnd > 0 && dur > 0 -> dur - creditsFromEnd
            else -> 0L
        }
        val atCredits = credits > 0 && pos >= credits
        if (next != null && !nextOffered && dur > 0 && (atCredits || dur - pos < 45_000) && player.isPlaying) {
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
        val from = if (pendingSeek >= 0) pendingSeek else position()
        noteJump(from, from + ms)
        val dur = length().takeIf { it > 0 } ?: Long.MAX_VALUE
        showFrame((from + ms).coerceIn(0, dur))
        seekToPosition((from + ms).coerceIn(0, dur))
        refreshTime()
    }

    // seekToPosition goes to a point in the title. Within what the stream has
    // loaded it just moves; for catch-up further away, it asks Cue for the
    // stream from that minute, once the remote has stopped moving.
    private fun seekToPosition(to: Long) {
        if (!catchup) {
            player.seekTo(to)
            return
        }
        val rel = to - cuOffset
        if (pendingSeek < 0 && rel >= 0 && rel <= player.bufferedPosition && player.isCurrentMediaItemSeekable) {
            player.seekTo(rel)
            return
        }
        pendingSeek = to
        main.removeCallbacks(restarter)
        main.postDelayed(restarter, 700)
    }

    private fun restartCatchupAt(to: Long) {
        if (to < 0 || server.isEmpty() || cuChannel.isEmpty()) return
        val minute = (to / 60_000) * 60_000
        val cookie = CookieManager.getInstance().getCookie(server)
        val token = info.optString("token")
        Thread {
            val answer = try {
                val c = URL("$server/api/live/catchup/${java.net.URLEncoder.encode(cuChannel, "UTF-8")}?start=${(cuStart + minute) / 1000}&stop=${cuStop / 1000}")
                    .openConnection() as HttpURLConnection
                c.connectTimeout = 8000
                c.readTimeout = 15000
                c.setRequestProperty("User-Agent", "CueTV/1")
                if (cookie != null) c.setRequestProperty("Cookie", cookie)
                if (token.isNotEmpty()) c.setRequestProperty("X-Cue-Profile", token)
                val ok = c.responseCode == 200
                val body = (if (ok) c.inputStream else c.errorStream)?.bufferedReader()?.use { it.readText() } ?: ""
                c.disconnect()
                if (ok) JSONObject(body) else null
            } catch (_: Exception) {
                null
            }
            runOnUiThread {
                pendingSeek = -1
                val direct = answer?.optString("direct").orEmpty()
                if (direct.isEmpty()) return@runOnUiThread // stays where it was
                cuOffset = minute
                started = false
                alts.clear()
                answer?.optJSONArray("directAlts")?.let { list ->
                    for (i in 0 until list.length()) list.optString(i).takeIf { it.isNotEmpty() }?.let { alts.addLast(it) }
                }
                player.setMediaItem(MediaItem.fromUri(direct))
                player.prepare()
                player.playWhenReady = true
                refreshTime()
            }
        }.start()
    }

    override fun dispatchKeyEvent(event: KeyEvent): Boolean {
        val code = event.keyCode
        if (code == KeyEvent.KEYCODE_BACK && guideShowing()) {
            if (event.action == KeyEvent.ACTION_UP) hideGuide()
            return true
        }
        if (guideShowing()) {
            // Up and down move through the list; OK picks (the rows' own).
            if (event.action == KeyEvent.ACTION_DOWN) {
                main.removeCallbacks(guideHider)
                main.postDelayed(guideHider, 15_000)
                if (code == KeyEvent.KEYCODE_DPAD_RIGHT) {
                    hideGuide()
                    return true
                }
            }
            return super.dispatchKeyEvent(event)
        }
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
                        if (skipButton.visibility == View.VISIBLE) {
                            skip()
                            return true
                        }
                        if (live && showGuide()) return true
                        if (!live) togglePlay()
                        showControls(playPause)
                        return true
                    }
                    KeyEvent.KEYCODE_DPAD_LEFT -> { if (live) { if (!showGuide()) showControls() } else { seekBy(-10_000); showControls(timeBar) }; return true }
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

    // ---- Skip intro, the credits, and learning them ----

    private fun buildSkipButton(): TextView = TextView(this).apply {
        size(16f)
        typeface = Typeface.create("sans-serif-medium", Typeface.BOLD)
        gravity = Gravity.CENTER
        setPadding(px(22), 0, px(22), 0)
        val colors = ColorStateList(
            arrayOf(intArrayOf(android.R.attr.state_focused), intArrayOf()),
            intArrayOf(0xFF0B0C0F.toInt(), Color.WHITE),
        )
        setTextColor(colors)
        background = StateListDrawable().apply {
            addState(intArrayOf(android.R.attr.state_focused), GradientDrawable().apply {
                cornerRadius = px(8).toFloat()
                setColor(Color.WHITE)
            })
            addState(intArrayOf(), GradientDrawable().apply {
                cornerRadius = px(8).toFloat()
                setColor(0xB3101114.toInt())
                setStroke(px(2), Color.WHITE)
            })
        }
        isFocusable = true
        isFocusableInTouchMode = true
        visibility = View.GONE
        setOnClickListener { skip() }
    }

    // updateSkip shows Skip recap / Skip intro while in one.
    private fun updateSkip(pos: Long) {
        if (live || catchup) return
        val (label, to) = when {
            recapEnd > 0 && pos >= recapStart && pos < recapEnd - 2000 -> "Skip recap" to recapEnd
            introEnd > 0 && pos >= introStart && pos < introEnd - 2000 -> "Skip intro" to introEnd
            else -> "" to 0L
        }
        if (label.isEmpty()) {
            if (skipButton.visibility == View.VISIBLE) {
                val hadFocus = skipButton.isFocused
                skipButton.visibility = View.GONE
                if (hadFocus && controlsShowing()) playPause.requestFocus()
            }
            return
        }
        skipTo = to
        skipButton.text = "$label  ›"
        if (skipButton.visibility != View.VISIBLE) {
            skipButton.visibility = View.VISIBLE
            if (!controlsShowing()) skipButton.requestFocus()
        }
    }

    private fun skip() {
        if (skipTo <= 0) return
        player.seekTo(skipTo)
        skipButton.visibility = View.GONE
        refreshTime()
    }

    // noteJump gathers the viewer's jumps forward; one over the start of an
    // episode teaches Cue its intro (learnIntro, once the remote rests).
    private fun noteJump(from: Long, to: Long) {
        if (!isShow || live || catchup || to <= from) return
        if (jumpFrom >= 0 && kotlin.math.abs(from - jumpTo) < 4000) {
            jumpTo = to
        } else {
            jumpFrom = from
            jumpTo = to
        }
        main.removeCallbacks(jumpDone)
        main.postDelayed(jumpDone, 4000)
    }

    private fun learnIntro() {
        val from = jumpFrom / 1000.0
        val to = jumpTo / 1000.0
        jumpFrom = -1
        jumpTo = -1
        if (from < 0 || from >= 480 || to - from < 15 || to - from > 200) return
        // Theirs wins from now on (TheIntroDB's may be for another cut).
        introStart = (from * 1000).toLong()
        introEnd = (to * 1000).toLong()
        postJson("/api/watch/skips/learn", JSONObject()
            .put("kind", "tv").put("tmdbId", info.optInt("tmdbId")).put("season", info.optInt("season"))
            .put("segment", "intro").put("start", from).put("end", to))
    }

    // learnCredits: Next pressed during the end of an episode marks where
    // its credits start.
    private fun learnCredits() {
        if (creditsStart > 0 || live || catchup) return
        val dur = player.duration.takeIf { it != C.TIME_UNSET } ?: return
        val pos = player.currentPosition
        val left = dur - pos
        if (left < 15_000 || left > 900_000 || pos < dur * 0.7) return
        postJson("/api/watch/skips/learn", JSONObject()
            .put("kind", info.optString("kind")).put("tmdbId", info.optInt("tmdbId")).put("season", info.optInt("season"))
            .put("segment", "credits").put("start", pos / 1000.0).put("duration", dur / 1000.0))
    }

    // ---- Frames over the time bar while moving through the video ----

    private var frameBox: LinearLayout? = null
    private var frameImage: ImageView? = null
    private var frameTime: TextView? = null
    private var frameWanted = -1L
    private val frameCache = object : android.util.LruCache<Long, Bitmap>(40) {}
    private val frameHider = Runnable { frameBox?.visibility = View.GONE }

    private fun buildFrameBox(): LinearLayout = LinearLayout(this).apply {
        orientation = LinearLayout.VERTICAL
        gravity = Gravity.CENTER_HORIZONTAL
        setPadding(px(4), px(4), px(4), px(4))
        background = GradientDrawable().apply {
            cornerRadius = px(6).toFloat()
            setColor(0xF0141414.toInt())
            setStroke(px(1), 0x40FFFFFF)
        }
        val img = ImageView(this@PlayerActivity).apply {
            scaleType = ImageView.ScaleType.CENTER_CROP
            setBackgroundColor(Color.BLACK)
        }
        frameImage = img
        addView(img, LinearLayout.LayoutParams(px(224), px(126)))
        val time = TextView(this@PlayerActivity).apply {
            setTextColor(Color.WHITE)
            size(13f)
            typeface = Typeface.MONOSPACE
            setPadding(0, px(4), 0, px(2))
        }
        frameTime = time
        addView(time)
        visibility = View.GONE
    }

    // showFrame shows the picture at `at` (ms) above the time bar, where the
    // knob is going; it fades a moment after the remote rests.
    private fun showFrame(at: Long) {
        val token = info.optString("thumbs")
        val box = frameBox ?: return
        if (token.isEmpty() || live || server.isEmpty()) return
        val dur = length()
        if (dur <= 0) return
        val bucket = at / 10_000 * 10
        frameWanted = bucket
        frameTime?.text = clock(at)
        // Over the knob's new place, kept on the screen.
        val loc = IntArray(2)
        timeBar.getLocationInWindow(loc)
        val f = (at.toFloat() / dur).coerceIn(0f, 1f)
        val x = loc[0] + px(12) + f * (timeBar.width - px(24))
        val w = px(232)
        box.translationX = (x - w / 2f).coerceIn(px(16).toFloat(), (window.decorView.width - w - px(16)).toFloat())
        box.visibility = View.VISIBLE
        main.removeCallbacks(frameHider)
        main.postDelayed(frameHider, 1500)
        frameCache.get(bucket)?.let {
            frameImage?.setImageBitmap(it)
            return
        }
        val url = "$server/api/thumb?u=${Uri.encode(token)}&t=$bucket"
        Thread {
            try {
                val c = URL(url).openConnection() as HttpURLConnection
                c.connectTimeout = 8000
                c.readTimeout = 20000
                val bmp = if (c.responseCode == 200) c.inputStream.use { BitmapFactory.decodeStream(it) } else null
                c.disconnect()
                if (bmp != null) {
                    main.post {
                        frameCache.put(bucket, bmp)
                        if (frameWanted == bucket) frameImage?.setImageBitmap(bmp)
                    }
                }
            } catch (_: Exception) {
                // No frame this time.
            }
        }.start()
    }

    // ---- Live TV: the channel list ----

    private fun guideShowing() = guide?.visibility == View.VISIBLE

    private fun buildGuide(): View? {
        val chans = info.optJSONArray("channels") ?: return null
        if (!live || chans.length() < 2) return null
        val list = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(px(16), px(20), px(16), px(40))
        }
        list.addView(TextView(this).apply {
            text = "Channels"
            setTextColor(0xFFC7C8CC.toInt())
            size(13f)
            typeface = Typeface.DEFAULT_BOLD
            letterSpacing = 0.08f
            setPadding(px(12), 0, 0, px(10))
        })
        val current = info.optString("id")
        for (i in 0 until chans.length()) {
            val c = chans.optJSONObject(i) ?: continue
            val id = c.optString("id")
            val row = TextView(this).apply {
                val num = c.optString("num")
                val now = c.optString("now")
                val until = c.optString("until")
                text = buildString {
                    append(if (num.isNotEmpty()) "$num   " else "")
                    append(c.optString("name"))
                    if (now.isNotEmpty()) append("\n").append(now).append(if (until.isNotEmpty()) "  ·  until $until" else "")
                }
                size(14f)
                maxLines = 2
                ellipsize = android.text.TextUtils.TruncateAt.END
                val colors = ColorStateList(
                    arrayOf(intArrayOf(android.R.attr.state_focused), intArrayOf()),
                    intArrayOf(0xFF0B0C0F.toInt(), Color.WHITE),
                )
                setTextColor(colors)
                setPadding(px(12), px(9), px(12), px(9))
                background = StateListDrawable().apply {
                    addState(intArrayOf(android.R.attr.state_focused), GradientDrawable().apply {
                        cornerRadius = px(8).toFloat()
                        setColor(Color.WHITE)
                    })
                    addState(intArrayOf(), GradientDrawable().apply {
                        cornerRadius = px(8).toFloat()
                        setColor(if (id == current) (teal and 0x00FFFFFF) or 0x40000000 else Color.TRANSPARENT)
                    })
                }
                isFocusable = true
                isFocusableInTouchMode = true
                tag = id
                setOnClickListener {
                    if (id == current) hideGuide() else end("channel", id)
                }
            }
            list.addView(row, LinearLayout.LayoutParams(MATCH, ViewGroup.LayoutParams.WRAP_CONTENT).apply { bottomMargin = px(2) })
        }
        guideList = list
        return ScrollView(this).apply {
            isFillViewport = true
            isVerticalScrollBarEnabled = false
            background = GradientDrawable(GradientDrawable.Orientation.LEFT_RIGHT, intArrayOf(0xF00B0C0F.toInt(), 0xD00B0C0F.toInt()))
            addView(list)
            visibility = View.GONE
        }
    }

    // showGuide opens the channel list on the channel playing; false when
    // there is none (one channel, catch-up).
    private fun showGuide(): Boolean {
        val g = guide ?: return false
        val list = guideList ?: return false
        hideControls()
        g.visibility = View.VISIBLE
        val current = info.optString("id")
        val row = (0 until list.childCount).map { list.getChildAt(it) }.firstOrNull { it.tag == current }
            ?: list.getChildAt(1)
        row?.requestFocus()
        main.removeCallbacks(guideHider)
        main.postDelayed(guideHider, 15_000)
        return true
    }

    private fun hideGuide() {
        main.removeCallbacks(guideHider)
        guide?.visibility = View.GONE
    }

    // ---- Subtitles ----

    // mediaItem is the video, with Watch's subtitle added when there is one.
    private fun mediaItem(subtitleUrl: String): MediaItem {
        val b = MediaItem.Builder().setUri(info.optString("url"))
        if (subtitleUrl.isNotEmpty() && !live) {
            externalSubs = true
            b.setSubtitleConfigurations(listOf(
                MediaItem.SubtitleConfiguration.Builder(Uri.parse(subtitleUrl))
                    .setMimeType(MimeTypes.TEXT_VTT)
                    .setLanguage("en")
                    .setLabel("English (OpenSubtitles)")
                    .setSelectionFlags(C.SELECTION_FLAG_DEFAULT)
                    .build(),
            ))
        }
        return b.build()
    }

    // styleSubtitles: the size and dark band chosen in Watch.
    private fun styleSubtitles(view: SubtitleView?) {
        view ?: return
        val scale = when (info.optString("subsSize")) {
            "small" -> 0.85f
            "large" -> 1.35f
            else -> 1.1f
        }
        view.setFractionalTextSize(SubtitleView.DEFAULT_TEXT_SIZE_FRACTION * scale)
        val band = info.optBoolean("subsBand", true)
        view.setStyle(CaptionStyleCompat(
            Color.WHITE,
            if (band) 0xB8000000.toInt() else Color.TRANSPARENT,
            Color.TRANSPARENT,
            if (band) CaptionStyleCompat.EDGE_TYPE_NONE else CaptionStyleCompat.EDGE_TYPE_DROP_SHADOW,
            Color.BLACK,
            null,
        ))
    }

    // subtitlesPressed: the first time, without a subtitle from Watch, Cue is
    // asked for an English one (it's added and shown); after that, the list
    // of subtitles to choose from.
    private fun subtitlesPressed() {
        val kind = info.optString("kind")
        if (externalSubs || live || catchup || server.isEmpty() || (kind != "movie" && kind != "tv")) {
            chooseTrack(C.TRACK_TYPE_TEXT, "Subtitles")
            return
        }
        if (subsAsking) return
        subsAsking = true
        Toast.makeText(this, "Finding English subtitles…", Toast.LENGTH_SHORT).show()
        val path = "/api/watch/subtitles/$kind/${info.optInt("tmdbId")}?season=${info.optInt("season")}&episode=${info.optInt("episode")}&release=${Uri.encode(info.optString("release"))}"
        val cookie = CookieManager.getInstance().getCookie(server)
        val token = info.optString("token")
        Thread {
            var url = ""
            var problem = "Couldn't find subtitles for this."
            try {
                val c = URL("$server$path").openConnection() as HttpURLConnection
                c.connectTimeout = 10000
                c.readTimeout = 30000
                if (cookie != null) c.setRequestProperty("Cookie", cookie)
                if (token.isNotEmpty()) c.setRequestProperty("X-Cue-Profile", token)
                val ok = c.responseCode == 200
                val text = (if (ok) c.inputStream else c.errorStream)?.bufferedReader()?.use { it.readText() } ?: ""
                c.disconnect()
                val j = JSONObject(text.ifEmpty { "{}" })
                if (ok) url = server + j.optString("url") else j.optString("error").takeIf { it.isNotEmpty() }?.let { problem = it }
            } catch (_: Exception) {
            }
            main.post {
                subsAsking = false
                if (finished) return@post
                if (url.isEmpty()) {
                    Toast.makeText(this, problem, Toast.LENGTH_LONG).show()
                    return@post
                }
                val pos = player.currentPosition
                val playing = player.playWhenReady
                player.trackSelectionParameters = player.trackSelectionParameters.buildUpon()
                    .setPreferredTextLanguage("en")
                    .build()
                player.setMediaItem(mediaItem(url), pos)
                player.prepare()
                player.playWhenReady = playing
                Toast.makeText(this, "English subtitles on", Toast.LENGTH_SHORT).show()
            }
        }.start()
    }

    // fetchSkips asks Cue for the skip times now the video's length is
    // known: TheIntroDB's are for one cut of an episode, and only the
    // length says which.
    private fun fetchSkips() {
        val dur = player.duration.takeIf { it != C.TIME_UNSET && it > 0 } ?: return
        if (server.isEmpty()) return
        val kind = info.optString("kind")
        val path = "/api/watch/skips/$kind/${info.optInt("tmdbId")}?season=${info.optInt("season")}&episode=${info.optInt("episode")}&duration=${dur / 1000}"
        val cookie = CookieManager.getInstance().getCookie(server)
        val token = info.optString("token")
        Thread {
            try {
                val c = URL("$server$path").openConnection() as HttpURLConnection
                c.connectTimeout = 8000
                c.readTimeout = 8000
                if (cookie != null) c.setRequestProperty("Cookie", cookie)
                if (token.isNotEmpty()) c.setRequestProperty("X-Cue-Profile", token)
                if (c.responseCode != 200) return@Thread
                val j = JSONObject(c.inputStream.bufferedReader().use { it.readText() })
                c.disconnect()
                main.post {
                    if (finished) return@post
                    j.optJSONObject("intro")?.let {
                        introStart = (it.optDouble("start", 0.0) * 1000).toLong()
                        introEnd = (it.optDouble("end", 0.0) * 1000).toLong()
                    }
                    j.optJSONObject("recap")?.let {
                        recapStart = (it.optDouble("start", 0.0) * 1000).toLong()
                        recapEnd = (it.optDouble("end", 0.0) * 1000).toLong()
                    }
                    val cs = (j.optDouble("creditsStart", 0.0) * 1000).toLong()
                    if (cs > 0) creditsStart = cs
                    val cf = (j.optDouble("creditsFromEnd", 0.0) * 1000).toLong()
                    if (cf > 0) creditsFromEnd = cf
                }
            } catch (_: Exception) {
                // No skip buttons this time.
            }
        }.start()
    }

    // postJson sends something to Cue as this profile, in the background.
    private fun postJson(path: String, body: JSONObject) {
        if (server.isEmpty()) return
        val cookie = CookieManager.getInstance().getCookie(server)
        val token = info.optString("token")
        Thread {
            try {
                val c = URL("$server$path").openConnection() as HttpURLConnection
                c.requestMethod = "POST"
                c.doOutput = true
                c.connectTimeout = 8000
                c.readTimeout = 8000
                c.setRequestProperty("Content-Type", "application/json")
                if (cookie != null) c.setRequestProperty("Cookie", cookie)
                if (token.isNotEmpty()) c.setRequestProperty("X-Cue-Profile", token)
                c.outputStream.use { it.write(body.toString().toByteArray()) }
                c.responseCode
                c.disconnect()
            } catch (_: Exception) {
                // Not learned this time.
            }
        }.start()
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
