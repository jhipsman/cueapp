package app.cue.tv

import android.content.Context

// The channel last watched on this TV: Live TV opens on it, and the player
// keeps it as you change channel.
object LastChannel {
    private const val KEY = "lastLiveChannel"

    fun save(ctx: Context, id: String) {
        if (id.isNotEmpty()) ctx.getSharedPreferences("cue", Context.MODE_PRIVATE).edit().putString(KEY, id).apply()
    }

    fun get(ctx: Context): String? = ctx.getSharedPreferences("cue", Context.MODE_PRIVATE).getString(KEY, null)
}
