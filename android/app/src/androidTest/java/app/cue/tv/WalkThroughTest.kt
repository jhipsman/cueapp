package app.cue.tv

import android.content.Context
import android.view.KeyEvent
import android.webkit.CookieManager
import androidx.test.core.app.ActivityScenario
import androidx.test.core.app.ApplicationProvider
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import androidx.test.uiautomator.By
import androidx.test.uiautomator.UiDevice
import androidx.test.uiautomator.Until
import app.cue.tv.home.HomeActivity
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertFalse
import org.junit.Before
import org.junit.Test
import org.junit.runner.RunWith
import java.io.File

// Cue on an Android TV emulator, driven with the remote's keys as someone on
// the couch would: through every place in the side menu, into Browse and a
// service's page, onto a title and back. Each step waits for its screen to
// show (failing if it never does) and saves a screenshot; a crash anywhere
// fails the run.
@RunWith(AndroidJUnit4::class)
class WalkThroughTest {
    private lateinit var server: MockWebServer
    private val fake = FakeCue()
    private val ctx: Context get() = ApplicationProvider.getApplicationContext()
    private val device: UiDevice by lazy { UiDevice.getInstance(InstrumentationRegistry.getInstrumentation()) }
    private var step = 0

    // Screenshots go where the emulator's shell keeps them (CI pulls them
    // from there; they outlive the app being uninstalled after the run).
    private fun shell(cmd: String) {
        InstrumentationRegistry.getInstrumentation().uiAutomation.executeShellCommand(cmd).use { pfd ->
            java.io.FileInputStream(pfd.fileDescriptor).use { it.readBytes() }
        }
    }

    @Before
    fun setUp() {
        server = MockWebServer()
        server.dispatcher = fake
        server.start()
        val base = server.url("/").toString().trimEnd('/')
        // Signed in on this TV already, with a profile chosen and the tour seen.
        ctx.getSharedPreferences("cue", Context.MODE_PRIVATE).edit().clear()
            .putString("server", base)
            .putString("profileToken", "test-profile").putInt("profileId", 1)
            .putString("profileName", "Sam").putString("profileAvatar", "teal")
            .putBoolean("tour.v1", true)
            .commit()
        File(ctx.filesDir, "last-crash.txt").delete()
        File(ctx.filesDir, "home-1.json").delete()
        shell("rm -rf $SHOTS")
        shell("mkdir -p $SHOTS")
        InstrumentationRegistry.getInstrumentation().runOnMainSync {
            CookieManager.getInstance().setCookie(base, "cue_session=test")
            CookieManager.getInstance().flush()
        }
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    private fun shot(name: String) {
        step++
        shell("screencap -p $SHOTS/%02d-%s.png".format(step, name))
    }

    // Waits for text on screen (part of it is enough), failing with a
    // screenshot of what showed instead.
    private fun see(text: String, timeout: Long = 15_000) {
        if (!device.wait(Until.hasObject(By.textContains(text)), timeout)) {
            shot("missing-" + text.replace(Regex("[^A-Za-z0-9]+"), "-"))
            throw AssertionError("\"$text\" never showed. On screen: ${onScreen()}. Asked of the server: ${fake.asked.takeLast(15)}")
        }
    }

    // The words on screen now (for a failure's message: the log shows what
    // was there instead).
    private fun onScreen(): List<String> {
        val out = java.io.ByteArrayOutputStream()
        runCatching { device.dumpWindowHierarchy(out) }
        return Regex("text=\"([^\"]+)\"").findAll(out.toString()).map { it.groupValues[1] }.distinct().take(40).toList()
    }

    // Moves the highlight with a key until it rests on the thing showing
    // this text, failing if it never does.
    private fun reach(text: String, key: Int, tries: Int = 8) {
        repeat(tries) {
            val on = device.findObject(By.focused(true))
            if (on != null && (on.text == text || on.findObject(By.text(text)) != null)) return
            press(key)
        }
        shot("unreached-" + text.replace(Regex("[^A-Za-z0-9]+"), "-"))
        throw AssertionError("never reached \"$text\". Focused: ${device.findObject(By.focused(true))?.let { o -> o.text ?: o.className }}. On screen: ${onScreen()}")
    }

    private fun press(key: Int, times: Int = 1) {
        repeat(times) {
            device.pressKeyCode(key)
            Thread.sleep(350)
        }
    }

    @Test
    fun walkThroughTheApp() {
        ActivityScenario.launch(HomeActivity::class.java).use {
            // Home, straight in (signed in already): its first row.
            see("Continue watching")
            Thread.sleep(800)
            shot("home")

            // Onto the side menu: it opens with the names.
            press(KeyEvent.KEYCODE_DPAD_LEFT, 3)
            see("Browse")
            shot("side-menu")

            // Down the menu, each place showing its page as the highlight rests on it.
            val pages = listOf(
                "Shows" to "Continue watching",
                "Movies" to "Top 10 on Netflix",
                "Sports" to "Chiefs",
                "Live TV" to "News 24",
                "My List" to "Nothing here yet",
                "Browse" to "Streaming services",
            )
            for ((place, shows) in pages) {
                press(KeyEvent.KEYCODE_DPAD_DOWN)
                Thread.sleep(500)
                see(shows)
                shot(place.lowercase().replace(" ", "-"))
            }

            // Into Browse: Coming soon is the first tile; then a service's page.
            press(KeyEvent.KEYCODE_DPAD_RIGHT)
            Thread.sleep(400)
            // Wherever the highlight landed: up to the top tiles (Coming
            // soon, Top 10), then down one to the services.
            press(KeyEvent.KEYCODE_DPAD_UP, 4)
            shot("browse-in")
            press(KeyEvent.KEYCODE_DPAD_DOWN)
            press(KeyEvent.KEYCODE_DPAD_CENTER)
            see("Netflix originals")
            Thread.sleep(600)
            shot("service-page")

            // A title from the service's page, then Back to it, then Back to Browse.
            press(KeyEvent.KEYCODE_DPAD_CENTER)
            see("More Like This")
            Thread.sleep(800)
            shot("title")
            press(KeyEvent.KEYCODE_BACK)
            see("Netflix originals")
            press(KeyEvent.KEYCODE_BACK)
            see("Streaming services")
            shot("back-to-browse")

            // Up the menu to Ask Cue: ask with one of the ideas, and the answer
            // and its picks show; then Search, at the top.
            press(KeyEvent.KEYCODE_BACK) // to the side menu
            press(KeyEvent.KEYCODE_DPAD_UP, 7)
            see("Something funny for tonight")
            shot("ask-cue")
            // Into the page, down to the idea, and press it (a TV's buttons
            // take the remote's select, not a touch).
            press(KeyEvent.KEYCODE_DPAD_RIGHT)
            reach("Something funny for tonight", KeyEvent.KEYCODE_DPAD_DOWN)
            press(KeyEvent.KEYCODE_DPAD_CENTER)
            see("easy laughs")
            Thread.sleep(600)
            shot("ask-cue-answer")
            press(KeyEvent.KEYCODE_BACK) // to the side menu
            press(KeyEvent.KEYCODE_DPAD_UP)
            see("Search from your phone")
            shot("search")

            // Back on the menu goes Home; Home's page again.
            press(KeyEvent.KEYCODE_BACK)
            see("Continue watching")
            shot("home-again")
        }
        // No crash saved along the way, and nothing asked of the server it didn't expect.
        assertFalse("the app crashed: " + File(ctx.filesDir, "last-crash.txt").takeIf { it.exists() }?.readText().orEmpty(),
            File(ctx.filesDir, "last-crash.txt").exists())
        // What the screens asked for that the stand-in doesn't answer (they
        // got a 404 and coped): in the log, to keep the stand-in up to date.
        android.util.Log.i("CueWalkThrough", "unknown requests: ${fake.unknown}")
    }

    companion object {
        const val SHOTS = "/data/local/tmp/cue-shots"
    }
}
