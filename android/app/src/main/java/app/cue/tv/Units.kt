package app.cue.tv

import android.content.Context

// screenUnit is 1/960 of the screen's long side, in pixels: the player's
// sizes follow the screen itself, so they look the same on every TV
// whatever its density or font-size setting (2 on a 1080p screen).
fun screenUnit(context: Context): Float {
    val m = context.resources.displayMetrics
    return maxOf(m.widthPixels, m.heightPixels) / 960f
}
