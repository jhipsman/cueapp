@file:OptIn(androidx.tv.material3.ExperimentalTvMaterial3Api::class)

package app.cue.tv.home

import android.annotation.SuppressLint
import android.app.ActivityManager
import android.content.Context
import android.webkit.JavascriptInterface
import android.webkit.WebView
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.Crossfade
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.LinearEasing
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import kotlinx.coroutines.delay

// Bringing the artwork alive: the banner's trailer and the screensaver.

// The signed-in connection to Cue, for pieces deep in the screens (a card's
// logo) that would otherwise need it passed through every layer.
val LocalApi = staticCompositionLocalOf<CueApi?> { null }

// ---- The living banner ----

// How long the highlight rests before the trailer starts, and how long it
// plays before the banner settles back to its picture.
private const val TRAILER_REST_MS = 2500L
private const val TRAILER_LENGTH_MS = 60_000L

// Boxes with little memory keep the still picture: a web player on top of
// the app's own screens is too much for them.
private fun trailersFit(ctx: Context): Boolean =
    !(ctx.getSystemService(Context.ACTIVITY_SERVICE) as ActivityManager).isLowRamDevice

// TrailerLayer plays a title's YouTube trailer, muted, after the highlight
// has rested on it a moment. It loads out of sight and fades in only once
// YouTube says it's playing (and its own title and buttons have gone), so a
// trailer that can't play here just never appears.
@SuppressLint("SetJavaScriptEnabled")
@Composable
fun TrailerLayer(server: String, videoId: String?, modifier: Modifier = Modifier) {
    val ctx = LocalContext.current
    if (videoId.isNullOrEmpty() || !trailersFit(ctx)) return
    var started by remember(videoId) { mutableStateOf(false) }
    var playing by remember(videoId) { mutableStateOf(false) }
    LaunchedEffect(videoId) {
        delay(TRAILER_REST_MS)
        started = true
        delay(TRAILER_LENGTH_MS)
        playing = false
        delay(900)
        started = false
    }
    val alpha by animateFloatAsState(if (playing) 1f else 0f, tween(900, easing = Settle), label = "trailer")
    if (!started) return
    AndroidView(
        factory = { c ->
            WebView(c).apply {
                isFocusable = false
                isFocusableInTouchMode = false
                setBackgroundColor(android.graphics.Color.TRANSPARENT)
                settings.javaScriptEnabled = true
                settings.domStorageEnabled = true
                settings.mediaPlaybackRequiresUserGesture = false
                addJavascriptInterface(object {
                    @JavascriptInterface
                    fun playing() { post { playing = true } }

                    @JavascriptInterface
                    fun stopped() { post { playing = false } }
                }, "Cue")
                loadDataWithBaseURL(server, trailerPage(videoId, server), "text/html", "utf-8", null)
            }
        },
        onRelease = { it.destroy() },
        modifier = modifier.graphicsLayer { this.alpha = alpha },
    )
}

// The page the trailer plays in: YouTube's player, filling the box (cropped
// like a picture, a little larger so its corners never show), reporting back
// when it's really playing.
private fun trailerPage(id: String, origin: String): String {
    val safe = id.filter { it.isLetterOrDigit() || it == '-' || it == '_' }
    return """<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1">
<style>html,body{margin:0;height:100%;background:transparent;overflow:hidden}
#p{position:absolute;top:50%;left:50%;width:max(100vw,177.78vh);height:max(100vh,56.25vw);transform:translate(-50%,-50%) scale(1.18);border:0}</style>
</head><body><div id="p"></div><script>
var t=document.createElement('script');t.src='https://www.youtube.com/iframe_api';document.head.appendChild(t);
function onYouTubeIframeAPIReady(){new YT.Player('p',{videoId:'$safe',host:'https://www.youtube-nocookie.com',
playerVars:{autoplay:1,mute:1,controls:0,playsinline:1,rel:0,modestbranding:1,iv_load_policy:3,disablekb:1,fs:0,start:4,loop:1,playlist:'$safe',origin:'$origin'},
events:{onReady:function(e){e.target.mute();e.target.playVideo()},
onStateChange:function(e){if(e.data==1){setTimeout(function(){Cue.playing()},1200)}else if(e.data==0||e.data==2){Cue.stopped()}},
onError:function(){Cue.stopped()}}})}
</script></body></html>"""
}

// ---- The screensaver ----

const val SAVER_IDLE_MS = 3 * 60_000L
private const val SLIDE_MS = 9000L

data class SaverSlide(val tile: Tile, val line: String, val detail: String)

fun saverSlides(home: Home): List<SaverSlide> {
    val out = ArrayList<SaverSlide>()
    val seen = HashSet<String>()
    fun add(t: Tile, line: String, detail: String) {
        val k = "${t.kind}-${t.tmdbId}"
        if (t.backdrop == null || !seen.add(k)) return
        out.add(SaverSlide(t, line, detail))
    }
    for (r in home.rows) for (t in r.items.take(8)) {
        val kind = if (t.kind == "tv") "Series" else "Movie"
        when {
            r.key == "new-episodes" -> add(t, "New episode", t.episodeLine)
            r.key == "continue" -> add(t, "You stopped here", t.episodeLine.ifEmpty { "${((t.progress ?: 0.0) * 100).toInt()}% watched" })
            r.key == "my-list" -> add(t, "On your list", listOfNotNull(t.year?.toString(), kind).joinToString(" · "))
            r.key.startsWith("trending") -> add(t, "Trending this week", listOfNotNull(t.year?.toString(), kind).joinToString(" · "))
        }
    }
    return out.take(14)
}

// Screensaver: the household's own titles full screen, one after another,
// each picture slowly moving, with its logo, a line about it and a clock.
@Composable
fun Screensaver(api: CueApi, slides: List<SaverSlide>) {
    var index by remember { mutableStateOf(0) }
    LaunchedEffect(slides) {
        while (true) {
            delay(SLIDE_MS)
            index++
        }
    }
    val slide = slides[index % slides.size]
    val ex = rememberExtras(api, slide.tile.kind, slide.tile.tmdbId)
    val tintState = LocalTint.current
    LaunchedEffect(ex?.colour) { ex?.colour?.let { tintState.value = it } }
    var clock by remember { mutableStateOf(System.currentTimeMillis()) }
    LaunchedEffect(Unit) {
        while (true) {
            clock = System.currentTimeMillis()
            delay(15_000)
        }
    }
    Box(Modifier.fillMaxSize().background(Color.Black)) {
        Crossfade(targetState = index % slides.size, animationSpec = tween(2200), label = "saver") { i ->
            val drift = remember(i) { Animatable(0f) }
            LaunchedEffect(i) { drift.animateTo(1f, tween((SLIDE_MS + 3000).toInt(), easing = LinearEasing)) }
            AsyncImage(
                model = slides[i].tile.backdrop,
                contentDescription = null,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxSize().graphicsLayer {
                    val z = 1.02f + 0.1f * drift.value
                    scaleX = z
                    scaleY = z
                    translationX = size.width * (0.01f - 0.03f * drift.value)
                    translationY = -size.height * 0.01f * drift.value
                },
            )
        }
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.5f to Color.Transparent, 1f to Color(0xC8000000))))
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(0f to Color(0x80000000), 0.55f to Color.Transparent)))
        Column(Modifier.align(Alignment.TopEnd).padding(top = 36.dp, end = Gutter + 8.dp), horizontalAlignment = Alignment.End) {
            Text(clockTime(clock), fontFamily = Type.display(), fontSize = 46.sp, fontWeight = FontWeight.SemiBold, color = Color.White)
            Text(java.text.SimpleDateFormat("EEEE, MMMM d", java.util.Locale.getDefault()).format(java.util.Date(clock)),
                fontSize = 13.sp, color = Color(0xB3FFFFFF))
        }
        AnimatedContent(
            targetState = slide,
            transitionSpec = {
                (fadeIn(tween(900, delayMillis = 500, easing = Settle)) + slideInVertically(tween(1000, delayMillis = 500, easing = Settle)) { it / 5 }) togetherWith
                    fadeOut(tween(600))
            },
            label = "saver-caption",
            modifier = Modifier.align(Alignment.BottomStart).padding(start = Gutter + 16.dp, bottom = 48.dp),
        ) { s ->
            val sx = rememberExtras(api, s.tile.kind, s.tile.tmdbId)
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                TitleMark(s.tile.title, sx?.logo, maxWidth = 420.dp, maxHeight = 130.dp, fontSize = 44.sp)
                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                    Box(Modifier.size(8.dp).clip(CircleShape).background(tint()))
                    Text(s.line, fontSize = 14.sp, fontWeight = FontWeight.SemiBold, color = Color.White)
                    if (s.detail.isNotEmpty()) Text(s.detail, fontSize = 14.sp, color = Color(0xD9FFFFFF))
                }
            }
        }
        Text("Press any button to come back", fontSize = 11.sp, color = Color(0x80FFFFFF),
            modifier = Modifier.align(Alignment.BottomEnd).padding(end = Gutter + 8.dp, bottom = 52.dp))
    }
}
