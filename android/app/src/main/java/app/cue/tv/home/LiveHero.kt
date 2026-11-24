package app.cue.tv.home

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.zIndex
import androidx.tv.material3.Border
import androidx.tv.material3.Card
import androidx.tv.material3.CardDefaults
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import kotlinx.coroutines.delay

// ---- Live TV's hero on the TV: the game that matters most as an event,
// both teams large over their colors, a countdown to the start (or the
// score, live), where it's played and the channel it's on. Featured beside
// it: the next games, moved through every few seconds (resting the
// highlight on one shows it, and holds it there). Picking the hero plays the
// game's channel.

private const val ROTATE_MS = 8000L

private fun teamColor(hex: String, alpha: Float): Color = try {
    if (hex.length == 6) Color(android.graphics.Color.parseColor("#$hex")).copy(alpha = alpha) else Color(0xFF3A3A3E).copy(alpha = alpha)
} catch (e: Exception) {
    Color(0xFF3A3A3E).copy(alpha = alpha)
}

// "47m", "2h 05m", or "" once it has started or when it's more than a day off.
private fun countdown(startMs: Long, now: Long): String {
    val m = ((startMs - now) / 60_000L).toInt()
    if (startMs <= 0 || m <= 0 || m > 24 * 60) return ""
    return if (m < 60) "${m}m" else "${m / 60}h ${(m % 60).toString().padStart(2, '0')}m"
}

private fun leagueMark(key: String): String? =
    if (key in setOf("nfl", "nba", "nhl", "mlb", "wnba", "mls")) "https://a.espncdn.com/i/teamlogos/leagues/500/$key.png" else null

@Composable
fun LiveHero(board: SportsBoard, onWatch: (String) -> Unit) {
    val games = remember(board) { listOfNotNull(board.big).plus(board.now).plus(board.soon).distinctBy { it.key }.take(4) }
    if (games.isEmpty()) return
    var index by remember { mutableIntStateOf(0) }
    var held by remember { mutableStateOf(false) } // a Featured card has the highlight
    var now by remember { mutableLongStateOf(System.currentTimeMillis()) }
    LaunchedEffect(Unit) {
        while (true) {
            delay(30_000)
            now = System.currentTimeMillis()
        }
    }
    LaunchedEffect(index, held, games.size) {
        if (held || games.size < 2) return@LaunchedEffect
        delay(ROTATE_MS)
        index = (index + 1) % games.size
    }
    val g = games[index.coerceIn(0, games.lastIndex)]
    Row(Modifier.fillMaxWidth().height(250.dp).padding(horizontal = Gutter), horizontalArrangement = Arrangement.spacedBy(14.dp)) {
        HeroCard(g, now, Modifier.weight(1f).fillMaxHeight()) { g.channelId?.let(onWatch) }
        if (games.size > 1) {
            Column(Modifier.width(236.dp).fillMaxHeight(), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                Row(Modifier.fillMaxWidth().padding(horizontal = 2.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text("Featured", fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = Color.White, modifier = Modifier.weight(1f))
                    Text("${index + 1} / ${games.size}", fontSize = 10.sp, color = Dim)
                }
                games.forEachIndexed { i, f ->
                    FeaturedCard(f, selected = i == index, running = i == index && !held, Modifier.weight(1f),
                        onFocus = { focused ->
                            held = focused
                            if (focused) index = i
                        }) { f.channelId?.let(onWatch) }
                }
            }
        }
    }
}

@Composable
private fun HeroCard(g: GameRow, now: Long, modifier: Modifier, onClick: () -> Unit) {
    val shape = RoundedCornerShape(18.dp)
    Card(
        onClick = onClick,
        modifier = modifier,
        shape = CardDefaults.shape(shape),
        scale = CardDefaults.scale(focusedScale = 1.01f),
        colors = CardDefaults.colors(containerColor = Color(0xFF111113), focusedContainerColor = Color(0xFF111113)),
        border = CardDefaults.border(
            border = Border(BorderStroke(1.dp, Color(0x14FFFFFF)), shape = shape),
            focusedBorder = Border(BorderStroke(2.dp, Color.White), shape = shape),
        ),
    ) {
        AnimatedContent(
            targetState = g,
            contentKey = { it.key },
            transitionSpec = { fadeIn(tween(450, easing = Settle)) togetherWith fadeOut(tween(250)) },
            label = "live-hero",
        ) { game ->
            Box(Modifier.fillMaxSize()) {
                // The two teams' colors, from each side, over graphite.
                Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(
                    0f to teamColor(game.away.color, 0.55f), 0.5f to Color(0xFF111113), 1f to teamColor(game.home.color, 0.55f))))
                Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0f to Color.Transparent, 1f to Color(0x99000000))))
                Column(Modifier.fillMaxSize()) {
                    Row(Modifier.weight(1f).fillMaxWidth().padding(horizontal = 24.dp), verticalAlignment = Alignment.CenterVertically) {
                        HeroTeam(game.away, Modifier.weight(1f))
                        HeroMiddle(game, now, Modifier.width(170.dp))
                        HeroTeam(game.home, Modifier.weight(1f))
                    }
                    Row(Modifier.fillMaxWidth().background(Color(0x59000000)).padding(vertical = 8.dp), horizontalArrangement = Arrangement.Center,
                        verticalAlignment = Alignment.CenterVertically) {
                        if (game.venue.isNotEmpty()) Text(game.venue, fontSize = 11.sp, color = Color(0xBFFFFFFF))
                        if (game.venue.isNotEmpty() && game.where.isNotEmpty()) HGap(16.dp)
                        if (game.where.isNotEmpty()) {
                            Text(if (game.channelId != null) "▶  Watch on ${game.where}" else game.where, fontSize = 11.sp,
                                fontWeight = FontWeight.SemiBold, color = Color.White)
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun HeroTeam(t: GameSide, modifier: Modifier) {
    Column(modifier, horizontalAlignment = Alignment.CenterHorizontally) {
        if (t.logo != null) {
            AsyncImage(model = t.logo, contentDescription = null, contentScale = ContentScale.Fit, modifier = Modifier.size(92.dp))
            Gap(10.dp)
        }
        if (t.city.isNotEmpty()) Text(t.city.uppercase(), fontSize = 10.sp, fontWeight = FontWeight.SemiBold, letterSpacing = 1.5.sp, color = Color(0xBFFFFFFF))
        Text(t.name.uppercase(), fontFamily = Type.display(), fontSize = 24.sp, fontWeight = FontWeight.ExtraBold, color = Color.White,
            maxLines = 1, overflow = TextOverflow.Ellipsis, textAlign = TextAlign.Center)
        if (t.record.isNotEmpty()) Text(t.record, fontSize = 12.sp, fontWeight = FontWeight.Bold, color = Color(0xD9FFFFFF))
    }
}

@Composable
private fun HeroMiddle(g: GameRow, now: Long, modifier: Modifier) {
    Column(modifier, horizontalAlignment = Alignment.CenterHorizontally) {
        val mark = leagueMark(g.leagueKey)
        if (mark != null) {
            AsyncImage(model = mark, contentDescription = null, contentScale = ContentScale.Fit, modifier = Modifier.size(34.dp))
            Gap(8.dp)
        }
        val left = countdown(g.startMs, now)
        when {
            g.live -> {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Box(Modifier.size(7.dp).clip(CircleShape).background(Color(0xFFFF3B52)))
                    HGap(6.dp)
                    Text("LIVE", fontSize = 11.sp, fontWeight = FontWeight.Bold, letterSpacing = 2.sp, color = Color(0xFFFF5A6E))
                }
                Text("${g.away.score.ifEmpty { "0" }} – ${g.home.score.ifEmpty { "0" }}", fontFamily = Type.display(), fontSize = 40.sp,
                    fontWeight = FontWeight.ExtraBold, color = Color.White)
                Text(g.detail, fontSize = 12.sp, color = Color(0xCCFFFFFF))
            }
            left.isNotEmpty() -> {
                Text("STARTS IN", fontSize = 10.sp, fontWeight = FontWeight.Bold, letterSpacing = 2.5.sp, color = Color(0xBFFFFFFF))
                Text(left, fontFamily = Type.display(), fontSize = 40.sp, fontWeight = FontWeight.ExtraBold, color = Color.White)
                Text(g.detail, fontSize = 12.sp, color = Color(0xCCFFFFFF))
            }
            else -> {
                Text(g.league.uppercase(), fontSize = 10.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.5.sp, color = Color(0xBFFFFFFF),
                    maxLines = 2, textAlign = TextAlign.Center)
                Gap(4.dp)
                Text(g.detail, fontSize = 18.sp, fontWeight = FontWeight.Bold, color = Color.White)
            }
        }
    }
}

@Composable
private fun FeaturedCard(g: GameRow, selected: Boolean, running: Boolean, modifier: Modifier, onFocus: (Boolean) -> Unit, onClick: () -> Unit) {
    val shape = RoundedCornerShape(12.dp)
    Card(
        onClick = onClick,
        modifier = modifier.fillMaxWidth().onFocusChanged { onFocus(it.isFocused) },
        shape = CardDefaults.shape(shape),
        scale = CardDefaults.scale(focusedScale = 1.03f),
        colors = CardDefaults.colors(
            containerColor = if (selected) Color(0x1FFFFFFF) else Color(0x0DFFFFFF),
            focusedContainerColor = Color(0x2EFFFFFF),
        ),
        border = CardDefaults.border(
            border = Border(BorderStroke(1.dp, if (selected) Color(0x3DFFFFFF) else Color(0x12FFFFFF)), shape = shape),
            focusedBorder = Border(BorderStroke(2.dp, Color.White), shape = shape),
        ),
    ) {
        Box(Modifier.fillMaxSize()) {
            Column(Modifier.fillMaxSize().padding(horizontal = 10.dp, vertical = 7.dp), verticalArrangement = Arrangement.Center) {
                Text(g.league.substringBefore(" · ").uppercase(), fontSize = 8.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp, color = Dim,
                    maxLines = 1, overflow = TextOverflow.Ellipsis)
                Gap(4.dp)
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Row(Modifier.weight(1f), verticalAlignment = Alignment.CenterVertically) {
                        g.away.logo?.let { AsyncImage(model = it, contentDescription = null, modifier = Modifier.size(18.dp)) }
                        HGap(5.dp)
                        Text(g.away.name, fontSize = 11.sp, fontWeight = FontWeight.Bold, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                    Text(if (g.live) "${g.away.score}–${g.home.score}" else g.detail.substringAfter(' ', g.detail),
                        fontSize = 10.sp, fontWeight = FontWeight.Bold, color = if (g.live) Color(0xFFFF5A6E) else Color.White, maxLines = 1)
                    Row(Modifier.weight(1f), horizontalArrangement = Arrangement.End, verticalAlignment = Alignment.CenterVertically) {
                        Text(g.home.name, fontSize = 11.sp, fontWeight = FontWeight.Bold, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
                        HGap(5.dp)
                        g.home.logo?.let { AsyncImage(model = it, contentDescription = null, modifier = Modifier.size(18.dp)) }
                    }
                }
            }
            // How long until the hero moves on: a line along the bottom.
            if (running) {
                var started by remember(g.key) { mutableStateOf(false) }
                LaunchedEffect(g.key) { started = true }
                val p by androidx.compose.animation.core.animateFloatAsState(
                    if (started) 1f else 0f, tween(ROTATE_MS.toInt(), easing = LinearEasing), label = "featured-bar")
                Box(Modifier.align(Alignment.BottomStart).fillMaxWidth(p).height(2.dp).background(Color.White).zIndex(1f))
            }
        }
    }
}

// A channel in Live TV's On now: its logo, what's on, how far through and
// how long is left, a star when it's a favourite.
@Composable
fun ChannelTile(api: CueApi, ch: Channel, modifier: Modifier = Modifier, onFocus: () -> Unit = {}, onClick: () -> Unit) {
    val shape = RoundedCornerShape(14.dp)
    Card(
        onClick = onClick,
        modifier = modifier.height(86.dp).onFocusChanged { if (it.isFocused) onFocus() },
        shape = CardDefaults.shape(shape),
        scale = CardDefaults.scale(focusedScale = 1.03f),
        colors = CardDefaults.colors(containerColor = Color(0x0DFFFFFF), focusedContainerColor = Color(0x24FFFFFF)),
        border = CardDefaults.border(
            border = Border(BorderStroke(1.dp, Color(0x12FFFFFF)), shape = shape),
            focusedBorder = Border(BorderStroke(2.dp, Color.White), shape = shape),
        ),
    ) {
        Row(Modifier.fillMaxSize().padding(horizontal = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.size(100.dp, 62.dp).clip(RoundedCornerShape(10.dp)).background(Color(0x0FFFFFFF)), contentAlignment = Alignment.Center) {
                ChannelLogo(api, ch, Modifier.fillMaxSize().padding(10.dp))
            }
            HGap(14.dp)
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Box(Modifier.size(6.dp).clip(CircleShape).background(Color(0xFFFF3B52)))
                    HGap(6.dp)
                    Text(ch.name.uppercase(), fontSize = 9.sp, fontWeight = FontWeight.Bold, letterSpacing = 0.8.sp, color = Color(0xB3FFFFFF),
                        maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                    if (ch.favorite) Text("★", fontSize = 12.sp, color = Color(0xFFF5C518))
                }
                Gap(3.dp)
                Text(ch.now?.title ?: ch.name, fontSize = 14.sp, fontWeight = FontWeight.Bold, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
                ch.now?.let { show ->
                    Gap(7.dp)
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Box(Modifier.weight(1f).height(3.dp).clip(RoundedCornerShape(2.dp)).background(Color(0x24FFFFFF))) {
                            Box(Modifier.fillMaxWidth(show.progress()).fillMaxHeight().background(Color(0xD9FFFFFF)))
                        }
                        HGap(10.dp)
                        val left = ((show.stop - System.currentTimeMillis()) / 60_000L).coerceAtLeast(0)
                        Text(if (left < 60) "${left}m left" else "${left / 60}h ${left % 60}m left", fontSize = 10.sp, color = Dim)
                    }
                }
            }
        }
    }
}
