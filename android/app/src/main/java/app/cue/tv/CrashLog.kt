package app.cue.tv

import android.content.Context
import android.os.Build
import app.cue.tv.home.CueApi
import org.json.JSONArray
import org.json.JSONObject
import java.io.File
import java.io.PrintWriter
import java.io.StringWriter

/**
 * When the app crashes, the stack trace is saved, and on the next start it's
 * sent to the server (Settings > System > Problems on the web, and its log),
 * so a crash seen on the couch can be traced to the line that caused it.
 * The last few crashes are also kept on the TV, for Settings > Problems here.
 */
object CrashLog {
    private const val FILE = "last-crash.txt"
    private const val HISTORY = "crash-history.json"
    private const val KEEP = 10
    @Volatile private var installed = false

    /** One crash on this TV: when, in which build, what it was, and whether Cue has it. */
    data class Entry(val at: Long, val version: String, val headline: String, val sent: Boolean)

    fun install(ctx: Context) {
        if (installed) return
        installed = true
        val app = ctx.applicationContext
        val file = File(app.filesDir, FILE)
        val previous = Thread.getDefaultUncaughtExceptionHandler()
        Thread.setDefaultUncaughtExceptionHandler { thread, e ->
            runCatching {
                val out = StringWriter()
                PrintWriter(out).use { e.printStackTrace(it) }
                val trace = "thread ${thread.name}\n" + out.toString().take(16_000)
                file.writeText(trace)
                add(app, Entry(System.currentTimeMillis(), BuildConfig.VERSION_NAME, headline(trace), sent = false))
            }
            previous?.uncaughtException(thread, e)
        }
    }

    /** Whether a crash is waiting to be sent. */
    fun pending(ctx: Context): Boolean = File(ctx.applicationContext.filesDir, FILE).exists()

    /** Sends a crash saved last time, if any; kept for next time if it can't be sent. */
    suspend fun sendSaved(ctx: Context, api: CueApi): Boolean {
        val file = File(ctx.applicationContext.filesDir, FILE)
        val trace = runCatching { if (file.exists()) file.readText() else "" }.getOrDefault("")
        if (trace.isBlank()) return true
        val sent = runCatching {
            api.request("POST", "/api/play/crash", JSONObject()
                .put("app", "tv")
                .put("version", BuildConfig.VERSION_NAME)
                .put("device", "${Build.MANUFACTURER} ${Build.MODEL} (Android ${Build.VERSION.RELEASE})")
                .put("trace", trace))
        }.isSuccess
        if (sent) {
            runCatching { file.delete() }
            markSent(ctx.applicationContext)
        }
        return sent
    }

    /** The crashes kept on this TV, newest first. */
    fun history(ctx: Context): List<Entry> = runCatching {
        val a = JSONArray(File(ctx.applicationContext.filesDir, HISTORY).readText())
        (0 until a.length()).mapNotNull { i ->
            val o = a.optJSONObject(i) ?: return@mapNotNull null
            Entry(o.optLong("at"), o.optString("version"), o.optString("headline"), o.optBoolean("sent"))
        }
    }.getOrDefault(emptyList())

    /** Forgets the crashes kept on this TV (one waiting to be sent is still sent). */
    fun clearHistory(ctx: Context) {
        runCatching { File(ctx.applicationContext.filesDir, HISTORY).delete() }
    }

    private fun add(ctx: Context, e: Entry) = save(ctx, (listOf(e) + history(ctx)).take(KEEP))

    private fun markSent(ctx: Context) = save(ctx, history(ctx).map { it.copy(sent = true) })

    private fun save(ctx: Context, list: List<Entry>) {
        val a = JSONArray()
        list.forEach { a.put(JSONObject().put("at", it.at).put("version", it.version).put("headline", it.headline).put("sent", it.sent)) }
        runCatching { File(ctx.filesDir, HISTORY).writeText(a.toString()) }
    }

    // The exception and its message: the trace's first line after the thread's name.
    private fun headline(trace: String): String =
        trace.lineSequence().map { it.trim() }.firstOrNull { it.isNotEmpty() && !it.startsWith("thread ") }?.take(300) ?: "A crash"
}
