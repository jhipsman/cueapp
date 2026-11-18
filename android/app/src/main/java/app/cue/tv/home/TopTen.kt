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
import androidx.tv.material3.Text
import org.json.JSONArray
import org.json.JSONObject

// Top 10 on each service: what's most watched right now on Netflix, Prime
// Video and the rest, with the service picked by a tab and big outlined
// numbers beside the posters. Everything in it plays on Cue.

data class TopList(val key: String, val name: String, val items: List<Tile>)

fun topLists(o: JSONObject): List<TopList> {
    val a = o.optJSONArray("top10") ?: return emptyList()
    return (0 until a.length()).map {
        val l = a.getJSONObject(it)
        TopList(l.optString("key"), l.optString("name"), (l.optJSONArray("items") ?: JSONArray()).cards())
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
            items(shown, key = { it.key }) { l -> ChoiceTab(l.name, selected = l.key == list.key) { key = l.key } }
        }
        CompositionLocalProvider(LocalBringIntoViewSpec provides PivotSpec) {
            LazyRow(
                modifier = Modifier.focusRestorer(),
                contentPadding = PaddingValues(horizontal = Gutter, vertical = 16.dp),
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                items(list.items.take(10).withIndex().toList(), key = { "${list.key}/${it.value.key}" }) { (i, c) ->
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
