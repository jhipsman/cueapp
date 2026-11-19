package app.cue.tv

import android.content.Context
import android.os.Build
import app.cue.tv.home.CueApi
import org.json.JSONObject
import java.io.File
import java.io.PrintWriter
import java.io.StringWriter

/**
 * When the app crashes, the stack trace is saved, and on the next start it's
 * sent to the server's log ("tv app crashed"), so a crash seen on the couch
 * can be traced to the line that caused it.
 */
object CrashLog {
    private const val FILE = "last-crash.txt"
    @Volatile private var installed = false

    fun install(ctx: Context) {
        if (installed) return
        installed = true
        val file = File(ctx.applicationContext.filesDir, FILE)
        val previous = Thread.getDefaultUncaughtExceptionHandler()
        Thread.setDefaultUncaughtExceptionHandler { thread, e ->
            runCatching {
                val out = StringWriter()
                PrintWriter(out).use { e.printStackTrace(it) }
                file.writeText("thread ${thread.name}\n" + out.toString().take(16_000))
            }
            previous?.uncaughtException(thread, e)
        }
    }

    /** Sends a crash saved last time, if any; kept for next time if it can't be sent. */
    suspend fun sendSaved(ctx: Context, api: CueApi) {
        val file = File(ctx.applicationContext.filesDir, FILE)
        val trace = runCatching { if (file.exists()) file.readText() else "" }.getOrDefault("")
        if (trace.isBlank()) return
        val sent = runCatching {
            api.request("POST", "/api/play/crash", JSONObject()
                .put("app", "tv")
                .put("version", BuildConfig.VERSION_NAME)
                .put("device", "${Build.MANUFACTURER} ${Build.MODEL} (Android ${Build.VERSION.RELEASE})")
                .put("trace", trace))
        }.isSuccess
        if (sent) runCatching { file.delete() }
    }
}
