@file:OptIn(androidx.tv.material3.ExperimentalTvMaterial3Api::class)

package app.cue.tv.home

import androidx.compose.foundation.BorderStroke
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
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
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

// A poster that grows and gets a ring in the profile's colour when the
// highlight is on it, its name underneath; wide is Continue Watching's.
@Composable
fun PosterCard(card: Tile, memory: FocusMemory, memKey: String, wide: Boolean = false, onFocus: (Tile) -> Unit = {}, onClick: (Tile) -> Unit) {
    val accent = LocalAccent.current
    val w: Dp = if (wide) 210.dp else 104.dp
    val h: Dp = if (wide) 118.dp else 156.dp
    Column(Modifier.width(w)) {
        Card(
            onClick = { onClick(card) },
            modifier = Modifier
                .size(w, h)
                .focusRequester(memory.requester(memKey))
                .onFocusChanged { if (it.isFocused) { memory.key = memKey; onFocus(card) } },
            shape = CardDefaults.shape(RoundedCornerShape(8.dp)),
            scale = CardDefaults.scale(focusedScale = 1.08f),
            border = CardDefaults.border(focusedBorder = Border(BorderStroke(2.5.dp, accent), shape = RoundedCornerShape(8.dp))),
        ) {
            Box(Modifier.fillMaxSize()) {
                Art(small(if (wide) card.backdrop ?: card.poster else card.poster), Modifier.fillMaxSize(), card.title)
                if (wide) {
                    Box(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(Color.Transparent, Color(0xDD000000)))))
                    Column(Modifier.align(Alignment.BottomStart).padding(10.dp)) {
                        Text(card.title, fontSize = 14.sp, fontWeight = FontWeight.Bold, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        if (card.episodeLine.isNotEmpty()) Text(card.episodeLine, fontSize = 10.sp, color = Color(0xCCFFFFFF), maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                }
                card.progress?.takeIf { it > 0 }?.let { p ->
                    Box(Modifier.align(Alignment.BottomStart).fillMaxWidth().height(3.dp).background(Color(0x55FFFFFF)))
                    Box(Modifier.align(Alignment.BottomStart).fillMaxWidth(p.toFloat().coerceIn(0f, 1f)).height(3.dp).background(accent))
                }
                if (card.isNew) {
                    Text("NEW", fontSize = 9.sp, fontWeight = FontWeight.Bold, color = Color.Black,
                        modifier = Modifier.padding(6.dp).clip(RoundedCornerShape(3.dp)).background(accent).padding(horizontal = 5.dp, vertical = 1.dp))
                }
            }
        }
        if (!wide) {
            Text(card.title, fontSize = 11.sp, color = Dim, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = 6.dp))
        }
    }
}

// A row's name and its cards, sideways.
@Composable
fun CardRow(row: Shelf, memory: FocusMemory, onFocus: (Tile) -> Unit = {}, onClick: (Tile) -> Unit) {
    val wide = row.key == "continue" || row.key == "new-episodes"
    Column(Modifier.padding(bottom = 14.dp)) {
        Text(row.title, fontSize = 16.sp, fontWeight = FontWeight.SemiBold, color = Color.White, modifier = Modifier.padding(start = 48.dp, bottom = 6.dp))
        LazyRow(
            contentPadding = PaddingValues(horizontal = 48.dp, vertical = 10.dp),
            horizontalArrangement = Arrangement.spacedBy(14.dp),
        ) {
            items(row.items, key = { it.key }) { c ->
                PosterCard(c, memory, "${row.key}/${c.key}", wide, onFocus, onClick)
            }
        }
    }
}

// Year, score and kind in a line.
@Composable
fun MetaLine(card: Tile, extra: String = "") {
    val bits = listOfNotNull(
        card.year?.toString(),
        card.rating?.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
        if (card.kind == "tv") "Series" else "Movie",
        extra.takeIf { it.isNotEmpty() },
    )
    Text(bits.joinToString("   ·   "), fontSize = 12.sp, color = Color(0xDDFFFFFF))
}

// The big picture at the top of Home and a title's page, faded into the
// background on the left and at the bottom so the words stand out.
@Composable
fun Backdrop(url: String?, modifier: Modifier = Modifier) {
    Box(modifier) {
        if (url != null) {
            AsyncImage(model = url, contentDescription = null, contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxHeight().fillMaxWidth(0.78f).align(Alignment.TopEnd))
        }
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(0.2f to Background, 0.55f to Background.copy(alpha = 0.6f), 1f to Color.Transparent)))
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.55f to Color.Transparent, 1f to Background)))
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
