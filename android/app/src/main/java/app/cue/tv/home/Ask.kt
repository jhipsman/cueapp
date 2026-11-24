package app.cue.tv.home

import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.OutlinedTextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateListOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Text
import kotlinx.coroutines.launch
import org.json.JSONArray
import org.json.JSONObject

// ---- Ask Cue on the TV: ask for something to watch, out loud where the TV
// lets apps listen, typed with the TV's keyboard, or with one of the ideas
// (one press, nothing to type). The answer and its picks show beside it;
// asking again carries on the conversation.

data class AskAnswer(val reply: String, val heading: String?, val cards: List<Tile>, val open: Tile?)

suspend fun CueApi.askReady(): Boolean = runCatching { obj("/api/watch/ask").optBoolean("ready") }.getOrDefault(false)

suspend fun CueApi.ask(turns: List<Pair<String, String>>): AskAnswer {
    val body = JSONObject().put("messages", JSONArray().apply {
        turns.forEach { (role, text) -> put(JSONObject().put("role", role).put("text", text)) }
    })
    val o = JSONObject(request("POST", "/api/watch/ask", body).ifEmpty { "{}" })
    val cards = (o.optJSONArray("cards") ?: JSONArray()).cards()
    // "Play X" or "Open X": its page (Play is there; nothing starts by surprise).
    val target = o.optJSONObject("play") ?: o.optJSONObject("open")
    val open = target?.let { t ->
        cards.firstOrNull { it.kind == t.optString("kind") && it.tmdbId == t.optInt("tmdbId") }
            ?: Tile.from(JSONObject().put("kind", t.optString("kind")).put("tmdbId", t.optInt("tmdbId")).put("title", ""))
    }
    return AskAnswer(o.optString("reply"), o.str("heading"), cards, open)
}

private val ideas = listOf(
    "Something funny for tonight", "A film under 90 minutes", "Best sci-fi of the 90s",
    "What should I watch next?", "Something the whole family will like", "A great thriller series",
)

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun AskScreen(api: CueApi, memory: FocusMemory, onCard: (Tile) -> Unit, takeFocus: Boolean = false, onClose: (() -> Unit)? = null) {
    // Opened from Search: Back goes back there.
    if (onClose != null) androidx.activity.compose.BackHandler { onClose() }
    // Opened from a button that's now gone: the highlight onto the first idea.
    val firstIdea = remember { androidx.compose.ui.focus.FocusRequester() }
    LaunchedEffect(Unit) {
        if (!takeFocus) return@LaunchedEffect
        kotlinx.coroutines.delay(150)
        runCatching { firstIdea.requestFocus() }
    }
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var ready by remember { mutableStateOf<Boolean?>(null) }
    var question by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    var answer by remember { mutableStateOf<AskAnswer?>(null) }
    var problem by remember { mutableStateOf("") }
    val turns = remember { mutableStateListOf<Pair<String, String>>() }
    LaunchedEffect(Unit) { ready = api.askReady() }

    fun send(text: String) {
        val q = text.trim()
        if (q.isEmpty() || busy) return
        busy = true
        problem = ""
        turns.add("user" to q)
        scope.launch {
            try {
                val a = api.ask(turns.toList())
                turns.add("assistant" to a.reply)
                answer = a
                question = ""
                a.open?.let { onCard(it) }
            } catch (e: kotlinx.coroutines.CancellationException) {
                throw e
            } catch (e: Exception) {
                turns.removeAt(turns.lastIndex)
                problem = e.message ?: "Couldn't ask Cue just now."
            } finally {
                busy = false
            }
        }
    }

    // Voice, where the TV lets apps listen (Google TV; not most Fire TVs).
    val speech = remember {
        android.content.Intent(android.speech.RecognizerIntent.ACTION_RECOGNIZE_SPEECH)
            .putExtra(android.speech.RecognizerIntent.EXTRA_LANGUAGE_MODEL, android.speech.RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
            .putExtra(android.speech.RecognizerIntent.EXTRA_PROMPT, "Ask Cue for something to watch")
    }
    val canListen = remember { speech.resolveActivity(ctx.packageManager) != null }
    val listener = androidx.activity.compose.rememberLauncherForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.StartActivityForResult(),
    ) { r -> r.data?.getStringArrayListExtra(android.speech.RecognizerIntent.EXTRA_RESULTS)?.firstOrNull()?.let { send(it) } }

    Row(Modifier.fillMaxSize()) {
        // ---- asking ----
        Column(
            Modifier.width(430.dp).fillMaxHeight()
                .background(Brush.horizontalGradient(0f to Color(0x66101318), 1f to Color.Transparent))
                .padding(start = Gutter, end = 28.dp, top = 22.dp),
        ) {
            Text("Ask Cue", fontFamily = Type.display(), fontSize = 34.sp, fontWeight = FontWeight.Bold, color = Color.White, modifier = Modifier.rise(0))
            Spacer(Modifier.height(4.dp))
            Text("Tell it what you're in the mood for.", fontSize = 13.sp, color = Dim)
            Spacer(Modifier.height(14.dp))
            if (ready == false) {
                Text("Ask Cue isn't set up on this server yet. The owner turns it on in Settings › Movie info on the web.",
                    fontSize = 14.sp, color = Dim)
                return@Column
            }
            // The ideas first: coming over from the side menu, the highlight
            // lands on one (one press asks), not on the typing box (which
            // would bring up the keyboard straight away).
            Text("TRY", fontSize = 11.sp, fontWeight = FontWeight.Bold, color = Dim, letterSpacing = 1.5.sp)
            Spacer(Modifier.height(8.dp))
            FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.rise(140)) {
                ideas.forEachIndexed { i, idea ->
                    Pill(idea, modifier = if (i == 0) Modifier.focusRequester(firstIdea) else Modifier) { question = idea; send(idea) }
                }
            }
            Spacer(Modifier.height(22.dp))
            Text("OR ASK YOUR OWN", fontSize = 11.sp, fontWeight = FontWeight.Bold, color = Dim, letterSpacing = 1.5.sp)
            Spacer(Modifier.height(8.dp))
            OutlinedTextField(
                question, { question = it }, singleLine = true, colors = fieldColors(),
                placeholder = { androidx.compose.material3.Text(if (turns.isEmpty()) "Something cosy for a rainy night" else "Ask a follow-up") },
                leadingIcon = { androidx.compose.material3.Text("✨", fontSize = 16.sp) },
                shape = RoundedCornerShape(14.dp),
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                keyboardActions = KeyboardActions(onSearch = { send(question) }, onDone = { send(question) }),
                modifier = Modifier.fillMaxWidth().leaveWithArrows().rise(60),
            )
            Spacer(Modifier.height(10.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp), modifier = Modifier.rise(100)) {
                Pill(if (busy) "Thinking…" else "Ask", primary = true) { send(question) }
                if (canListen) Pill("🎙  Speak") { runCatching { listener.launch(speech) } }
                if (turns.isNotEmpty()) Pill("New question") {
                    turns.clear()
                    answer = null
                    question = ""
                }
            }
        }
        // ---- the answer ----
        Column(Modifier.weight(1f).fillMaxHeight().padding(top = 30.dp, end = Gutter)) {
            val a = answer
            when {
                busy -> Text("Thinking…", fontSize = 16.sp, color = Dim, modifier = Modifier.padding(start = 24.dp))
                problem.isNotEmpty() -> Text(problem, fontSize = 15.sp, color = Color(0xFFFF8A8A), modifier = Modifier.padding(start = 24.dp))
                a == null -> Box(Modifier.padding(start = 24.dp)) {
                    Text("Ask anything: a mood, a length, an actor, \"like Severance but funnier\"…", fontSize = 16.sp, color = Dim)
                }
                else -> {
                    Text(a.reply, fontSize = 17.sp, color = Color(0xF2FFFFFF), lineHeight = 25.sp, maxLines = 8, overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.padding(start = 24.dp))
                    if (a.cards.isNotEmpty()) {
                        Spacer(Modifier.height(18.dp))
                        CardRow(Shelf("ask-${turns.size}", a.heading ?: "Picks", a.cards), memory, onClick = onCard)
                    }
                }
            }
        }
    }
}
