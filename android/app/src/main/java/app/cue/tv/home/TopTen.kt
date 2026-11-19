@file:OptIn(
    androidx.compose.ui.ExperimentalComposeUiApi::class,
    androidx.compose.foundation.ExperimentalFoundationApi::class,
)

package app.cue.tv.home

import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.focusRestorer
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.graphics.graphicsLayer
import androidx.tv.material3.Text
import org.json.JSONArray
import org.json.JSONObject

// Top 10 on each service: what's most watched right now on Netflix, Prime
// Video and the rest, with the service picked by a tab and big outlined
// numbers beside the posters. Everything in it plays on Cue.

data class TopList(val key: String, val name: String, val items: List<Tile>, val logo: String? = null)

fun topLists(o: JSONObject): List<TopList> {
    val a = o.optJSONArray("top10") ?: return emptyList()
    return (0 until a.length()).map {
        val l = a.getJSONObject(it)
        TopList(l.optString("key"), l.optString("name"), (l.optJSONArray("items") ?: JSONArray()).cards(), l.str("logo"))
    }
}

@Composable
fun TopTenRow(lists: List<TopList>, only: String?, memory: FocusMemory, onFocus: (Tile) -> Unit, onClick: (Tile) -> Unit) {
    val shown = lists.map { l -> if (only == null) l else l.copy(items = l.items.filter { it.kind == only }) }.filter { it.items.isNotEmpty() }
    if (shown.isEmpty()) return
    var key by rememberSaveable { mutableStateOf(shown.first().key) }
    val list = shown.firstOrNull { it.key == key } ?: shown.first()
    Column(Modifier.padding(bottom = 14.dp)) {
        Text("Top 10 on ${list.name} today", fontSize = 14.sp, fontWeight = FontWeight.SemiBold, color = Color(0xF2FFFFFF),
            modifier = Modifier.padding(start = Gutter + 2.dp, bottom = 6.dp))
        LazyRow(Modifier.focusRestorer(), contentPadding = PaddingValues(horizontal = Gutter), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            items(shown.distinctBy { it.key }, key = { it.key }) { l -> ServiceTab(l.name, l.logo, selected = l.key == list.key) { key = l.key } }
        }
        CompositionLocalProvider(LocalBringIntoViewSpec provides PivotSpec) {
            LazyRow(
                modifier = Modifier.focusRestorer(),
                contentPadding = PaddingValues(horizontal = Gutter, vertical = 16.dp),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                items(list.items.distinctBy { it.key }.take(10).withIndex().toList(), key = { "${list.key}/${it.value.key}" }) { (i, c) ->
                    Row(verticalAlignment = Alignment.Bottom) {
                        Box(Modifier.offset(x = 10.dp, y = (-18).dp)) {
                            // The number: the page's dark inside a light outline.
                            Text("${i + 1}", style = TextStyle(fontSize = 104.sp, fontWeight = FontWeight.Black, color = Color(0xFF101114), letterSpacing = (-6).sp))
                            Text("${i + 1}", style = TextStyle(fontSize = 104.sp, fontWeight = FontWeight.Black, color = Color(0x8CFFFFFF), letterSpacing = (-6).sp,
                                drawStyle = Stroke(width = 4f)))
                        }
                        PosterCard(c, memory, "top-${list.key}/${c.key}", false, onFocus, onClick)
                    }
                }
            }
        }
    }
}

// A streaming service as a tab: its logo (a rounded square, ringed when
// chosen), or its name when TMDB has no logo for it.
@Composable
fun ServiceTab(name: String, logo: String?, selected: Boolean, onClick: () -> Unit) {
    if (logo == null) {
        ChoiceTab(name, selected = selected, onClick = onClick)
        return
    }
    val shape = RoundedCornerShape(10.dp)
    androidx.tv.material3.Card(
        onClick = onClick,
        modifier = Modifier.size(40.dp),
        shape = androidx.tv.material3.CardDefaults.shape(shape),
        scale = androidx.tv.material3.CardDefaults.scale(focusedScale = 1.15f),
        border = androidx.tv.material3.CardDefaults.border(
            border = if (selected) androidx.tv.material3.Border(androidx.compose.foundation.BorderStroke(2.dp, Color.White), shape = shape) else androidx.tv.material3.Border.None,
            focusedBorder = androidx.tv.material3.Border(androidx.compose.foundation.BorderStroke(2.dp, Color.White), shape = shape),
        ),
    ) {
        coil.compose.AsyncImage(model = logo, contentDescription = name, contentScale = androidx.compose.ui.layout.ContentScale.Crop,
            modifier = Modifier.fillMaxSize().graphicsLayer { alpha = if (selected) 1f else 0.6f })
    }
}

// ---- The Top 10 tab: a Top 10 for any service, genre, decade or country,
// chosen with the remote, the big numbers beside the posters.

data class TopPage(val title: String, val items: List<Tile>, val services: List<TopList>, val genres: List<String>, val countries: List<Pair<String, String>>)

private val decades = listOf(0, 2020, 2010, 2000, 1990, 1980, 1970)

@Composable
fun TopScreen(api: CueApi, memory: FocusMemory, onCard: (Tile) -> Unit) {
    var kind by rememberSaveable { mutableStateOf("all") }
    var service by rememberSaveable { mutableStateOf("") }
    var genre by rememberSaveable { mutableStateOf("") }
    var decade by rememberSaveable { mutableStateOf(0) }
    var country by rememberSaveable { mutableStateOf("US") }
    var page by remember { mutableStateOf<TopPage?>(null) }
    var loading by remember { mutableStateOf(true) }
    LaunchedEffect(kind, service, genre, decade, country) {
        loading = true
        page = runCatching { api.top(kind, service, genre, decade, country) }.getOrNull() ?: page
        loading = false
    }
    val p = page
    androidx.compose.foundation.lazy.LazyColumn(contentPadding = PaddingValues(top = 14.dp, bottom = 60.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
        item(key = "title") {
            Text(p?.title ?: "Top 10", fontSize = 22.sp, fontWeight = FontWeight.Bold, color = Color.White, modifier = Modifier.padding(start = Gutter + 2.dp))
        }
        item(key = "kind") {
            FilterRow(listOf("all" to "All", "movie" to "Movies", "tv" to "Shows"), kind) { kind = it }
        }
        item(key = "services") {
            LazyRow(Modifier.focusRestorer(), contentPadding = PaddingValues(horizontal = Gutter, vertical = 6.dp), horizontalArrangement = Arrangement.spacedBy(10.dp),
                verticalAlignment = Alignment.CenterVertically) {
                item { ChoiceTab("Everywhere", selected = service == "") { service = "" } }
                items((p?.services ?: emptyList()).distinctBy { it.key }, key = { it.key }) { s -> ServiceTab(s.name, s.logo, selected = s.key == service) { service = s.key } }
            }
        }
        item(key = "genres") {
            FilterRow(listOf("" to "Any genre") + (p?.genres ?: emptyList()).map { it to it }, genre) { genre = it }
        }
        item(key = "decades") {
            FilterRow(decades.map { (if (it == 0) "" else "$it") to (if (it == 0) "Right now" else "The ${it}s") }, if (decade == 0) "" else "$decade") { decade = it.toIntOrNull() ?: 0 }
        }
        if ((p?.countries?.size ?: 0) > 1) item(key = "countries") {
            FilterRow(p?.countries ?: emptyList(), country) { country = it }
        }
        item(key = "grid") {
            val items = p?.items ?: emptyList()
            when {
                items.isEmpty() && loading -> Text("Loading…", color = Dim, modifier = Modifier.padding(start = Gutter, top = 20.dp))
                items.isEmpty() -> Text("Nothing found for that. Try another service or genre.", color = Dim, modifier = Modifier.padding(start = Gutter, top = 20.dp))
                else -> CompositionLocalProvider(LocalBringIntoViewSpec provides PivotSpec) {
                    LazyRow(Modifier.focusRestorer(), contentPadding = PaddingValues(horizontal = Gutter, vertical = 16.dp), horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                        items(items.distinctBy { it.key }.take(10).withIndex().toList(), key = { "top-page/${it.value.key}" }) { (i, c) ->
                            Row(verticalAlignment = Alignment.Bottom) {
                                Box(Modifier.offset(x = 10.dp, y = (-18).dp)) {
                                    Text("${i + 1}", style = TextStyle(fontSize = 104.sp, fontWeight = FontWeight.Black, color = Color(0xFF101114), letterSpacing = (-6).sp))
                                    Text("${i + 1}", style = TextStyle(fontSize = 104.sp, fontWeight = FontWeight.Black, color = Color(0x8CFFFFFF), letterSpacing = (-6).sp,
                                        drawStyle = Stroke(width = 4f)))
                                }
                                PosterCard(c, memory, "top-page/${c.key}", false, {}, onCard)
                            }
                        }
                    }
                }
            }
        }
    }
}

// A row of choices (the one chosen lit).
@Composable
private fun FilterRow(choices: List<Pair<String, String>>, chosen: String, onChoose: (String) -> Unit) {
    LazyRow(Modifier.focusRestorer(), contentPadding = PaddingValues(horizontal = Gutter, vertical = 4.dp), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        items(choices.distinctBy { it.first }, key = { it.first }) { (value, label) -> ChoiceTab(label, selected = value == chosen) { onChoose(value) } }
    }
}
