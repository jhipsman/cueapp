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
import androidx.compose.ui.focus.focusRequester
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

// ---- Live TV: the highlighted channel, live ----

// LivePreview plays the highlighted channel, muted, once the remote has
// rested on it a moment, fading in at its first picture (the logo shows
// until then). It stops whenever this screen isn't in front, so the full
// player never shares the TV's video decoder with it.
@androidx.annotation.OptIn(androidx.media3.common.util.UnstableApi::class)
@Composable
fun LivePreview(api: CueApi, ch: Channel, modifier: Modifier = Modifier) {
    val ctx = LocalContext.current
    if (!trailersFit(ctx)) return
    var url by remember(ch.id) { mutableStateOf<String?>(null) }
    var shown by remember(ch.id) { mutableStateOf(false) }
    LaunchedEffect(ch.id) {
        delay(1100)
        url = runCatching { api.livePlay(ch.id).optString("direct").ifEmpty { null } }.getOrNull()
    }
    val u = url ?: return
    val player = remember(u) {
        androidx.media3.exoplayer.ExoPlayer.Builder(ctx).build().apply {
            volume = 0f
            setMediaItem(androidx.media3.common.MediaItem.fromUri(u))
            addListener(object : androidx.media3.common.Player.Listener {
                override fun onRenderedFirstFrame() { shown = true }
            })
            prepare()
            playWhenReady = true
        }
    }
    val owner = androidx.compose.ui.platform.LocalLifecycleOwner.current
    androidx.compose.runtime.DisposableEffect(player, owner) {
        val watch = androidx.lifecycle.LifecycleEventObserver { _, e ->
            when (e) {
                androidx.lifecycle.Lifecycle.Event.ON_PAUSE -> { player.stop(); shown = false }
                androidx.lifecycle.Lifecycle.Event.ON_RESUME -> { player.prepare(); player.playWhenReady = true }
                else -> {}
            }
        }
        owner.lifecycle.addObserver(watch)
        onDispose {
            owner.lifecycle.removeObserver(watch)
            player.release()
        }
    }
    val alpha by animateFloatAsState(if (shown) 1f else 0f, tween(500), label = "live-preview")
    AndroidView(
        factory = { c ->
            androidx.media3.ui.PlayerView(c).apply {
                useController = false
                isFocusable = false
                resizeMode = androidx.media3.ui.AspectRatioFrameLayout.RESIZE_MODE_ZOOM
                setShutterBackgroundColor(android.graphics.Color.TRANSPARENT)
                this.player = player
            }
        },
        modifier = modifier.graphicsLayer { this.alpha = alpha },
    )
}

// ---- Up next ----

private const val UP_NEXT_SECONDS = 10

// UpNextScreen: an episode has ended. The next one fills the screen (its
// still, the show's logo, its name and story) and plays after a countdown
// that runs round the Play button; Back stays where you were.
@Composable
fun UpNextScreen(api: CueApi, next: Playback.UpNext, play: () -> Unit, dismiss: () -> Unit) {
    androidx.activity.compose.BackHandler { dismiss() }
    val ring = remember(next) { Animatable(0f) }
    LaunchedEffect(next) {
        ring.animateTo(1f, tween(UP_NEXT_SECONDS * 1000, easing = LinearEasing))
        play()
    }
    val ex = rememberExtras(api, "tv", next.showId)
    val ep = next.episode
    val focus = remember { androidx.compose.ui.focus.FocusRequester() }
    LaunchedEffect(Unit) { runCatching { focus.requestFocus() } }
    Box(Modifier.fillMaxSize().background(Color.Black)) {
        AsyncImage(
            model = ep.still?.replace("/w300/", "/w780/") ?: next.backdrop,
            contentDescription = null,
            contentScale = ContentScale.Crop,
            modifier = Modifier.fillMaxSize().settleIn(1.08f),
        )
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(0f to Color(0xF20C0D10), 0.5f to Color(0x990C0D10), 0.85f to Color.Transparent)))
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.55f to Color.Transparent, 1f to Color(0xE60C0D10))))
        Column(Modifier.align(Alignment.CenterStart).padding(start = Gutter + 8.dp).fillMaxSize(0.5f), verticalArrangement = Arrangement.Center) {
            Text("UP NEXT", fontSize = 12.sp, fontWeight = FontWeight.Bold, color = tint(), letterSpacing = 2.sp, modifier = Modifier.rise(0))
            androidx.compose.foundation.layout.Spacer(Modifier.size(12.dp))
            Box(Modifier.rise(80)) { TitleMark(next.show, ex?.logo, maxWidth = 360.dp, maxHeight = 110.dp, fontSize = 36.sp) }
            androidx.compose.foundation.layout.Spacer(Modifier.size(16.dp))
            Text("S${ep.season} · E${ep.episode}  ${ep.title}", fontSize = 18.sp, fontWeight = FontWeight.SemiBold, color = Color.White, modifier = Modifier.rise(160))
            ep.runtime?.takeIf { it > 0 }?.let { Text("$it min", fontSize = 12.sp, color = Dim, modifier = Modifier.padding(top = 4.dp).rise(200)) }
            ep.overview?.let {
                Text(it, fontSize = 13.sp, color = Color(0xD9FFFFFF), maxLines = 3, lineHeight = 19.sp,
                    modifier = Modifier.padding(top = 10.dp).rise(240))
            }
            androidx.compose.foundation.layout.Spacer(Modifier.size(22.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp), verticalAlignment = Alignment.CenterVertically, modifier = Modifier.rise(300)) {
                Box(contentAlignment = Alignment.Center) {
                    // The countdown, running round the button.
                    androidx.compose.foundation.Canvas(Modifier.size(58.dp)) {
                        drawArc(Color(0x33FFFFFF), 0f, 360f, false, style = androidx.compose.ui.graphics.drawscope.Stroke(4.dp.toPx()))
                        drawArc(Color.White, -90f, 360f * ring.value, false,
                            style = androidx.compose.ui.graphics.drawscope.Stroke(4.dp.toPx(), cap = androidx.compose.ui.graphics.StrokeCap.Round))
                    }
                }
                Pill("▶  Play now", primary = true, modifier = Modifier.focusRequester(focus), onClick = play)
                Pill("Back", onClick = dismiss)
            }
        }
    }
}
