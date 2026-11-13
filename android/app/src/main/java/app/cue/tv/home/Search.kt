package app.cue.tv.home

import android.content.Context
import android.graphics.Bitmap
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.OutlinedTextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Text
import com.google.zxing.BarcodeFormat
import com.google.zxing.EncodeHintType
import com.google.zxing.qrcode.QRCodeWriter
import kotlinx.coroutines.delay

// Search: the field and the ways in down the left (the TV's keyboard, Speak
// where the TV lets apps listen, a phone through the QR code, recent
// searches), and the posters on the right: what's popular until something's
// typed, then what matches, as you go.
//
// A Fire TV keeps its remote's microphone for Alexa, so the phone is how Cue
// hears you there: scan the code, then type or say it on the phone, and the
// words show up here (the server passes them along, /api/watch/remote).

private const val ALPHABET = "ABCDEFGHJKMNPQRSTUVWXYZ23456789" // no 0/O, 1/I/L

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun SearchScreen(api: CueApi, memory: FocusMemory, listen: Boolean = false, listened: () -> Unit = {}, onCard: (Tile) -> Unit) {
    val ctx = LocalContext.current
    var query by remember { mutableStateOf("") }

    // Voice, where the TV lets apps listen (Google TV, Android TV).
    val speech = remember {
        android.content.Intent(android.speech.RecognizerIntent.ACTION_RECOGNIZE_SPEECH)
            .putExtra(android.speech.RecognizerIntent.EXTRA_LANGUAGE_MODEL, android.speech.RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
            .putExtra(android.speech.RecognizerIntent.EXTRA_PROMPT, "Say a movie or show")
    }
    val canListen = remember { speech.resolveActivity(ctx.packageManager) != null }
    val listener = androidx.activity.compose.rememberLauncherForActivityResult(
        androidx.activity.result.contract.ActivityResultContracts.StartActivityForResult(),
    ) { r ->
        r.data?.getStringArrayListExtra(android.speech.RecognizerIntent.EXTRA_RESULTS)?.firstOrNull()?.let { query = it }
    }
    LaunchedEffect(listen) {
        if (listen) {
            listened()
            if (canListen) runCatching { listener.launch(speech) }
        }
    }

    // A phone, through the code: what it types becomes the search.
    val code = remember { (1..5).map { ALPHABET.random() }.joinToString("") }
    val phoneUrl = "${api.server.trimEnd('/')}/watch/remote/$code"
    LaunchedEffect(code) {
        var seen = 0
        while (true) {
            delay(800)
            val (q, seq) = runCatching { api.remoteSearch(code) }.getOrNull() ?: continue
            if (seq > seen) {
                seen = seq
                query = q
            }
        }
    }

    // What's popular, until something's typed.
    var popular by remember { mutableStateOf<List<Tile>>(emptyList()) }
    LaunchedEffect(Unit) {
        popular = runCatching {
            api.home().rows.filter { it.key.startsWith("trending") }.flatMap { it.items }.distinctBy { it.key }.take(24)
        }.getOrDefault(emptyList())
    }
    var results by remember { mutableStateOf<List<Tile>?>(null) }
    LaunchedEffect(query) {
        val q = query.trim()
        if (q.length < 2) { results = null; return@LaunchedEffect }
        delay(350)
        results = runCatching { api.search(q) }.getOrDefault(results ?: emptyList())
    }
    var recent by remember { mutableStateOf(recentSearches(ctx)) }
    val open: (Tile) -> Unit = { c ->
        val q = query.trim()
        if (q.length >= 2) { saveRecent(ctx, q); recent = recentSearches(ctx) }
        onCard(c)
    }

    Row(Modifier.fillMaxSize()) {
        // ---- the ways in ----
        Column(
            Modifier.width(400.dp).fillMaxHeight()
                .background(Brush.horizontalGradient(0f to Color(0x66101318), 1f to Color.Transparent))
                .padding(start = Gutter, end = 28.dp, top = 22.dp),
        ) {
            Text("Search", fontFamily = Type.display(), fontSize = 34.sp, fontWeight = FontWeight.Bold, color = Color.White, modifier = Modifier.rise(0))
            Spacer(Modifier.height(14.dp))
            OutlinedTextField(
                query, { query = it }, singleLine = true, colors = fieldColors(),
                placeholder = { androidx.compose.material3.Text("Movies and shows") },
                leadingIcon = { androidx.compose.material3.Text("🔍", fontSize = 16.sp) },
                shape = RoundedCornerShape(14.dp),
                modifier = Modifier.fillMaxWidth().leaveWithArrows().rise(60),
            )
            if (canListen) {
                Spacer(Modifier.height(10.dp))
                Box(Modifier.rise(100)) { Pill("🎙  Speak") { runCatching { listener.launch(speech) } } }
            }
            Spacer(Modifier.height(20.dp))
            PhoneCard(phoneUrl, code, Modifier.rise(140))
            if (recent.isNotEmpty()) {
                Spacer(Modifier.height(20.dp))
                Text("RECENT", fontSize = 11.sp, fontWeight = FontWeight.Bold, color = Dim, letterSpacing = 1.5.sp)
                Spacer(Modifier.height(8.dp))
                FlowRow(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
                    recent.forEach { r -> Pill(r) { query = r } }
                }
            }
        }
        // ---- the posters ----
        Column(Modifier.weight(1f).fillMaxHeight().padding(top = 26.dp)) {
            val shown = results
            val q = query.trim()
            val heading = when {
                shown == null -> "Popular right now"
                shown.isEmpty() -> "Nothing matches “$q”"
                else -> "Results for “$q”"
            }
            Text(heading, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, color = Color.White, modifier = Modifier.padding(start = 4.dp))
            if (shown != null && shown.isEmpty()) {
                Text("Try fewer words, or another spelling.", fontSize = 13.sp, color = Dim, modifier = Modifier.padding(start = 4.dp, top = 6.dp))
            }
            PosterGrid(shown ?: popular, memory, if (shown == null) "popular" else "search", open, sidePadding = 4.dp)
        }
    }
}

// The QR code for searching from a phone, with its address in words for
// anyone without a camera app that reads codes.
@Composable
private fun PhoneCard(url: String, code: String, modifier: Modifier = Modifier) {
    val qr = remember(url) { qrBitmap(url, 360) }
    Row(
        modifier.fillMaxWidth().clip(RoundedCornerShape(18.dp))
            .background(Brush.linearGradient(listOf(Color(0x2234D1BF), Color(0x0FFFFFFF))))
            .border(1.dp, Color(0x1FFFFFFF), RoundedCornerShape(18.dp))
            .padding(14.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp),
    ) {
        Box(Modifier.size(104.dp).clip(RoundedCornerShape(12.dp)).background(Color.White).padding(6.dp)) {
            if (qr != null) Image(qr.asImageBitmap(), contentDescription = "QR code to search from your phone", modifier = Modifier.fillMaxSize())
        }
        Column(Modifier.weight(1f)) {
            Text("Search from your phone", fontSize = 15.sp, fontWeight = FontWeight.Bold, color = Color.White)
            Spacer(Modifier.height(4.dp))
            Text("Scan with the camera, then type or say it. It shows up here.", fontSize = 12.sp, color = Dim, lineHeight = 16.sp)
            Spacer(Modifier.height(8.dp))
            Text("Code $code", fontSize = 12.sp, fontWeight = FontWeight.SemiBold, color = tint(), letterSpacing = 1.sp)
        }
    }
}

internal fun qrBitmap(text: String, size: Int): Bitmap? = runCatching {
    val m = QRCodeWriter().encode(text, BarcodeFormat.QR_CODE, size, size, mapOf(EncodeHintType.MARGIN to 0))
    val px = IntArray(m.width * m.height) { i -> if (m.get(i % m.width, i / m.width)) 0xFF0B0C0F.toInt() else 0xFFFFFFFF.toInt() }
    Bitmap.createBitmap(px, m.width, m.height, Bitmap.Config.ARGB_8888)
}.getOrNull()

// The last few searches that led somewhere, newest first.
private fun recentSearches(ctx: Context): List<String> =
    (ctx.getSharedPreferences("cue", Context.MODE_PRIVATE).getString("recentSearches", "") ?: "").split('\n').filter { it.isNotBlank() }

private fun saveRecent(ctx: Context, q: String) {
    val list = (listOf(q) + recentSearches(ctx).filter { !it.equals(q, ignoreCase = true) }).take(6)
    ctx.getSharedPreferences("cue", Context.MODE_PRIVATE).edit().putString("recentSearches", list.joinToString("\n")).apply()
}
