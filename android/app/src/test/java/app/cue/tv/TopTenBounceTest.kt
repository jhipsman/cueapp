package app.cue.tv

import android.content.Context
import android.view.KeyEvent
import androidx.compose.ui.test.hasText
import androidx.compose.ui.test.isFocused
import androidx.compose.ui.test.junit4.createEmptyComposeRule
import androidx.test.core.app.ActivityScenario
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import app.cue.tv.home.HomeActivity
import okhttp3.mockwebserver.MockWebServer
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith
import org.robolectric.annotation.Config
import org.robolectric.annotation.GraphicsMode

// Moving along a row of posters, the page mustn't bounce: the highlighted
// poster rises once (its lift) and stays, it doesn't rise and drop back (the
// page re-lining the row up around the lift at every press did that).
@RunWith(AndroidJUnit4::class)
@GraphicsMode(GraphicsMode.Mode.NATIVE)
@Config(sdk = [35], qualifiers = "w960dp-h540dp-land-television-xhdpi")
class TopTenBounceTest {
    @get:Rule val compose = createEmptyComposeRule()

    @Test
    fun movingAlongARowDoesNotBounce() {
        val server = MockWebServer(); val fake = FakeCue(); server.dispatcher = fake; server.start()
        fake.base = server.url("/").toString().trimEnd('/')
        val ctx: Context = ApplicationProvider.getApplicationContext()
        ctx.getSharedPreferences("cue", Context.MODE_PRIVATE).edit().clear().putString("server", fake.base)
            .putString("profileToken", "t").putInt("profileId", 1).putString("profileName", "Sam").putBoolean("tour.v1", true).commit()
        compose.mainClock.autoAdvance = false
        val sc = ActivityScenario.launch(HomeActivity::class.java)
        fun press(k: Int) { sc.onActivity { it.dispatchKeyEvent(KeyEvent(KeyEvent.ACTION_DOWN, k)); it.dispatchKeyEvent(KeyEvent(KeyEvent.ACTION_UP, k)) } }
        // Waits (up to a minute: a slow build machine) for text on screen, or fails.
        fun waitFor(t: String) {
            repeat(1200) { if (compose.onAllNodes(hasText(t, substring = true)).fetchSemanticsNodes().isNotEmpty()) return; Thread.sleep(50); compose.mainClock.advanceTimeBy(100) }
            throw AssertionError("never saw \"$t\"")
        }
        waitFor("Continue watching")
        press(KeyEvent.KEYCODE_DPAD_LEFT); compose.mainClock.advanceTimeBy(500)
        press(KeyEvent.KEYCODE_DPAD_DOWN); compose.mainClock.advanceTimeBy(1500) // Movies: Top 10 first
        waitFor("Top 10 on")
        press(KeyEvent.KEYCODE_DPAD_RIGHT); compose.mainClock.advanceTimeBy(1500)
        repeat(3) {
            // Hold the highlight on the row: down to the posters if on the tabs.
            press(KeyEvent.KEYCODE_DPAD_DOWN); compose.mainClock.advanceTimeBy(800)
        }
        for (step in 1..4) {
            press(KeyEvent.KEYCODE_DPAD_RIGHT)
            val tops = mutableListOf<Float>()
            repeat(40) {
                compose.mainClock.advanceTimeBy(16)
                compose.onAllNodes(isFocused(), useUnmergedTree = true).fetchSemanticsNodes().lastOrNull()?.boundsInRoot?.let { tops.add(it.top) }
            }
            // Only ever up (or still), never back down.
            for (i in 1 until tops.size) {
                org.junit.Assert.assertTrue("step $step bounced: ${tops.map { it.toInt() }}", tops[i] <= tops[i - 1] + 1f)
            }
        }
        server.shutdown()
    }
}
