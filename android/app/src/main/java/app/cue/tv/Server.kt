package app.cue.tv

import android.content.Context
import java.net.HttpURLConnection
import java.net.URL

// Where the Cue server is (saved on the device), and checking that an
// address really is one.
object Server {
    private const val PREFS = "cue"
    private const val KEY = "server"

    fun address(ctx: Context): String? =
        ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE).getString(KEY, null)

    fun save(ctx: Context, address: String) {
        ctx.getSharedPreferences(PREFS, Context.MODE_PRIVATE).edit().putString(KEY, address).apply()
    }

    // normalize turns what people type into a base address. A domain
    // ("cuetv.me", "watch.example.com") is a cloud server: https, no port.
    // An IP address, a bare computer name or a .local/.lan name is a computer
    // at home: http, with Cue's port 8264 unless one is given. Typed
    // http:// or https:// addresses are kept as they are.
    fun normalize(typed: String): String {
        var s = typed.trim().trimEnd('/')
        if (s.isEmpty()) return s
        if (s.startsWith("http://", true) || s.startsWith("https://", true)) return s
        val hostPart = s.substringBefore('/')
        val host = hostPart.substringBefore(':')
        val hasPort = hostPart.contains(':')
        val ip = host.matches(Regex("""\d{1,3}(\.\d{1,3}){3}"""))
        val atHome = ip || !host.contains('.') || host.endsWith(".local", true) || host.endsWith(".lan", true) || host.endsWith(".home", true)
        return if (atHome) {
            "http://" + if (hasPort) s else s.replaceFirst(hostPart, "$hostPart:8264")
        } else {
            "https://$s"
        }
    }

    // check answers null when address is a Cue server, or what went wrong.
    // It runs on a background thread.
    fun check(address: String): String? {
        return try {
            val c = URL("$address/api/onboarding/status").openConnection() as HttpURLConnection
            c.connectTimeout = 6000
            c.readTimeout = 6000
            val code = c.responseCode
            val body = (if (code in 200..299) c.inputStream else c.errorStream)?.bufferedReader()?.use { it.readText() } ?: ""
            c.disconnect()
            when {
                code == 200 && body.contains("firstRunNeeded") -> null
                else -> "That address answered, but it isn't Cue (status $code)."
            }
        } catch (e: Exception) {
            "Couldn't reach $address: ${e.message ?: e.javaClass.simpleName}. Check the address and that the server is on."
        }
    }
}
