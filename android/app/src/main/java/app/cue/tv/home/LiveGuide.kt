package app.cue.tv.home

import app.cue.tv.LastChannel

import androidx.activity.compose.BackHandler
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Border
import androidx.tv.material3.Card
import androidx.tv.material3.CardDefaults
import androidx.tv.material3.Text
import kotlinx.coroutines.delay

// ---- Live TV ----
//
// Live TV as a TV guide. At the top, what's on the channel the highlight is
// on, big: its name, how long it runs and when it started, what it's about
// and how far through it is, with the channel itself playing on the right
// (once the remote rests), fading into the page. Under it, the games on now
// (when there are any), then the topics and the country as tabs, then the
// guide: a row per channel with its shows laid along the next two hours,
// sized by how long they run, and a line at the time now. Select plays the
// channel; in the player, up and down change channel.

private const val HALF_HOUR = 30 * 60_000L
private const val WINDOW = 4 * HALF_HOUR // two hours across the guide
private val NameWidth = 176.dp // the channel names down the left of the guide
private val CellGap = 4.dp
private val RowHeight = 44.dp
private val LiveRed = Color(0xFFE5484D)

@OptIn(androidx.compose.foundation.ExperimentalFoundationApi::class)
@Composable
fun LiveScreen(api: CueApi, playback: Playback, openWeb: (String) -> Unit) {
    val ctx = androidx.compose.ui.platform.LocalContext.current
    var live by remember { mutableStateOf<LiveChannels?>(null) }
    var problem by remember { mutableStateOf("") }
    var group by remember { mutableStateOf("") } // "★" favourites, "*" every topic, "you" For you, "tonight", "list:<id>" a list, else a topic's key
    var picks by remember { mutableStateOf(LivePicks(emptyList(), emptyList())) }
    LaunchedEffect(Unit) { runCatching { api.livePicks() }.getOrNull()?.let { picks = it } }
    var country by remember { mutableStateOf<String?>(null) } // "" is every country
    var picking by remember { mutableStateOf(false) } // the tabs list the countries
    var find by remember { mutableStateOf("") } // a game no channel was matched to: channels showing it
    var board by remember { mutableStateOf(SportsBoard(null, emptyList(), emptyList())) }
    // The time now, a minute at a time: the guide's line and the shows' progress follow it.
    var now by remember { mutableLongStateOf(System.currentTimeMillis()) }
    LaunchedEffect(Unit) {
        while (true) {
            delay(30_000)
            now = System.currentTimeMillis()
        }
    }
    LaunchedEffect(Unit) {
        // Scores change: read again every minute while Live TV is open.
        while (true) {
            runCatching { api.sportsBoard() }.getOrNull()?.let { board = it }
            delay(60_000)
        }
    }
    LaunchedEffect(Unit) {
        try {
            val l = api.channels()
            live = l
            if (group.isEmpty()) group = if (l.channels.any { it.favorite }) "★" else "*"
            if (country == null) country = api.liveCountry ?: l.home
        } catch (e: Exception) {
            problem = e.message ?: "Couldn't load Live TV."
        }
    }
    val l = live
    if (l == null) {
        Centered { Text(problem.ifEmpty { "Loading…" }, color = Dim) }
        return
    }
    if (!l.configured) {
        Centered { Text("Live TV isn't set up on this Cue yet: add your IPTV provider in Cue's settings.", color = Dim) }
        return
    }
    // Sorted by Cue's catalog, not the provider's groups: one of each
    // channel (no copies, no dividers), in the picked country.
    val place = country.orEmpty()
    // The big networks first; the 24/7 loops, events and adult last; then by number.
    val real = remember(l) { l.channels.filter { !it.divider && !it.copy }.sortedWith(compareBy({ it.rank }, { it.now == null }, { if (it.num > 0) it.num else Int.MAX_VALUE })) }
    // A channel the list gives no country shows in every one.
    val here = remember(real, place) { if (place.isEmpty()) real else real.filter { it.country == place || it.country.isEmpty() } }
    val topicCounts = remember(here) { here.groupingBy { it.topic }.eachCount() }
    val favorites = remember(l) { l.channels.filter { it.favorite } }
    // The channel last watched on this TV leads the list (when it's in it),
    // so Live TV opens on it, playing at the top.
    val lastId = remember { LastChannel.get(ctx) }
    val shown = remember(l, here, group, find, picks) {
        val list = when {
            find.isNotEmpty() -> real.filter { it.name.contains(find, true) || it.now?.title?.contains(find, true) == true }
            group == "★" -> favorites
            group == "you" -> picks.forYou.mapNotNull { id -> real.firstOrNull { it.id == id } }
            group == "tonight" -> picks.tonight.mapNotNull { (id, _) -> real.firstOrNull { it.id == id } }
            group.startsWith("list:") -> l.lists.firstOrNull { "list:${it.id}" == group }?.items.orEmpty().mapNotNull { id -> real.firstOrNull { it.id == id } }
            group == "*" || group.isEmpty() -> here
            else -> here.filter { it.topic == group }
        }.distinctBy { it.id }
        val last = list.firstOrNull { it.id == lastId }
        val ownOrder = group.startsWith("list:") || group == "you" || group == "tonight"
        (if (last != null && find.isEmpty() && !ownOrder) listOf(last) + (list - last) else list).take(300)
    }
    val countryName = l.countries.firstOrNull { it.first == place }?.second ?: "Every country"
    // Back closes the country tabs, or the search for a game, first.
    BackHandler(enabled = picking) { picking = false }
    BackHandler(enabled = find.isNotEmpty() && !picking) { find = "" }

    // The guide's two hours start at the half hour before now.
    // Tonight: the guide opens at the evening's first show.
    val evening = picks.tonight.minOfOrNull { it.second }?.takeIf { group == "tonight" && it > now }
    val start = (evening ?: now) - (evening ?: now) % HALF_HOUR
    var guide by remember { mutableStateOf<Map<String, List<GuideShow>>>(emptyMap()) }
    LaunchedEffect(shown, start) {
        runCatching { api.guide(shown.map { it.id }, start, 3) }.getOrNull()?.let { guide = guide + it }
    }

    // The channel the highlight is on, for the top.
    var spot by remember { mutableStateOf<Channel?>(null) }
    val games = remember(board) { listOfNotNull(board.big) + board.now.filter { it.key != board.big?.key } }

    Column(Modifier.fillMaxSize()) {
        val at = spot ?: shown.firstOrNull()
        if (at != null) GuideHero(api, at, guide[at.id], now)
        else Box(Modifier.fillMaxWidth().height(HeroHeight))

        if (games.isNotEmpty()) {
            LazyRow(contentPadding = PaddingValues(horizontal = 24.dp), horizontalArrangement = Arrangement.spacedBy(10.dp),
                modifier = Modifier.padding(bottom = 6.dp)) {
                items(games, key = { it.key }) { g ->
                    GamePill(g) {
                        val ch = g.channelId?.let { id -> l.channels.firstOrNull { it.id == id } }
                        // No channel matched: every channel whose name or show has the team in it.
                        if (ch != null) playback.playLive(ch, shown) else find = g.home.name
                    }
                }
            }
        }

        // The topics (or, picking one, the countries) as tabs.
        LazyRow(contentPadding = PaddingValues(horizontal = 18.dp), horizontalArrangement = Arrangement.spacedBy(2.dp)) {
            if (picking) {
                items(l.countries + Triple("", "Every country", real.size), key = { "c-" + it.first }) { (code, name, n) ->
                    ChoiceTab("$name  ·  $n", selected = code == place) {
                        country = code
                        api.liveCountry = code
                        picking = false
                        find = ""
                        if (group != "★") group = "*"
                    }
                }
            } else {
                if (find.isNotEmpty()) item(key = "find") { ChoiceTab("Showing “$find”  ·  ${shown.size}", selected = true) { find = "" } }
                if (favorites.isNotEmpty()) item(key = "★") {
                    ChoiceTab("Favorites  ·  ${favorites.size}", selected = group == "★" && find.isEmpty()) { group = "★"; find = "" }
                }
                if (picks.forYou.isNotEmpty()) item(key = "you") {
                    ChoiceTab("For you", selected = group == "you" && find.isEmpty()) { group = "you"; find = "" }
                }
                if (picks.tonight.isNotEmpty()) item(key = "tonight") {
                    ChoiceTab("Tonight", selected = group == "tonight" && find.isEmpty()) { group = "tonight"; find = "" }
                }
                items(l.lists, key = { "list-" + it.id }) { li ->
                    ChoiceTab("${li.name}  ·  ${li.items.size}", selected = group == "list:${li.id}" && find.isEmpty()) { group = "list:${li.id}"; find = "" }
                }
                item(key = "*") {
                    ChoiceTab("All  ·  ${here.size}", selected = group == "*" && find.isEmpty()) { group = "*"; find = "" }
                }
                items(l.topics.filter { (topicCounts[it.first] ?: 0) > 0 }, key = { "t-" + it.first }) { (key, name) ->
                    ChoiceTab("$name  ·  ${topicCounts[key] ?: 0}", selected = group == key && find.isEmpty()) { group = key; find = "" }
                }
                if (l.countries.isNotEmpty()) item(key = "country") {
                    ChoiceTab("$countryName  ›", selected = false) { picking = true }
                }
                item(key = "web") { Pill("Full guide", modifier = Modifier.padding(start = 10.dp)) { openWeb("/watch/live") } }
            }
        }

        Gap(8.dp)
        BoxWithConstraints(Modifier.fillMaxSize().padding(start = 24.dp, end = Gutter)) {
            val area = maxWidth - NameWidth - CellGap
            val at: (Long) -> Dp = { t -> area * ((t - start).toFloat() / WINDOW).coerceIn(0f, 1f) }
            Column(Modifier.fillMaxSize()) {
                GuideRuler(start, area)
                Gap(6.dp)
                CompositionLocalProvider(LocalBringIntoViewSpec provides PivotSpec) {
                    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(bottom = 40.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                        items(shown, key = { it.id }) { ch ->
                            GuideRow(ch, guide[ch.id], start, now, area, onFocus = { spot = ch }) { playback.playLive(ch, shown) }
                        }
                    }
                }
            }
            // The time now, down the whole guide.
            Box(Modifier.offset(x = NameWidth + CellGap + at(now)).width(2.dp).fillMaxHeight().background(Color(0xCCFFFFFF)))
        }
    }
}

private val HeroHeight = 192.dp

// The top: the highlighted channel's show, big, with the channel playing on
// the right. The words fade and rise in as the highlight moves.
@Composable
private fun GuideHero(api: CueApi, ch: Channel, shows: List<GuideShow>?, now: Long) {
    val show = shows?.firstOrNull { it.start <= now && it.stop > now }
        ?: ch.now?.let { GuideShow(it.title, "", it.start, it.stop) }
    Box(Modifier.fillMaxWidth().height(HeroHeight)) {
        // The channel, live, on the right; its logo until the picture comes.
        Box(Modifier.align(Alignment.CenterEnd).fillMaxHeight().fillMaxWidth(0.58f)) {
            AnimatedContent(targetState = ch, contentKey = { it.id }, transitionSpec = { fadeIn(tween(400)) togetherWith fadeOut(tween(200)) }, label = "live-picture") { c ->
                Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                    ChannelLogo(api, c, Modifier.size(180.dp, 100.dp))
                    LivePreview(api, c, Modifier.fillMaxSize())
                }
            }
            // Fading into the page, to the left and at the bottom.
            Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(0f to Background, 0.45f to Background.copy(alpha = 0.35f), 1f to Color.Transparent)))
            Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.6f to Color.Transparent, 1f to Background)))
            Row(Modifier.align(Alignment.TopEnd).padding(top = 18.dp, end = Gutter), verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.size(7.dp).clip(CircleShape).background(LiveRed))
                HGap(6.dp)
                Text("LIVE", fontSize = 11.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.sp, color = Color.White)
            }
        }
        AnimatedContent(
            targetState = ch to show,
            contentKey = { it.first.id + "|" + it.second?.title },
            transitionSpec = {
                (fadeIn(tween(380, delayMillis = 80, easing = Settle)) + slideInVertically(tween(480, delayMillis = 80, easing = Settle)) { it / 8 }) togetherWith
                    fadeOut(tween(150))
            },
            label = "live-words",
            modifier = Modifier.fillMaxHeight().fillMaxWidth(0.56f).padding(start = 24.dp, top = 18.dp),
        ) { (c, s) ->
            Column {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Box(Modifier.size(64.dp, 36.dp).clip(RoundedCornerShape(8.dp)).background(Color(0x1FFFFFFF)), contentAlignment = Alignment.Center) {
                        ChannelLogo(api, c, Modifier.fillMaxSize().padding(5.dp))
                    }
                    HGap(12.dp)
                    Text((if (c.num > 0) "${c.num}  " else "") + c.name, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = Dim, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    if (c.favorite) { HGap(8.dp); Text("★", fontSize = 13.sp, color = Color(0xFFF5C518)) }
                }
                Gap(12.dp)
                Text(s?.title ?: "No guide for this channel", fontFamily = Type.display(), fontSize = 30.sp, fontWeight = FontWeight.Bold,
                    color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
                if (s != null) {
                    Gap(6.dp)
                    Text(showTimes(s, now), fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = Color(0xE6FFFFFF), maxLines = 1)
                    if (s.desc.isNotBlank()) {
                        Gap(6.dp)
                        Text(s.desc, fontSize = 13.sp, lineHeight = 18.sp, color = Color(0xCCFFFFFF), maxLines = 2, overflow = TextOverflow.Ellipsis)
                    }
                    Gap(12.dp)
                    val through = if (s.stop > s.start) ((now - s.start).toFloat() / (s.stop - s.start)).coerceIn(0f, 1f) else 0f
                    Box(Modifier.width(240.dp).height(3.dp).clip(RoundedCornerShape(2.dp)).background(Color(0x33FFFFFF))) {
                        Box(Modifier.fillMaxWidth(through).fillMaxHeight().background(Color.White))
                    }
                }
            }
        }
    }
}

// "1h 0m · Started at 11:00 PM", or "· Starts at" for one still to come.
private fun showTimes(s: GuideShow, now: Long): String {
    val mins = ((s.stop - s.start) / 60_000L).coerceAtLeast(0)
    val length = if (mins >= 60) "${mins / 60}h ${mins % 60}m" else "${mins}m"
    return length + " · " + (if (s.start <= now) "Started at " else "Starts at ") + clockTime(s.start)
}

// The times along the top of the guide, every half hour.
@Composable
private fun GuideRuler(start: Long, area: Dp) {
    Row(Modifier.fillMaxWidth().height(18.dp), verticalAlignment = Alignment.CenterVertically) {
        Text("Today", fontSize = 13.sp, fontWeight = FontWeight.Bold, color = Color.White, modifier = Modifier.width(NameWidth + CellGap))
        Box(Modifier.width(area).fillMaxHeight()) {
            for (i in 0 until 4) {
                Text(clockTime(start + i * HALF_HOUR), fontSize = 11.sp, color = Dim, modifier = Modifier.offset(x = area * (i / 4f)))
            }
        }
    }
}

// A channel's row: its name, then its shows along the two hours. The whole
// row takes the highlight (the name turns into a white pill, the show on
// now lights up); select plays the channel.
@Composable
private fun GuideRow(ch: Channel, shows: List<GuideShow>?, start: Long, now: Long, area: Dp, onFocus: () -> Unit, onClick: () -> Unit) {
    var focused by remember { mutableStateOf(false) }
    val shape = RoundedCornerShape(8.dp)
    Card(
        onClick = onClick,
        modifier = Modifier.fillMaxWidth().height(RowHeight).onFocusChanged { focused = it.isFocused; if (it.isFocused) onFocus() },
        shape = CardDefaults.shape(shape),
        scale = CardDefaults.scale(focusedScale = 1f),
        colors = CardDefaults.colors(containerColor = Color.Transparent, focusedContainerColor = Color.Transparent),
        border = CardDefaults.border(border = Border.None, focusedBorder = Border.None),
    ) {
        Row(Modifier.fillMaxSize()) {
            Box(
                Modifier.width(NameWidth).fillMaxHeight().clip(shape).background(if (focused) Color.White else Color(0x1AFFFFFF)),
                contentAlignment = Alignment.CenterStart,
            ) {
                Row(Modifier.padding(horizontal = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                    if (ch.num > 0) {
                        Text("${ch.num}", fontSize = 11.sp, color = if (focused) Color(0x990B0C0F) else Dim, modifier = Modifier.width(30.dp))
                    }
                    Text(ch.name, fontSize = 14.sp, fontWeight = FontWeight.SemiBold, color = if (focused) Color(0xFF0B0C0F) else Color.White,
                        maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                    if (ch.favorite) Text("★", fontSize = 11.sp, color = Color(0xFFF5C518))
                }
            }
            HGap(CellGap)
            Box(Modifier.width(area).fillMaxHeight()) {
                val end = start + WINDOW
                val list = shows?.filter { it.stop > start && it.start < end }.orEmpty().ifEmpty {
                    // No guide: what the channel list says is on, else the channel's name across the row.
                    listOf(ch.now?.let { GuideShow(it.title, "", it.start, it.stop) } ?: GuideShow(ch.name, "", start, end))
                }
                list.forEach { s ->
                    val from = maxOf(s.start, start)
                    val to = minOf(s.stop, end)
                    val x = area * ((from - start).toFloat() / WINDOW)
                    val w = (area * ((to - from).toFloat() / WINDOW) - CellGap).coerceAtLeast(6.dp)
                    val on = s.start <= now && s.stop > now
                    val cell = RoundedCornerShape(6.dp)
                    Box(
                        Modifier.offset(x = x).width(w).fillMaxHeight().clip(cell)
                            .background(if (focused && on) Color(0x47FFFFFF) else if (on) Color(0x1FFFFFFF) else Color(0x0DFFFFFF))
                            .border(BorderStroke(if (focused && on) 1.5.dp else 1.dp, if (focused && on) Color.White else Color(0x24FFFFFF)), cell),
                        contentAlignment = Alignment.CenterStart,
                    ) {
                        if (w > 28.dp) {
                            Text(s.title, fontSize = 13.sp, fontWeight = if (on) FontWeight.SemiBold else FontWeight.Normal,
                                color = if (on) Color.White else Color(0xB3FFFFFF), maxLines = 1, overflow = TextOverflow.Ellipsis,
                                modifier = Modifier.padding(horizontal = 10.dp))
                        }
                    }
                }
            }
        }
    }
}

// A game on now, small: the score (or when it starts) and where; select
// plays the channel it's on.
@Composable
private fun GamePill(g: GameRow, onClick: () -> Unit) {
    val score = if (g.away.score.isNotEmpty() || g.home.score.isNotEmpty()) "${g.away.name} ${g.away.score} – ${g.home.score} ${g.home.name}"
    else "${g.away.name} at ${g.home.name}"
    Pill(score, small = listOf(g.detail, g.where).filter { it.isNotBlank() }.joinToString(" · ").ifEmpty { null },
        icon = null, primary = g.live, onClick = onClick)
}
