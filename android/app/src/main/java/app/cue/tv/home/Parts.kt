@file:OptIn(
    androidx.tv.material3.ExperimentalTvMaterial3Api::class,
    androidx.compose.ui.ExperimentalComposeUiApi::class,
    androidx.compose.foundation.ExperimentalFoundationApi::class,
)

package app.cue.tv.home

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.layout.offset
import androidx.compose.ui.zIndex
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.widthIn
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.draw.drawBehind
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.foundation.gestures.BringIntoViewSpec
import androidx.compose.ui.focus.FocusDirection
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.focusRestorer
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Border
import androidx.tv.material3.Card
import androidx.tv.material3.CardDefaults
import androidx.tv.material3.Text
import coil.compose.AsyncImage

// Pieces the app's screens share: colours, artwork, posters, rows.

val Background = Color(0xFF101114)
val Panel = Color(0xFF1B1D22)
val Dim = Color(0xFFA6A8AE)
val Teal = Color(0xFF34D1BF)

// The profile's colour (or Cue's teal), for focus rings and progress.
val LocalAccent = compositionLocalOf { Teal }

fun accentOf(hex: String): Color = try {
    if (hex.length == 7 && hex.startsWith("#")) Color(android.graphics.Color.parseColor(hex)) else Teal
} catch (e: Exception) {
    Teal
}

// TMDB pictures a TV needs are smaller than the web's: they load faster.
fun small(url: String?): String? = url?.replace("/t/p/w500/", "/t/p/w342/")?.replace("/t/p/w1280/", "/t/p/w780/")

// Which card had the highlight on a screen, so coming back puts it there.
class FocusMemory {
    var key: String? = null
    private val requesters = HashMap<String, FocusRequester>()
    fun requester(k: String): FocusRequester = requesters.getOrPut(k) { FocusRequester() }
    fun restore(fallback: String?): Boolean {
        val k = key ?: fallback ?: return false
        return runCatching { requester(k).requestFocus() }.isSuccess
    }
}

@Composable
fun Art(url: String?, modifier: Modifier = Modifier, label: String = "") {
    Box(modifier.background(Panel), contentAlignment = Alignment.Center) {
        if (label.isNotEmpty()) Text(label, color = Dim, fontSize = 11.sp, maxLines = 3, modifier = Modifier.padding(8.dp))
        if (url != null) AsyncImage(model = url, contentDescription = null, contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize())
    }
}

// A card that lifts, gets a thin white edge and a soft glow when the
// highlight is on it, its name underneath (white while highlighted).
// wide is Continue Watching's: the picture with the episode's code on it,
// and the show and the episode's name underneath, as in the TV apps.
@Composable
fun PosterCard(card: Tile, memory: FocusMemory, memKey: String, wide: Boolean = false, onFocus: (Tile) -> Unit = {}, onClick: (Tile) -> Unit) {
    val accent = LocalAccent.current
    val w: Dp = if (wide) 224.dp else 112.dp
    val h: Dp = if (wide) 126.dp else 168.dp
    var focused by remember { mutableStateOf(false) }
    val shape = RoundedCornerShape(CardCorner)
    val api = LocalApi.current
    val ex = if (wide && api != null) rememberExtras(api, card.kind, card.tmdbId, ask = focused) else null
    val showLogo = focused && !ex?.logo.isNullOrEmpty()
    // Drawn above its neighbours while it's lifted.
    Column(Modifier.width(w).zIndex(if (focused) 1f else 0f)) {
        Card(
            onClick = { onClick(card) },
            modifier = Modifier
                .size(w, h)
                .lift(focused, if (wide) 1.09f else 1.11f, shape)
                .focusRequester(memory.requester(memKey))
                .onFocusChanged {
                    focused = it.isFocused
                    if (it.isFocused) { memory.key = memKey; onFocus(card) }
                },
            shape = CardDefaults.shape(shape),
            scale = CardDefaults.scale(focusedScale = 1f),
            border = CardDefaults.border(focusedBorder = Border(BorderStroke(1.dp, Color(0x59FFFFFF)), shape = shape)),
        ) {
            Box(Modifier.fillMaxSize()) {
                Art(small(if (wide) card.backdrop ?: card.poster else card.poster), Modifier.fillMaxSize(), card.title)
                if (wide) {
                    Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.55f to Color.Transparent, 1f to Color(0x99000000))))
                    if (!showLogo) epCode(card.season, card.episode)?.let { Chip(it, Modifier.align(Alignment.BottomStart).padding(start = 8.dp, bottom = 10.dp)) }
                    if (showLogo) {
                        AsyncImage(
                            model = ex?.logo, contentDescription = null, contentScale = ContentScale.Fit, alignment = Alignment.BottomStart,
                            modifier = Modifier.align(Alignment.BottomStart).padding(start = 10.dp, bottom = 14.dp)
                                .widthIn(max = 130.dp).heightIn(max = 40.dp),
                        )
                    }
                }
                val tintState = LocalTint.current
                card.progress?.takeIf { it > 0 }?.let { p ->
                    Box(Modifier.align(Alignment.BottomStart).padding(horizontal = 8.dp, vertical = 5.dp).fillMaxWidth().height(3.dp)
                        .clip(RoundedCornerShape(2.dp)).background(Color(0x40FFFFFF))) {
                        Box(Modifier.fillMaxWidth(p.toFloat().coerceIn(0f, 1f)).fillMaxHeight().drawBehind { drawRect(tintState.value ?: accent) })
                    }
                }
                if (card.isNew) Chip("NEW", Modifier.align(Alignment.TopStart).padding(7.dp), bg = accent, fg = Color.Black)
                Sheen(focused)
            }
        }
        // The name brightens and steps down with the lift: drawn, not laid out
        // again, each frame.
        val on by animateFloatAsState(if (focused) 1f else 0f, tween(190, easing = Settle), label = "name")
        val under = Modifier.graphicsLayer { alpha = 0.7f + 0.3f * on; translationY = 8.dp.toPx() * on }
        Text(card.title, fontSize = if (wide) 12.sp else 11.sp, fontWeight = FontWeight.Medium, color = Color.White, maxLines = 1,
            overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = 9.dp, start = 2.dp).then(under))
        if (wide) card.episodeTitle?.let {
            Text(it, fontSize = 10.sp, color = Dim, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(start = 2.dp, top = 1.dp).then(under))
        }
    }
}

// A row's name and its cards, sideways.
@Composable
fun CardRow(row: Shelf, memory: FocusMemory, onFocus: (Tile) -> Unit = {}, onClick: (Tile) -> Unit) {
    val wide = row.key == "continue" || row.key == "new-episodes"
    Column(Modifier.padding(bottom = 14.dp)) {
        Text(row.title, fontSize = 14.sp, fontWeight = FontWeight.SemiBold, color = Color(0xF2FFFFFF), modifier = Modifier.padding(start = Gutter + 2.dp, bottom = 2.dp))
        // Coming back to a row puts the highlight on the poster it was on.
        LazyRow(
            modifier = Modifier.focusRestorer(),
            // Room above and below for a card's lift and shadow.
            contentPadding = PaddingValues(horizontal = Gutter, vertical = 16.dp),
            horizontalArrangement = Arrangement.spacedBy(18.dp),
        ) {
            items(row.items, key = { it.key }) { c ->
                PosterCard(c, memory, "${row.key}/${c.key}", wide, onFocus, onClick)
            }
        }
    }
}

@Composable
fun Centered(content: @Composable () -> Unit) {
    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { content() }
}

@Composable
fun Gap(h: Dp) = Spacer(Modifier.height(h))

@Composable
fun HGap(w: Dp) = Spacer(Modifier.width(w))

// Scrolling the TV way: the highlighted row or poster moves to near the
// start of the list (rows to the top, posters to the left), instead of
// only as far as it takes to show it.
val PivotSpec = object : BringIntoViewSpec {
    // Gliding there with a long, soft settle, as Apple TV's lists do.
    override val scrollAnimationSpec: androidx.compose.animation.core.AnimationSpec<Float> =
        androidx.compose.animation.core.tween(480, easing = Settle)

    override fun calculateScrollDistance(offset: Float, size: Float, containerSize: Float): Float =
        offset - containerSize * 0.04f
}

// A title's page scrolls only as far as the highlight needs: opening it with
// Play highlighted leaves the logo, scores and story in view, and going down
// to More Like This lifts the page just enough to show the row.
val GentleSpec = object : BringIntoViewSpec {
    override val scrollAnimationSpec: androidx.compose.animation.core.AnimationSpec<Float> =
        androidx.compose.animation.core.tween(420, easing = Settle)

    override fun calculateScrollDistance(offset: Float, size: Float, containerSize: Float): Float {
        val top = containerSize * 0.08f
        val bottom = containerSize * 0.94f
        return when {
            offset >= top && offset + size <= bottom -> 0f
            offset < top || size > bottom - top -> offset - top
            else -> offset + size - bottom
        }
    }
}

// On a TV a text box would keep the highlight (up and down move its
// cursor): up and down leave it instead.
@Composable
fun Modifier.leaveWithArrows(): Modifier {
    val focus = LocalFocusManager.current
    return onPreviewKeyEvent { e ->
        if (e.type != KeyEventType.KeyDown) return@onPreviewKeyEvent false
        when (e.key) {
            Key.DirectionDown -> focus.moveFocus(FocusDirection.Down)
            Key.DirectionUp -> focus.moveFocus(FocusDirection.Up)
            else -> false
        }
    }
}
