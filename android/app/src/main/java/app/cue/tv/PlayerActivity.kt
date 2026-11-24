package app.cue.tv

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.content.res.ColorStateList
import android.graphics.Bitmap
import android.graphics.BitmapFactory
import android.graphics.Color
import android.graphics.Typeface
import android.graphics.drawable.GradientDrawable
import android.graphics.drawable.StateListDrawable
import android.media.MediaFormat
import android.net.Uri
import android.os.Build
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.os.SystemClock
import android.util.TypedValue
import android.view.Gravity
import android.view.Display
import android.view.KeyEvent
import android.view.Surface
import android.view.SurfaceView
import android.view.View
import android.view.ViewGroup
import android.view.WindowManager
import android.webkit.CookieManager
import android.widget.FrameLayout
import android.widget.ImageView
import android.widget.LinearLayout
import android.widget.ScrollView
import android.widget.TextView
import android.widget.Toast
import androidx.annotation.OptIn
import androidx.media3.common.C
import androidx.media3.common.Format
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.Tracks
import androidx.media3.common.VideoSize
import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.exoplayer.DefaultRenderersFactory
import androidx.media3.exoplayer.audio.AudioCapabilities
import androidx.media3.exoplayer.audio.AudioSink
import androidx.media3.exoplayer.audio.DefaultAudioSink
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.Renderer
import androidx.media3.exoplayer.mediacodec.MediaCodecSelector
import androidx.media3.exoplayer.video.VideoFrameMetadataListener
import androidx.media3.exoplayer.video.VideoRendererEventListener
import androidx.media3.exoplayer.trackselection.DefaultTrackSelector
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
    private lateinit var video: PlayerView
    private lateinit var listener: Player.Listener
    private var surround = true // Dolby and DTS sent on as they are (see buildPlayer)
    private var soundButton: TextView? = null
    private var rateMatched = false
    private var measuredFps = 0f // from the frames' own times, when the file doesn't say
    private var pictureLabel = ""
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
    private lateinit var timeNow: TextView
    private var speed = 1f
    private lateinit var playPause: TextView
    private lateinit var spinner: View // loading: the Cue mark, breathing
    private lateinit var pauseBadge: ImageView
    private var nextButton: TextView? = null
    private var otherButton: View? = null
    // Times this version was picked up again after the stream dropped.
    private var hiccups = 0
    private var subtitle: TextView? = null
    // Whether Watch has another version to try, and whether the sound was
    // checked yet (a version whose sound this TV can't play moves on).
    private var hasOther = false
    private var soundChecked = false
    private var nearEnd = false

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

    // A watch party (PartyLink): where the party is, and this TV's place in it.
    private var party: PartyLink? = null
    private var partyState: JSONObject? = null
    private var partyGot = 0L // when partyState arrived (elapsedRealtime)
    private var partyApplying = 0L // until then, play/pause/seek events are the party's, not the viewer's
    private var partyPanel: LinearLayout? = null
    private var partyHead: TextView? = null
    private var partyLines: LinearLayout? = null
    private var partyQr: ImageView? = null
    // Listen on phone: the sound plays on a phone (joined through a watch
    // party, picture hidden there) and the TV goes quiet.
    private var listenMode = false
    // The episode this TV last moved on from in place, and when (a party's
    // late word about it is ignored for a moment).
    private var leftEpisode: Pair<Int, Int>? = null
    private var leftAt = 0L
    private var listenButton: TextView? = null
    // Volume boost: Android's loudness enhancer on the player's sound, about
    // twice as loud (for quiet films and soft dialogue).
    private var booster: android.media.audiofx.LoudnessEnhancer? = null
    private var boostButton: TextView? = null
    private var partyFloats: FrameLayout? = null
    private var partyButton: TextView? = null
    private var partyWait: TextView? = null // "Waiting for …", while the party holds
    private var partyHost = false // this TV started the party
    private var partyWaitSaid = false // this TV told the party it's loading
    private var reactButton: TextView? = null
    private var reactRow: LinearLayout? = null

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

    // Send to TV: what's playing goes to Cue (for the phone's remote) every
    // few seconds and on every change.
    private var reported = 0L
    private fun reportScreen(force: Boolean = false) {
        if (live) return
        val now = SystemClock.elapsedRealtime()
        if (!force && now - reported < 5000) return
        reported = now
        val dur = player.duration.let { if (it == C.TIME_UNSET || it < 0) 0.0 else it / 1000.0 }
        ScreenLink.report(JSONObject()
            .put("kind", info.optString("kind"))
            .put("tmdbId", info.optInt("tmdbId"))
            .put("season", info.optInt("season"))
            .put("episode", info.optInt("episode"))
            .put("title", info.optString("title"))
            .put("pos", position() / 1000.0)
            .put("duration", dur)
            .put("playing", player.playWhenReady))
    }
    private val screenTicker = object : Runnable {
        override fun run() {
            reportScreen()
            main.postDelayed(this, 5000)
        }
    }

    // A phone's remote.
    private fun onRemote(c: JSONObject) {
        if (live) return
        when (c.optString("action")) {
            "play" -> player.playWhenReady = true
            "pause" -> player.playWhenReady = false
            "back" -> seekToPosition(maxOf(0L, position() - 10_000))
            "forward" -> seekToPosition(position() + 30_000)
            "seek" -> seekToPosition((c.optDouble("pos", 0.0) * 1000).toLong())
            "stop" -> end("back")
        }
        showControls()
        reportScreen(force = true)
    }
    private val hider = Runnable { if (player.isPlaying) hideControls() }

    // A frozen picture: the sound plays on but no new frame has been shown for
    // a few seconds (a TV's video decoder that stuck). The player jumps to
    // where it is, which starts the decoder again; if it keeps happening, it
    // suggests another version. (Not when tunneled: the TV's chip draws the
    // frames itself and they aren't counted.)
    private var frozenFrames = -1
    private var frozenSince = 0L
    private val frozenAt = ArrayDeque<Long>()
    private val freezeWatch = object : Runnable {
        override fun run() {
            main.postDelayed(this, 2000)
            if (!player.isPlaying || player.playbackState != Player.STATE_READY || live || !canChangeRefreshRate()) {
                frozenFrames = -1
                return
            }
            val counters = player.videoDecoderCounters ?: return
            counters.ensureUpdated()
            val rendered = counters.renderedOutputBufferCount
            val now = SystemClock.elapsedRealtime()
            if (rendered != frozenFrames) {
                frozenFrames = rendered
                frozenSince = now
                return
            }
            if (now - frozenSince < 4000) return
            // Stuck: start the decoder again where the sound is.
            frozenFrames = -1
            frozenAt.addLast(now)
            while (frozenAt.isNotEmpty() && now - frozenAt.first() > 120_000) frozenAt.removeFirst()
            partyApplying = now + 1500
            player.seekTo(player.currentPosition)
            if (frozenAt.size >= 3) {
                frozenAt.clear()
                Toast.makeText(this@PlayerActivity, "The picture keeps freezing with this version. Try Other version.", Toast.LENGTH_LONG).show()
                if (hasOther) showControls(otherButton)
            }
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        CrashLog.install(this)
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

        surround = getSharedPreferences("player", MODE_PRIVATE).getBoolean("surround", true)
        player = buildPlayer()

        video = PlayerView(this).apply {
            player = this@PlayerActivity.player
            useController = false
            resizeMode = AspectRatioFrameLayout.RESIZE_MODE_FIT
            setShutterBackgroundColor(Color.BLACK)
            setKeepContentOnPlayerReset(true)
            styleSubtitles(subtitleView)
        }
        // Loading shows the Cue mark gently breathing (growing and fading a
        // touch), not a spinning ring.
        val mark = ImageView(this).apply {
            setImageResource(R.drawable.ic_cue_mark)
            visibility = View.GONE
        }
        android.animation.ValueAnimator.ofFloat(0f, 1f).apply {
            duration = 900
            repeatMode = android.animation.ValueAnimator.REVERSE
            repeatCount = android.animation.ValueAnimator.INFINITE
            interpolator = android.view.animation.AccelerateDecelerateInterpolator()
            addUpdateListener { a ->
                val t = a.animatedValue as Float
                mark.alpha = 0.55f + 0.45f * t
                mark.scaleX = 0.92f + 0.12f * t
                mark.scaleY = 0.92f + 0.12f * t
            }
            start()
        }
        spinner = mark
        pauseBadge = ImageView(this).apply {
            setImageDrawable(getDrawable(R.drawable.ic_cue_pause)?.mutate()?.apply { setTint(Color.WHITE) })
            setPadding(px(16), px(16), px(16), px(16))
            background = GradientDrawable().apply {
                shape = GradientDrawable.OVAL
                setColor(0x59000000)
            }
            visibility = View.GONE
        }
        controls = buildControls()

        val root = FrameLayout(this).apply {
            setBackgroundColor(Color.BLACK)
            addView(video, FrameLayout.LayoutParams(MATCH, MATCH))
            val dim = View(this@PlayerActivity).apply {
                setBackgroundColor(0x66000000)
                visibility = View.GONE
            }
            pauseDim = dim
            addView(dim, FrameLayout.LayoutParams(MATCH, MATCH))
            val startView = buildStartScreen()
            startScreen = startView
            addView(startView, FrameLayout.LayoutParams(MATCH, MATCH))
            addView(spinner, FrameLayout.LayoutParams(px(40), px(40), Gravity.CENTER))
            addView(pauseBadge, FrameLayout.LayoutParams(px(72), px(72), Gravity.CENTER))
            addView(controls, FrameLayout.LayoutParams(MATCH, MATCH))
            skipButton = buildSkipButton()
            val fb = buildFrameBox()
            frameBox = fb
            addView(fb, FrameLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.BOTTOM or Gravity.START).apply {
                bottomMargin = px(118)
            })
            buildGuide()?.let {
                guide = it
                addView(it, FrameLayout.LayoutParams(px(400), MATCH, Gravity.START))
            }
            addView(skipButton, FrameLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, px(42), Gravity.BOTTOM or Gravity.END).apply {
                marginEnd = px(44)
                bottomMargin = px(112)
            })
            val floats = FrameLayout(this@PlayerActivity)
            partyFloats = floats
            addView(floats, FrameLayout.LayoutParams(MATCH, MATCH))
            // The party: a small card in the top corner, not a strip down the side.
            addView(buildPartyPanel(), FrameLayout.LayoutParams(px(200), ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.TOP or Gravity.END).apply {
                topMargin = px(16)
                marginEnd = px(16)
            })
            addView(TextView(this@PlayerActivity).also { partyWait = it }.apply {
                setTextColor(0xFFFFFFFF.toInt())
                textSize = 18f
                setPadding(px(22), px(10), px(22), px(10))
                background = GradientDrawable().apply {
                    cornerRadius = px(30).toFloat()
                    setColor(0xB3000000.toInt())
                }
                visibility = View.GONE
            }, FrameLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.TOP or Gravity.CENTER_HORIZONTAL).apply {
                topMargin = px(28)
            })
        }
        setContentView(root)

        listener = object : Player.Listener {
            override fun onPlaybackStateChanged(state: Int) {
                if (state == Player.STATE_READY && !skipsAsked && !live && !catchup) {
                    skipsAsked = true
                    fetchSkips()
                }
                spinner.visibility = if (state == Player.STATE_BUFFERING) View.VISIBLE else View.GONE
                if (state == Player.STATE_ENDED) {
                    when {
                        live -> end("error", "The channel stopped.")
                        // The next episode is ready: a short count here, then it plays
                        // in this player (no trip back to Home).
                        nextReady != null && countdownLeft < 0 && !partyFollower() -> startNextCountdown()
                        countdownLeft >= 0 -> Unit
                        else -> end("ended", party?.code ?: "")
                    }
                }
            }

            override fun onRenderedFirstFrame() {
                matchRefreshRate()
                hideStartScreen()
            }

            override fun onIsPlayingChanged(isPlaying: Boolean) {
                if (isPlaying) {
                    started = true
                    hideStartScreen()
                }
                playPause.setCompoundDrawablesRelative(ctrlIcon(if (isPlaying) R.drawable.ic_cue_pause else R.drawable.ic_cue_play, big = true), null, null, null)
                val paused = !isPlaying && player.playbackState == Player.STATE_READY
                // Paused: the picture dims and the pause mark pops in, softly;
                // playing again, both fade away.
                showSoftly(pauseBadge, paused, pop = true)
                pauseDim?.let { showSoftly(it, paused) }
                if (!isPlaying) cancelCountdown()
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
                matchRefreshRate()
                // By width: a film cropped wide is 1920 x 800 and still 1080p.
                val label = when {
                    w >= 3200 -> "4K"
                    w >= 1800 -> "1080p"
                    w >= 1200 -> "720p"
                    else -> "SD"
                }
                pictureLabel = label
                showPictureLabel()
            }

            override fun onPlayerError(error: PlaybackException) {
                if (surround && (error.errorCode == PlaybackException.ERROR_CODE_AUDIO_TRACK_INIT_FAILED ||
                        error.errorCode == PlaybackException.ERROR_CODE_AUDIO_TRACK_WRITE_FAILED)) {
                    // The TV or receiver said it takes this sound and then
                    // didn't: Cue decodes it instead, for this video.
                    setSurround(false, remember = false)
                    return
                }
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
                if (started && !live) {
                    // Once the picture has played, Cue never swaps the
                    // version by itself: a dropped stream is picked up where
                    // it was (twice), then the viewer chooses.
                    if (hiccups < 2) {
                        hiccups++
                        val pos = player.currentPosition
                        Toast.makeText(this@PlayerActivity, "Lost the stream for a moment, picking up where it was…", Toast.LENGTH_SHORT).show()
                        player.prepare()
                        player.seekTo(pos)
                        player.playWhenReady = true
                        return
                    }
                    val msg = if (hasOther) "This version keeps stopping. Choose Other version to try another, or play to try again." else "This version keeps stopping. Press play to try again."
                    Toast.makeText(this@PlayerActivity, msg, Toast.LENGTH_LONG).show()
                    showControls(otherButton)
                    return
                }
                end("error", error.errorCodeName)
            }
        }
        player.addListener(listener)

        player.setMediaItem(mediaItem(info.optString("subtitleUrl")))
        val start = (info.optDouble("startSec", 0.0) * 1000).toLong()
        if (start > 0 && !live) player.seekTo(start)
        player.prepare()
        player.playWhenReady = true
        if (!live && !catchup) main.postDelayed(saver, SAVE_EVERY_MS)
        main.post(ticker)
        main.postDelayed(freezeWatch, 4000)
        showControls()
        // A new title from a phone: this player hands over to Home, which opens it.
        ScreenLink.onPlayInPlayer = { t ->
            end("remote-play", "${t.optString("kind")}|${t.optInt("tmdbId")}|${t.optInt("season")}|${t.optInt("episode")}")
        }
        if (!live) {
            ScreenLink.onControl = { onRemote(it) }
            player.addListener(object : Player.Listener {
                override fun onIsPlayingChanged(isPlaying: Boolean) = reportScreen(force = true)
            })
            main.post(screenTicker)
        }
        if (!live && !catchup) {
            player.addListener(partyListener)
            info.optString("party").takeIf { it.isNotEmpty() }?.let { joinParty(it) }
        }
    }

    // ---- Watch party ----

    private fun partyTitle(): JSONObject = JSONObject()
        .put("kind", info.optString("kind"))
        .put("tmdbId", info.optInt("tmdbId"))
        .put("season", info.optInt("season"))
        .put("episode", info.optInt("episode"))
        .put("title", info.optString("title"))

    private fun posSec(): Double = position() / 1000.0

    private fun partyLink(code: String) = PartyLink(server, info.optString("token"), code,
        onState = { onPartyState(it) },
        onMsg = { onPartyMsg(it) },
        onProblem = { msg -> addPartyLine(msg, event = true) },
    )

    private fun joinParty(code: String) {
        val link = partyLink(code)
        party = link
        showPartyPanel(true)
        link.start { joined(it) }
    }

    // The Watch party button: start one (the household sees it on Home), or
    // show and hide its panel.
    private fun partyPressed() {
        val link = party
        if (link != null) {
            showPartyPanel(partyPanel?.visibility != View.VISIBLE)
            return
        }
        val l = partyLink("")
        party = l
        showPartyPanel(true)
        addPartyLine("Starting a watch party…", event = true)
        partyHost = true
        l.create(partyTitle(), posSec(), player.playWhenReady) { joined(it) }
    }

    private fun joined(state: JSONObject) {
        main.removeCallbacks(partyDrift)
        main.postDelayed(partyDrift, 1000)
        // Still loading as it joins: the party waits for it.
        if (player.playbackState != Player.STATE_READY) {
            partyWaitSaid = true
            party?.send("wait", posSec())
        }
        // This TV started the party: everyone plays the party's version (the
        // one every device opens most easily). When that isn't this one, the
        // TV moves to it, from the same moment, so all are on the same cut.
        if (partyHost) party?.let { l ->
            val kind = info.optString("kind")
            val path = if (kind == "tv") "/api/play/tmdb/tv/${info.optInt("tmdbId")}/${info.optInt("season")}/${info.optInt("episode")}" else "/api/play/tmdb/movie/${info.optInt("tmdbId")}"
            Thread {
                val rel = runCatching { l.partyRelease(path) }.getOrDefault("")
                if (rel.isNotEmpty() && rel != info.optString("release")) main.post {
                    end("party", "${l.code}|${info.optInt("tmdbId")}|${info.optInt("season")}|${info.optInt("episode")}|$kind")
                }
            }.start()
        }
        reactButton?.visibility = View.VISIBLE
        partyButton?.text = "Party"
        onPartyState(state, first = true)
        party?.let { l ->
            partyQr?.setImageBitmap(app.cue.tv.home.qrBitmap("$server/watch/party/${l.code}?chat=1", px(92)))
            if (listenMode) showListenQr(l.code)
            // This TV moved on to another episode (Next): the party follows.
            val t = state.optJSONObject("title")
            if (isShow && t != null && (t.optInt("season") != info.optInt("season") || t.optInt("episode") != info.optInt("episode"))) {
                Thread { runCatching { l.sendEpisode(partyTitle()) } }.start()
            }
        }
    }

    private val partyListener = object : Player.Listener {
        override fun onPlayWhenReadyChanged(playWhenReady: Boolean, reason: Int) {
            if (reason == Player.PLAY_WHEN_READY_CHANGE_REASON_USER_REQUEST && SystemClock.elapsedRealtime() > partyApplying) {
                party?.send(if (playWhenReady) "play" else "pause", posSec())
            }
        }

        override fun onPositionDiscontinuity(oldPosition: Player.PositionInfo, newPosition: Player.PositionInfo, reason: Int) {
            if (reason == Player.DISCONTINUITY_REASON_SEEK && SystemClock.elapsedRealtime() > partyApplying) {
                party?.send("seek", posSec())
            }
        }

        // Wait for everyone: loading for more than a moment while the party
        // plays tells it to wait (for half a minute at most); ready again
        // lets it go on.
        override fun onPlaybackStateChanged(state: Int) {
            val link = party ?: return
            main.removeCallbacks(partyLoading)
            when (state) {
                Player.STATE_BUFFERING -> main.postDelayed(partyLoading, 800)
                Player.STATE_READY -> if (partyWaitSaid) {
                    partyWaitSaid = false
                    link.send("ready", posSec())
                }
            }
        }
    }

    private val partyLoading = Runnable {
        val link = party
        if (link != null && !partyWaitSaid && player.playbackState == Player.STATE_BUFFERING && partyState?.optBoolean("playing") == true) {
            partyWaitSaid = true
            link.send("wait", posSec())
        }
    }

    private fun onPartyState(s: JSONObject, first: Boolean = false) {
        val link = party ?: return
        partyState = s
        partyGot = SystemClock.elapsedRealtime()
        val members = s.optJSONArray("members")
        val names = (0 until (members?.length() ?: 0)).mapNotNull { members?.optJSONObject(it)?.optString("name") }
        partyHead?.text = "Watch party · ${names.size} watching\n${names.joinToString(", ")}"
        // Held: who everyone waits for.
        val waiting = if (s.optBoolean("held")) (0 until (members?.length() ?: 0)).mapNotNull { i ->
            members?.optJSONObject(i)?.takeIf { it.optBoolean("waiting") }?.optString("name")
        } else emptyList()
        partyWait?.let {
            it.text = "Waiting for ${waiting.joinToString(", ")}…"
            it.visibility = if (waiting.isEmpty()) View.GONE else View.VISIBLE
        }
        val t = s.optJSONObject("title") ?: return
        // Word still naming the episode this TV just moved on from (sent
        // before the party heard about Next): not a reason to go back to it,
        // which played the same episode over again.
        val l = leftEpisode
        if (l != null && SystemClock.elapsedRealtime() - leftAt < 15_000 &&
            t.optInt("season") == l.first && t.optInt("episode") == l.second) return
        if (isShow && (t.optInt("season") != info.optInt("season") || t.optInt("episode") != info.optInt("episode"))) {
            if (!first) end("party", "${link.code}|${t.optInt("tmdbId")}|${t.optInt("season")}|${t.optInt("episode")}")
            return
        }
        if (first || s.optString("by") != link.client) applyParty(s)
    }

    // Brings the player in line with the party.
    private fun applyParty(s: JSONObject, jumpFrom: Long = 4000) {
        // Held (waiting for someone's player to load) counts as paused.
        val playing = s.optBoolean("playing") && !s.optBoolean("held")
        val elapsed = if (playing) SystemClock.elapsedRealtime() - partyGot else 0L
        val target = (s.optDouble("pos", 0.0) * 1000).toLong() + elapsed
        val off = kotlin.math.abs(position() - target)
        // While playing, small differences are closed by speed (partyDrift).
        if (off > (if (playing) jumpFrom else 300)) {
            partyApplying = SystemClock.elapsedRealtime() + 1500
            seekToPosition(target)
        }
        if (player.playWhenReady != playing) {
            partyApplying = SystemClock.elapsedRealtime() + 1500
            player.playWhenReady = playing
        }
    }

    // Drift: a TV a little behind or ahead catches up by playing a touch
    // faster or slower (nobody notices 5%), with no jump; only a big
    // difference is jumped.
    private val partyDrift = object : Runnable {
        override fun run() {
            val s = partyState
            var speed = 1f
            if (party != null && s != null && s.optBoolean("playing") && !s.optBoolean("held") && player.isPlaying && SystemClock.elapsedRealtime() > partyApplying) {
                val target = (s.optDouble("pos", 0.0) * 1000).toLong() + (SystemClock.elapsedRealtime() - partyGot)
                val off = position() - target // above 0: ahead
                val far = kotlin.math.abs(off)
                when {
                    // With surround sent on as it is (Dolby, DTS to the TV or
                    // receiver), the sound can't change speed and the picture
                    // stalls behind it: small jumps instead.
                    surround && far > 1500 -> applyParty(s, jumpFrom = 1500)
                    surround -> Unit
                    far > 4000 -> applyParty(s)
                    far >= 200 -> speed = if (off < 0) (if (far > 2000) 1.1f else 1.05f) else (if (far > 2000) 0.9f else 0.95f)
                }
            }
            if (party != null && player.playbackParameters.speed != speed) player.setPlaybackSpeed(speed)
            main.postDelayed(this, 1000)
        }
    }

    private fun onPartyMsg(m: JSONObject) {
        val text = m.optString("text")
        when (m.optString("kind")) {
            "reaction" -> floatReaction(text, m.optString("name"))
            "event" -> addPartyLine(text, event = true)
            else -> addPartyLine("${m.optString("name")}: $text", event = false)
        }
    }

    private fun addPartyLine(text: String, event: Boolean) {
        val lines = partyLines ?: return
        lines.addView(TextView(this).apply {
            this.text = text
            setTextColor(if (event) 0xFFA6A8AE.toInt() else Color.WHITE)
            size(if (event) 8.5f else 10f)
            typeface = Fonts.text(this@PlayerActivity, if (event) 400 else 500)
            maxLines = 3
            setPadding(0, px(2), 0, px(2))
            alpha = 0f
            animate().alpha(1f).setDuration(250).start()
        })
        while (lines.childCount > 4) lines.removeViewAt(0)
        if (!event || partyPanel?.visibility == View.VISIBLE) showPartyPanel(true)
    }

    // A reaction, as in a Teams meeting: it pops up on the picture with who
    // sent it (a little bounce), drifts up with a gentle sway, and fades away.
    private fun floatReaction(emoji: String, name: String) {
        val layer = partyFloats ?: return
        val v = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            gravity = Gravity.CENTER_HORIZONTAL
            addView(TextView(this@PlayerActivity).apply {
                text = emoji
                size(34f)
                setShadowLayer(14f, 0f, 4f, 0x73000000)
            })
            addView(TextView(this@PlayerActivity).apply {
                text = name
                setTextColor(Color.WHITE)
                size(10f)
                typeface = Fonts.text(this@PlayerActivity, 700)
                setPadding(px(9), px(2), px(9), px(3))
                background = GradientDrawable().apply {
                    cornerRadius = px(10).toFloat()
                    setColor(0xB8141418.toInt())
                }
            })
        }
        val lp = FrameLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.BOTTOM or Gravity.START).apply {
            leftMargin = px((60..380).random())
            bottomMargin = px(130)
        }
        layer.addView(v, lp)
        val sway = if (Math.random() < 0.5) -1f else 1f
        val rise = px(300).toFloat()
        val wobble = px(14).toFloat()
        val pop = android.view.animation.OvershootInterpolator(2.2f)
        val ease = android.view.animation.DecelerateInterpolator(1.4f)
        android.animation.ValueAnimator.ofFloat(0f, 1f).apply {
            duration = 4000
            addUpdateListener { a ->
                val t = a.animatedValue as Float
                val grow = pop.getInterpolation((t / 0.14f).coerceAtMost(1f))
                v.scaleX = 0.25f + 0.75f * grow
                v.scaleY = v.scaleX
                v.translationY = -rise * ease.getInterpolation(t)
                v.translationX = sway * wobble * kotlin.math.sin(t * 2.4f * Math.PI.toFloat())
                v.alpha = if (t < 0.7f) (t / 0.06f).coerceAtMost(1f) else 1f - (t - 0.7f) / 0.3f
            }
            addListener(object : android.animation.AnimatorListenerAdapter() {
                override fun onAnimationEnd(animation: android.animation.Animator) {
                    layer.removeView(v)
                }
            })
            start()
        }
    }

    // The card shows for a few seconds on news (a chat line, someone joining)
    // and while the controls are up, then gets out of the way.
    private val partyHide = Runnable { if (player.isPlaying && !controlsShowing()) showPartyPanel(false) }
    private fun showPartyPanel(show: Boolean) {
        partyPanel?.let { showSoftly(it, show) }
        main.removeCallbacks(partyHide)
        if (show) main.postDelayed(partyHide, 8000)
    }

    // The party's panel, at the right: who's watching, the last few lines of
    // the chat, and a QR code that opens the chat on a phone (typing with a
    // remote is no fun).
    private fun buildPartyPanel(): View = LinearLayout(this).also { partyPanel = it }.apply {
        orientation = LinearLayout.VERTICAL
        visibility = View.GONE
        setPadding(px(11), px(9), px(11), px(10))
        background = GradientDrawable().apply {
            cornerRadius = px(12).toFloat()
            setColor(0xCC0B0C0F.toInt())
        }
        addView(TextView(this@PlayerActivity).also { partyHead = it }.apply {
            text = "Watch party"
            setTextColor(Color.WHITE)
            size(10f)
            typeface = Fonts.text(this@PlayerActivity, 600)
            maxLines = 2
        })
        addView(LinearLayout(this@PlayerActivity).also { partyLines = it }.apply {
            orientation = LinearLayout.VERTICAL
            setPadding(0, px(4), 0, px(6))
        }, LinearLayout.LayoutParams(MATCH, ViewGroup.LayoutParams.WRAP_CONTENT))
        addView(LinearLayout(this@PlayerActivity).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
            addView(ImageView(this@PlayerActivity).also { partyQr = it }, LinearLayout.LayoutParams(px(46), px(46)))
            addView(TextView(this@PlayerActivity).apply {
                text = "Scan to chat\nfrom your phone"
                setTextColor(0xFFA6A8AE.toInt())
                size(8f)
                typeface = Fonts.text(this@PlayerActivity)
                setPadding(px(8), 0, 0, 0)
            })
        })
    }

    // React: a row of reactions under the controls, for the remote.
    private fun showReactions() {
        val row = reactRow ?: return
        val show = row.visibility != View.VISIBLE
        row.visibility = if (show) View.VISIBLE else View.GONE
        if (show) row.getChildAt(0)?.requestFocus()
        showControls()
    }

    private fun emojiPill(e: String): TextView = TextView(this).apply {
        text = e
        size(18f)
        gravity = Gravity.CENTER
        setPadding(px(10), px(4), px(10), px(4))
        background = StateListDrawable().apply {
            addState(intArrayOf(android.R.attr.state_focused), GradientDrawable().apply {
                cornerRadius = px(17).toFloat()
                setColor(Color.WHITE)
            })
            addState(intArrayOf(), GradientDrawable().apply { setColor(Color.TRANSPARENT) })
        }
        isFocusable = true
        isFocusableInTouchMode = true
        setOnClickListener { party?.react(e) }
        layoutParams = LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, px(40)).apply { marginEnd = px(4) }
    }

    // ---- The controls ----

    // The controls: the title at the top, and at the bottom a rounded,
    // see-through panel: the time and the bar, then play and the jumps on
    // the left, the speeds in a group, and Next episode, Versions, Audio,
    // Subtitles and Surround on the right (icons that show their name when
    // the remote is on them).
    private fun buildControls(): View {
        val title = TextView(this).also { titleText = it }.apply {
            text = info.optString("title")
            setTextColor(Color.WHITE)
            size(16f)
            typeface = Fonts.display(this@PlayerActivity)
            setShadowLayer(8f, 0f, 2f, Color.BLACK)
        }
        val sub = TextView(this).also { subtitle = it }.apply {
            text = info.optString("subtitle")
            setTextColor(0xFFC7C8CC.toInt())
            size(11f)
            typeface = Fonts.text(this@PlayerActivity)
            setPadding(0, px(3), 0, 0)
            visibility = if (text.isNullOrEmpty()) View.GONE else View.VISIBLE
        }
        val top = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(px(44), px(22), px(44), px(40))
            background = GradientDrawable(GradientDrawable.Orientation.TOP_BOTTOM, intArrayOf(0xB3000000.toInt(), 0x00000000))
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
        timeNow = TextView(this).apply {
            setTextColor(0xFFE6E7EA.toInt())
            size(11f)
            typeface = Fonts.mono(this@PlayerActivity)
        }
        timeLeft = TextView(this).apply {
            setTextColor(0xFFE6E7EA.toInt())
            size(11f)
            typeface = Fonts.mono(this@PlayerActivity)
        }
        val timeRow = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
            if (live) {
                addView(liveBadge())
                addView(TextView(this@PlayerActivity).also { liveNowText = it }.apply {
                    text = info.optString("subtitle")
                    setTextColor(0xFFE6E7EA.toInt())
                    size(13f)
                    typeface = Fonts.text(this@PlayerActivity, 500)
                    maxLines = 1
                    setPadding(px(14), 0, 0, 0)
                })
            } else {
                addView(timeNow, LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT).apply { marginEnd = px(14) })
                addView(timeBar, LinearLayout.LayoutParams(0, ViewGroup.LayoutParams.WRAP_CONTENT, 1f))
                addView(timeLeft, LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, ViewGroup.LayoutParams.WRAP_CONTENT).apply { marginStart = px(14) })
            }
        }

        playPause = pill("", R.drawable.ic_cue_pause) { togglePlay() }
        val buttons = LinearLayout(this).apply {
            orientation = LinearLayout.HORIZONTAL
            gravity = Gravity.CENTER_VERTICAL
            setPadding(0, px(8), 0, 0)
            addView(playPause)
            if (!live) {
                addView(pill("", R.drawable.ic_cue_replay) { seekBy(-10_000) })
                addView(pill("", R.drawable.ic_cue_forward) { seekBy(10_000) })
            }
            addView(View(context), LinearLayout.LayoutParams(0, 1, 1f))
            // A short row: what's used most, the rest in two menus (Sound,
            // and More) instead of a dozen icons, several of them the same.
            if (isShow && !live && !catchup) {
                nextButton = pill("Next episode", R.drawable.ic_cue_next) {
                    learnCredits()
                    goNext()
                }.also { addView(it) }
            }
            if (hasOther && !live && !catchup) {
                otherButton = iconPill("Other version", R.drawable.ic_cue_versions) { end("other") }.also { addView(it) }
            }
            if (!live && !catchup) {
                // Kept for their labels (the menus read them); not on the row.
                partyButton = iconPill("Watch party", R.drawable.ic_cue_party) { partyPressed() }
                listenButton = iconPill("Listen on phone", R.drawable.ic_cue_audio) { listenOnPhone() }
                reactButton = iconPill("React", R.drawable.ic_cue_react) { showReactions() }.also {
                    it.visibility = View.GONE // shown in a watch party
                    addView(it)
                }
            }
            boostButton = iconPill("Volume boost", R.drawable.ic_cue_audio) { toggleBoost() }
            soundButton = iconPill(soundLabel(), R.drawable.ic_cue_audio) { setSurround(!surround) }
            addView(iconPill("Subtitles", R.drawable.ic_cue_subtitles) { subtitlesPressed() })
            addView(iconPill("Sound", R.drawable.ic_cue_sound) { soundMenu() })
            addView(iconPill("More", R.drawable.ic_cue_more) { moreMenu() })
        }
        val panel = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(0, 0, 0, 0)
            addView(timeRow)
            addView(buttons)
            addView(LinearLayout(this@PlayerActivity).also { reactRow = it }.apply {
                orientation = LinearLayout.HORIZONTAL
                gravity = Gravity.CENTER
                visibility = View.GONE
                setPadding(0, px(6), 0, 0)
                PartyLink.REACTIONS.forEach { addView(emojiPill(it)) }
            })
        }
        // A soft shade behind the panel, so it reads over a bright picture.
        val shade = View(this).apply {
            background = GradientDrawable(GradientDrawable.Orientation.BOTTOM_TOP, intArrayOf(0xD9000000.toInt(), 0x80000000.toInt(), 0x00000000))
        }

        return FrameLayout(this).apply {
            addView(top, FrameLayout.LayoutParams(MATCH, ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.TOP))
            addView(shade, FrameLayout.LayoutParams(MATCH, px(170), Gravity.BOTTOM))
            addView(panel, FrameLayout.LayoutParams(MATCH, ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.BOTTOM).apply {
                setMargins(px(44), 0, px(44), px(18))
            })
        }
    }

    // The speeds, as a group of chips: the one playing is lit, the one the
    // remote is on is white.
    // A list to pick from, in the same look as the audio and subtitle
    // choosers; the controls stay up while it's open.
    private fun menu(title: String, items: List<Pair<String, () -> Unit>>) {
        main.removeCallbacks(hider)
        android.app.AlertDialog.Builder(this, android.R.style.Theme_DeviceDefault_Dialog_Alert)
            .setTitle(title)
            .setItems(items.map { it.first }.toTypedArray()) { d, i ->
                d.dismiss()
                items[i].second()
            }
            .setOnDismissListener { scheduleHide() }
            .show()
    }

    // Sound: the language or track, Surround, Volume boost, Listen on phone.
    private fun soundMenu() = menu("Sound", buildList {
        add("Audio track…" to { chooseTrack(C.TRACK_TYPE_AUDIO, "Audio") })
        add((if (surround) "Surround: on" else "Surround: off") to { setSurround(!surround) })
        add((if (booster != null) "Volume boost: on" else "Volume boost: off") to { toggleBoost() })
        listenButton?.let { b -> add(((b.tag as? String) ?: "Listen on phone") to { listenOnPhone() }) }
    })

    // More: speed, the versions, a watch party, and reporting a problem.
    private fun moreMenu() = menu("More", buildList {
        if (!live) add("Speed: ${speedText(speed)}" to { speedMenu() })
        if (hasOther && !live && !catchup) add("Other version" to { end("other") })
        if (!live && !catchup && info.optString("release").isNotEmpty()) add("Wrong video (never use this one again)" to { end("wrong", info.optString("release")) })
        partyButton?.let { b -> add((if (b.text.isNullOrEmpty() || b.text == "Watch party") "Watch party" else "Party") to { partyPressed() }) }
        if (!live && !catchup) add("Report a problem" to { reportProblem() })
    })

    private fun speedText(sp: Float) = (if (sp % 1f == 0f) "${sp.toInt()}" else "$sp") + "×"

    private fun speedMenu() = menu("Speed", listOf(0.5f, 0.75f, 1f, 1.25f, 1.5f, 2f).map { sp ->
        (speedText(sp) + if (sp == speed) "  ✓" else "") to {
            speed = sp
            player.setPlaybackSpeed(sp)
        }
    })

    // iconPill is a round icon button that shows its name while the remote
    // is on it, so the row stays tidy.
    private fun iconPill(label: String, icon: Int, onClick: () -> Unit): TextView = pill("", icon, onClick).apply {
        tag = label
        setOnFocusChangeListener { v, has ->
            val t = v as TextView
            t.text = if (has) (t.tag as? String).orEmpty() else ""
            t.compoundDrawablePadding = if (has) px(6) else 0
            t.setPadding(px(9), px(5), px(if (has) 13 else 9), px(5))
            t.animate().scaleX(if (has) 1.06f else 1f).scaleY(if (has) 1.06f else 1f).setDuration(120).start()
        }
    }

    private fun liveBadge(): View = TextView(this).apply {
        text = "●  LIVE"
        setTextColor(Color.WHITE)
        size(10f)
        typeface = Fonts.text(this@PlayerActivity, 700)
        letterSpacing = 0.08f
        setPadding(px(12), px(5), px(12), px(5))
        background = GradientDrawable().apply {
            cornerRadius = px(4).toFloat()
            setColor(0x33000000)
            setStroke(px(2), teal)
        }
    }

    // An icon for the bottom row, white whatever the box: the colour is put
    // into the drawable itself (some Fire TVs ignore a TextView's icon tint,
    // which drew a white icon on a white button: a plain white circle).
    private fun ctrlIcon(icon: Int, big: Boolean = false) = getDrawable(icon)?.mutate()?.apply {
        setTint(Color.WHITE)
        val n = px(if (big) 18 else 15)
        setBounds(0, 0, n, n)
    }

    // pill is a button in the bottom row, as in the web player: a bare white
    // icon (and label); the one the remote is on gets a soft light ring and
    // grows a touch, so it's clear without shouting.
    private fun pill(label: String, icon: Int, onClick: () -> Unit): TextView = TextView(this).apply {
        text = label
        size(11f)
        typeface = Fonts.text(this@PlayerActivity, 500)
        setTextColor(Color.WHITE)
        setShadowLayer(6f, 0f, 1f, 0x99000000.toInt())
        setCompoundDrawablesRelative(ctrlIcon(icon, big = label.isEmpty()), null, null, null)
        compoundDrawablePadding = if (label.isEmpty()) 0 else px(6)
        gravity = Gravity.CENTER
        setPadding(px(9), px(5), px(if (label.isEmpty()) 9 else 13), px(5))
        background = StateListDrawable().apply {
            addState(intArrayOf(android.R.attr.state_focused), GradientDrawable().apply {
                cornerRadius = px(16).toFloat()
                setColor(0x33FFFFFF)
                setStroke(px(2), 0xE6FFFFFF.toInt())
            })
            addState(intArrayOf(), GradientDrawable().apply {
                cornerRadius = px(16).toFloat()
                setColor(Color.TRANSPARENT)
            })
        }
        isFocusable = true
        isFocusableInTouchMode = true
        setOnClickListener { onClick() }
        layoutParams = LinearLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT, px(30)).apply { marginEnd = px(4) }
    }

    // Sound: the box's own decoders first, the app's FFmpeg ones only for
    // what the box can't decode (DTS, TrueHD); the picture likewise, since
    // FFmpeg decoding it on the box is soft, can't deinterlace live TV and
    // can't keep the voices in step. With Surround on, Dolby and DTS go to
    // the TV or receiver as they are when it says it takes them, so they
    // reach surround speakers; off, Cue decodes the sound itself, for a TV
    // that says it takes Dolby and then plays nothing.
    private fun buildPlayer(): ExoPlayer {
        val passthrough = surround
        val renderers = object : DefaultRenderersFactory(this) {
            override fun buildAudioSink(context: Context, enableFloatOutput: Boolean, enableAudioTrackPlaybackParams: Boolean): AudioSink {
                val sink = DefaultAudioSink.Builder(context)
                    .setEnableFloatOutput(enableFloatOutput)
                    .setEnableAudioTrackPlaybackParams(enableAudioTrackPlaybackParams)
                if (!passthrough) sink.setAudioCapabilities(AudioCapabilities.DEFAULT_AUDIO_CAPABILITIES)
                return sink.build()
            }

            // The picture only ever on the TV's video chip: FFmpeg would take
            // a 4K film the chip says is beyond it (often it isn't), and
            // decoding 4K on the processor stutters and drops frames.
            override fun buildVideoRenderers(
                context: Context,
                extensionRendererMode: Int,
                mediaCodecSelector: MediaCodecSelector,
                enableDecoderFallback: Boolean,
                eventHandler: Handler,
                eventListener: VideoRendererEventListener,
                allowedVideoJoiningTimeMs: Long,
                out: ArrayList<Renderer>,
            ) = super.buildVideoRenderers(
                context, DefaultRenderersFactory.EXTENSION_RENDERER_MODE_OFF, mediaCodecSelector, enableDecoderFallback,
                eventHandler, eventListener, allowedVideoJoiningTimeMs, out,
            )
        }
            .setEnableDecoderFallback(true)
            .setExtensionRendererMode(DefaultRenderersFactory.EXTENSION_RENDERER_MODE_ON)
        // Debrid links often redirect, sometimes from http to https.
        val http = DefaultHttpDataSource.Factory()
            .setAllowCrossProtocolRedirects(true)
            .setConnectTimeoutMs(15000)
            .setReadTimeoutMs(30000)
            .setUserAgent("CueTV/1")
        // Tunneled when the TV can: the picture goes from the video chip
        // straight to the screen, timed by the chip, as Netflix and IPTV
        // players do. Live TV always (it deinterlaces channels properly);
        // films and shows when the TV can't change its refresh rate (many
        // TCL and other built-in Google TVs stay at 60 Hz), where the chip
        // timing the frames is what keeps 24 fps films smooth. Where the TV
        // can change it, they aren't tunneled, so the frame rate can be
        // measured and the TV switched (matchRefreshRate). The player turns
        // tunneling off by itself when the TV or the sound can't do it.
        val selector = DefaultTrackSelector(this).apply {
            setParameters(buildUponParameters().setTunnelingEnabled(live || !canChangeRefreshRate()))
        }
        val p = ExoPlayer.Builder(this, renderers)
            .setTrackSelector(selector)
            .setMediaSourceFactory(DefaultMediaSourceFactory(http))
            .setSeekBackIncrementMs(10_000)
            .setSeekForwardIncrementMs(10_000)
            .build()
        // Subtitles follow the last choice on this TV (Off stays off for the
        // next episode); with none made, they show when on in Watch. A file's
        // own "default" subtitle doesn't switch them on by itself; forced ones
        // (lines in another language) still show.
        val subsPref = getSharedPreferences("player", MODE_PRIVATE).getString("subs", "") ?: ""
        p.trackSelectionParameters = p.trackSelectionParameters.buildUpon()
            .setPreferredAudioLanguage("en")
            .setIgnoredTextSelectionFlags(C.SELECTION_FLAG_DEFAULT)
            .setPreferredTextLanguage(when {
                subsPref == "off" -> null
                subsPref.isNotEmpty() -> subsPref
                info.optBoolean("subsOn") && info.optString("subtitleUrl").isNotEmpty() -> "en"
                else -> null
            })
            .setTrackTypeDisabled(C.TRACK_TYPE_TEXT, subsPref == "off")
            .build()
        // The frame rate is asked for in matchRefreshRate, in a way the TV
        // follows even when switching isn't seamless.
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) p.videoChangeFrameRateStrategy = C.VIDEO_CHANGE_FRAME_RATE_STRATEGY_OFF
        // Many files (MKV especially) don't say their frame rate: it's
        // measured from the gaps between the first frames' times instead.
        p.setVideoFrameMetadataListener(object : VideoFrameMetadataListener {
            private val gaps = ArrayList<Long>()
            private var last = -1L
            override fun onVideoFrameAboutToBeRendered(presentationTimeUs: Long, releaseTimeNs: Long, format: Format, mediaFormat: MediaFormat?) {
                if (gaps.size >= FPS_SAMPLES) return
                if (last >= 0) {
                    val gap = presentationTimeUs - last
                    if (gap in 5_000L..100_000L) gaps += gap
                }
                last = presentationTimeUs
                if (gaps.size == FPS_SAMPLES) {
                    val fps = 1_000_000f / gaps.sorted()[FPS_SAMPLES / 2]
                    main.post {
                        measuredFps = fps
                        matchRefreshRate()
                    }
                }
            }
        })
        if (speed != 1f) p.setPlaybackSpeed(speed) // kept when the player is made again (Surround)
        return p
    }

    // The line under the title: what's on and, quietly, the picture's size.
    private fun showPictureLabel() {
        if (pictureLabel.isEmpty()) return
        val picture = pictureLabel
        subtitle?.apply {
            text = listOf(info.optString("subtitle"), picture).filter { it.isNotEmpty() }.joinToString("  ·  ")
            visibility = View.VISIBLE
        }
    }

    private fun soundLabel() = if (surround) "Surround on" else "Surround off"

    // Surround on or off: the player is made again with the new sound and
    // carries on from the same place. Remembered on this TV unless it was
    // turned off because the sound failed.
    private fun setSurround(on: Boolean, remember: Boolean = true) {
        surround = on
        if (remember) getSharedPreferences("player", MODE_PRIVATE).edit().putBoolean("surround", on).apply()
        soundButton?.let { b ->
            b.tag = soundLabel()
            if (b.isFocused) b.text = soundLabel()
        }
        val item = player.currentMediaItem
        val pos = player.currentPosition
        val playing = player.playWhenReady
        player.removeListener(listener)
        player.release()
        player = buildPlayer()
        player.addListener(listener)
        video.player = player
        if (item != null) player.setMediaItem(item)
        if (!live && pos > 0) player.seekTo(pos)
        player.prepare()
        player.playWhenReady = playing
        Toast.makeText(
            this,
            if (on) "Surround on: Dolby and DTS go to your TV or speakers as they are." else "Surround off: Cue decodes the sound itself.",
            Toast.LENGTH_SHORT,
        ).show()
    }

    // The TV's refresh rate matched to the video's frame rate, as the TV's
    // own apps do: film at 24 frames a second on a 60 Hz screen stutters,
    // and 50 Hz channels do too. Same resolution only, and only when the
    // current rate isn't already a match (switching blanks the screen for a
    // moment).
    // The refresh rates the TV offers at its current resolution.
    private fun refreshRates(): List<Float> {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.M) return emptyList()
        val display = window.decorView.display ?: windowManager.defaultDisplay ?: return emptyList()
        val now = display.mode
        return display.supportedModes
            .filter { it.physicalWidth == now.physicalWidth && it.physicalHeight == now.physicalHeight }
            .map { it.refreshRate }
            .distinctBy { Math.round(it * 10) }
            .sorted()
    }

    // Whether the TV can show films at their own rate: a 24, 25, 30, 48
    // or 50 Hz mode alongside 60.
    private fun canChangeRefreshRate(): Boolean = refreshRates().any { it < 55f }

    private fun frameRate(): Float? =
        player.videoFormat?.frameRate?.takeIf { it > 0 } ?: measuredFps.takeIf { it > 0 }

    private fun matchRefreshRate() {
        if (rateMatched || Build.VERSION.SDK_INT < Build.VERSION_CODES.M) return
        val fps = frameRate() ?: return
        if (fps < 10f) return
        rateMatched = true
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
            // Android 12 and later (Google TV): the rate asked for on the
            // video's own surface. The TV switches to it unless "Match
            // content frame rate" is Never in its display settings.
            val surface = (video.videoSurfaceView as? SurfaceView)?.holder?.surface
            if (surface != null && surface.isValid) {
                surface.setFrameRate(fps, Surface.FRAME_RATE_COMPATIBILITY_FIXED_SOURCE, Surface.CHANGE_FRAME_RATE_ALWAYS)
                return
            }
        }
        val display: Display = window.decorView.display ?: return
        val now = display.mode
        fun fits(hz: Float): Boolean {
            val n = Math.round(hz / fps)
            return n >= 1 && Math.abs(hz - n * fps) < 0.05f * n
        }
        if (fits(now.refreshRate)) return
        val same = display.supportedModes.filter {
            it.physicalWidth == now.physicalWidth && it.physicalHeight == now.physicalHeight && fits(it.refreshRate)
        }
        val best = same.filter { it.refreshRate >= 47f }.minByOrNull { it.refreshRate } ?: same.minByOrNull { it.refreshRate } ?: return
        window.attributes = window.attributes.also { it.preferredDisplayModeId = best.modeId }
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
        timeNow.text = clock(pos)
        timeLeft.text = if (dur > 0) "-" + clock(dur - pos) else ""
        // From the credits (or the last 30 seconds) the corner offers the
        // next episode, as Skip intro does: the controls stay away, and
        // nothing starts by itself before the episode ends (credits times
        // can be minutes off); at the end the next one plays.
        val credits = when {
            creditsStart > 0 -> creditsStart
            creditsFromEnd > 0 && dur > 0 -> dur - creditsFromEnd
            else -> 0L
        }
        // An episode's credits are a few minutes at most: a time further back
        // (a guess, or one for another cut) isn't trusted.
        val creditsOk = credits > 0 && dur - credits <= MAX_CREDITS
        nearEnd = nextButton != null && dur > 0 && pos < dur - 1000 && ((creditsOk && pos >= credits) || dur - pos < 30_000)
        // From five minutes before the end (or the credits), the next episode
        // is found, so moving on is instant.
        if (isShow && !live && !catchup && dur > 0 && (nearEnd || dur - pos < 5 * 60_000)) prepareNext()
        updateSkip(pos)
    }

    private fun clock(ms: Long): String {
        val s = (ms / 1000).coerceAtLeast(0)
        val h = s / 3600
        val m = (s % 3600) / 60
        val sec = s % 60
        return if (h > 0) "%d:%02d:%02d".format(h, m, sec) else "%d:%02d".format(m, sec)
    }

    private fun controlsShowing() = controls.visibility == View.VISIBLE

    // Shows or hides a view with a short fade (and, with pop, a slight grow
    // from smaller), instead of it snapping on and off.
    private fun showSoftly(v: View, show: Boolean, pop: Boolean = false) {
        val ease = android.view.animation.DecelerateInterpolator(2f)
        v.animate().cancel()
        if (show) {
            if (v.visibility != View.VISIBLE) {
                v.alpha = 0f
                if (pop) { v.scaleX = 0.8f; v.scaleY = 0.8f }
                v.visibility = View.VISIBLE
            }
            v.animate().alpha(1f).scaleX(1f).scaleY(1f).setDuration(220).setInterpolator(ease).start()
        } else if (v.visibility == View.VISIBLE) {
            v.animate().alpha(0f).apply { if (pop) scaleX(1.1f).scaleY(1.1f) }.setDuration(200).setInterpolator(ease)
                .withEndAction { v.visibility = View.GONE; v.scaleX = 1f; v.scaleY = 1f }.start()
        }
    }

    private fun showControls(focus: View? = null) {
        if (!controlsShowing()) {
            // The bar glides up into place as it fades in.
            controls.alpha = 0f
            controls.translationY = px(14).toFloat()
            controls.visibility = View.VISIBLE
            controls.animate().alpha(1f).translationY(0f).setDuration(240)
                .setInterpolator(android.view.animation.DecelerateInterpolator(2f)).start()
        }
        val target = focus ?: currentFocus?.takeIf { it.isShown } ?: (if (live) playPause else timeBar)
        if (!target.isFocused) target.requestFocus()
        scheduleHide()
    }

    private fun hideControls() {
        main.removeCallbacks(hider)
        controls.animate().alpha(0f).translationY(px(10).toFloat()).setDuration(260)
            .setInterpolator(android.view.animation.AccelerateInterpolator(1.5f))
            .withEndAction { controls.visibility = View.GONE; controls.translationY = 0f }.start()
    }

    private fun scheduleHide() {
        main.removeCallbacks(hider)
        main.postDelayed(hider, 4500)
    }

    private fun togglePlay() {
        if (player.isPlaying) {
            player.pause()
            return
        }
        // After the stream dropped for good the player is idle: try again
        // from where it was.
        if (player.playbackState == Player.STATE_IDLE) {
            val pos = player.currentPosition
            player.prepare()
            player.seekTo(pos)
        }
        player.play()
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
        // Someone's there: the "Still watching?" count starts again.
        if (event.action == KeyEvent.ACTION_DOWN) autoInARow = 0
        // Any key but OK stops the next episode's countdown.
        if (event.action == KeyEvent.ACTION_DOWN && countdownLeft >= 0 &&
            code != KeyEvent.KEYCODE_DPAD_CENTER && code != KeyEvent.KEYCODE_ENTER) {
            cancelCountdown()
        }
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
                    KeyEvent.KEYCODE_CHANNEL_UP, KeyEvent.KEYCODE_PAGE_UP -> { zap(1); return true }
                    KeyEvent.KEYCODE_CHANNEL_DOWN, KeyEvent.KEYCODE_PAGE_DOWN -> { zap(-1); return true }
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
                    KeyEvent.KEYCODE_DPAD_UP -> { if (live) zap(1) else showControls(); return true }
                    KeyEvent.KEYCODE_DPAD_DOWN -> { if (live) zap(-1) else showControls(); return true }
                }
            } else {
                // OK on the time bar (where the highlight rests) pauses and
                // plays, as OK does with the controls hidden; on a button it
                // presses that button.
                if (!live && (code == KeyEvent.KEYCODE_DPAD_CENTER || code == KeyEvent.KEYCODE_ENTER) &&
                    (currentFocus == null || currentFocus === timeBar)) {
                    togglePlay()
                    showControls(timeBar)
                    return true
                }
                scheduleHide()
            }
        }
        return super.dispatchKeyEvent(event)
    }

    // ---- Skip intro, the credits, and learning them ----

    private fun buildSkipButton(): TextView = TextView(this).apply {
        size(13f)
        typeface = Fonts.text(this@PlayerActivity, 600)
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
    // Skip intro / recap shows for a few seconds as one starts, then gets out
    // of the way (it comes back with the controls while the intro lasts).
    // A segment longer than 3 minutes is taken for bad timings and ignored.
    private var skipShownAt = 0L
    private val NEXT = -2L // the corner button's target when it offers the next episode
    private val MAX_CREDITS = 240_000L // the longest an episode's credits are taken to be (as on the server)
    private var skipDismissed = ""

    private fun updateSkip(pos: Long) {
        if (live || catchup) return
        fun sane(start: Long, end: Long) = end > 0 && end - start in 5_000..180_000
        val (label, to) = when {
            sane(recapStart, recapEnd) && pos >= recapStart && pos < recapEnd - 2000 -> "Skip recap" to recapEnd
            sane(introStart, introEnd) && pos >= introStart && pos < introEnd - 2000 -> "Skip intro" to introEnd
            nearEnd -> "Next episode" to NEXT
            else -> "" to 0L
        }
        val key = "$label@$to"
        val now = android.os.SystemClock.elapsedRealtime()
        if (label.isNotEmpty() && !controlsShowing()) {
            if (key == skipDismissed) {
                hideSkip()
                return
            }
            if (skipButton.visibility == View.VISIBLE && now - skipShownAt > 8_000) {
                skipDismissed = key
                hideSkip()
                return
            }
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
            skipShownAt = now
            if (!controlsShowing()) skipButton.requestFocus()
        }
    }

    private fun hideSkip() {
        if (skipButton.visibility != View.VISIBLE) return
        val hadFocus = skipButton.isFocused
        skipButton.visibility = View.GONE
        if (hadFocus && controlsShowing()) playPause.requestFocus()
    }

    private fun skip() {
        if (skipTo == NEXT) {
            skipButton.visibility = View.GONE
            learnCredits()
            goNext()
            return
        }
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
        if (left < 15_000 || left > MAX_CREDITS || pos < dur * 0.7) return
        postJson("/api/watch/skips/learn", JSONObject()
            .put("kind", info.optString("kind")).put("tmdbId", info.optInt("tmdbId")).put("season", info.optInt("season"))
            .put("segment", "credits").put("start", pos / 1000.0).put("duration", dur / 1000.0))
    }

    // ---- Starting, pausing, and the next episode ----

    private var startScreen: View? = null
    private var pauseDim: View? = null
    private var countdownLeft = -1
    private val countdown = object : Runnable {
        override fun run() {
            val next = nextButton ?: return
            if (countdownLeft <= 0) {
                countdownLeft = -1
                // Three episodes in a row with nobody touching the remote:
                // ask before a fourth, so it doesn't play all night.
                if (autoInARow >= STILL_WATCHING_AFTER && party == null) {
                    askStillWatching()
                    return
                }
                autoInARow++
                goNext()
                return
            }
            next.text = "Next episode in $countdownLeft"
            countdownLeft--
            main.postDelayed(this, 1000)
        }
    }

    // ---- The next episode, ready before it's needed ----
    //
    // Near the end of an episode the player finds the next one (its version,
    // as Home would) in the background. Next (the button, the countdown,
    // Skip at the credits, or the episode ending) then plays it right here:
    // a quick fade, the new episode, no trip back to Home and no second
    // "Finding a version" screen. If it isn't ready, Home does it as before.

    private var nextReady: JSONObject? = null
    private var nextAsked = false
    private var swapped = false // this player moved on to another episode (for Home)

    // A member of a party who didn't start it follows the party's own moves.
    private fun partyFollower() = party != null && partyHost.not() && partyState?.optString("by") != party?.client

    private fun getJson(path: String): JSONObject? {
        if (server.isEmpty()) return null
        return runCatching {
            val c = URL("$server$path").openConnection() as HttpURLConnection
            c.connectTimeout = 8000
            c.readTimeout = 30_000
            CookieManager.getInstance().getCookie(server)?.let { c.setRequestProperty("Cookie", it) }
            info.optString("token").takeIf { it.isNotEmpty() }?.let { c.setRequestProperty("X-Cue-Profile", it) }
            val ok = c.responseCode in 200..299
            val text = if (ok) c.inputStream.bufferedReader().use { it.readText() } else ""
            c.disconnect()
            if (ok) JSONObject(text) else null
        }.getOrNull()
    }

    private fun prepareNext() {
        if (nextAsked) return
        nextAsked = true
        val id = info.optInt("tmdbId")
        val s = info.optInt("season")
        val e = info.optInt("episode")
        val partyCode = party?.code.orEmpty()
        Thread {
            val n = getJson("/api/watch/tv/$id/next?season=$s&episode=$e") ?: return@Thread
            val ns = n.optInt("season")
            val ne = n.optInt("episode")
            if (ne < 1) return@Thread
            val a = getJson("/api/play/tmdb/tv/$id/$ns/$ne?option=1" + (if (partyCode.isNotEmpty()) "&party=$partyCode" else "")) ?: return@Thread
            if (a.optString("url").isEmpty() || a.optString("youtube").isNotEmpty()) return@Thread
            val label = "S$ns:E$ne"
            val sub = listOf(label, a.optString("quality"), a.optString("source").takeIf { it.isNotEmpty() }?.let { "via $it" } ?: "")
                .filter { it.isNotEmpty() }.joinToString(" · ")
            val j = JSONObject(info.toString())
                .put("url", a.optString("url"))
                .put("title", a.optString("title"))
                .put("subtitle", sub)
                .put("startSec", 0.0)
                .put("hasOther", a.optInt("option", 1) < a.optInt("options", 1))
                .put("season", ns)
                .put("episode", ne)
                .put("thumbs", a.optString("thumbs"))
                .put("release", a.optString("release"))
                // The new episode's skip times come once it's playing (fetchSkips).
                .put("introStart", 0.0).put("introEnd", 0.0).put("recapStart", 0.0).put("recapEnd", 0.0)
                .put("creditsStart", 0.0).put("creditsFromEnd", 0.0)
            // Kept only while the episode it follows is still the one playing.
            main.post { if (!finished && info.optInt("season") == s && info.optInt("episode") == e) nextReady = j }
        }.start()
    }

    private fun startNextCountdown() {
        val next = nextButton ?: return
        showControls(next)
        countdownLeft = 5
        main.removeCallbacks(countdown)
        main.post(countdown)
    }

    // goNext plays the next episode: here when it's ready, else through Home.
    // "Still watching?": the picture stops, and OK carries on with the next
    // episode; Back or Stop goes back to Home.
    private fun askStillWatching() {
        player.playWhenReady = false
        android.app.AlertDialog.Builder(this, android.R.style.Theme_DeviceDefault_Dialog_Alert)
            .setTitle("Still watching?")
            .setMessage(info.optString("title").ifEmpty { "The next episode is ready." })
            .setPositiveButton("Keep watching") { d, _ ->
                d.dismiss()
                autoInARow = 0
                goNext()
            }
            .setNegativeButton("Stop") { d, _ ->
                d.dismiss()
                end("back")
            }
            .setCancelable(true)
            .setOnCancelListener { end("back") }
            .show()
    }

    private fun goNext() {
        val j = nextReady
        if (j == null) {
            end("next", party?.code ?: "")
            return
        }
        cancelCountdown()
        hideSkip()
        saveProgress()
        nextReady = null
        // Nothing is prepared while the old episode is still loaded (during
        // the fade it's near its end, which would prepare the episode about
        // to play as "next" and play it twice); asked again once it's swapped.
        nextAsked = true
        swapped = true
        // A short fade to black over the change.
        val veil = View(this).apply { setBackgroundColor(Color.BLACK); alpha = 0f }
        (video.parent as? FrameLayout)?.addView(veil, FrameLayout.LayoutParams(MATCH, MATCH))
        veil.animate().alpha(1f).setDuration(220).withEndAction {
            leftEpisode = info.optInt("season") to info.optInt("episode")
            leftAt = SystemClock.elapsedRealtime()
            info = j
            nextReady = null
            nextAsked = false
            hiccups = 0
            introStart = 0; introEnd = 0; recapStart = 0; recapEnd = 0; creditsStart = 0; creditsFromEnd = 0
            skipTo = 0
            skipsAsked = false
            nearEnd = false
            externalSubs = false
            hasOther = info.optBoolean("hasOther")
            otherButton?.visibility = if (hasOther) View.VISIBLE else View.GONE
            titleText?.text = info.optString("title")
            subtitle?.text = info.optString("subtitle")
            frameCache.evictAll()
            player.setMediaItem(mediaItem(""))
            player.prepare()
            player.playWhenReady = true
            party?.let { l -> Thread { runCatching { l.sendEpisode(partyTitle()) } }.start() }
            reportScreen(force = true)
            hideControls()
            veil.animate().alpha(0f).setStartDelay(350).setDuration(320).withEndAction {
                (veil.parent as? FrameLayout)?.removeView(veil)
            }.start()
        }.start()
    }

    private fun cancelCountdown() {
        if (countdownLeft < 0) return
        countdownLeft = -1
        main.removeCallbacks(countdown)
        nextButton?.text = "Next episode"
    }

    // The starting screen: the title's artwork and name while the video
    // loads, instead of a spinner on black.
    private fun buildStartScreen(): View {
        val box = FrameLayout(this).apply { setBackgroundColor(0xFF0B0C0F.toInt()) }
        val art = ImageView(this).apply {
            scaleType = ImageView.ScaleType.CENTER_CROP
            alpha = 0f
        }
        box.addView(art, FrameLayout.LayoutParams(MATCH, MATCH))
        box.addView(View(this).apply {
            background = GradientDrawable(GradientDrawable.Orientation.BOTTOM_TOP, intArrayOf(0xF00B0C0F.toInt(), 0x800B0C0F.toInt(), 0x400B0C0F))
        }, FrameLayout.LayoutParams(MATCH, MATCH))
        val text = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(px(56), 0, px(56), px(56))
            addView(TextView(this@PlayerActivity).apply {
                text = info.optString("title")
                setTextColor(Color.WHITE)
                size(30f)
                typeface = Fonts.display(this@PlayerActivity)
                maxLines = 2
            })
            addView(TextView(this@PlayerActivity).apply {
                text = info.optString("subtitle").ifEmpty { "Starting…" }
                setTextColor(0xFFC7C8CC.toInt())
                size(15f)
                setPadding(0, px(6), 0, 0)
            })
        }
        box.addView(text, FrameLayout.LayoutParams(MATCH, ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.BOTTOM))
        val backdrop = info.optString("backdrop")
        if (backdrop.startsWith("http")) {
            Thread {
                try {
                    val c = URL(backdrop).openConnection() as HttpURLConnection
                    c.connectTimeout = 6000
                    c.readTimeout = 10000
                    val bmp = c.inputStream.use { BitmapFactory.decodeStream(it) }
                    c.disconnect()
                    if (bmp != null) main.post {
                        art.setImageBitmap(bmp)
                        art.animate().alpha(0.6f).setDuration(300).start()
                    }
                } catch (_: Exception) {
                }
            }.start()
        }
        return box
    }

    private fun hideStartScreen() {
        val v = startScreen ?: return
        startScreen = null
        v.animate().alpha(0f).setDuration(250).withEndAction { v.visibility = View.GONE }.start()
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
            typeface = Fonts.mono(this@PlayerActivity)
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
            typeface = Fonts.text(this@PlayerActivity, 700)
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
                    hideGuide()
                    if (id != info.optString("id")) zapTo(id)
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

    // ---- Flipping channels, as on a TV ----
    //
    // Up and down (or channel up and down) change channel without leaving the
    // player: a banner with the channel's number, name and what's on shows at
    // once, and the stream is asked for once the remote rests a moment, so
    // flicking past several channels only tunes the last.
    private var titleText: TextView? = null
    private var liveNowText: TextView? = null
    private var zapBanner: TextView? = null
    private var zapTarget = ""
    private val zapHider = Runnable { zapBanner?.animate()?.alpha(0f)?.setDuration(400)?.start() }
    private val zapTuner = Runnable { tune(zapTarget) }

    private fun channelList(): List<JSONObject> {
        val a = info.optJSONArray("channels") ?: return emptyList()
        return (0 until a.length()).mapNotNull { a.optJSONObject(it) }
    }

    private fun zap(delta: Int) {
        val list = channelList()
        if (list.size < 2) return
        val current = zapTarget.ifEmpty { info.optString("id") }
        val i = list.indexOfFirst { it.optString("id") == current }.coerceAtLeast(0)
        zapTo(list[(i + delta + list.size) % list.size].optString("id"))
    }

    private fun zapTo(id: String) {
        val c = channelList().firstOrNull { it.optString("id") == id } ?: return
        zapTarget = id
        hideControls()
        showZapBanner(c)
        main.removeCallbacks(zapTuner)
        main.postDelayed(zapTuner, 450)
    }

    private fun showZapBanner(c: JSONObject) {
        val b = zapBanner ?: TextView(this).also { tv ->
            tv.setTextColor(Color.WHITE)
            tv.size(16f)
            tv.typeface = Fonts.text(this, 500)
            tv.setLineSpacing(0f, 1.15f)
            tv.setPadding(px(22), px(14), px(26), px(14))
            tv.background = GradientDrawable().apply {
                cornerRadius = px(16).toFloat()
                setColor(0xE6141519.toInt())
                setStroke(px(1), 0x1FFFFFFF)
            }
            findViewById<ViewGroup>(android.R.id.content).addView(tv, FrameLayout.LayoutParams(ViewGroup.LayoutParams.WRAP_CONTENT,
                ViewGroup.LayoutParams.WRAP_CONTENT, Gravity.TOP or Gravity.START).apply { leftMargin = px(48); topMargin = px(36) })
            zapBanner = tv
        }
        val num = c.optString("num")
        val now = c.optString("now")
        val until = c.optString("until")
        val text = android.text.SpannableStringBuilder()
        if (num.isNotEmpty()) {
            val start = text.length
            text.append(num).append("   ")
            text.setSpan(android.text.style.ForegroundColorSpan(teal), start, start + num.length, 0)
            text.setSpan(android.text.style.RelativeSizeSpan(1.5f), start, start + num.length, 0)
        }
        val nameStart = text.length
        text.append(c.optString("name"))
        text.setSpan(android.text.style.StyleSpan(android.graphics.Typeface.BOLD), nameStart, text.length, 0)
        text.setSpan(android.text.style.RelativeSizeSpan(1.25f), nameStart, text.length, 0)
        if (now.isNotEmpty()) {
            val s0 = text.length
            text.append("\n").append(now).append(if (until.isNotEmpty()) "  ·  until $until" else "")
            text.setSpan(android.text.style.ForegroundColorSpan(0xFFC7C8CC.toInt()), s0, text.length, 0)
        }
        b.text = text
        b.animate().cancel()
        if (b.alpha < 1f) {
            b.translationY = -px(8).toFloat()
            b.animate().alpha(1f).translationY(0f).setDuration(220).start()
        }
        main.removeCallbacks(zapHider)
        main.postDelayed(zapHider, 3500)
    }

    // tune asks Cue for the channel's stream and switches to it in place.
    private fun tune(id: String) {
        if (id.isEmpty() || server.isEmpty()) return
        val c = channelList().firstOrNull { it.optString("id") == id } ?: return
        val cookie = CookieManager.getInstance().getCookie(server)
        val token = info.optString("token")
        Thread {
            val answer = try {
                val u = URL("$server/api/live/play/${java.net.URLEncoder.encode(id, "UTF-8").replace("+", "%20")}")
                val h = u.openConnection() as HttpURLConnection
                h.connectTimeout = 8000
                h.readTimeout = 15000
                h.setRequestProperty("User-Agent", "CueTV/1 (native)")
                if (cookie != null) h.setRequestProperty("Cookie", cookie)
                if (token.isNotEmpty()) h.setRequestProperty("X-Cue-Profile", token)
                val ok = h.responseCode == 200
                val body = (if (ok) h.inputStream else h.errorStream)?.bufferedReader()?.use { it.readText() } ?: ""
                h.disconnect()
                if (ok) JSONObject(body) else null
            } catch (_: Exception) {
                null
            }
            runOnUiThread {
                if (finished || id != zapTarget) return@runOnUiThread
                val direct = answer?.optString("direct").orEmpty()
                val url = direct.ifEmpty { answer?.optString("url")?.takeIf { it.isNotEmpty() }?.let { server + it }.orEmpty() }
                if (url.isEmpty()) {
                    Toast.makeText(this, "${c.optString("name")} won't play right now.", Toast.LENGTH_LONG).show()
                    return@runOnUiThread
                }
                info.put("id", id)
                val num = c.optString("num")
                info.put("title", (if (num.isNotEmpty()) "$num  " else "") + c.optString("name"))
                val now = c.optString("now")
                info.put("subtitle", if (now.isNotEmpty()) "$now · until ${c.optString("until")}" else "")
                titleText?.text = info.optString("title")
                liveNowText?.text = info.optString("subtitle")
                started = false
                hiccups = 0
                alts.clear()
                answer?.optJSONArray("directAlts")?.let { list ->
                    for (i in 0 until list.length()) list.optString(i).takeIf { it.isNotEmpty() }?.let { alts.addLast(it) }
                }
                player.setMediaItem(MediaItem.fromUri(url))
                player.prepare()
                player.playWhenReady = true
            }
        }.start()
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

    // subtitlesPressed: the list, one press to choose: Off, the subtitles in
    // the video, and (when Cue hasn't added one yet) finding English ones.
    // The choice is remembered on this TV for what plays next.
    private fun subtitlesPressed() {
        main.removeCallbacks(hider)
        val kind = info.optString("kind")
        val canFind = !externalSubs && !live && !catchup && server.isNotEmpty() && (kind == "movie" || kind == "tv")
        val off = player.trackSelectionParameters.disabledTrackTypes.contains(C.TRACK_TYPE_TEXT)
        val labels = ArrayList<String>()
        val picks = ArrayList<() -> Unit>()
        var current = 0
        labels += "Off"
        picks += { setSubtitles(null, 0) }
        for (g in player.currentTracks.groups) {
            if (g.type != C.TRACK_TYPE_TEXT) continue
            for (i in 0 until g.length) {
                if (!g.isTrackSupported(i)) continue
                if (g.isTrackSelected(i) && !off) current = labels.size
                labels += subtitleName(g.getTrackFormat(i))
                picks += { setSubtitles(g, i) }
            }
        }
        if (canFind) {
            labels += "Find English subtitles…"
            picks += { findSubtitles() }
        }
        android.app.AlertDialog.Builder(this, android.R.style.Theme_DeviceDefault_Dialog_Alert)
            .setTitle("Subtitles")
            .setSingleChoiceItems(labels.toTypedArray(), current) { d, which ->
                d.dismiss()
                picks[which]()
            }
            .setOnDismissListener { scheduleHide() }
            .show()
    }

    private fun setSubtitles(group: Tracks.Group?, index: Int) {
        val b = player.trackSelectionParameters.buildUpon().clearOverridesOfType(C.TRACK_TYPE_TEXT)
        val choice = if (group == null) {
            b.setTrackTypeDisabled(C.TRACK_TYPE_TEXT, true)
            "off"
        } else {
            b.setTrackTypeDisabled(C.TRACK_TYPE_TEXT, false)
                .setOverrideForType(androidx.media3.common.TrackSelectionOverride(group.mediaTrackGroup, index))
            group.getTrackFormat(index).language?.takeIf { it.isNotEmpty() && it != "und" } ?: "en"
        }
        player.trackSelectionParameters = b.build()
        getSharedPreferences("player", MODE_PRIVATE).edit().putString("subs", choice).apply()
    }

    private fun subtitleName(f: Format): String {
        val lang = f.language?.takeIf { it.isNotEmpty() && it != "und" }?.let { java.util.Locale.forLanguageTag(it).displayLanguage } ?: ""
        val name = f.label?.takeIf { it.isNotEmpty() } ?: lang.ifEmpty { "Subtitles" }
        val extra = listOfNotNull(
            lang.takeIf { it.isNotEmpty() && !name.contains(it, ignoreCase = true) },
            "forced".takeIf { f.selectionFlags and C.SELECTION_FLAG_FORCED != 0 },
        )
        return if (extra.isEmpty()) name else "$name (${extra.joinToString(", ")})"
    }

    // findSubtitles asks Cue for an English subtitle (from OpenSubtitles),
    // adds it to the video and shows it.
    private fun findSubtitles() {
        if (subsAsking) return
        subsAsking = true
        val kind = info.optString("kind")
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
                    .setTrackTypeDisabled(C.TRACK_TYPE_TEXT, false)
                    .clearOverridesOfType(C.TRACK_TYPE_TEXT)
                    .setPreferredTextLanguage("en")
                    .build()
                getSharedPreferences("player", MODE_PRIVATE).edit().putString("subs", "en").apply()
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

    // Report a problem: what's wrong, in one press; the owner hears about it
    // (and answers in Watch), and whoever reported it hears back.
    private fun reportProblem() {
        main.removeCallbacks(hider)
        val keys = listOf("sound", "subtitles", "picture", "wrong", "buffering", "other")
        val labels = arrayOf("No sound, or it's out of step", "Subtitles missing or out of step", "Picture blurry, frozen or squashed",
            "Wrong video (not this title or episode)", "Keeps stopping to load", "Something else")
        android.app.AlertDialog.Builder(this, android.R.style.Theme_DeviceDefault_Dialog_Alert)
            .setTitle("What's wrong?")
            .setItems(labels) { d, which ->
                d.dismiss()
                postJson("/api/reports", JSONObject()
                    .put("kind", info.optString("kind"))
                    .put("tmdbId", info.optInt("tmdbId"))
                    .put("season", info.optInt("season"))
                    .put("episode", info.optInt("episode"))
                    .put("title", info.optString("title"))
                    .put("release", info.optString("release"))
                    .put("problem", keys[which]))
                Toast.makeText(this, "Sent. The owner will look at it, and you'll hear back.", Toast.LENGTH_LONG).show()
            }
            .setOnDismissListener { scheduleHide() }
            .show()
    }

    // Listen on phone: a party carries the sound to a phone (in sync, the
    // picture hidden there) and the TV goes quiet; pressed again, the sound
    // comes back to the TV.
    private fun listenOnPhone() {
        if (listenMode) {
            listenMode = false
            player.volume = 1f
            listenButton?.tag = "Listen on phone"
            Toast.makeText(this, "The sound is back on the TV.", Toast.LENGTH_SHORT).show()
            return
        }
        listenMode = true
        player.volume = 0f
        listenButton?.tag = "Sound back on TV"
        val l = party
        if (l == null) partyPressed() else showListenQr(l.code)
    }

    private fun toggleBoost() {
        val on = booster == null
        if (on) {
            booster = if (player.audioSessionId == C.AUDIO_SESSION_ID_UNSET) null else runCatching {
                android.media.audiofx.LoudnessEnhancer(player.audioSessionId).apply {
                    setTargetGain(600) // millibels: about twice as loud
                    enabled = true
                }
            }.getOrNull()
            if (booster == null) {
                Toast.makeText(this, "This TV can't boost the volume.", Toast.LENGTH_SHORT).show()
                return
            }
        } else {
            runCatching { booster?.release() }
            booster = null
        }
        boostButton?.tag = if (on) "Boost on" else "Volume boost"
        Toast.makeText(this, if (on) "Volume boost on" else "Volume boost off", Toast.LENGTH_SHORT).show()
    }

    private fun showListenQr(code: String) {
        main.removeCallbacks(hider)
        val qr = ImageView(this).apply {
            setImageBitmap(app.cue.tv.home.qrBitmap("$server/watch/party/$code?listen=1", px(220)))
            setPadding(px(16), px(16), px(16), px(16))
        }
        android.app.AlertDialog.Builder(this, android.R.style.Theme_DeviceDefault_Dialog_Alert)
            .setTitle("Listen on your phone")
            .setMessage("Scan with your phone's camera and put headphones on. The sound plays there, in step with the picture, and the TV stays quiet.")
            .setView(qr)
            .setPositiveButton("Done") { d, _ -> d.dismiss() }
            .setOnDismissListener { scheduleHide() }
            .show()
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
        runCatching { booster?.release() }
        party?.close()
        saveProgress()
        val dur = player.duration.let { if (it == C.TIME_UNSET) 0 else it / 1000 }
        setResult(
            RESULT_OK,
            Intent()
                .putExtra(RESULT_REASON, reason)
                .putExtra(RESULT_POSITION, player.currentPosition / 1000)
                .putExtra(RESULT_DURATION, dur)
                .putExtra(RESULT_MESSAGE, message)
                .putExtra(RESULT_EPISODE, if (swapped) "${info.optInt("season")}|${info.optInt("episode")}" else ""),
        )
        finish()
    }

    // Leaving the player fades back to the app.
    @Suppress("DEPRECATION")
    override fun finish() {
        super.finish()
        overridePendingTransition(android.R.anim.fade_in, android.R.anim.fade_out)
    }

    override fun onStop() {
        super.onStop()
        if (!finished) {
            saveProgress()
            player.pause()
        }
    }

    override fun onDestroy() {
        ScreenLink.onControl = null
        ScreenLink.onPlayInPlayer = null
        if (!live) ScreenLink.report(JSONObject().put("playing", false)) // back on Home
        party?.close()
        main.removeCallbacksAndMessages(null)
        player.release()
        super.onDestroy()
    }

    companion object {
        // Episodes played on by themselves in a row, with no key pressed
        // (kept across players: the next episode can open a new one).
        @JvmStatic var autoInARow = 0
        const val STILL_WATCHING_AFTER = 3
        const val EXTRA_JSON = "json"
        const val EXTRA_SERVER = "server"
        const val RESULT_REASON = "reason"
        const val RESULT_POSITION = "position"
        const val RESULT_DURATION = "duration"
        const val RESULT_MESSAGE = "message"
        const val RESULT_EPISODE = "episode" // "season|episode" when the player moved on to another episode

        private const val SAVE_EVERY_MS = 15_000L
        private const val FPS_SAMPLES = 48
        private const val MATCH = ViewGroup.LayoutParams.MATCH_PARENT
    }
}
