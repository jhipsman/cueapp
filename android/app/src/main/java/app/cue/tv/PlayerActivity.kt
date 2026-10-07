package app.cue.tv

import android.app.Activity
import android.content.Intent
import android.graphics.Color
import android.graphics.Typeface
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.util.TypedValue
import android.view.Gravity
import android.view.KeyEvent
import android.view.View
import android.view.WindowManager
import android.webkit.CookieManager
import android.widget.FrameLayout
import android.widget.LinearLayout
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
import androidx.media3.ui.PlayerView
import org.json.JSONObject
import java.net.HttpURLConnection
import java.net.URL

// The player: ExoPlayer, full screen. It plays MKV and MP4, H.264, HEVC and
// AV1 as far as the TV can, and sends Dolby and DTS sound to the TV or
// receiver as it is when they take it. It resumes where Watch said, saves
// where you are to Cue every 15 seconds and on the way out, and tells Watch
// how it ended (the end, an error, or Back) so Watch can play the next
// episode or another version.
@OptIn(UnstableApi::class)
class PlayerActivity : Activity() {
    private lateinit var player: ExoPlayer
    private lateinit var view: PlayerView
    private lateinit var info: JSONObject
    private var server = ""
    private val main = Handler(Looper.getMainLooper())
    private var finished = false

    private val saver = object : Runnable {
        override fun run() {
            if (player.isPlaying) saveProgress()
            main.postDelayed(this, SAVE_EVERY_MS)
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        window.addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON)
        info = JSONObject(intent.getStringExtra(EXTRA_JSON) ?: "{}")
        server = intent.getStringExtra(EXTRA_SERVER) ?: ""

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
            .setSeekForwardIncrementMs(30_000)
            .build()
        player.trackSelectionParameters = player.trackSelectionParameters.buildUpon()
            .setPreferredAudioLanguage("en")
            .setPreferredTextLanguage(null)
            .build()

        view = PlayerView(this).apply {
            player = this@PlayerActivity.player
            setShowSubtitleButton(true)
            setShowNextButton(false)
            setShowPreviousButton(false)
            controllerShowTimeoutMs = 4000
            setKeepContentOnPlayerReset(true)
            setShutterBackgroundColor(Color.BLACK)
        }
        val title = titleBlock()
        view.setControllerVisibilityListener(PlayerView.ControllerVisibilityListener { v -> title.visibility = v })
        val root = FrameLayout(this).apply {
            setBackgroundColor(Color.BLACK)
            addView(view, FrameLayout.LayoutParams(FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT))
            addView(title, FrameLayout.LayoutParams(FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.WRAP_CONTENT, Gravity.TOP))
        }
        setContentView(root)

        player.addListener(object : Player.Listener {
            override fun onPlaybackStateChanged(state: Int) {
                if (state == Player.STATE_ENDED) end("ended")
            }

            override fun onPlayerError(error: PlaybackException) {
                end("error", error.errorCodeName)
            }
        })

        player.setMediaItem(MediaItem.fromUri(info.optString("url")))
        val start = (info.optDouble("startSec", 0.0) * 1000).toLong()
        if (start > 0) player.seekTo(start)
        player.prepare()
        player.playWhenReady = true
        main.postDelayed(saver, SAVE_EVERY_MS)
    }

    private fun titleBlock(): View {
        val dp = { v: Int -> TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_DIP, v.toFloat(), resources.displayMetrics).toInt() }
        return LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(32), dp(24), dp(32), dp(24))
            setBackgroundColor(0x99000000.toInt())
            addView(TextView(context).apply {
                text = info.optString("title")
                setTextColor(Color.WHITE)
                textSize = 24f
                typeface = Typeface.DEFAULT_BOLD
            })
            val sub = info.optString("subtitle")
            if (sub.isNotEmpty()) addView(TextView(context).apply {
                text = sub
                setTextColor(0xFFA6A8AE.toInt())
                textSize = 16f
            })
        }
    }

    // The remote: with the controls hidden, left and right skip back 10 s and
    // forward 30 s, and OK pauses; anything else shows the controls.
    override fun dispatchKeyEvent(event: KeyEvent): Boolean {
        if (event.keyCode == KeyEvent.KEYCODE_BACK) {
            if (event.action == KeyEvent.ACTION_UP) {
                if (view.isControllerFullyVisible) view.hideController() else end("back")
            }
            return true
        }
        if (event.action == KeyEvent.ACTION_DOWN && !view.isControllerFullyVisible) {
            when (event.keyCode) {
                KeyEvent.KEYCODE_DPAD_LEFT -> { player.seekBack(); view.showController(); return true }
                KeyEvent.KEYCODE_DPAD_RIGHT -> { player.seekForward(); view.showController(); return true }
                KeyEvent.KEYCODE_DPAD_CENTER, KeyEvent.KEYCODE_ENTER -> {
                    player.playWhenReady = !player.playWhenReady
                    view.showController()
                    return true
                }
            }
        }
        return view.dispatchKeyEvent(event) || super.dispatchKeyEvent(event)
    }

    // saveProgress tells Cue where this profile is in the title, the same way
    // Watch in a browser does.
    private fun saveProgress() {
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
        main.removeCallbacks(saver)
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
    }
}
