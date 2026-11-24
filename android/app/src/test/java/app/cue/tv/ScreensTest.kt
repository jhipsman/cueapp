package app.cue.tv

import android.content.Context
import android.view.KeyEvent
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.compose.ui.test.onRoot
import androidx.test.core.app.ActivityScenario
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import app.cue.tv.home.HomeActivity
import com.github.takahirom.roborazzi.captureRoboImage
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Before
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode
import java.io.File

// Every main screen of the TV app as a picture, taken on the build machine:
// Robolectric runs the real app (drawing with Android's own graphics) against
// the stand-in server the emulator walk-through uses, the remote's keys move
// through it, and each screen is saved to app/build/screens at a 1080p TV's
// size. For looking at the screens without a TV; the walk-through on the
// emulator (app/src/androidTest) is what tests the remote end to end.
@RunWith(AndroidJUnit4::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w960dp-h540dp-land-television-xhdpi")
class ScreensTest {
    @get:Rule
    val compose = createEmptyComposeRule()

    private lateinit var server: MockWebServer
    private lateinit var scenario: ActivityScenario<HomeActivity>
    private val ctx: Context get() = ApplicationProvider.getApplicationContext()
    private var n = 0

    @Before
    fun setUp() {
        server = MockWebServer()
        val fake = FakeCue()
        server.dispatcher = fake
        server.start()
        fake.base = server.url("/").toString().trimEnd('/')
        ctx.getSharedPreferences("cue", Context.MODE_PRIVATE).edit().clear()
            .putString("server", server.url("/").toString().trimEnd('/'))
            .putString("profileToken", "test-profile").putInt("profileId", 1)
            .putString("profileName", "Sam").putString("profileAvatar", "teal")
            .putBoolean("tour.v1", true)
            .commit()
        File("build/screens").mkdirs()
    }

    @After
    fun tearDown() {
        runCatching { scenario.close() }
        server.shutdown()
    }

    // The clock moves only when told: some screens never stop moving (Live
    // TV's Featured games turn over every few seconds), so waiting for
    // them to be still would wait forever.
    private fun see(text: String) {
        val until = System.currentTimeMillis() + 15_000
        while (compose.onAllNodes(hasText(text, substring = true)).fetchSemanticsNodes().isEmpty()) {
            if (System.currentTimeMillis() > until) {
                compose.onRoot().captureRoboImage("build/screens/never-saw-" + text.replace(Regex("[^A-Za-z0-9]+"), "-") + ".png")
                throw AssertionError("never saw \"$text\"")
            }
            Thread.sleep(50) // the stand-in server answers on its own thread
            compose.mainClock.advanceTimeBy(100)
        }
    }

    private fun press(code: Int, times: Int = 1) = repeat(times) {
        scenario.onActivity { a ->
            a.dispatchKeyEvent(KeyEvent(KeyEvent.ACTION_DOWN, code))
            a.dispatchKeyEvent(KeyEvent(KeyEvent.ACTION_UP, code))
        }
        compose.mainClock.advanceTimeBy(400)
    }

    private fun shot(name: String) {
        compose.mainClock.advanceTimeBy(900) // let it settle
        n++
        compose.onRoot().captureRoboImage("build/screens/%02d-%s.png".format(n, name))
    }

    @Test
    fun screens() {
        compose.mainClock.autoAdvance = false
        scenario = ActivityScenario.launch(HomeActivity::class.java)
        see("Continue watching")
        shot("home")
        press(KeyEvent.KEYCODE_DPAD_LEFT, 3)
        see("Browse")
        shot("side-menu")
        for ((place, marker) in listOf("movies" to "Top 10 on Netflix", "shows" to "Series", "live-tv" to "BILLS", "browse" to "Streaming services")) {
            press(KeyEvent.KEYCODE_DPAD_DOWN)
            see(marker)
            shot(place)
        }
        // Live TV with the highlight on the channels, the menu closed.
        press(KeyEvent.KEYCODE_DPAD_UP)
        see("BILLS")
        press(KeyEvent.KEYCODE_DPAD_RIGHT)
        press(KeyEvent.KEYCODE_DPAD_RIGHT, 2)
        shot("live-tv-channels")
        press(KeyEvent.KEYCODE_BACK)
        press(KeyEvent.KEYCODE_DPAD_UP, 4)
        see("Search from your phone")
        shot("search")
        // Ask Cue, from its button on Search.
        press(KeyEvent.KEYCODE_DPAD_RIGHT) // onto Ask Cue
        press(KeyEvent.KEYCODE_DPAD_CENTER)
        see("Something funny for tonight")
        shot("ask-cue")
        press(KeyEvent.KEYCODE_DPAD_CENTER)
        see("easy laughs")
        shot("ask-cue-answer")
        // A title, from Home's first row.
        press(KeyEvent.KEYCODE_BACK) // Ask Cue to Search
        see("Search from your phone")
        press(KeyEvent.KEYCODE_BACK) // to the menu
        press(KeyEvent.KEYCODE_BACK) // Home
        see("Continue watching")
        press(KeyEvent.KEYCODE_DPAD_RIGHT) // into Home: its first card
        press(KeyEvent.KEYCODE_DPAD_CENTER)
        see("Not for me")
        shot("title")
    }
}
