package app.cue.tv

import android.content.Context
import android.graphics.Typeface
import android.os.Build

// Cue's own fonts, as on its web pages: Sora for names and headings, IBM
// Plex Sans for everything else, IBM Plex Mono for times and numbers
// (assets/fonts, the same files as web/public/fonts).
object Fonts {
    private val cache = HashMap<String, Typeface>()

    fun display(ctx: Context, weight: Int = 700): Typeface = load(ctx, "fonts/Sora-Variable.ttf", weight)

    fun text(ctx: Context, weight: Int = 400): Typeface = load(ctx, "fonts/IBMPlexSans-Variable.ttf", weight)

    fun mono(ctx: Context): Typeface = load(ctx, "fonts/IBMPlexMono-Medium.ttf", 500)

    private fun load(ctx: Context, path: String, weight: Int): Typeface = cache.getOrPut("$path@$weight") {
        try {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                Typeface.Builder(ctx.assets, path).setFontVariationSettings("'wght' $weight").build()
            } else {
                Typeface.create(Typeface.createFromAsset(ctx.assets, path), if (weight >= 600) Typeface.BOLD else Typeface.NORMAL)
            }
        } catch (e: Exception) {
            if (weight >= 600) Typeface.DEFAULT_BOLD else Typeface.DEFAULT
        }
    }
}
