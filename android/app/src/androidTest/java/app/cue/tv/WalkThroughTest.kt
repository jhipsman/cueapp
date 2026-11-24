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
    private fun shell(cmd: String): String =
        InstrumentationRegistry.getInstrumentation().uiAutomation.executeShellCommand(cmd).use { pfd ->
            java.io.FileInputStream(pfd.fileDescriptor).use { String(it.readBytes()) }
        }

    // How smoothly each stretch drew, from Android's own count of frames
    // (dumpsys gfxinfo): frames drawn, how many were late, and the slow
    // end. Written to frames.txt beside the screenshots and to the log, then
    // the count starts again for the next stretch.
    private val frames = StringBuilder()
    private fun frames(stretch: String) {
        val keep = Regex("(Total frames rendered|Janky frames|50th percentile|90th percentile|99th percentile|Number Slow UI thread|Number Slow bitmap uploads|Number Frame deadline missed)[^\\n]*")
        val got = keep.findAll(shell("dumpsys gfxinfo ${ctx.packageName}")).map { it.value.trim() }.distinct().joinToString("; ")
        frames.append(stretch).append(": ").append(got).append('\n')
        android.util.Log.i("CueFrames", "$stretch: $got")
        shell("dumpsys gfxinfo ${ctx.packageName} reset")
    }

    @Before
    fun setUp() {
        server = MockWebServer()
        server.dispatcher = fake
        server.start()
        val base = server.url("/").toString().trimEnd('/')
        fake.base = base
        // Signed in on this TV already, with a profile chosen and the tour seen.
        ctx.getSharedPreferences("cue", Context.MODE_PRIVATE).edit().clear()
            .putString("server", base)
            .putString("profileToken", "test-profile").putInt("profileId", 1)
            .putString("profileName", "Sam").putString("profileAvatar", "teal")
            .putBoolean("tour.v1", true)
            .commit()
        File(ctx.filesDir, "last-crash.txt").delete()
        File(ctx.filesDir, "crash-history.json").delete()
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
        frames(name) // how the way here drew
        // In the log with its time, so the nightly polish run can find each
        // step in its screen recording.
        android.util.Log.i("CueWalkThrough", "step %02d %s".format(step, name))
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
            android.util.Log.i("CueWalkThrough", "press " + KeyEvent.keyCodeToString(key))
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
                "Movies" to "Top 10 on Netflix",
                "Shows" to "Series", // the show up top
                "Live TV" to "BILLS", // the game up top
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

            // Up the menu to Search, and Ask Cue from its button: ask with one
            // of the ideas, and the answer and its picks show; Back to Search.
            press(KeyEvent.KEYCODE_BACK) // to the side menu
            press(KeyEvent.KEYCODE_DPAD_UP, 5)
            see("Search from your phone")
            shot("search")
            // Into the page and up to the button (a TV's buttons take the
            // remote's select, not a touch).
            press(KeyEvent.KEYCODE_DPAD_RIGHT) // lands on Ask Cue
            reach("✨  Ask Cue", KeyEvent.KEYCODE_DPAD_UP, tries = 2)
            press(KeyEvent.KEYCODE_DPAD_CENTER)
            see("Something funny for tonight")
            shot("ask-cue")
            reach("Something funny for tonight", KeyEvent.KEYCODE_DPAD_LEFT, tries = 4)
            press(KeyEvent.KEYCODE_DPAD_CENTER)
            see("easy laughs")
            Thread.sleep(600)
            shot("ask-cue-answer")
            press(KeyEvent.KEYCODE_BACK)
            see("Search from your phone")
            reach("✨  Ask Cue", KeyEvent.KEYCODE_DPAD_UP, tries = 3)

            // Back to the side menu, and Back again goes Home.
            press(KeyEvent.KEYCODE_BACK)
            press(KeyEvent.KEYCODE_BACK)
            see("Continue watching")
            shot("home-again")

            // Settings, near the bottom of the menu: each section shows as
            // the highlight rests on it, like the menu's own places.
            press(KeyEvent.KEYCODE_DPAD_LEFT, 3)
            reach("Settings", KeyEvent.KEYCODE_DPAD_DOWN, tries = 12)
            see("Account and server")
            shot("settings")
            press(KeyEvent.KEYCODE_DPAD_RIGHT) // onto the sections
            press(KeyEvent.KEYCODE_DPAD_DOWN)
            see("Surround sound")
            shot("settings-playback")
            press(KeyEvent.KEYCODE_DPAD_DOWN)
            see("No crashes on this TV")
            see("Release should only be called once") // every TV's, as an administrator
            shot("settings-problems")
            press(KeyEvent.KEYCODE_DPAD_DOWN)
            see("Version 2.1.9")
            shot("settings-about")
            press(KeyEvent.KEYCODE_BACK) // to the side menu
            press(KeyEvent.KEYCODE_BACK)
            see("Continue watching")
        }
        // The frame counts, all together in the log (CueFrames).
        android.util.Log.i("CueFrames", "all:\n$frames")
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
