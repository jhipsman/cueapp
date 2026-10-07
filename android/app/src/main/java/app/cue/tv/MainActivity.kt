package app.cue.tv

import android.annotation.SuppressLint
import android.app.Activity
import android.content.Intent
import android.graphics.Color
import android.os.Bundle
import android.view.KeyEvent
import android.view.View
import android.webkit.CookieManager
import android.webkit.JavascriptInterface
import android.webkit.WebResourceError
import android.webkit.WebResourceRequest
import android.webkit.WebSettings
import android.webkit.WebView
import android.webkit.WebViewClient
import org.json.JSONObject

// Cue on the TV: Watch (the server's own pages) full screen in a web view.
// The pages see window.CueTV, move focus with the remote's arrows, and hand
// videos to PlayerActivity, which plays every format the TV can.
class MainActivity : Activity() {
    private var web: WebView? = null
    private var server: String = ""

    @SuppressLint("SetJavaScriptEnabled", "JavascriptInterface")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val saved = Server.address(this)
        if (saved == null) {
            startActivity(Intent(this, SetupActivity::class.java))
            finish()
            return
        }
        server = saved

        val w = WebView(this)
        web = w
        w.setBackgroundColor(Color.parseColor("#101114"))
        w.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true // the profile this TV is on lives in localStorage
            mediaPlaybackRequiresUserGesture = false
            mixedContentMode = WebSettings.MIXED_CONTENT_ALWAYS_ALLOW
            userAgentString = "$userAgentString CueTV/1"
        }
        CookieManager.getInstance().setAcceptCookie(true)
        CookieManager.getInstance().setAcceptThirdPartyCookies(w, true)
        w.addJavascriptInterface(Bridge(), "CueTV")
        w.webViewClient = object : WebViewClient() {
            override fun onReceivedError(view: WebView, request: WebResourceRequest, error: WebResourceError) {
                if (request.isForMainFrame) cantReach(error.description?.toString() ?: "no answer")
            }
        }
        w.isFocusable = true
        w.isFocusableInTouchMode = true
        setContentView(w)
        w.requestFocus(View.FOCUS_DOWN)

        if (savedInstanceState != null) w.restoreState(savedInstanceState) else w.loadUrl("$server/watch")
    }

    // The server didn't answer: back to the address screen, saying why.
    private fun cantReach(why: String) {
        startActivity(
            Intent(this, SetupActivity::class.java)
                .putExtra(SetupActivity.EXTRA_ADDRESS, server)
                .putExtra(SetupActivity.EXTRA_PROBLEM, "Couldn't reach Cue at $server ($why). Is the server on? Press Connect to try again."),
        )
        finish()
    }

    override fun onSaveInstanceState(outState: Bundle) {
        super.onSaveInstanceState(outState)
        web?.saveState(outState)
    }

    override fun onPause() {
        super.onPause()
        CookieManager.getInstance().flush() // stay signed in after a restart
    }

    // Back goes back a page in Watch; on the first page it leaves the app.
    override fun dispatchKeyEvent(event: KeyEvent): Boolean {
        val w = web
        if (w != null && event.keyCode == KeyEvent.KEYCODE_BACK && event.action == KeyEvent.ACTION_UP && w.canGoBack()) {
            w.goBack()
            return true
        }
        if (w != null && event.keyCode == KeyEvent.KEYCODE_BACK && w.canGoBack()) return true
        return super.dispatchKeyEvent(event)
    }

    @Deprecated("startActivityForResult keeps this simple on every Android version")
    override fun onActivityResult(requestCode: Int, resultCode: Int, data: Intent?) {
        @Suppress("DEPRECATION")
        super.onActivityResult(requestCode, resultCode, data)
        if (requestCode != PLAY) return
        val r = JSONObject()
            .put("reason", data?.getStringExtra(PlayerActivity.RESULT_REASON) ?: "back")
            .put("position", data?.getLongExtra(PlayerActivity.RESULT_POSITION, 0) ?: 0)
            .put("duration", data?.getLongExtra(PlayerActivity.RESULT_DURATION, 0) ?: 0)
            .put("message", data?.getStringExtra(PlayerActivity.RESULT_MESSAGE) ?: "")
        web?.evaluateJavascript("window.cueTvPlayerDone && window.cueTvPlayerDone($r)", null)
    }

    // What Watch's pages can ask of the app.
    inner class Bridge {
        // play opens the player for {url, title, subtitle, startSec, kind,
        // tmdbId, season, episode, token}.
        @JavascriptInterface
        fun play(json: String) {
            runOnUiThread {
                @Suppress("DEPRECATION")
                startActivityForResult(
                    Intent(this@MainActivity, PlayerActivity::class.java)
                        .putExtra(PlayerActivity.EXTRA_JSON, json)
                        .putExtra(PlayerActivity.EXTRA_SERVER, server),
                    PLAY,
                )
            }
        }

        @JavascriptInterface
        fun changeServer() {
            runOnUiThread {
                startActivity(Intent(this@MainActivity, SetupActivity::class.java).putExtra(SetupActivity.EXTRA_ADDRESS, server))
                finish()
            }
        }
    }

    companion object {
        private const val PLAY = 1
    }
}
