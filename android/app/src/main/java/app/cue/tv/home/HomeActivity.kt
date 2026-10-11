package app.cue.tv.home

import android.content.Intent
import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.result.contract.ActivityResultContracts
import androidx.lifecycle.lifecycleScope
import app.cue.tv.MainActivity
import app.cue.tv.PlayerActivity
import app.cue.tv.Server
import app.cue.tv.SetupActivity

// Cue on the TV: the app's own screens (Compose for TV), playing in
// PlayerActivity. What isn't native yet (the full Live TV guide, settings)
// opens in the web view (MainActivity), signed in and on the same profile.
class HomeActivity : ComponentActivity() {
    private lateinit var api: CueApi
    private lateinit var playback: Playback

    private val player = registerForActivityResult(ActivityResultContracts.StartActivityForResult()) { res ->
        val d = res.data
        playback.ended(
            d?.getStringExtra(PlayerActivity.RESULT_REASON) ?: "back",
            d?.getLongExtra(PlayerActivity.RESULT_POSITION, 0L) ?: 0L,
            d?.getStringExtra(PlayerActivity.RESULT_MESSAGE) ?: "",
        )
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val server = Server.address(this)
        if (server == null) {
            startActivity(Intent(this, SetupActivity::class.java))
            finish()
            return
        }
        api = CueApi(this, server)
        playback = Playback(api, lifecycleScope) { json ->
            player.launch(
                Intent(this, PlayerActivity::class.java)
                    .putExtra(PlayerActivity.EXTRA_JSON, json)
                    .putExtra(PlayerActivity.EXTRA_SERVER, server),
            )
        }
        setContent {
            CueRoot(
                api,
                playback,
                openWeb = { path ->
                    startActivity(
                        Intent(this, MainActivity::class.java)
                            .putExtra(MainActivity.EXTRA_PATH, path)
                            .putExtra(MainActivity.EXTRA_PROFILE, api.profileToken),
                    )
                },
                changeServer = {
                    startActivity(Intent(this, SetupActivity::class.java).putExtra(SetupActivity.EXTRA_ADDRESS, server))
                    finish()
                },
            )
        }
    }
}
