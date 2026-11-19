package app.cue.tv.home

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Border
import androidx.tv.material3.Card
import androidx.tv.material3.CardDefaults
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import kotlinx.coroutines.delay
import org.json.JSONArray
import org.json.JSONObject

// ---- Browse: Coming soon and the Top 10, a page per streaming service,
// collections across services, and the big film series. Opening any of them
// shows it in place; Back comes back here.

data class BrowseService(val key: String, val name: String, val logo: String?)
data class BrowseCollection(val key: String, val title: String, val blurb: String)
data class BrowseFranchise(val id: Int, val name: String, val poster: String?)
data class BrowseHome(val services: List<BrowseService>, val collections: List<BrowseCollection>, val franchises: List<BrowseFranchise>)

private fun JSONArray?.objects(): List<JSONObject> = if (this == null) emptyList() else (0 until length()).map { getJSONObject(it) }

suspend fun CueApi.browse(): BrowseHome {
    val o = obj("/api/watch/browse")
    return BrowseHome(
        o.optJSONArray("services").objects().map { BrowseService(it.optString("key"), it.optString("name"), it.str("logo")) },
        o.optJSONArray("collections").objects().map { BrowseCollection(it.optString("key"), it.optString("title"), it.optString("blurb")) },
        o.optJSONArray("franchises").objects().map { BrowseFranchise(it.optInt("id"), it.optString("name"), it.str("poster")) },
    )
}

suspend fun CueApi.browseService(key: String): List<Shelf> =
    obj("/api/watch/browse/service/${q(key)}").optJSONArray("rows").objects().map {
        Shelf("svc-$key-" + it.optString("key"), it.optString("title"), (it.optJSONArray("items") ?: JSONArray()).cards())
    }

suspend fun CueApi.browseCollection(key: String): List<Tile> =
    (obj("/api/watch/browse/collection/${q(key)}").optJSONArray("items") ?: JSONArray()).cards()

suspend fun CueApi.browseFranchise(id: Int): List<Tile> =
    (obj("/api/watch/browse/franchise/$id").optJSONArray("items") ?: JSONArray()).cards()

sealed interface BrowseAt {
    data object Soon : BrowseAt
    data object Top : BrowseAt
    data class Service(val s: BrowseService) : BrowseAt
    data class Collection(val c: BrowseCollection) : BrowseAt
    data class Franchise(val f: BrowseFranchise) : BrowseAt
}

// Where Browse has got to, kept by the main screen: opening a title covers
// Browse (it leaves the screen), and Back from the title must land on the
// same service or collection, the highlight on the title that was opened.
class BrowseState {
    var at by mutableStateOf<BrowseAt?>(null)
    var cameBack by mutableStateOf(false)
    // The open page's own memory (a new page starts on its first title).
    var pageMemory = FocusMemory()
        private set

    fun open(to: BrowseAt) {
        at = to
        pageMemory = FocusMemory()
    }

    fun back() {
        at = null
        cameBack = true
    }
}

private var browseCache: BrowseHome? = null

@Composable
fun BrowseScreen(api: CueApi, memory: FocusMemory, state: BrowseState, onCard: (Tile) -> Unit) {
    // Back from a service, collection or Coming soon: here again, the
    // highlight back on the menu. (Arriving from the side menu, the menu
    // doesn't take the highlight: moving down the side menu past Browse
    // mustn't pull the remote into it.)
    BackHandler(enabled = state.at != null) { state.back() }
    val page = state.pageMemory
    when (val a = state.at) {
        null -> BrowseMenu(api, takeFocus = state.cameBack, tookFocus = { state.cameBack = false }) { state.open(it) }
        BrowseAt.Soon -> ComingSoonScreen(api, onCard)
        BrowseAt.Top -> TopScreen(api, memory, onCard)
        is BrowseAt.Service -> ServicePage(api, a.s, page, onCard)
        is BrowseAt.Collection -> GridPage(a.c.title, a.c.blurb, "col-${a.c.key}", page, onCard) { api.browseCollection(a.c.key) }
        is BrowseAt.Franchise -> GridPage(a.f.name, "In the order they came out.", "fr-${a.f.id}", page, onCard) { api.browseFranchise(a.f.id) }
    }
}

@Composable
private fun BrowseMenu(api: CueApi, takeFocus: Boolean, tookFocus: () -> Unit, open: (BrowseAt) -> Unit) {
    var home by remember { mutableStateOf(browseCache) }
    var problem by remember { mutableStateOf("") }
    LaunchedEffect(Unit) {
        runCatching { api.browse() }
            .onSuccess { home = it; browseCache = it }
            .onFailure { if (home == null) problem = it.message ?: "Couldn't load Browse." }
    }
    val first = remember { FocusRequester() }
    LaunchedEffect(Unit) {
        if (!takeFocus) return@LaunchedEffect
        delay(120)
        runCatching { first.requestFocus() }
        tookFocus() // once: coming here from the side menu later mustn't take it
    }
    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(top = 18.dp, bottom = 48.dp), verticalArrangement = Arrangement.spacedBy(22.dp)) {
        item(key = "head") {
            Column(Modifier.padding(horizontal = Gutter)) {
                Text("Browse", fontSize = 26.sp, fontWeight = FontWeight.Bold, color = Color.White)
                Gap(14.dp)
                Row(horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                    QuickTile("Coming soon", "New films and episodes on the way", listOf(Color(0xFF3E63DD), Color(0xFF1B2A6B)), Modifier.focusRequester(first)) { open(BrowseAt.Soon) }
                    QuickTile("Top 10", "By service, genre, decade and country", listOf(Color(0xFFD6409F), Color(0xFF5E1A47))) { open(BrowseAt.Top) }
                }
            }
        }
        val h = home
        if (h == null) {
            item(key = "wait") { Text(problem.ifEmpty { "Loading…" }, color = Dim, modifier = Modifier.padding(horizontal = Gutter)) }
            return@LazyColumn
        }
        if (h.services.isNotEmpty()) item(key = "services") {
            Section("Streaming services") {
                items(h.services, key = { it.key }) { s -> ServiceTile(s) { open(BrowseAt.Service(s)) } }
            }
        }
        if (h.collections.isNotEmpty()) item(key = "collections") {
            Section("Collections") {
                items(h.collections, key = { it.key }) { c -> CollectionTile(c) { open(BrowseAt.Collection(c)) } }
            }
        }
        if (h.franchises.isNotEmpty()) item(key = "franchises") {
            Section("Film series") {
                items(h.franchises, key = { it.id }) { f -> FranchiseTile(f) { open(BrowseAt.Franchise(f)) } }
            }
        }
    }
}

@Composable
private fun Section(title: String, content: androidx.compose.foundation.lazy.LazyListScope.() -> Unit) {
    Column {
        Text(title, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, color = Color(0xF2FFFFFF), modifier = Modifier.padding(start = Gutter + 2.dp, bottom = 8.dp))
        LazyRow(contentPadding = PaddingValues(horizontal = Gutter), horizontalArrangement = Arrangement.spacedBy(14.dp), content = content)
    }
}

private val tileShape = RoundedCornerShape(14.dp)

@Composable
private fun TileCard(onClick: () -> Unit, modifier: Modifier = Modifier, content: @Composable () -> Unit) {
    Card(
        onClick = onClick,
        modifier = modifier,
        shape = CardDefaults.shape(tileShape),
        scale = CardDefaults.scale(focusedScale = 1.06f),
        border = CardDefaults.border(focusedBorder = Border(androidx.compose.foundation.BorderStroke(2.dp, Color.White), shape = tileShape)),
        colors = CardDefaults.colors(containerColor = Color(0x14FFFFFF), focusedContainerColor = Color(0x26FFFFFF)),
    ) { content() }
}

@Composable
private fun QuickTile(title: String, blurb: String, colors: List<Color>, modifier: Modifier = Modifier, onClick: () -> Unit) {
    TileCard(onClick, modifier.width(300.dp).height(92.dp)) {
        Column(Modifier.fillMaxSize().background(Brush.linearGradient(colors)).padding(horizontal = 18.dp, vertical = 14.dp), verticalArrangement = Arrangement.Center) {
            Text(title, fontSize = 18.sp, fontWeight = FontWeight.Bold, color = Color.White)
            Gap(4.dp)
            Text(blurb, fontSize = 12.sp, color = Color(0xD9FFFFFF), maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}

@Composable
private fun ServiceTile(s: BrowseService, onClick: () -> Unit) {
    TileCard(onClick, Modifier.width(132.dp).height(112.dp)) {
        Column(Modifier.fillMaxSize().padding(10.dp), horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.Center) {
            if (s.logo != null) {
                AsyncImage(model = s.logo, contentDescription = null, contentScale = ContentScale.Crop,
                    modifier = Modifier.size(54.dp).clip(RoundedCornerShape(12.dp)))
                Gap(8.dp)
            }
            Text(s.name, fontSize = 12.sp, fontWeight = FontWeight.Medium, color = Color(0xD9FFFFFF), maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}

// Each collection has its own colour, so the row reads as a set of shelves.
private fun collectionColors(key: String): List<Color> = when (key) {
    "holiday" -> listOf(Color(0xFFB3261E), Color(0xFF3B0D0A))
    "halloween" -> listOf(Color(0xFFE86A1A), Color(0xFF3A1606))
    "acclaimed" -> listOf(Color(0xFFC9A227), Color(0xFF3D2F06))
    "family" -> listOf(Color(0xFF30A46C), Color(0xFF0D3320))
    "short" -> listOf(Color(0xFF0091FF), Color(0xFF062A4D))
    "gems" -> listOf(Color(0xFF8E4EC6), Color(0xFF2A1240))
    else -> listOf(Color(0xFF4A4D55), Color(0xFF16171B))
}

@Composable
private fun CollectionTile(c: BrowseCollection, onClick: () -> Unit) {
    TileCard(onClick, Modifier.width(240.dp).height(112.dp)) {
        Column(Modifier.fillMaxSize().background(Brush.linearGradient(collectionColors(c.key))).padding(horizontal = 16.dp, vertical = 12.dp), verticalArrangement = Arrangement.Bottom) {
            Text(c.title, fontSize = 17.sp, fontWeight = FontWeight.Bold, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Gap(3.dp)
            Text(c.blurb, fontSize = 11.sp, color = Color(0xCCFFFFFF), maxLines = 2, overflow = TextOverflow.Ellipsis)
        }
    }
}

@Composable
private fun FranchiseTile(f: BrowseFranchise, onClick: () -> Unit) {
    Column(Modifier.width(118.dp)) {
        TileCard(onClick, Modifier.fillMaxWidth().aspectRatio(2f / 3f)) {
            AsyncImage(model = f.poster, contentDescription = f.name, contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize())
        }
        Gap(6.dp)
        Text(f.name, fontSize = 11.sp, color = Color(0xCCFFFFFF), maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

// A service's page: its rows, as on Home.
@Composable
private fun ServicePage(api: CueApi, s: BrowseService, memory: FocusMemory, onCard: (Tile) -> Unit) {
    var rows by remember(s.key) { mutableStateOf<List<Shelf>?>(null) }
    var problem by remember(s.key) { mutableStateOf("") }
    LaunchedEffect(s.key) {
        runCatching { api.browseService(s.key) }
            .onSuccess { rows = it }
            .onFailure { problem = it.message ?: "Couldn't load ${s.name}." }
        // The highlight onto the first title (the tile that opened this is gone).
        rows?.firstOrNull()?.let { r ->
            r.items.firstOrNull()?.let { c ->
                delay(150)
                memory.restore("${r.key}/${c.key}")
            }
        }
    }
    val r = rows
    LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(top = 18.dp, bottom = 48.dp), verticalArrangement = Arrangement.spacedBy(18.dp)) {
        item(key = "head") {
            Row(Modifier.padding(horizontal = Gutter), verticalAlignment = Alignment.CenterVertically) {
                if (s.logo != null) {
                    AsyncImage(model = s.logo, contentDescription = null, contentScale = ContentScale.Crop,
                        modifier = Modifier.size(40.dp).clip(RoundedCornerShape(10.dp)))
                    HGap(14.dp)
                }
                Text(s.name, fontSize = 26.sp, fontWeight = FontWeight.Bold, color = Color.White)
            }
        }
        when {
            r == null -> item(key = "wait") { Text(problem.ifEmpty { "Loading…" }, color = Dim, modifier = Modifier.padding(horizontal = Gutter)) }
            r.isEmpty() -> item(key = "none") { Text("Nothing to show for ${s.name} right now.", color = Dim, modifier = Modifier.padding(horizontal = Gutter)) }
            else -> items(r, key = { it.key }) { shelf -> CardRow(shelf, memory, onClick = onCard) }
        }
    }
}

// A collection or film series: its titles in a grid.
@Composable
private fun GridPage(title: String, blurb: String, key: String, memory: FocusMemory, onCard: (Tile) -> Unit, load: suspend () -> List<Tile>) {
    var items by remember(key) { mutableStateOf<List<Tile>?>(null) }
    var problem by remember(key) { mutableStateOf("") }
    LaunchedEffect(key) {
        runCatching { load() }
            .onSuccess { items = it }
            .onFailure { problem = it.message ?: "Couldn't load $title." }
        items?.firstOrNull()?.let { c ->
            delay(150)
            memory.restore("$key/${c.key}")
        }
    }
    Column(Modifier.fillMaxSize().padding(top = 18.dp)) {
        Column(Modifier.padding(horizontal = Gutter)) {
            Text(title, fontSize = 26.sp, fontWeight = FontWeight.Bold, color = Color.White)
            if (blurb.isNotEmpty()) {
                Gap(4.dp)
                Text(blurb, fontSize = 13.sp, color = Dim)
            }
        }
        val list = items
        when {
            list == null -> Box(Modifier.padding(horizontal = Gutter, vertical = 20.dp)) { Text(problem.ifEmpty { "Loading…" }, color = Dim) }
            list.isEmpty() -> Box(Modifier.padding(horizontal = Gutter, vertical = 20.dp)) { Text("Nothing here right now.", color = Dim) }
            else -> PosterGrid(list, memory, key, onCard)
        }
    }
}
