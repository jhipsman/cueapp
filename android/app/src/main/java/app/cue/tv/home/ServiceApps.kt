package app.cue.tv.home

import android.app.SearchManager
import android.content.Context
import android.content.Intent
import android.net.Uri
import org.json.JSONObject

// The official apps of the streaming services, for "Watch on Netflix": a
// title on a service the profile has (My services, or a free one) opens in
// that service's own app on this TV, at the title where the app allows a
// search to be handed to it, else at its home. Netflix, Disney+ and the
// rest can't be played inside Cue (their video is copy-protected), so this
// is the next best thing: one press.

data class ServiceApp(val key: String, val name: String)

object ServiceApps {
    // Each service's app, as Google TV and Fire TV each have it.
    private val packages = mapOf(
        "netflix" to listOf("com.netflix.ninja", "com.netflix.mediaclient"),
        "prime" to listOf("com.amazon.amazonvideo.livingroom", "com.amazon.avod", "com.amazon.firebat", "com.amazon.avod.thirdpartyclient"),
        "disney" to listOf("com.disney.disneyplus"),
        "max" to listOf("com.wbd.stream", "com.hbo.hbonow"),
        "hulu" to listOf("com.hulu.livingroomplus", "com.hulu.plus"),
        "peacock" to listOf("com.peacocktv.peacockandroid"),
        "paramount" to listOf("com.cbs.ott", "com.cbs.app"),
        "appletv" to listOf("com.apple.atve.androidtv.appletv", "com.apple.atve.amazon.appletv"),
        "tubi" to listOf("com.tubitv"),
        "pluto" to listOf("tv.pluto.android"),
        "roku" to listOf("com.roku.web.trc", "com.roku.trc"),
        "plex" to listOf("com.plexapp.android"),
    )

    // The installed app for a service, or null.
    private fun installed(ctx: Context, key: String): String? =
        packages[key]?.firstOrNull { p -> runCatching { ctx.packageManager.getLaunchIntentForPackage(p) != null }.getOrDefault(false) }

    /** The service to offer for a title (the profile's own first), from /api/watch/providers; null when none is installed. */
    fun pick(ctx: Context, providers: JSONObject): ServiceApp? {
        for (group in listOf("subscription", "free")) {
            val list = providers.optJSONArray(group) ?: continue
            for (i in 0 until list.length()) {
                val p = list.optJSONObject(i) ?: continue
                val key = p.optString("key")
                if (key.isEmpty() || !p.optBoolean("mine") || installed(ctx, key) == null) continue
                return ServiceApp(key, p.optString("name"))
            }
        }
        return null
    }

    /** Opens the service's app at the title (a search for it) where it takes one, else at its home. */
    fun open(ctx: Context, key: String, title: String): Boolean {
        val pkg = installed(ctx, key) ?: return false
        val tries = buildList {
            // A search handed to the app: most TV apps take this, and land on
            // the title (or a list with it first).
            add(Intent(Intent.ACTION_SEARCH).setPackage(pkg).putExtra(SearchManager.QUERY, title))
            if (key == "netflix") add(Intent(Intent.ACTION_VIEW, Uri.parse("https://www.netflix.com/search?q=" + Uri.encode(title))).setPackage(pkg))
            ctx.packageManager.getLaunchIntentForPackage(pkg)?.let { add(it) }
        }
        for (i in tries) {
            i.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
            if (i.resolveActivity(ctx.packageManager) == null) continue
            if (runCatching { ctx.startActivity(i) }.isSuccess) return true
        }
        return false
    }
}
