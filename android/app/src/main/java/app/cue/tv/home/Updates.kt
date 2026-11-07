package app.cue.tv.home

import android.content.ActivityNotFoundException
import android.content.Context
import android.content.Intent
import android.net.Uri
import android.os.Build
import android.provider.Settings
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.FileProvider
import androidx.tv.material3.Text
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import java.io.File
import java.net.HttpURLConnection
import java.net.URL

// The app updating itself. Cue forwards /tv.version to the newest build's
// number (published beside the APK by the TV app workflow) and /tv.apk to
// the APK. When the number is above this build's, Cue offers the update on
// opening and from Profile: Update downloads the APK into the app's cache
// and hands it to Android's installer, which asks the household to confirm
// and installs it over this one (the same signing key, so nothing is lost).
object AppUpdate {
    /** The update on offer (null: none, or not checked yet), and whether its card shows. */
    var offer by mutableStateOf<UpdateOffer?>(null)
    var showing by mutableStateOf(false)
    private var checkedAt = 0L
    private var setAside = 0L

    /** Asks for the newest build, at most every six hours, and offers it once per opening. */
    suspend fun check(ctx: Context, server: String) {
        val now = System.currentTimeMillis()
        if (now - checkedAt < 6 * 3600_000L) return
        checkedAt = now
        val newest = latest(server) ?: return
        if (newest <= current(ctx)) return
        if (offer?.build != newest) offer = UpdateOffer(newest)
        if (newest != setAside) showing = true
    }

    /** Later: not again until Cue is next opened (Profile still offers it). */
    fun later() {
        setAside = offer?.build ?: 0L
        showing = false
    }

    /** This build's number (its versionCode). */
    @Suppress("DEPRECATION")
    fun current(ctx: Context): Long {
        val info = ctx.packageManager.getPackageInfo(ctx.packageName, 0)
        return if (Build.VERSION.SDK_INT >= 28) info.longVersionCode else info.versionCode.toLong()
    }

    /** The newest build's number on offer, or null when it can't be told. */
    suspend fun latest(server: String): Long? = withContext(Dispatchers.IO) {
        runCatching {
            val c = open("$server/tv.version")
            try {
                if (c.responseCode !in 200..299) return@runCatching null
                c.inputStream.bufferedReader().use { it.readText() }.trim().toLongOrNull()
            } finally {
                c.disconnect()
            }
        }.getOrNull()
    }

    /** Downloads the newest APK, reporting how far it has got (0 to 1). */
    suspend fun download(ctx: Context, server: String, progress: (Float) -> Unit): File = withContext(Dispatchers.IO) {
        val dir = File(ctx.cacheDir, "updates").apply { mkdirs() }
        dir.listFiles()?.forEach { it.delete() }
        val file = File(dir, "cue-tv.apk")
        val c = open("$server/tv.apk")
        try {
            if (c.responseCode !in 200..299) throw IllegalStateException("The download answered ${c.responseCode}.")
            val total = c.getHeaderField("Content-Length")?.toLongOrNull() ?: -1L
            var got = 0L
            c.inputStream.use { input ->
                file.outputStream().use { out ->
                    val buf = ByteArray(64 * 1024)
                    while (true) {
                        val n = input.read(buf)
                        if (n < 0) break
                        out.write(buf, 0, n)
                        got += n
                        if (total > 0) progress((got.toFloat() / total).coerceIn(0f, 1f))
                    }
                }
            }
        } finally {
            c.disconnect()
        }
        file
    }

    /**
     * Whether Android lets Cue install apps. On Android 8 and later each app
     * needs the household's permission once; when it's missing this opens the
     * page to give it and returns false.
     */
    fun allowedToInstall(ctx: Context): Boolean {
        if (Build.VERSION.SDK_INT < 26 || ctx.packageManager.canRequestPackageInstalls()) return true
        val page = Intent(Settings.ACTION_MANAGE_UNKNOWN_APP_SOURCES, Uri.parse("package:${ctx.packageName}"))
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
        runCatching { ctx.startActivity(page) }
        return false
    }

    /** Hands the downloaded APK to Android's installer. */
    fun install(ctx: Context, apk: File) {
        val uri = FileProvider.getUriForFile(ctx, "${ctx.packageName}.updates", apk)
        val intent = Intent(Intent.ACTION_VIEW)
            .setDataAndType(uri, "application/vnd.android.package-archive")
            .addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION or Intent.FLAG_ACTIVITY_NEW_TASK)
        ctx.startActivity(intent)
    }

    // Follows the forwards itself: Cue at home is often http:// and GitHub is
    // https://, a change of scheme HttpURLConnection won't follow on its own.
    private fun open(start: String): HttpURLConnection {
        var url = URL(start)
        repeat(6) {
            val c = url.openConnection() as HttpURLConnection
            c.instanceFollowRedirects = false
            c.connectTimeout = 15000
            c.readTimeout = 30000
            c.setRequestProperty("User-Agent", "CueTV/1 (native)")
            val code = c.responseCode
            if (code in 300..399) {
                val next = c.getHeaderField("Location") ?: return c
                c.disconnect()
                url = URL(url, next)
            } else {
                return c
            }
        }
        throw IllegalStateException("Too many forwards.")
    }
}

/** What the update offer is doing: offered, downloading, or stuck. */
class UpdateOffer(val build: Long) {
    var progress by mutableStateOf<Float?>(null)
    var problem by mutableStateOf("")
}

/**
 * The update offer: a card in the corner on opening (Update, or Later), the
 * download's progress, then Android's installer. Later hides it until Cue is
 * next opened; Profile offers it too.
 */
@Composable
fun UpdateCard(api: CueApi, offer: UpdateOffer, later: () -> Unit) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    val focus = remember { FocusRequester() }
    LaunchedEffect(Unit) { runCatching { focus.requestFocus() } }
    androidx.activity.compose.BackHandler { later() }
    Box(Modifier.fillMaxSize().background(Color(0x990C0D10)), contentAlignment = Alignment.Center) {
        Column(
            Modifier.width(520.dp).clip(RoundedCornerShape(PanelCorner)).background(Panel).padding(32.dp).rise(0),
        ) {
            Text("NEW VERSION", fontSize = 12.sp, fontWeight = FontWeight.Bold, color = tint(), letterSpacing = 2.sp)
            Spacer(Modifier.height(10.dp))
            Text("A new version of Cue is ready", fontSize = 24.sp, fontWeight = FontWeight.Bold, color = Color.White)
            Spacer(Modifier.height(8.dp))
            Text(
                "Build ${offer.build}. It installs over this one: you stay signed in, on your profile.",
                fontSize = 14.sp, color = Dim, lineHeight = 20.sp,
            )
            val p = offer.progress
            if (p != null) {
                Spacer(Modifier.height(20.dp))
                Box(Modifier.fillMaxWidth().height(6.dp).clip(RoundedCornerShape(3.dp)).background(Color(0x33FFFFFF))) {
                    Box(Modifier.fillMaxWidth(p.coerceAtLeast(0.02f)).height(6.dp).clip(RoundedCornerShape(3.dp)).background(tint()))
                }
                Spacer(Modifier.height(8.dp))
                Text("Downloading… ${(p * 100).toInt()}%", fontSize = 13.sp, color = Dim)
            }
            if (offer.problem.isNotEmpty()) {
                Spacer(Modifier.height(14.dp))
                Text(offer.problem, fontSize = 13.sp, color = Color(0xFFFF8A80), lineHeight = 19.sp)
            }
            Spacer(Modifier.height(24.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Pill(if (offer.problem.isNotEmpty()) "Try again" else "Update", primary = true, modifier = Modifier.focusRequester(focus)) {
                    if (offer.progress != null) return@Pill
                    offer.problem = ""
                    if (Build.VERSION.SDK_INT < 24) {
                        // Android 6's installer can't open a file another app shares.
                        offer.problem = "This TV can't install it from inside Cue. Install the update with Downloader from ${api.server}/tv.apk."
                        return@Pill
                    }
                    if (!AppUpdate.allowedToInstall(ctx)) {
                        offer.problem = "Allow Cue to install apps on the page that opened, then come back and choose Try again. " +
                            "No page? On a Fire TV: Settings > My Fire TV > Developer options > Install unknown apps > Cue."
                        return@Pill
                    }
                    scope.launch {
                        offer.progress = 0f
                        try {
                            val apk = AppUpdate.download(ctx, api.server) { v -> scope.launch { offer.progress = v } }
                            offer.progress = 1f
                            AppUpdate.install(ctx, apk)
                        } catch (e: ActivityNotFoundException) {
                            offer.problem = "This TV has no installer Cue can open. Install the update with Downloader from ${api.server}/tv.apk."
                        } catch (e: Exception) {
                            offer.problem = "Couldn't download the update (${e.message ?: "no answer"}). Try again, or install it with Downloader from ${api.server}/tv.apk."
                        } finally {
                            offer.progress = null
                        }
                    }
                }
                Pill("Later", onClick = later)
            }
        }
    }
}
