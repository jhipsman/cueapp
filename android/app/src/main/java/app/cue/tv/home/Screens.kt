@file:OptIn(androidx.tv.material3.ExperimentalTvMaterial3Api::class)

package app.cue.tv.home

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.OutlinedTextFieldDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Border
import androidx.tv.material3.Button
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.Card
import androidx.tv.material3.CardDefaults
import androidx.tv.material3.Tab
import androidx.tv.material3.TabRow
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import coil.request.ImageRequest
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

// The app's own screens: signing in, the profile, then the tabs along the
// top (Home, Shows, Movies, Live TV, Search, My List, the profile) and a
// title's page over them.

enum class Stage { Checking, SignIn, Profiles, Ready }

@Composable
fun CueRoot(api: CueApi, playback: Playback, openWeb: (String) -> Unit, changeServer: () -> Unit) {
    var stage by remember { mutableStateOf(Stage.Checking) }
    var problem by remember { mutableStateOf("") }
    var accent by remember { mutableStateOf(accentOf(api.accent)) }
    LaunchedEffect(stage) {
        if (stage != Stage.Checking) return@LaunchedEffect
        problem = ""
        stage = try {
            when {
                !api.signedIn() -> Stage.SignIn
                api.profileToken.isEmpty() -> Stage.Profiles
                else -> Stage.Ready
            }
        } catch (e: Exception) {
            problem = e.message ?: "Couldn't reach Cue."
            Stage.SignIn
        }
    }
    androidx.tv.material3.MaterialTheme(
        colorScheme = androidx.tv.material3.darkColorScheme(primary = accent, onPrimary = Color.Black, surface = Panel, background = Background),
    ) { CompositionLocalProvider(LocalAccent provides accent) {
        Box(Modifier.fillMaxSize().background(Background)) {
            when (stage) {
                Stage.Checking -> Centered { Text("Cue", fontSize = 48.sp, fontWeight = FontWeight.Black, color = Teal) }
                Stage.SignIn -> SignInScreen(api, problem, changeServer) { stage = Stage.Profiles }
                Stage.Profiles -> ProfilesScreen(api) {
                    accent = accentOf(api.accent)
                    stage = Stage.Ready
                }
                Stage.Ready -> MainScreen(api, playback, openWeb, changeServer) {
                    api.forgetProfile()
                    stage = Stage.Profiles
                }
            }
            if (playback.note.isNotEmpty()) {
                Text(playback.note, fontSize = 14.sp, color = Color.White,
                    modifier = Modifier.align(Alignment.BottomCenter).padding(28.dp).clip(RoundedCornerShape(10.dp))
                        .background(Color(0xE6202228)).padding(horizontal = 20.dp, vertical = 12.dp))
                LaunchedEffect(playback.note) {
                    delay(6000)
                    playback.clearNote()
                }
            }
        }
    } }
}

// ---- Signing in ----

@Composable
fun SignInScreen(api: CueApi, startProblem: String, changeServer: () -> Unit, done: () -> Unit) {
    var user by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var problem by remember { mutableStateOf(startProblem) }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val first = remember { FocusRequester() }
    LaunchedEffect(Unit) { runCatching { first.requestFocus() } }
    Centered {
        Column(horizontalAlignment = Alignment.CenterHorizontally, modifier = Modifier.width(420.dp)) {
            Text("Cue", fontSize = 48.sp, fontWeight = FontWeight.Black, color = Teal)
            Text("Sign in to ${api.server.removePrefix("https://").removePrefix("http://")}", color = Dim, fontSize = 14.sp)
            Gap(20.dp)
            OutlinedTextField(user, { user = it }, label = { androidx.compose.material3.Text("Username") }, singleLine = true,
                colors = fieldColors(), modifier = Modifier.fillMaxWidth().focusRequester(first))
            Gap(10.dp)
            OutlinedTextField(password, { password = it }, label = { androidx.compose.material3.Text("Password") }, singleLine = true,
                visualTransformation = PasswordVisualTransformation(), keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                colors = fieldColors(), modifier = Modifier.fillMaxWidth())
            if (problem.isNotEmpty()) {
                Gap(10.dp)
                Text(problem, color = Color(0xFFFF8A80), fontSize = 13.sp)
            }
            Gap(16.dp)
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Button(onClick = {
                    if (busy || user.isBlank() || password.isEmpty()) return@Button
                    busy = true
                    problem = ""
                    scope.launch {
                        try {
                            api.signIn(user.trim(), password)
                            done()
                        } catch (e: Exception) {
                            problem = e.message ?: "Couldn't sign in."
                        }
                        busy = false
                    }
                }) { Text(if (busy) "Signing in…" else "Sign in") }
                Button(onClick = changeServer, colors = quietButton()) { Text("Change server") }
            }
        }
    }
}

@Composable
fun fieldColors() = OutlinedTextFieldDefaults.colors(
    focusedTextColor = Color.White, unfocusedTextColor = Color.White,
    focusedBorderColor = LocalAccent.current, unfocusedBorderColor = Color(0x55FFFFFF),
    focusedLabelColor = LocalAccent.current, unfocusedLabelColor = Dim, cursorColor = LocalAccent.current,
)

@Composable
fun quietButton() = ButtonDefaults.colors(containerColor = Color(0x22FFFFFF), contentColor = Color.White)

// ---- Who's watching ----

@Composable
fun ProfilesScreen(api: CueApi, done: () -> Unit) {
    var list by remember { mutableStateOf<List<Profile>>(emptyList()) }
    var problem by remember { mutableStateOf("") }
    var asking by remember { mutableStateOf<Profile?>(null) }
    var pin by remember { mutableStateOf("") }
    val scope = rememberCoroutineScope()
    val first = remember { FocusRequester() }
    fun choose(p: Profile, pinCode: String = "") = scope.launch {
        try {
            api.choose(p, pinCode)
            done()
        } catch (e: Exception) {
            problem = e.message ?: "Couldn't open that profile."
        }
    }
    LaunchedEffect(Unit) {
        try {
            list = api.profiles()
            if (list.size == 1 && !list[0].hasPin) choose(list[0])
        } catch (e: Exception) {
            problem = e.message ?: "Couldn't load the profiles."
        }
    }
    LaunchedEffect(list) { if (list.isNotEmpty()) { delay(100); runCatching { first.requestFocus() } } }
    Centered {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text("Who's watching?", fontSize = 30.sp, fontWeight = FontWeight.Bold, color = Color.White)
            Gap(28.dp)
            Row(horizontalArrangement = Arrangement.spacedBy(28.dp)) {
                list.forEachIndexed { i, p ->
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Card(
                            onClick = { if (p.hasPin) { pin = ""; asking = p } else choose(p) },
                            modifier = Modifier.size(110.dp).let { if (i == 0) it.focusRequester(first) else it },
                            shape = CardDefaults.shape(CircleShape),
                            scale = CardDefaults.scale(focusedScale = 1.1f),
                            border = CardDefaults.border(focusedBorder = Border(BorderStroke(3.dp, Color.White), shape = CircleShape)),
                        ) {
                            Box(Modifier.fillMaxSize().background(p.accent?.let { accentOf(it) } ?: avatarColor(p.avatar)), contentAlignment = Alignment.Center) {
                                Text(p.name.take(1).uppercase(), fontSize = 44.sp, fontWeight = FontWeight.Bold, color = Color.White)
                            }
                        }
                        Gap(10.dp)
                        Text(p.name, color = Color.White, fontSize = 15.sp)
                    }
                }
            }
            asking?.let { p ->
                Gap(24.dp)
                Text("PIN for ${p.name}", color = Dim)
                Gap(8.dp)
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    OutlinedTextField(pin, { pin = it.filter(Char::isDigit).take(8) }, singleLine = true, colors = fieldColors(),
                        visualTransformation = PasswordVisualTransformation(), keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword),
                        modifier = Modifier.width(180.dp))
                    Button(onClick = { choose(p, pin) }) { Text("OK") }
                }
            }
            if (problem.isNotEmpty()) {
                Gap(16.dp)
                Text(problem, color = Color(0xFFFF8A80), fontSize = 13.sp)
            }
        }
    }
}

fun avatarColor(name: String): Color = when (name) {
    "red" -> Color(0xFFE5484D)
    "orange" -> Color(0xFFF76B15)
    "yellow" -> Color(0xFFE2A336)
    "green" -> Color(0xFF30A46C)
    "blue" -> Color(0xFF3E63DD)
    "purple" -> Color(0xFF8E4EC6)
    "pink" -> Color(0xFFD6409F)
    else -> Teal
}

// ---- The tabs ----

private val tabs = listOf("Home", "Shows", "Movies", "Live TV", "Search", "My List")

@Composable
fun MainScreen(api: CueApi, playback: Playback, openWeb: (String) -> Unit, changeServer: () -> Unit, switchProfile: () -> Unit) {
    var tab by remember { mutableIntStateOf(0) }
    val open = remember { mutableStateListOf<Tile>() } // titles opened, one over another
    val memories = remember { List(tabs.size + 1) { FocusMemory() } }
    val titleMemory = remember { FocusMemory() }
    BackHandler(enabled = open.isNotEmpty()) { open.removeAt(open.lastIndex) }
    val onCard: (Tile) -> Unit = { c -> if (c.resumes) playback.play(c) else open.add(c) }

    if (open.isNotEmpty()) {
        val t = open.last()
        TitleScreen(api, playback, t, titleMemory) { c -> open.add(c) }
        return
    }
    Column(Modifier.fillMaxSize()) {
        Row(Modifier.fillMaxWidth().padding(start = 40.dp, end = 40.dp, top = 14.dp), verticalAlignment = Alignment.CenterVertically) {
            Text("Cue", fontSize = 22.sp, fontWeight = FontWeight.Black, color = LocalAccent.current)
            HGap(24.dp)
            TabRow(selectedTabIndex = tab) {
                tabs.forEachIndexed { i, name ->
                    Tab(selected = i == tab, onFocus = { tab = i }) {
                        Text(name, fontSize = 14.sp, modifier = Modifier.padding(horizontal = 14.dp, vertical = 6.dp))
                    }
                }
                Tab(selected = tab == tabs.size, onFocus = { tab = tabs.size }) {
                    Text(api.profileName.ifEmpty { "Profile" }, fontSize = 14.sp, modifier = Modifier.padding(horizontal = 14.dp, vertical = 6.dp))
                }
            }
        }
        Box(Modifier.fillMaxSize()) {
            when (tab) {
                0 -> HomeScreen(api, null, memories[0], onCard)
                1 -> HomeScreen(api, "tv", memories[1], onCard)
                2 -> HomeScreen(api, "movie", memories[2], onCard)
                3 -> LiveScreen(api, playback, openWeb)
                4 -> SearchScreen(api, memories[4], onCard)
                5 -> MyListScreen(api, memories[5], onCard)
                else -> ProfileScreen(api, switchProfile, changeServer, openWeb)
            }
        }
    }
}

// ---- Home ----

// Home (and Shows and Movies): the banner at the top shows whatever poster
// the highlight is on, as the TV's own apps do; the rows slide beneath.
@Composable
fun HomeScreen(api: CueApi, only: String?, memory: FocusMemory, onCard: (Tile) -> Unit) {
    var home by remember { mutableStateOf<Home?>(homeCache) }
    var problem by remember { mutableStateOf("") }
    var spot by remember { mutableStateOf<Tile?>(null) }
    val listState = rememberLazyListState()
    LaunchedEffect(Unit) {
        try {
            val h = api.home()
            homeCache = h
            home = h
        } catch (e: Exception) {
            if (home == null) problem = e.message ?: "Couldn't load Home."
        }
    }
    val h = home
    if (h == null) {
        Centered { Text(problem.ifEmpty { "Loading…" }, color = Dim) }
        return
    }
    val rows = h.rows.mapNotNull { r ->
        val items = if (only == null) r.items else r.items.filter { it.kind == only }
        if (items.isEmpty()) null else Shelf(r.key, r.title, items)
    }
    val featured = h.hero?.takeIf { only == null || it.kind == only } ?: rows.flatMap { it.items }.firstOrNull { it.backdrop != null && !it.resumes }
    val shown = spot ?: featured
    LaunchedEffect(rows.size) {
        delay(80)
        memory.restore(rows.firstOrNull()?.let { r -> "${r.key}/${r.items.first().key}" })
    }
    Column(Modifier.fillMaxSize()) {
        Box(Modifier.fillMaxWidth().height(210.dp)) {
            Backdrop(small(shown?.backdrop), Modifier.fillMaxSize())
            shown?.let { c ->
                Column(Modifier.align(Alignment.BottomStart).padding(start = 48.dp, bottom = 8.dp).width(470.dp)) {
                    Text(c.title, fontSize = 30.sp, fontWeight = FontWeight.Bold, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Gap(4.dp)
                    MetaLine(c, c.episodeLine)
                    c.overview?.let {
                        Gap(6.dp)
                        Text(it, fontSize = 13.sp, color = Color(0xE6FFFFFF), maxLines = 3, overflow = TextOverflow.Ellipsis, lineHeight = 18.sp)
                    }
                }
            }
        }
        LazyColumn(state = listState, contentPadding = PaddingValues(top = 6.dp, bottom = 120.dp)) {
            items(rows, key = { it.key }) { r ->
                CardRow(r, memory, onFocus = { spot = it }, onClick = onCard)
            }
        }
    }
}

private var homeCache: Home? = null

// ---- A title's page ----

@Composable
fun TitleScreen(api: CueApi, playback: Playback, card: Tile, memory: FocusMemory, openTitle: (Tile) -> Unit) {
    var title by remember(card.key) { mutableStateOf<Title?>(null) }
    var problem by remember(card.key) { mutableStateOf("") }
    var season by remember(card.key) { mutableIntStateOf(0) }
    var episodes by remember(card.key) { mutableStateOf<List<Episode>>(emptyList()) }
    var inList by remember(card.key) { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val playFocus = remember(card.key) { FocusRequester() }
    LaunchedEffect(card.key) {
        try {
            val t = api.title(card.kind, card.tmdbId)
            title = t
            inList = t.inList
            if (card.kind == "tv") season = t.resumeSeason ?: t.seasons.firstOrNull { it.number > 0 }?.number ?: t.seasons.firstOrNull()?.number ?: 1
            delay(80)
            runCatching { playFocus.requestFocus() }
        } catch (e: Exception) {
            problem = e.message ?: "Couldn't load this title."
        }
    }
    LaunchedEffect(card.key, season) {
        if (card.kind == "tv" && season >= 0 && title != null) episodes = runCatching { api.season(card.tmdbId, season) }.getOrDefault(emptyList())
    }
    val t = title
    if (t == null) {
        Centered { Text(problem.ifEmpty { "Loading…" }, color = Dim) }
        return
    }
    val c = t.card
    Box(Modifier.fillMaxSize()) {
        Backdrop(small(c.backdrop), Modifier.fillMaxWidth().height(420.dp))
        LazyColumn(contentPadding = PaddingValues(bottom = 120.dp)) {
            item {
                Column(Modifier.padding(start = 48.dp, top = 110.dp, bottom = 18.dp).width(560.dp)) {
                    Text(c.title, fontSize = 38.sp, fontWeight = FontWeight.Black, color = Color.White, maxLines = 2, lineHeight = 42.sp)
                    t.tagline?.let { Text(it, fontSize = 13.sp, color = Dim) }
                    Gap(8.dp)
                    MetaLine(c, listOfNotNull(t.certification, t.runtime?.takeIf { it > 0 }?.let { if (it >= 60) "${it / 60}h ${it % 60}m" else "${it}m" },
                        t.genres.take(3).joinToString(", ").takeIf { it.isNotEmpty() }).joinToString("   ·   "))
                    c.overview?.let {
                        Gap(10.dp)
                        Text(it, fontSize = 13.sp, color = Color(0xE6FFFFFF), maxLines = 4, overflow = TextOverflow.Ellipsis, lineHeight = 19.sp)
                    }
                    Gap(16.dp)
                    Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        val resumeTv = c.kind == "tv" && t.resumeSeason != null && t.resumeEpisode != null
                        Button(onClick = {
                            if (c.kind == "tv") {
                                val s = t.resumeSeason ?: season.takeIf { it > 0 } ?: 1
                                playback.playTitle("tv", c.tmdbId, s, t.resumeEpisode ?: 1)
                            } else {
                                playback.playTitle("movie", c.tmdbId, 0, 0)
                            }
                        }, modifier = Modifier.focusRequester(playFocus)) {
                            Text(when {
                                resumeTv -> "▶  Resume S${t.resumeSeason}:E${t.resumeEpisode}"
                                (t.resumePosition ?: 0.0) > 0 -> "▶  Resume"
                                else -> "▶  Play"
                            })
                        }
                        Button(onClick = {
                            scope.launch {
                                val on = !inList
                                if (runCatching { api.setInList(c.kind, c.tmdbId, on) }.isSuccess) inList = on
                            }
                        }, colors = quietButton()) { Text(if (inList) "✓  In My List" else "+  My List") }
                    }
                }
            }
            if (c.kind == "tv" && t.seasons.isNotEmpty()) {
                item {
                    LazyRow(contentPadding = PaddingValues(horizontal = 48.dp, vertical = 8.dp), horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                        items(t.seasons, key = { it.number }) { s ->
                            Button(onClick = { season = s.number }, colors = if (s.number == season) ButtonDefaults.colors() else quietButton()) {
                                Text(s.name, fontSize = 13.sp)
                            }
                        }
                    }
                }
                item {
                    LazyRow(contentPadding = PaddingValues(horizontal = 48.dp, vertical = 12.dp), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                        items(episodes, key = { "${it.season}-${it.episode}" }) { e ->
                            EpisodeCard(e) { playback.playTitle("tv", c.tmdbId, e.season, e.episode) }
                        }
                    }
                }
            }
            if (t.more.isNotEmpty()) {
                item {
                    CardRow(Shelf("more", "More Like This", t.more.take(18)), memory) { m -> openTitle(m) }
                }
            }
        }
    }
}

@Composable
fun EpisodeCard(e: Episode, onClick: () -> Unit) {
    val accent = LocalAccent.current
    Column(Modifier.width(220.dp)) {
        Card(
            onClick = onClick,
            modifier = Modifier.size(220.dp, 124.dp),
            shape = CardDefaults.shape(RoundedCornerShape(8.dp)),
            scale = CardDefaults.scale(focusedScale = 1.06f),
            border = CardDefaults.border(focusedBorder = Border(BorderStroke(2.5.dp, accent), shape = RoundedCornerShape(8.dp))),
        ) {
            Box(Modifier.fillMaxSize()) {
                Art(small(e.still), Modifier.fillMaxSize(), e.title)
                e.progress?.takeIf { it > 0 && !e.finished }?.let { p ->
                    Box(Modifier.align(Alignment.BottomStart).fillMaxWidth().height(3.dp).background(Color(0x55FFFFFF)))
                    Box(Modifier.align(Alignment.BottomStart).fillMaxWidth(p.toFloat().coerceIn(0f, 1f)).height(3.dp).background(accent))
                }
                if (e.finished) {
                    Text("✓", color = Color.Black, fontSize = 12.sp, fontWeight = FontWeight.Bold,
                        modifier = Modifier.align(Alignment.TopEnd).padding(6.dp).clip(CircleShape).background(accent).padding(horizontal = 6.dp))
                }
            }
        }
        Text("${e.episode}. ${e.title}", fontSize = 12.sp, fontWeight = FontWeight.SemiBold, color = Color.White, maxLines = 1,
            overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = 6.dp))
        e.overview?.let { Text(it, fontSize = 10.sp, color = Dim, maxLines = 2, overflow = TextOverflow.Ellipsis, lineHeight = 13.sp) }
    }
}

// ---- Search and My List ----

@Composable
fun SearchScreen(api: CueApi, memory: FocusMemory, onCard: (Tile) -> Unit) {
    var query by remember { mutableStateOf("") }
    var results by remember { mutableStateOf<List<Tile>>(emptyList()) }
    LaunchedEffect(query) {
        val q = query.trim()
        if (q.length < 2) { results = emptyList(); return@LaunchedEffect }
        delay(400)
        results = runCatching { api.search(q) }.getOrDefault(results)
    }
    Column(Modifier.fillMaxSize().padding(top = 16.dp)) {
        OutlinedTextField(query, { query = it }, singleLine = true, colors = fieldColors(),
            placeholder = { androidx.compose.material3.Text("Movies and shows") },
            modifier = Modifier.padding(horizontal = 48.dp).width(420.dp))
        PosterGrid(results, memory, "search", onCard)
    }
}

@Composable
fun MyListScreen(api: CueApi, memory: FocusMemory, onCard: (Tile) -> Unit) {
    var items by remember { mutableStateOf<List<Tile>?>(null) }
    LaunchedEffect(Unit) {
        items = runCatching { api.home().rows.firstOrNull { it.key == "my-list" }?.items ?: emptyList() }.getOrDefault(emptyList())
        delay(80)
        items?.firstOrNull()?.let { memory.restore("list/${it.key}") }
    }
    val list = items
    when {
        list == null -> Centered { Text("Loading…", color = Dim) }
        list.isEmpty() -> Centered { Text("Nothing here yet. Add titles with My List on their page.", color = Dim) }
        else -> PosterGrid(list, memory, "list", onCard)
    }
}

@Composable
fun PosterGrid(items: List<Tile>, memory: FocusMemory, prefix: String, onCard: (Tile) -> Unit) {
    LazyVerticalGrid(
        columns = GridCells.Adaptive(118.dp),
        contentPadding = PaddingValues(horizontal = 48.dp, vertical = 20.dp),
        horizontalArrangement = Arrangement.spacedBy(14.dp),
        verticalArrangement = Arrangement.spacedBy(18.dp),
    ) {
        items(items, key = { it.key }) { c -> PosterCard(c, memory, "$prefix/${c.key}", onClick = onCard) }
    }
}

// ---- Live TV ----

// Live TV: the groups down the left, the channels with what's on now (and
// how far through) and next. A channel plays in the player, where up and
// down change channel. The full guide is in the web version for now.
@Composable
fun LiveScreen(api: CueApi, playback: Playback, openWeb: (String) -> Unit) {
    var live by remember { mutableStateOf<LiveChannels?>(null) }
    var problem by remember { mutableStateOf("") }
    var group by remember { mutableStateOf("") }
    LaunchedEffect(Unit) {
        try {
            val l = api.channels()
            live = l
            if (group.isEmpty()) group = if (l.channels.any { it.favorite }) "★" else "*"
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
    val groups = buildList {
        if (l.channels.any { it.favorite }) add("★" to "Favorites")
        add("*" to "All channels")
        val used = l.channels.map { it.category }.toSet()
        addAll(l.groups.filter { it.first in used })
    }
    val shown = when (group) {
        "★" -> l.channels.filter { it.favorite }
        "*", "" -> l.channels
        else -> l.channels.filter { it.category == group }
    }
    Row(Modifier.fillMaxSize().padding(top = 12.dp)) {
        LazyColumn(Modifier.width(200.dp), contentPadding = PaddingValues(start = 40.dp, bottom = 60.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
            item {
                Button(onClick = { openWeb("/watch/live") }, colors = quietButton()) { Text("Full guide", fontSize = 12.sp) }
                Gap(8.dp)
            }
            items(groups, key = { it.first }) { (id, name) ->
                Button(onClick = { group = id }, colors = if (id == group) ButtonDefaults.colors() else quietButton(),
                    modifier = Modifier.fillMaxWidth()) {
                    Text(name, fontSize = 12.sp, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
        }
        LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(start = 20.dp, end = 40.dp, bottom = 60.dp),
            verticalArrangement = Arrangement.spacedBy(8.dp)) {
            items(shown.take(400), key = { it.id }) { ch ->
                ChannelCard(api, ch) { playback.playLive(ch, shown) }
            }
        }
    }
}

@Composable
fun ChannelCard(api: CueApi, ch: Channel, onClick: () -> Unit) {
    val accent = LocalAccent.current
    Card(
        onClick = onClick,
        modifier = Modifier.fillMaxWidth().height(72.dp),
        shape = CardDefaults.shape(RoundedCornerShape(10.dp)),
        scale = CardDefaults.scale(focusedScale = 1.02f),
        colors = CardDefaults.colors(containerColor = Panel),
        border = CardDefaults.border(focusedBorder = Border(BorderStroke(2.dp, accent), shape = RoundedCornerShape(10.dp))),
    ) {
        Row(Modifier.fillMaxSize().padding(horizontal = 12.dp), verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.size(96.dp, 54.dp).clip(RoundedCornerShape(6.dp)).background(Color(0xFF2A2D33)), contentAlignment = Alignment.Center) {
                Text(ch.name.take(3).uppercase(), fontSize = 12.sp, fontWeight = FontWeight.Bold, color = Dim)
                ch.logo?.let { path ->
                    AsyncImage(
                        model = ImageRequest.Builder(androidx.compose.ui.platform.LocalContext.current)
                            .data(api.server + path)
                            .addHeader("Cookie", api.cookie)
                            .addHeader("X-Cue-Profile", api.profileToken)
                            .build(),
                        contentDescription = null,
                        modifier = Modifier.fillMaxSize().padding(6.dp),
                    )
                }
            }
            HGap(14.dp)
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    if (ch.num > 0) Text("${ch.num}  ", fontSize = 11.sp, color = Dim)
                    Text(ch.name, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    if (ch.favorite) Text("  ★", fontSize = 11.sp, color = Color(0xFFFFD60A))
                }
                ch.now?.let { now ->
                    Text(now.title, fontSize = 12.sp, color = Color(0xE6FFFFFF), maxLines = 1, overflow = TextOverflow.Ellipsis)
                    Box(Modifier.padding(top = 4.dp).width(240.dp).height(3.dp).background(Color(0x33FFFFFF))) {
                        Box(Modifier.fillMaxWidth(now.progress()).height(3.dp).background(accent))
                    }
                }
            }
            ch.next?.let { n ->
                Column(Modifier.width(220.dp)) {
                    Text("Next · ${java.text.DateFormat.getTimeInstance(java.text.DateFormat.SHORT).format(java.util.Date(n.start))}", fontSize = 10.sp, color = Dim)
                    Text(n.title, fontSize = 12.sp, color = Dim, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
        }
    }
}

// ---- The profile ----

@Composable
fun ProfileScreen(api: CueApi, switchProfile: () -> Unit, changeServer: () -> Unit, openWeb: (String) -> Unit) {
    Centered {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Box(Modifier.size(96.dp).clip(CircleShape).background(LocalAccent.current), contentAlignment = Alignment.Center) {
                Text(api.profileName.take(1).uppercase(), fontSize = 40.sp, fontWeight = FontWeight.Bold, color = Color.White)
            }
            Gap(12.dp)
            Text(api.profileName, fontSize = 20.sp, fontWeight = FontWeight.Bold, color = Color.White)
            Text(api.server, fontSize = 12.sp, color = Dim)
            Gap(20.dp)
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                Button(onClick = switchProfile) { Text("Switch profile") }
                Button(onClick = { openWeb("/watch") }, colors = quietButton()) { Text("Web version") }
                Button(onClick = changeServer, colors = quietButton()) { Text("Change server") }
            }
        }
    }
}
