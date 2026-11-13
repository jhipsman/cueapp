@file:OptIn(androidx.tv.material3.ExperimentalTvMaterial3Api::class)

package app.cue.tv.home

import android.os.Build
import androidx.compose.animation.Crossfade
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.foundation.layout.offset
import androidx.tv.material3.Border
import androidx.compose.foundation.BorderStroke
import androidx.tv.material3.LocalContentColor
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.width
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.mutableStateMapOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.blur
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.TextUnit
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Button
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.Text
import app.cue.tv.Fonts
import coil.compose.AsyncImage
import coil.request.ImageRequest

// The look of the app's screens, as in the TV apps people compare it with
// (Apple's TV app, Strand, Netflix): the picture of whatever is highlighted
// glows softly behind everything, titles are drawn with their own logo
// artwork, buttons are pills that turn white when highlighted, and the
// words are small and quiet so the pictures lead.

// Cue's fonts in the app's screens (the player has them too): IBM Plex
// Sans for words, Sora for a name when a title has no logo.
object Type {
    @Volatile private var text: FontFamily? = null
    @Volatile private var display: FontFamily? = null

    @Composable
    fun text(): FontFamily {
        val ctx = LocalContext.current
        return text ?: FontFamily(Fonts.text(ctx, 450)).also { text = it }
    }

    @Composable
    fun display(): FontFamily {
        val ctx = LocalContext.current
        return display ?: FontFamily(Fonts.display(ctx, 750)).also { display = it }
    }
}

// TMDB's smallest backdrop, for the glow behind a screen: drawn full screen
// it is soft by itself, even on TVs too old for a real blur.
fun tiny(url: String?): String? = url?.replace(Regex("/t/p/w\\d+/"), "/t/p/w300/")

// The highlighted title's picture, soft and dark, filling the screen behind
// everything, changing with a slow fade as the highlight moves.
@Composable
fun AmbientBackground(url: String?) {
    Box(Modifier.fillMaxSize().background(Background)) {
        Crossfade(targetState = tiny(url), animationSpec = tween(700), label = "ambient") { u ->
            if (u != null) {
                AsyncImage(
                    model = ImageRequest.Builder(LocalContext.current).data(u).crossfade(false).build(),
                    contentDescription = null,
                    contentScale = ContentScale.Crop,
                    alpha = 0.55f,
                    modifier = Modifier.fillMaxSize().let { if (Build.VERSION.SDK_INT >= 31) it.blur(48.dp) else it },
                )
            }
        }
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0f to Color(0x99000000), 0.45f to shade(0.80f), 1f to shade(0.95f))))
        // With no picture (Live TV, Search, the profile): a soft glow of the
        // profile's colour from the top left instead.
        val glow by androidx.compose.animation.core.animateFloatAsState(if (url == null) 1f else 0.75f, tween(700), label = "accent-glow")
        if (glow > 0f) {
            val accent = tint()
            Box(Modifier.fillMaxSize().drawBehind {
                drawRect(Brush.radialGradient(
                    listOf(accent.copy(alpha = (0.18f * glow * GlowAmount).coerceAtMost(1f)), Color.Transparent),
                    center = androidx.compose.ui.geometry.Offset(size.width * 0.12f, 0f),
                    radius = size.width * 0.6f,
                ))
            })
        }
    }
}

// Logo artwork per title, looked up once as the highlight reaches it ("" is
// none: the name is drawn instead).
// A title's extras: its logo artwork, the main colour of its picture and a
// YouTube trailer, looked up once per title (GET /api/watch/extras).
data class Extras(val logo: String, val colour: Color?, val trailer: String)

object ExtrasStore {
    val found = mutableStateMapOf<String, Extras>()
    private val asking = HashSet<String>()

    suspend fun load(api: CueApi, kind: String, id: Int) {
        val k = "$kind-$id"
        if (found.containsKey(k) || !asking.add(k)) return
        try {
            val o = api.obj("/api/watch/extras/$kind/$id")
            found[k] = Extras(o.optString("logo"), parseColour(o.optString("colour")), o.optString("trailer"))
        } catch (e: Exception) {
            found[k] = Extras("", null, "")
        } finally {
            asking.remove(k)
        }
    }

    // "r, g, b" as the server sends it.
    private fun parseColour(s: String): Color? {
        val p = s.split(",").mapNotNull { it.trim().toIntOrNull() }
        return if (p.size == 3) Color(p[0], p[1], p[2]) else null
    }
}

@Composable
fun rememberExtras(api: CueApi, kind: String, tmdbId: Int, ask: Boolean = true): Extras? {
    val k = "$kind-$tmdbId"
    LaunchedEffect(k, ask) { if (ask) ExtrasStore.load(api, kind, tmdbId) }
    return ExtrasStore.found[k]
}

@Composable
fun rememberLogo(api: CueApi, kind: String, tmdbId: Int, known: String? = null): String? {
    val ex = rememberExtras(api, kind, tmdbId, ask = known == null)
    return known ?: ex?.logo
}

// The colour of the picture in view: the glow, halos and progress bars take
// it (the profile's colour until one is known).
val LocalTint = compositionLocalOf { mutableStateOf<Color?>(null) }

@Composable
fun tint(): Color {
    val t = LocalTint.current.value ?: LocalAccent.current
    return androidx.compose.animation.animateColorAsState(t, tween(800), label = "tint").value
}

// A title's name as its logo, left-aligned, or in Sora when it has none.
// Until the lookup answers nothing is drawn, so the name doesn't flash
// before the logo replaces it.
@Composable
fun TitleMark(title: String, logo: String?, maxWidth: Dp, maxHeight: Dp, fontSize: TextUnit) {
    when {
        logo == null -> Box(Modifier.heightIn(min = maxHeight * 0.6f))
        logo.isNotEmpty() -> AsyncImage(
            model = ImageRequest.Builder(LocalContext.current).data(logo).crossfade(250).build(),
            contentDescription = title,
            contentScale = ContentScale.Fit,
            alignment = Alignment.BottomStart,
            modifier = Modifier.widthIn(max = maxWidth).heightIn(max = maxHeight),
        )
        else -> Text(title, fontFamily = Type.display(), fontSize = fontSize, fontWeight = FontWeight.Bold, color = Color.White,
            maxLines = 2, overflow = TextOverflow.Ellipsis, lineHeight = fontSize * 1.08f, modifier = Modifier.widthIn(max = maxWidth))
    }
}

// A pill button, as on Apple TV: every one frosted glass with white words
// (the main one a little brighter), and only the one the remote is on turns
// solid white with dark words and grows a touch. One look throughout, so
// it's always clear where the highlight is. An icon (a drawable) can sit
// before the words.
@Composable
fun Pill(text: String, primary: Boolean = false, modifier: Modifier = Modifier, small: String? = null, icon: Int? = null, onClick: () -> Unit) {
    val shape = RoundedCornerShape(50)
    Button(
        onClick = onClick,
        modifier = modifier.height(36.dp),
        shape = ButtonDefaults.shape(shape),
        scale = ButtonDefaults.scale(focusedScale = 1.05f, pressedScale = 0.98f),
        colors = ButtonDefaults.colors(
            containerColor = if (primary) Color(0x40FFFFFF) else Color(0x24FFFFFF),
            contentColor = Color.White,
            focusedContainerColor = Color.White,
            focusedContentColor = Color(0xFF0B0C0F),
            pressedContainerColor = Color(0xFFE6E7EA),
            pressedContentColor = Color(0xFF0B0C0F),
        ),
        border = ButtonDefaults.border(
            border = Border(BorderStroke(1.dp, Color(0x1FFFFFFF)), shape = shape),
            focusedBorder = Border(BorderStroke(0.dp, Color.Transparent), shape = shape),
        ),
        contentPadding = PaddingValues(start = if (icon != null) 14.dp else 18.dp, end = 18.dp, top = 0.dp, bottom = 0.dp),
    ) {
        if (icon != null) {
            androidx.tv.material3.Icon(
                androidx.compose.ui.res.painterResource(icon), contentDescription = null,
                modifier = Modifier.size(14.dp),
            )
            Spacer(Modifier.width(8.dp))
        }
        Text(text, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
        small?.let {
            Spacer(Modifier.width(8.dp))
            Text(it, fontSize = 12.sp, fontWeight = FontWeight.SemiBold, color = LocalContentColor.current.copy(alpha = 0.6f), maxLines = 1)
        }
    }
}

// A choice among several (a season, a channel group): plain words, the
// chosen one bright with a short line under it in the profile's colour; the
// one the remote is on turns into a white pill, as buttons do.
@Composable
fun ChoiceTab(text: String, selected: Boolean, modifier: Modifier = Modifier, onClick: () -> Unit) {
    val shape = RoundedCornerShape(50)
    val line = tint()
    Button(
        onClick = onClick,
        modifier = modifier.height(34.dp),
        shape = ButtonDefaults.shape(shape),
        scale = ButtonDefaults.scale(focusedScale = 1.05f, pressedScale = 0.98f),
        colors = ButtonDefaults.colors(
            containerColor = Color.Transparent,
            contentColor = if (selected) Color.White else Color(0xA6FFFFFF),
            focusedContainerColor = Color.White,
            focusedContentColor = Color(0xFF0B0C0F),
            pressedContainerColor = Color(0xFFE6E7EA),
            pressedContentColor = Color(0xFF0B0C0F),
        ),
        contentPadding = PaddingValues(horizontal = 16.dp, vertical = 0.dp),
    ) {
        Box(contentAlignment = Alignment.BottomCenter) {
            Text(text, fontSize = 13.sp, fontWeight = if (selected) FontWeight.Bold else FontWeight.Medium, maxLines = 1,
                modifier = Modifier.padding(bottom = 3.dp))
            if (selected) {
                Box(Modifier.offset(y = 4.dp).width(18.dp).height(2.dp).clip(RoundedCornerShape(1.dp)).background(line))
            }
        }
    }
}

// The small facts under a logo: scores (each with its source), year,
// length, genre, as in the TV apps.
@Composable
fun FactsRow(card: Tile, ratings: Ratings? = null, extra: List<String> = emptyList()) {
    Row(horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
        ratings?.imdb?.let { Badge("IMDb", Color(0xFFF5C518), Color.Black); Fact(it) }
        ratings?.rottenTomatoes?.let { Fact("🍅 $it") }
        ratings?.metacritic?.let { Badge("MC", Color(0xFF3B3F45), Color.White); Fact(it) }
        if (ratings?.imdb == null) card.rating?.takeIf { it > 0 }?.let { Fact("★ %.1f".format(it)) }
        card.year?.let { Fact(it.toString()) }
        extra.forEach { Fact(it) }
    }
}

@Composable
private fun Fact(s: String) = Text(s, fontSize = 11.sp, fontWeight = FontWeight.Medium, color = Color(0xD9FFFFFF), maxLines = 1)

@Composable
fun Badge(s: String, bg: Color, fg: Color) = Text(s, fontSize = 8.sp, fontWeight = FontWeight.Black, color = fg,
    modifier = Modifier.clip(RoundedCornerShape(3.dp)).background(bg).padding(horizontal = 4.dp, vertical = 1.dp))

// A dark chip on a picture: "S04E04", "NEW".
@Composable
fun Chip(s: String, modifier: Modifier = Modifier, bg: Color = Color(0xB3000000), fg: Color = Color.White) =
    Text(s, fontSize = 8.sp, fontWeight = FontWeight.Bold, color = fg, letterSpacing = 0.4.sp,
        modifier = modifier.clip(RoundedCornerShape(4.dp)).background(bg).padding(horizontal = 5.dp, vertical = 2.dp))

fun epCode(season: Int?, episode: Int?): String? =
    if (season != null && episode != null) "S%02dE%02d".format(season, episode) else null

// ---- Motion, as on Apple TV ----
//
// Nothing snaps. The highlighted card springs toward you (it grows, rises a
// little and its shadow deepens) and a soft light sweeps across it once;
// lists glide to their place with a long ease-out; things arriving fade in
// as they rise into place, one after another.

// One margin, corner sizes and gaps for every screen, so edges line up.
val Gutter = 48.dp
val CardCorner = 12.dp
val PanelCorner = 20.dp

// Apple's curve: quick to start, a long gentle settle.
val Settle = androidx.compose.animation.core.CubicBezierEasing(0.2f, 0.8f, 0.2f, 1f)


// The lift a highlighted card gets: drawn above its neighbours, bigger,
// raised, with a shadow under it.
@Composable
fun Modifier.lift(focused: Boolean, scale: Float, shape: androidx.compose.ui.graphics.Shape): Modifier {
    // A short glide, no spring: a TV's processor keeps up with it, and the
    // highlight lands where the remote put it at once.
    val s by androidx.compose.animation.core.animateFloatAsState(if (focused) scale else 1f,
        androidx.compose.animation.core.tween(190, easing = Settle), label = "lift")
    // The shadow glows in the picture's colour. It's read as the frame is
    // drawn, so a new colour doesn't rebuild every card on the screen.
    val tintState = LocalTint.current
    val accent = LocalAccent.current
    val light = lightMode() // a slow TV: no soft shadow (costly to draw)
    return this.graphicsLayer {
        val glow = tintState.value ?: accent
        val k = ((s - 1f) / (scale - 1f)).coerceIn(0f, 1f)
        // The theme's glow: none (a plain shadow), soft, or strong.
        val g = GlowAmount
        spotShadowColor = if (g == 0f) Color.Black else glow.copy(alpha = (0.55f * g).coerceAtMost(1f))
        ambientShadowColor = if (g == 0f) Color.Black else glow.copy(alpha = (0.35f * g).coerceAtMost(1f))
        scaleX = s
        scaleY = s
        translationY = -4.dp.toPx() * k
        // The shadow steps on with the highlight rather than growing frame by
        // frame (redrawing a big soft shadow every frame is what makes a
        // Fire TV stutter).
        shadowElevation = if (focused && !light) 18.dp.toPx() else 0f
        this.shape = shape
        clip = false
    }
}

// The light on a highlighted card: a soft shine from the top, and a band of
// light that sweeps across it once as the highlight arrives.
@Composable
fun Sheen(focused: Boolean, modifier: Modifier = Modifier) {
    val light = lightMode() // a slow TV: the shine, without the moving band
    val sweep = remember { androidx.compose.animation.core.Animatable(-1f) }
    val glow by androidx.compose.animation.core.animateFloatAsState(if (focused) 1f else 0f,
        androidx.compose.animation.core.tween(300), label = "glow")
    LaunchedEffect(focused) {
        if (focused && !light) {
            sweep.snapTo(-0.5f)
            sweep.animateTo(1.5f, androidx.compose.animation.core.tween(1100, easing = Settle))
        } else {
            sweep.snapTo(-1f)
        }
    }
    if (!focused && glow == 0f) return
    Box(modifier.fillMaxSize().drawBehind {
        drawRect(Brush.verticalGradient(0f to Color.White.copy(alpha = 0.16f * glow), 0.45f to Color.Transparent))
        val p = sweep.value
        if (p > -1f && p < 1.5f) {
            val w = size.width
            drawRect(Brush.linearGradient(
                0f to Color.Transparent, 0.5f to Color.White.copy(alpha = 0.20f), 1f to Color.Transparent,
                start = androidx.compose.ui.geometry.Offset(w * (p - 0.35f), 0f),
                end = androidx.compose.ui.geometry.Offset(w * (p + 0.35f), size.height),
            ))
        }
    })
}

// Fades in as it rises into place, after delayMs: a page's parts arrive
// one after another.
@Composable
fun Modifier.rise(delayMs: Int = 0, distance: Dp = 16.dp): Modifier {
    val a = remember { androidx.compose.animation.core.Animatable(0f) }
    LaunchedEffect(Unit) {
        kotlinx.coroutines.delay(delayMs.toLong())
        a.animateTo(1f, androidx.compose.animation.core.tween(560, easing = Settle))
    }
    return graphicsLayer {
        alpha = a.value
        translationY = (1f - a.value) * distance.toPx()
    }
}

// A picture that settles from a slight zoom when it first shows.
@Composable
fun Modifier.settleIn(from: Float = 1.06f): Modifier {
    val a = remember { androidx.compose.animation.core.Animatable(0f) }
    LaunchedEffect(Unit) { a.animateTo(1f, androidx.compose.animation.core.tween(1400, easing = Settle)) }
    return graphicsLayer {
        val z = from - (from - 1f) * a.value
        scaleX = z
        scaleY = z
        alpha = (a.value * 2f).coerceAtMost(1f)
    }
}

// A slow drift on a big picture (Apple's top shelf moves gently).
@Composable
fun Modifier.drift(): Modifier {
    val t = androidx.compose.animation.core.rememberInfiniteTransition(label = "drift")
    val z by t.animateFloat(1f, 1.05f, androidx.compose.animation.core.infiniteRepeatable(
        androidx.compose.animation.core.tween(18000, easing = androidx.compose.animation.core.LinearEasing),
        androidx.compose.animation.core.RepeatMode.Reverse), label = "z")
    return graphicsLayer {
        scaleX = z
        scaleY = z
    }
}
