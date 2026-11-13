package app.cue.tv.home

import android.content.Context
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.FastOutSlowInEasing
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.RepeatMode
import androidx.compose.animation.core.animateDpAsState
import androidx.compose.animation.core.animateFloat
import androidx.compose.animation.core.infiniteRepeatable
import androidx.compose.animation.core.rememberInfiniteTransition
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.focusGroup
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxScope
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
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
import androidx.compose.ui.graphics.ColorFilter
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.res.painterResource
import androidx.compose.ui.text.SpanStyle
import androidx.compose.ui.text.buildAnnotatedString
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.withStyle
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Text
import app.cue.tv.R

// The tour: four slides the first time Cue opens on this TV, on what's
// worth knowing before the first bad version (Other version on the
// player's bar), Skip intro and Next episode, and subtitles and searching
// from a phone. Each has a small drawing of the real control, lit. Profile
// > How Cue works shows it again.
object Tour {
    private const val SEEN = "tour.v1"
    var showing by mutableStateOf(false)

    fun maybeShow(ctx: Context) {
        if (!ctx.getSharedPreferences("cue", Context.MODE_PRIVATE).getBoolean(SEEN, false)) showing = true
    }

    fun done(ctx: Context) {
        ctx.getSharedPreferences("cue", Context.MODE_PRIVATE).edit().putBoolean(SEEN, true).apply()
        showing = false
    }
}

private class TourSlide(val kicker: String, val title: String, val body: List<Pair<String, Boolean>>, val art: @Composable BoxScope.() -> Unit)

// Words, with the names of buttons in bold.
private fun words(vararg parts: String) = parts.mapIndexed { i, s -> s to (i % 2 == 1) }

private val slides = listOf(
    TourSlide(
        "WELCOME", "Everything to watch, in one place",
        words("Movies, shows and live TV for the whole house, each with your own profile, list and place in every episode. Here are three things worth knowing."),
    ) { WelcomeArt() },
    TourSlide(
        "IF SOMETHING LOOKS WRONG", "Try another version",
        words(
            "Cue finds several versions of everything. If one is blurry, in the wrong language, out of sync or keeps stopping, press down for the bar and choose ",
            "Other version", ". Cue plays the next one from the same moment.",
        ),
    ) { VersionsArt() },
    TourSlide(
        "BINGE-FRIENDLY", "Skip the intro, roll into the next",
        words(
            "", "Skip intro", " shows as an episode starts: just press OK. ", "Next episode",
            " is always on the bar, and when an episode ends the next one plays after a short countdown.",
        ),
    ) { SkipArt() },
    TourSlide(
        "SMALL THINGS, BIG DIFFERENCE", "Subtitles, and search from your phone",
        words(
            "", "Subtitles", " on the bar turns them on or off. In ", "Search",
            ", scan the code with your phone: type or say what you want there and it appears here.",
        ),
    ) { PhoneArt() },
)

@Composable
fun TourScreen(done: () -> Unit) {
    var at by remember { mutableIntStateOf(0) }
    var forward by remember { mutableStateOf(true) }
    val next = remember { FocusRequester() }
    // The remote starts on Next (after the slide has drawn), and stays in
    // the tour: left and right move between its buttons.
    LaunchedEffect(at) {
        kotlinx.coroutines.delay(60)
        runCatching { next.requestFocus() }
    }
    androidx.activity.compose.BackHandler {
        if (at > 0) {
            forward = false
            at--
        } else done()
    }
    Box(
        Modifier.fillMaxSize().background(Brush.radialGradient(listOf(Color(0xFF15211F), Color(0xFF0A0B0E)), radius = 1400f))
            .focusGroup(),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            Modifier.width(620.dp).clip(RoundedCornerShape(26.dp)).background(
                Brush.verticalGradient(listOf(Color(0xFF1E2026), Color(0xFF131418))),
            ).border(1.dp, Color(0x14FFFFFF), RoundedCornerShape(26.dp)).padding(14.dp).rise(0),
        ) {
            AnimatedContent(
                targetState = at,
                transitionSpec = {
                    val d = if (forward) 1 else -1
                    (fadeIn(tween(420, delayMillis = 60)) + slideInHorizontally(tween(480, easing = Settle)) { it / 14 * d }) togetherWith
                        (fadeOut(tween(200)) + slideOutHorizontally(tween(300)) { -it / 20 * d })
                },
                label = "tour",
            ) { i ->
                val s = slides[i]
                Column {
                    Box(
                        Modifier.fillMaxWidth().aspectRatio(16f / 9f).clip(RoundedCornerShape(16.dp)).background(Color(0xFF07080B)),
                        content = s.art,
                    )
                    Column(Modifier.padding(start = 14.dp, end = 14.dp, top = 22.dp)) {
                        Text(s.kicker, fontSize = 11.sp, fontWeight = FontWeight.Bold, color = tint(), letterSpacing = 1.6.sp)
                        Spacer(Modifier.height(6.dp))
                        Text(s.title, fontFamily = Type.display(), fontSize = 24.sp, fontWeight = FontWeight.Bold, color = Color.White)
                        Spacer(Modifier.height(8.dp))
                        Text(
                            buildAnnotatedString {
                                s.body.forEach { (t, bold) ->
                                    if (bold) withStyle(SpanStyle(color = Color.White, fontWeight = FontWeight.SemiBold)) { append(t) } else append(t)
                                }
                            },
                            fontSize = 14.sp, lineHeight = 21.sp, color = Color(0xB8FFFFFF),
                            modifier = Modifier.heightIn(min = 63.dp),
                        )
                    }
                }
            }
            Row(
                Modifier.fillMaxWidth().padding(start = 14.dp, end = 14.dp, top = 20.dp, bottom = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Row(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                    slides.indices.forEach { i ->
                        val w by animateDpAsState(if (i == at) 22.dp else 8.dp, tween(350, easing = Settle), label = "dot")
                        Box(Modifier.width(w).height(8.dp).clip(CircleShape).background(if (i == at) tint() else Color(0x38FFFFFF)))
                    }
                }
                Spacer(Modifier.weight(1f))
                Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    if (at > 0) {
                        Pill("Back") {
                            forward = false
                            at--
                        }
                    } else {
                        Pill("Skip", onClick = done)
                    }
                    val last = at == slides.lastIndex
                    Pill(if (last) "Start watching" else if (at == 0) "Show me" else "Next", primary = true, modifier = Modifier.focusRequester(next)) {
                        if (last) done() else {
                            forward = true
                            at++
                        }
                    }
                }
            }
        }
    }
}

// ---- The drawings: small, calm versions of the real screens ----

@Composable
private fun BoxScope.Picture(first: Color, second: Color) {
    val t = rememberInfiniteTransition(label = "drift")
    val z by t.animateFloat(1f, 1.08f, infiniteRepeatable(tween(9000, easing = LinearEasing), RepeatMode.Reverse), label = "z")
    Box(
        Modifier.matchParentSize().graphicsLayer { scaleX = z; scaleY = z }.background(
            Brush.radialGradient(listOf(first, Color.Transparent), radius = 520f, center = androidx.compose.ui.geometry.Offset(560f, 160f)),
        ).background(
            Brush.radialGradient(listOf(second, Color.Transparent), radius = 420f, center = androidx.compose.ui.geometry.Offset(160f, 300f)),
        ),
    )
}

// The breathing ring round the control to press.
@Composable
private fun Modifier.ring(shape: RoundedCornerShape = RoundedCornerShape(50)): Modifier {
    val t = rememberInfiniteTransition(label = "ring")
    val a by t.animateFloat(1f, 0f, infiniteRepeatable(tween(1800, easing = FastOutSlowInEasing)), label = "a")
    return border(2.dp, Teal.copy(alpha = a), shape).padding(3.dp)
}

@Composable
private fun MiniButton(label: String, icon: Int, lit: Boolean = false, ring: Boolean = false) {
    val inner = @Composable {
        Row(
            Modifier.height(24.dp).clip(RoundedCornerShape(50)).background(if (lit) Color.White else Color.Transparent)
                .padding(horizontal = if (lit) 10.dp else 6.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Image(painterResource(icon), null, Modifier.size(12.dp), colorFilter = ColorFilter.tint(if (lit) Color(0xFF0B0C0F) else Color.White))
            if (label.isNotEmpty()) {
                Spacer(Modifier.width(5.dp))
                Text(label, fontSize = 10.sp, fontWeight = FontWeight.SemiBold, color = if (lit) Color(0xFF0B0C0F) else Color.White)
            }
        }
    }
    if (ring) Box(Modifier.ring()) { inner() } else inner()
}

@Composable
private fun BoxScope.Bar(content: @Composable () -> Unit) {
    Column(
        Modifier.align(Alignment.BottomCenter).padding(horizontal = 18.dp, vertical = 14.dp).fillMaxWidth()
            .clip(RoundedCornerShape(12.dp)).background(Color(0xD1141519)).border(1.dp, Color(0x14FFFFFF), RoundedCornerShape(12.dp))
            .padding(horizontal = 10.dp, vertical = 7.dp),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text("18:42", fontSize = 9.sp, color = Color(0xFFE6E7EA))
            Spacer(Modifier.width(8.dp))
            Box(Modifier.weight(1f).height(3.dp).clip(CircleShape).background(Color(0x33FFFFFF))) {
                Box(Modifier.fillMaxWidth(0.34f).height(3.dp).background(Teal))
            }
            Spacer(Modifier.width(8.dp))
            Text("-31:06", fontSize = 9.sp, color = Color(0xFFE6E7EA))
        }
        Spacer(Modifier.height(6.dp))
        Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(4.dp)) {
            MiniButton("", R.drawable.ic_cue_pause)
            Spacer(Modifier.weight(1f))
            content()
        }
    }
}

@Composable
private fun BoxScope.WelcomeArt() {
    val t = rememberInfiniteTransition(label = "glow")
    val g by t.animateFloat(0.85f, 1.15f, infiniteRepeatable(tween(3600), RepeatMode.Reverse), label = "g")
    Box(
        Modifier.matchParentSize().background(
            Brush.verticalGradient(listOf(Color.Transparent, Teal.copy(alpha = 0.14f))),
        ),
    )
    Row(
        Modifier.align(Alignment.BottomCenter).padding(bottom = 18.dp).graphicsLayer { alpha = 0.5f },
        horizontalArrangement = Arrangement.spacedBy(10.dp),
    ) {
        listOf(0xFF2A5A55, 0xFF2A3F5E, 0xFF4A2F5E, 0xFF5E2F45, 0xFF5E4A2F).forEachIndexed { i, c ->
            Box(Modifier.width(80.dp).height(50.dp).rise(150 + i * 60).clip(RoundedCornerShape(6.dp))
                .background(Brush.linearGradient(listOf(Color(c), Background))))
        }
    }
    Box(Modifier.align(Alignment.Center).size(200.dp).graphicsLayer { scaleX = g; scaleY = g }
        .background(Brush.radialGradient(listOf(Teal.copy(alpha = 0.4f), Color.Transparent))))
    Image(painterResource(R.drawable.ic_cue_mark), "Cue", Modifier.align(Alignment.Center).size(110.dp).rise(80),
        colorFilter = ColorFilter.tint(Teal))
}

@Composable
private fun BoxScope.VersionsArt() {
    Picture(Color(0xFF3D5A73), Color(0xFF6B4A3A))
    Bar {
        MiniButton("Next episode", R.drawable.ic_cue_next)
        MiniButton("Other version", R.drawable.ic_cue_versions, lit = true, ring = true)
        MiniButton("", R.drawable.ic_cue_audio)
        MiniButton("", R.drawable.ic_cue_subtitles)
    }
}

@Composable
private fun BoxScope.SkipArt() {
    Picture(Color(0xFF9A5B2E), Color(0xFF3B2A4F))
    Text("S2 · E4", fontSize = 10.sp, fontWeight = FontWeight.Bold, color = Color(0xCCFFFFFF),
        modifier = Modifier.align(Alignment.TopStart).padding(18.dp))
    Box(Modifier.align(Alignment.BottomEnd).padding(end = 18.dp, bottom = 92.dp).rise(400).ring()) {
        Text("Skip intro  ›", fontSize = 11.sp, fontWeight = FontWeight.Bold, color = Color(0xFF0B0C0F),
            modifier = Modifier.clip(RoundedCornerShape(50)).background(Color.White).padding(horizontal = 14.dp, vertical = 6.dp))
    }
    Bar {
        MiniButton("Next episode", R.drawable.ic_cue_next, lit = true)
        MiniButton("", R.drawable.ic_cue_versions)
        MiniButton("", R.drawable.ic_cue_subtitles)
    }
}

@Composable
private fun BoxScope.PhoneArt() {
    Picture(Color(0xFF1F5C63), Color(0xFF26324F))
    Column(Modifier.align(Alignment.TopStart).padding(22.dp)) {
        Box(Modifier.ring()) {
            MiniButton("Subtitles", R.drawable.ic_cue_subtitles, lit = true)
        }
        Spacer(Modifier.height(28.dp))
        Qr(96.dp)
    }
    // The phone, rising into view with the search being typed.
    var typed by remember { mutableIntStateOf(0) }
    LaunchedEffect(Unit) {
        kotlinx.coroutines.delay(900)
        while (typed < 4) {
            kotlinx.coroutines.delay(220)
            typed++
        }
    }
    Column(
        Modifier.align(Alignment.TopEnd).padding(top = 30.dp, end = 60.dp).width(150.dp).height(320.dp).rise(250, 40.dp)
            .clip(RoundedCornerShape(20.dp)).background(Color(0xFF15161B)).border(2.dp, Color(0x24FFFFFF), RoundedCornerShape(20.dp))
            .padding(horizontal = 12.dp, vertical = 30.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text("⌕  " + "Dune".take(typed) + "▏", fontSize = 11.sp, color = Color.White,
            modifier = Modifier.fillMaxWidth().clip(RoundedCornerShape(7.dp)).background(Color(0x1AFFFFFF)).padding(horizontal = 8.dp, vertical = 5.dp))
        Spacer(Modifier.height(20.dp))
        Box(Modifier.size(34.dp).border(6.dp, Teal.copy(alpha = 0.2f), CircleShape).padding(6.dp).clip(CircleShape).background(Teal))
        Spacer(Modifier.height(20.dp))
        if (typed >= 4) {
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                Box(Modifier.width(54.dp).height(80.dp).rise(0).clip(RoundedCornerShape(5.dp))
                    .background(Brush.linearGradient(listOf(Color(0xFFC08A3E), Color(0xFF4A2F1A)))))
                Box(Modifier.width(54.dp).height(80.dp).rise(100).clip(RoundedCornerShape(5.dp))
                    .background(Brush.linearGradient(listOf(Color(0xFF3D6A8A), Color(0xFF182634)))))
            }
        }
    }
}

// A QR code's look (not a working one): its three corner squares and a
// pattern between.
@Composable
private fun Qr(size: Dp) {
    val cells = 9
    Box(Modifier.size(size).clip(RoundedCornerShape(6.dp)).background(Color.White).padding(6.dp)) {
        Column {
            for (y in 0 until cells) {
                Row {
                    for (x in 0 until cells) {
                        val corner = (x < 3 && y < 3) || (x >= cells - 3 && y < 3) || (x < 3 && y >= cells - 3)
                        val on = corner || (x * 7 + y * 3 + x * y) % 3 == 0
                        Box(Modifier.size((size - 12.dp) / cells).background(if (on) Color(0xFF0B0C0F) else Color.White))
                    }
                }
            }
        }
    }
}
