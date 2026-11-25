@file:OptIn(
    androidx.tv.material3.ExperimentalTvMaterial3Api::class,
    androidx.compose.ui.ExperimentalComposeUiApi::class,
    androidx.compose.foundation.ExperimentalFoundationApi::class,
)

package app.cue.tv.home

import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.focusGroup
import androidx.activity.compose.BackHandler
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.Crossfade
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.scaleOut
import androidx.compose.animation.scaleIn
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.togetherWith
import androidx.compose.ui.zIndex
import androidx.compose.animation.core.tween
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.animateFloat
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.runtime.key
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.graphics.luminance
import androidx.compose.ui.layout.ContentScale
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
import androidx.compose.foundation.lazy.itemsIndexed
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
import androidx.compose.ui.focus.focusProperties
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.foundation.gestures.LocalBringIntoViewSpec
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Border
import androidx.tv.material3.Button
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.Card
import androidx.tv.material3.CardDefaults
import androidx.tv.material3.Text
import coil.compose.AsyncImage
import coil.request.ImageRequest
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

// The app's own screens: signing in, the profile, then the rail down the
// top (Home, Shows, Movies, Live TV, Search, My List, the profile) and a
// title's page over them.

enum class Stage { Checking, SignIn, Profiles, Ready }

@Composable
fun CueRoot(api: CueApi, playback: Playback, openWeb: (String) -> Unit, changeServer: () -> Unit) {
    // Signed in on this TV with a profile chosen: straight in (Home opens on
    // what it showed last time), checking with the server meanwhile. Only a
    // first start, or a sign-in the server has dropped, waits on the logo.
    val signedInHere = remember { api.profileToken.isNotEmpty() && api.cookie.isNotEmpty() }
    var stage by remember { mutableStateOf(if (signedInHere) Stage.Ready else Stage.Checking) }
    // The logo's moment when going straight in (otherwise Checking shows it).
    var intro by remember { mutableStateOf(signedInHere) }
    LaunchedEffect(Unit) {
        if (!intro) return@LaunchedEffect
        delay(1300)
        intro = false
    }
    // Switch profile asked for: "Who's watching?" stays up even with one profile.
    var switching by remember { mutableStateOf(false) }
    val rootScope = rememberCoroutineScope()
    var problem by remember { mutableStateOf("") }
    // The profile's theme colours the whole app; it's read again whenever
    // Cue comes to the front, so a change made on the web shows here.
    remember { CueTheme.from(api); true }
    val accent = CueTheme.accent
    val tintState = remember { mutableStateOf<Color?>(null) }
    // The screensaver: after a few minutes without a button pressed (and
    // only while this screen is in front, not while something plays).
    var saver by remember { mutableStateOf<List<SaverSlide>?>(null) }
    var saverSpent by remember { mutableStateOf(false) } // ran its hour: not again until a button
    var lastInput by remember { mutableStateOf(System.currentTimeMillis()) }
    var inFront by remember { mutableStateOf(true) }
    val owner = androidx.compose.ui.platform.LocalLifecycleOwner.current
    androidx.compose.runtime.DisposableEffect(owner) {
        val watch = androidx.lifecycle.LifecycleEventObserver { _, e ->
            if (e == androidx.lifecycle.Lifecycle.Event.ON_RESUME) {
                inFront = true
                lastInput = System.currentTimeMillis()
                if (api.profileToken.isNotEmpty()) rootScope.launch { runCatching { api.refreshTheme() }.onSuccess { CueTheme.from(api) } }
            }
            if (e == androidx.lifecycle.Lifecycle.Event.ON_PAUSE) { inFront = false; saver = null }
        }
        owner.lifecycle.addObserver(watch)
        onDispose { owner.lifecycle.removeObserver(watch) }
    }
    LaunchedEffect(stage) {
        if (stage != Stage.Ready) return@LaunchedEffect
        while (true) {
            delay(10_000)
            if (saver == null && !saverSpent && inFront && System.currentTimeMillis() - lastInput > SAVER_IDLE_MS) {
                val s = runCatching { saverSlides(api.home()) }.getOrDefault(emptyList())
                if (s.size >= 2 && System.currentTimeMillis() - lastInput > SAVER_IDLE_MS) saver = s
            }
        }
    }
    // A newer build of this app: offered on opening and on coming back to it.
    val ctx = androidx.compose.ui.platform.LocalContext.current
    LaunchedEffect(stage, inFront) {
        if (stage == Stage.Ready && inFront) AppUpdate.check(ctx, api.server)
    }
    LaunchedEffect(stage) { if (stage == Stage.Ready) Tour.maybeShow(ctx) }
    LaunchedEffect(Unit) {
        if (!signedInHere) return@LaunchedEffect
        if (homeCache == null) homeCache = withContext(Dispatchers.IO) { api.savedHome() }
        // The server no longer knows this sign-in: back to signing in. (Not
        // reaching it isn't that: Home says so, over what it last showed.)
        val known = runCatching { api.signedIn() }.getOrDefault(true)
        if (!known) {
            api.forgetProfile()
            stage = Stage.SignIn
        }
    }
    LaunchedEffect(stage) {
        if (stage != Stage.Checking) return@LaunchedEffect
        problem = ""
        val began = System.currentTimeMillis()
        val next = try {
            when {
                !api.signedIn() -> Stage.SignIn
                api.profileToken.isEmpty() -> Stage.Profiles
                else -> Stage.Ready
            }
        } catch (e: Exception) {
            problem = e.message ?: "Couldn't reach Cue."
            Stage.SignIn
        }
        // The logo has its moment, rather than flashing past.
        delay((1100 - (System.currentTimeMillis() - began)).coerceAtLeast(0))
        stage = next
    }
    androidx.tv.material3.MaterialTheme(
        colorScheme = androidx.tv.material3.darkColorScheme(primary = accent, onPrimary = Color.Black, surface = Panel, background = Background),
    ) { CompositionLocalProvider(LocalAccent provides accent, LocalTint provides tintState, LocalApi provides api) { androidx.tv.material3.ProvideTextStyle(androidx.compose.ui.text.TextStyle(fontFamily = Type.text())) {
        Box(Modifier.fillMaxSize().background(Background).onPreviewKeyEvent {
            // Any button wakes the screen; the first one only ends the screensaver.
            lastInput = System.currentTimeMillis()
            saverSpent = false
            if (saver != null) {
                if (it.type == KeyEventType.KeyUp) saver = null
                true
            } else {
                false
            }
        }) {
            // Each stage fades into the next (the logo into sign-in or Home).
            Crossfade(targetState = stage, animationSpec = tween(500, easing = Settle), label = "stage") { shown ->
            when (shown) {
                Stage.Checking -> LaunchLogo()
                Stage.SignIn -> SignInScreen(api, problem, changeServer) { stage = Stage.Profiles }
                Stage.Profiles -> ProfilesScreen(api, pickAlone = !switching, signOut = {
                    rootScope.launch {
                        runCatching { api.signOut() }
                        switching = false
                        stage = Stage.SignIn
                    }
                }) {
                    switching = false
                    CueTheme.from(api)
                    stage = Stage.Ready
                }
                // The tour, once on a new TV (after the update offer), in
                // place of the screens so the remote only moves within it.
                Stage.Ready -> if (Tour.showing && !(AppUpdate.showing && AppUpdate.offer != null)) {
                    TourScreen { Tour.done(ctx) }
                } else MainScreen(api, playback, openWeb, changeServer) {
                    api.forgetProfile()
                    homeCache = null // the next profile's own Home, not this one's
                    switching = true
                    stage = Stage.Profiles
                }
            }
            }
            // Opening the app signed in: the logo still has its moment, over
            // Home as it loads underneath (so it costs no waiting), then fades.
            androidx.compose.animation.AnimatedVisibility(
                visible = intro,
                enter = androidx.compose.animation.EnterTransition.None,
                exit = androidx.compose.animation.fadeOut(tween(450, easing = Settle)),
            ) { Box(Modifier.fillMaxSize().background(Background)) { LaunchLogo() } }
            playback.upNext?.let { u -> UpNextScreen(api, u, play = { playback.playUpNext() }, dismiss = { playback.dismissUpNext() }) }
            playback.starting?.let { st -> StartingScreen(api, st, playback.note) { playback.cancelStarting() } }
            AppUpdate.offer?.takeIf { AppUpdate.showing && playback.upNext == null && stage == Stage.Ready }?.let { o ->
                UpdateCard(api, o, later = { AppUpdate.later() })
            }
            saver?.let { Screensaver(api, it) }
            // While Cue's screensaver shows, the TV stays awake (otherwise the
            // Fire TV's own screensaver takes over a minute or two later). After
            // an hour of it, Cue lets go, so the TV can still sleep.
            val view = androidx.compose.ui.platform.LocalView.current
            androidx.compose.runtime.DisposableEffect(saver != null) {
                view.keepScreenOn = saver != null
                onDispose { view.keepScreenOn = false }
            }
            LaunchedEffect(saver) {
                if (saver != null) {
                    delay(60 * 60_000L)
                    saverSpent = true
                    saver = null
                }
            }
            if (playback.note.isNotEmpty() && playback.starting == null) {
                Text(playback.note, fontSize = 14.sp, color = Color.White,
                    modifier = Modifier.align(Alignment.BottomCenter).padding(28.dp).clip(RoundedCornerShape(10.dp))
                        .background(Color(0xE6202228)).padding(horizontal = 20.dp, vertical = 12.dp))
                LaunchedEffect(playback.note) {
                    delay(6000)
                    playback.clearNote()
                }
            }
        }
    } } }
}

// The logo as Cue opens: the mark grows into place with a soft glow
// breathing behind it, and "cue" slides in beside it.
@Composable
fun LaunchLogo() {
    val mark = remember { androidx.compose.animation.core.Animatable(0f) }
    val word = remember { androidx.compose.animation.core.Animatable(0f) }
    LaunchedEffect(Unit) {
        launch { mark.animateTo(1f, tween(700, easing = Settle)) }
        delay(220)
        word.animateTo(1f, tween(600, easing = Settle))
    }
    val breathe = androidx.compose.animation.core.rememberInfiniteTransition(label = "glow")
    val glow by breathe.animateFloat(0.7f, 1f, androidx.compose.animation.core.infiniteRepeatable(tween(1400), androidx.compose.animation.core.RepeatMode.Reverse), label = "g")
    Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        Box(Modifier.size(360.dp).graphicsLayer { alpha = mark.value * glow; scaleX = 0.8f + 0.2f * glow; scaleY = 0.8f + 0.2f * glow }
            .background(Brush.radialGradient(listOf(Teal.copy(alpha = 0.32f), Color.Transparent))))
        Row(verticalAlignment = Alignment.CenterVertically) {
            androidx.compose.foundation.Image(
                androidx.compose.ui.res.painterResource(app.cue.tv.R.drawable.ic_cue_mark), contentDescription = "Cue",
                colorFilter = androidx.compose.ui.graphics.ColorFilter.tint(Teal),
                modifier = Modifier.size(84.dp).graphicsLayer {
                    alpha = mark.value
                    val sc = 0.7f + 0.3f * mark.value
                    scaleX = sc
                    scaleY = sc
                    rotationZ = -18f * (1f - mark.value)
                },
            )
            HGap(18.dp)
            Text("cue", fontFamily = Type.display(), fontSize = 64.sp, fontWeight = FontWeight.Bold, color = Color.White,
                modifier = Modifier.graphicsLayer { alpha = word.value; translationX = -24.dp.toPx() * (1f - word.value) })
        }
    }
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
    // Signing in from a phone: a code (and its QR code) that a signed-in
    // phone confirms; this screen asks every couple of seconds, and a new
    // code comes when one runs out.
    var link by remember { mutableStateOf<Pair<String, String>?>(null) }
    LaunchedEffect(Unit) {
        while (true) {
            val l = link ?: runCatching { api.tvLinkStart() }.getOrNull()
            if (l == null) {
                delay(10_000)
                continue
            }
            link = l
            delay(2000)
            try {
                if (api.tvLinkDone(l.second)) {
                    done()
                    return@LaunchedEffect
                }
            } catch (e: kotlinx.coroutines.CancellationException) {
                throw e
            } catch (e: Exception) {
                if (e is CueApi.Problem && e.code == 410) link = null // ran out: a new code
                else delay(5000)
            } catch (e: Exception) {
                delay(5000)
            }
        }
    }
    val host = api.server.removePrefix("https://").removePrefix("http://").trimEnd('/')
    Centered {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text("Cue", fontSize = 48.sp, fontWeight = FontWeight.Black, color = Teal)
            Text("Sign in to $host", color = Dim, fontSize = 14.sp)
            Gap(26.dp)
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(40.dp)) {
                // With a phone that's signed in: scan, tap, done.
                Column(
                    Modifier.width(300.dp).clip(RoundedCornerShape(PanelCorner))
                        .background(Brush.linearGradient(listOf(Color(0x2634D1BF), Color(0x0FFFFFFF))))
                        .border(1.dp, Color(0x1FFFFFFF), RoundedCornerShape(PanelCorner)).padding(22.dp),
                    horizontalAlignment = Alignment.CenterHorizontally,
                ) {
                    Text("Sign in with your phone", fontSize = 17.sp, fontWeight = FontWeight.Bold, color = Color.White)
                    Gap(14.dp)
                    val l = link
                    val qr = remember(l?.first) { l?.let { qrBitmap("${api.server.trimEnd('/')}/link/${it.first}", 360) } }
                    Box(Modifier.size(150.dp).clip(RoundedCornerShape(12.dp)).background(Color.White).padding(8.dp), contentAlignment = Alignment.Center) {
                        if (qr != null) androidx.compose.foundation.Image(qr.asImageBitmap(), contentDescription = "QR code to sign in from your phone", modifier = Modifier.fillMaxSize())
                    }
                    Gap(14.dp)
                    Text("Scan it with your phone's camera", fontSize = 12.sp, color = Dim)
                    Gap(10.dp)
                    Text("or go to $host/link and enter", fontSize = 12.sp, color = Dim)
                    Gap(4.dp)
                    Text(l?.first ?: "······", fontSize = 26.sp, fontWeight = FontWeight.Black, color = Teal, letterSpacing = 4.sp)
                }
                // Or the usual way.
                Column(horizontalAlignment = Alignment.CenterHorizontally, modifier = Modifier.width(360.dp)) {
                    Text("Or sign in here", fontSize = 15.sp, fontWeight = FontWeight.SemiBold, color = Color.White)
                    Gap(12.dp)
                    OutlinedTextField(user, { user = it }, label = { androidx.compose.material3.Text("Username") }, singleLine = true,
                        colors = fieldColors(), modifier = Modifier.fillMaxWidth().focusRequester(first).leaveWithArrows())
                    Gap(10.dp)
                    OutlinedTextField(password, { password = it }, label = { androidx.compose.material3.Text("Password") }, singleLine = true,
                        visualTransformation = PasswordVisualTransformation(), keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                        colors = fieldColors(), modifier = Modifier.fillMaxWidth().leaveWithArrows())
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
fun ProfilesScreen(api: CueApi, pickAlone: Boolean = true, signOut: () -> Unit = {}, done: () -> Unit) {
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
            // Only one profile: straight in, unless this is Switch profile.
            if (pickAlone && list.size == 1 && !list[0].hasPin) choose(list[0])
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
                                AvatarFace(p.avatar, p.name, 44.sp)
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
                        modifier = Modifier.width(180.dp).leaveWithArrows())
                    Button(onClick = { choose(p, pin) }) { Text("OK") }
                }
            }
            if (problem.isNotEmpty()) {
                Gap(16.dp)
                Text(problem, color = Color(0xFFFF8A80), fontSize = 13.sp)
            }
            if (list.isNotEmpty()) {
                Gap(32.dp)
                Pill("Sign out", onClick = signOut)
                Gap(6.dp)
                Text("To sign in with another account on this TV", fontSize = 11.sp, color = Dim)
            }
        }
    }
}

// The pictures a profile can wear on its color (the server's AvatarIcons),
// as emoji. An avatar is "teal" or "teal/fox".
internal val avatarEmoji = mapOf(
    "fox" to "🦊", "cat" to "🐱", "dog" to "🐶", "panda" to "🐼", "koala" to "🐨", "lion" to "🦁", "frog" to "🐸", "monkey" to "🐵",
    "unicorn" to "🦄", "octopus" to "🐙", "penguin" to "🐧", "owl" to "🦉", "rocket" to "🚀", "star" to "⭐", "game" to "🎮",
    "ball" to "⚽", "pizza" to "🍕", "alien" to "👾",
)

fun avatarPicture(avatar: String): String? = avatarEmoji[avatar.substringAfter('/', "")]

// A profile's face: its picture, or the first letter of its name.
@Composable
fun AvatarFace(avatar: String, name: String, size: androidx.compose.ui.unit.TextUnit, letterColor: Color = Color.White) {
    val pic = avatarPicture(avatar)
    if (pic != null) Text(pic, fontSize = size * 1.25f)
    else Text(name.take(1).uppercase().ifEmpty { "?" }, fontSize = size, fontWeight = FontWeight.Bold, color = letterColor)
}

fun avatarColor(avatar: String): Color = when (avatar.substringBefore('/')) {
    "red" -> Color(0xFFE5484D)
    "orange" -> Color(0xFFF76B15)
    "yellow" -> Color(0xFFE2A336)
    "green" -> Color(0xFF30A46C)
    "blue" -> Color(0xFF3E63DD)
    "purple" -> Color(0xFF8E4EC6)
    "pink" -> Color(0xFFD6409F)
    else -> Teal
}

// ---- The side rail ----

// Each place in the rail and the page it shows. Coming soon and the Top 10
// live in Browse now (pages 6 and 8 stay for it); the profile sits at the
// rail's foot.
private class NavItem(val name: String, val icon: Int, val page: Int)

// Kept short, as the TV apps do: Ask Cue opens from Search, Sports leads
// Live TV, and My List is in Browse.
private val navItems = listOf(
    NavItem("Search", app.cue.tv.R.drawable.ic_nav_search, 4),
    NavItem("Home", app.cue.tv.R.drawable.ic_nav_home, 0),
    NavItem("Movies", app.cue.tv.R.drawable.ic_nav_movies, 2),
    NavItem("Shows", app.cue.tv.R.drawable.ic_nav_shows, 1),
    NavItem("Live TV", app.cue.tv.R.drawable.ic_nav_live, 3),
    NavItem("Browse", app.cue.tv.R.drawable.ic_nav_browse, 9),
)
private const val ProfilePage = 10
private const val SettingsPage = 12
private const val PageCount = 13
private val RailWidth = 72.dp
private val RailOpenWidth = 212.dp // the rail's box, closed too (only its names fade in)

// The rail's place for a page: the places in navItems, then Settings, then
// the profile last.
private fun railSlot(page: Int): Int = navItems.indexOfFirst { it.page == page }.let {
    when {
        it >= 0 -> it
        page == SettingsPage -> navItems.size
        else -> navItems.size + 1
    }
}

@Composable
fun MainScreen(api: CueApi, playback: Playback, openWeb: (String) -> Unit, changeServer: () -> Unit, switchProfile: () -> Unit) {
    var tab by remember { mutableIntStateOf(0) }
    val open = remember { mutableStateListOf<Tile>() } // titles opened, one over another
    val memories = remember { List(PageCount) { FocusMemory() } }
    val titleMemory = remember { FocusMemory() }
    val browseState = remember { BrowseState() } // outlives a title opened over Browse
    // Ask Cue, opened from Search (Back returns to Search).
    var asking by remember { mutableStateOf(false) }
    var askedBack by remember { mutableStateOf(false) } // Back from Ask: the highlight on its button again
    val tabFocus = remember { List(navItems.size + 2) { FocusRequester() } }
    var tabsFocused by remember { mutableStateOf(false) }
    // Whether a page may take the highlight when it shows: on opening the
    // app and on coming back from a title, but not while the remote moves
    // along the rail (each place's page shows as the highlight passes it).
    var pageTakesFocus by remember { mutableStateOf(true) }
    BackHandler(enabled = open.isNotEmpty()) {
        open.removeAt(open.lastIndex)
        pageTakesFocus = true
    }
    // Back in a page goes over to the rail; on the rail, to Home; on Home's
    // tab it leaves the app.
    BackHandler(enabled = open.isEmpty() && !tabsFocused) { runCatching { tabFocus[railSlot(tab)].requestFocus() } }
    BackHandler(enabled = open.isEmpty() && tabsFocused && tab != 0) {
        tab = 0
        runCatching { tabFocus[railSlot(0)].requestFocus() }
    }
    // A card opens its page, Continue Watching and new episodes too: Resume
    // (or Play) is there, with the episodes, so nothing starts by surprise.
    val onCard: (Tile) -> Unit = { c -> open.add(c) }
    val tookFocus = { pageTakesFocus = false }
    // The remote's search (or mic) key: Search, listening straight away.
    var listenNow by remember { mutableStateOf(false) }
    // Home's highlight: its picture fills the screen behind the tabs and the
    // rows, fading into the page, as on a title's page.
    var hero by remember { mutableStateOf<Tile?>(null) }
    val setAmbient: (Tile?) -> Unit = { hero = it }

    var page by remember { mutableIntStateOf(0) }
    LaunchedEffect(tab) {
        if (page == tab) return@LaunchedEffect
        if (tabsFocused) delay(240)
        page = tab
    }

    // Opening a title zooms it in over Home; Back zooms it away again, Home
    // settling back into place. The fade is 320 ms, the short end of T6,
    // so a slow TV keeps up.
    AnimatedContent(
        targetState = open.lastOrNull(),
        contentKey = { it?.key ?: "home" },
        transitionSpec = {
            val opening = targetState != null && initialState == null
            val closing = targetState == null
            when {
                opening -> (fadeIn(tween(320, easing = Settle)) + scaleIn(tween(320, easing = Settle), initialScale = 1.05f)) togetherWith
                    (fadeOut(tween(160)) + scaleOut(tween(320, easing = Settle), targetScale = 0.96f))
                closing -> (fadeIn(tween(320, easing = Settle)) + scaleIn(tween(320, easing = Settle), initialScale = 0.96f)) togetherWith
                    (fadeOut(tween(160)) + scaleOut(tween(320, easing = Settle), targetScale = 1.05f))
                else -> fadeIn(tween(260, delayMillis = 60)) togetherWith fadeOut(tween(140))
            }
        },
        label = "screen",
    ) { t ->
    // A title opened from a title (More Like This) goes on top of it, so
    // Back returns to the one before (a few deep at most).
    if (t != null) TitleScreen(api, playback, t, titleMemory) { c ->
        open.add(c)
        if (open.size > 8) open.removeAt(0)
    }
    else Box(Modifier.fillMaxSize().onPreviewKeyEvent {
        if (it.type == KeyEventType.KeyDown && (it.key == androidx.compose.ui.input.key.Key.Search || it.key == androidx.compose.ui.input.key.Key.VoiceAssist)) {
            tab = 4
            asking = false
            listenNow = true
            true
        } else {
            false
        }
    }) {
        if (page <= 2) HomeBackdrop(api, hero) else AmbientBackground(null)
        // The pages sit right of the rail; the rail opens over them.
        Box(Modifier.fillMaxSize().padding(start = RailWidth)) {
            // The page follows the highlight once it rests on a tab (passing
            // over one doesn't build its page), and the old page has faded
            // before the new one shows, so two pages never overlap.
            AnimatedContent(
                targetState = page,
                transitionSpec = { fadeIn(tween(220, delayMillis = 90, easing = Settle)) togetherWith fadeOut(tween(90)) },
                label = "tab",
                modifier = Modifier.fillMaxSize(),
            ) { showing ->
                // Shown from the side menu: no rising into place (see rise).
                CompositionLocalProvider(LocalCalm provides tabsFocused) {
                when (showing) {
                    0 -> HomeScreen(api, null, memories[0], pageTakesFocus, tookFocus, setAmbient, onCard, playback)
                    1 -> HomeScreen(api, "tv", memories[1], pageTakesFocus, tookFocus, setAmbient, onCard)
                    2 -> HomeScreen(api, "movie", memories[2], pageTakesFocus, tookFocus, setAmbient, onCard)
                    3 -> LiveScreen(api, playback, openWeb)
                    4 -> if (asking) AskScreen(api, memories[11], onCard, takeFocus = true, onClose = { asking = false; askedBack = true })
                    else SearchScreen(api, memories[4], listen = listenNow, listened = { listenNow = false }, onAsk = { asking = true }, askFocus = askedBack, askFocused = { askedBack = false }, onCard = onCard)
                    5 -> MyListScreen(api, memories[5], pageTakesFocus, tookFocus, onCard)
                    6 -> ComingSoonScreen(api, onCard)
                    7 -> SportsScreen(api, playback)
                    8 -> TopScreen(api, memories[8], onCard)
                    9 -> BrowseScreen(api, memories[9], browseState, onCard)
                    11 -> AskScreen(api, memories[11], onCard)
                    SettingsPage -> SettingsScreen(api, switchProfile, changeServer, openWeb)
                    else -> ProfileScreen(api, switchProfile, changeServer, openWeb) {
                        tab = SettingsPage
                        runCatching { tabFocus[railSlot(SettingsPage)].requestFocus() }
                    }
                }
                }
            }
        }
        // The menu open: the page dims behind it, darkest at the menu, so
        // nothing on the page shows through or beside the names.
        val dim by animateFloatAsState(if (tabsFocused) 1f else 0f, tween(220), label = "rail-dim")
        // (Always there, read only when drawn: opening it recomposes nothing.)
        Box(Modifier.fillMaxSize().graphicsLayer { alpha = dim }
            .background(Brush.horizontalGradient(0f to Background, 0.2f to Background.copy(alpha = 0.94f), 0.32f to Background.copy(alpha = 0.78f), 0.6f to Background.copy(alpha = 0.35f), 0.9f to Color.Transparent)))
        SideRail(
            api = api,
            current = tab,
            focusers = tabFocus,
            expanded = tabsFocused,
            modifier = Modifier
                .align(Alignment.CenterStart)
                // Back from Ask Cue: the idea that had the highlight goes, and
                // the highlight would fall into the menu (its own place) before
                // Search can put it on Ask Cue's button. The menu turns it away
                // until it's there; moving onto the menu works again just after.
                .focusProperties { enter = { if (askedBack) FocusRequester.Cancel else tabFocus[railSlot(tab)] } }
                .focusGroup()
                .onFocusChanged {
                    tabsFocused = it.hasFocus
                    if (it.hasFocus) {
                        pageTakesFocus = false
                        askedBack = false // gone to the menu: Search mustn't pull the highlight back to Ask Cue
                    }
                },
            onFocusPage = { tab = it },
        )
    }
    }
}

// The rail: icons down the left, opening to show their names (and the
// screen dimming behind) when the remote comes onto it. The page showing is
// marked with a short bar in the profile's colour.
@Composable
private fun SideRail(api: CueApi, current: Int, focusers: List<FocusRequester>, expanded: Boolean, modifier: Modifier, onFocusPage: (Int) -> Unit) {
    // One width throughout: opening only fades the names in (changing the
    // width laid every item out again on every frame, which a slow TV
    // can't keep up with). Opens over 260 ms; closes in about two-thirds
    // of that (T7, S5).
    val railMs = if (expanded) 260 else 180
    val shade by animateFloatAsState(if (expanded) 1f else 0f, tween(railMs, easing = Settle), label = "rail-shade")
    Box(modifier.fillMaxHeight().width(RailOpenWidth)) {
        // Behind it: dark at the rail, fading out to the right.
        // (Open, the dimmed page behind takes over: no edge where the rail ends.)
        Box(Modifier.matchParentSize().graphicsLayer { alpha = 1f - shade }
            .background(Brush.horizontalGradient(listOf(Background.copy(alpha = 0.95f), Background.copy(alpha = 0.55f), Color.Transparent))))
        Column(Modifier.fillMaxHeight().padding(start = 10.dp, end = 10.dp, top = 20.dp, bottom = 18.dp)) {
            androidx.compose.foundation.Image(
                androidx.compose.ui.res.painterResource(app.cue.tv.R.drawable.ic_cue_mark), contentDescription = null,
                modifier = Modifier.padding(start = 14.dp).size(24.dp),
            )
            Gap(28.dp)
            navItems.forEachIndexed { i, n ->
                RailItem(n.name, selected = n.page == current, expanded = expanded, reveal = { shade }, modifier = Modifier.focusRequester(focusers[i]),
                    onFocus = { onFocusPage(n.page) }) {
                    androidx.tv.material3.Icon(androidx.compose.ui.res.painterResource(n.icon), contentDescription = null, modifier = Modifier.size(20.dp))
                }
                Gap(4.dp)
            }
            Spacer(Modifier.weight(1f))
            RailItem("Settings", selected = current == SettingsPage, expanded = expanded, reveal = { shade },
                modifier = Modifier.focusRequester(focusers[navItems.size]), onFocus = { onFocusPage(SettingsPage) }) {
                androidx.tv.material3.Icon(androidx.compose.ui.res.painterResource(app.cue.tv.R.drawable.ic_nav_settings), contentDescription = null, modifier = Modifier.size(20.dp))
            }
            Gap(4.dp)
            RailItem(api.profileName.ifEmpty { "Profile" }, selected = current == ProfilePage, expanded = expanded, reveal = { shade },
                modifier = Modifier.focusRequester(focusers[navItems.size + 1]), onFocus = { onFocusPage(ProfilePage) }) {
                Box(Modifier.size(22.dp).clip(CircleShape).background(LocalAccent.current), contentAlignment = Alignment.Center) {
                    AvatarFace(api.profileAvatar, api.profileName, 11.sp,
                        letterColor = if (LocalAccent.current.luminance() > 0.45f) Color.Black else Color.White)
                }
            }
        }
    }
}

@Composable
private fun RailItem(label: String, selected: Boolean, expanded: Boolean, reveal: () -> Float, modifier: Modifier, onFocus: () -> Unit, icon: @Composable () -> Unit) {
    val accent = LocalAccent.current
    Box(contentAlignment = Alignment.CenterStart) {
        Button(
            onClick = onFocus,
            // Closed, just the icon's width (moving left from a page finds
            // it); open, the full width, once, while the names fade in.
            modifier = modifier.then(if (expanded) Modifier.fillMaxWidth() else Modifier.width(RailWidth - 20.dp))
                .height(40.dp).onFocusChanged { if (it.isFocused) onFocus() },
            shape = ButtonDefaults.shape(RoundedCornerShape(10.dp)),
            scale = ButtonDefaults.scale(focusedScale = 1f, pressedScale = 0.98f),
            colors = ButtonDefaults.colors(
                containerColor = Color.Transparent,
                contentColor = if (selected) Color.White else Color(0x99FFFFFF),
                focusedContainerColor = Color.White,
                focusedContentColor = Color(0xFF0B0C0F),
                pressedContainerColor = Color(0xFFE6E7EA),
                pressedContentColor = Color(0xFF0B0C0F),
            ),
            contentPadding = PaddingValues(start = 14.dp, end = 10.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                icon()
                HGap(16.dp)
                // Read in the drawing step only: the fade redraws, nothing recomposes.
                Text(label, fontSize = 14.sp, fontWeight = if (selected) FontWeight.Bold else FontWeight.Medium, maxLines = 1,
                    modifier = Modifier.graphicsLayer { alpha = reveal() })
            }
        }
        // The page showing: a short bar at the rail's edge.
        if (selected) Box(Modifier.offset(x = (-8).dp).width(3.dp).height(18.dp).clip(RoundedCornerShape(2.dp)).background(accent))
    }
}

// ---- Home ----

// Home (and Shows and Movies), laid out like the TV apps: a rounded banner
// card at the top shows whatever the highlight is on (its picture, logo,
// scores and story), its picture glows behind the whole screen, and the
// rows slide up beneath it.
@Composable
fun HomeScreen(api: CueApi, only: String?, memory: FocusMemory, takeFocus: Boolean, tookFocus: () -> Unit,
               setAmbient: (Tile?) -> Unit, onCard: (Tile) -> Unit, playback: Playback? = null) {
    var home by remember { mutableStateOf<Home?>(homeCache) }
    var problem by remember { mutableStateOf("") }
    // Where the highlight is. Held outside composition on purpose: reading it
    // here would rebuild the whole page (every row and card) on each press
    // of the remote; only the banner follows it, once it settles.
    val spot = remember { mutableStateOf<Tile?>(null) }
    val listState = rememberLazyListState()
    LaunchedEffect(Unit) {
        // What this profile saw last time, at once; then the server's.
        if (home == null) withContext(Dispatchers.IO) { api.savedHome() }?.let { home = it; homeCache = it }
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
    val rows = h.rows.distinctBy { it.key }.mapNotNull { r ->
        val items = if (only == null) r.items else r.items.filter { it.kind == only }
        if (items.isEmpty()) null else Shelf(r.key, r.title, items)
    }
    val featured = h.hero?.takeIf { only == null || it.kind == only } ?: rows.flatMap { it.items }.firstOrNull { it.backdrop != null && !it.resumes }
    // The highlight settles before the banner follows it, so moving quickly
    // along a row doesn't redraw it for every poster passed.
    var settled by remember { mutableStateOf(featured) }
    LaunchedEffect(featured?.key) {
        androidx.compose.runtime.snapshotFlow { spot.value ?: featured }.collectLatest { t ->
            delay(300)
            settled = t
        }
    }
    LaunchedEffect(settled?.key) { setAmbient(settled) }
    // The page takes the colour of the picture in the banner.
    val settledExtras = rememberExtras(api, settled?.kind ?: "movie", settled?.tmdbId ?: 0, ask = settled != null)
    val tintState = LocalTint.current
    LaunchedEffect(settledExtras?.colour) { settledExtras?.colour?.let { tintState.value = it } }
    LaunchedEffect(rows.size, takeFocus) {
        if (!takeFocus || rows.isEmpty()) return@LaunchedEffect
        delay(80)
        if (memory.restore(rows.firstOrNull()?.let { r -> "${r.key}/${r.items.first().key}" })) tookFocus()
    }
    Column(Modifier.fillMaxSize().rise(0, 10.dp)) {
        settled?.let { Banner(api, it) } ?: Gap(16.dp)
        // The highlighted row moves up to just under the banner, and the
        // highlighted poster stays near the left, as in the TV's own apps.
        CompositionLocalProvider(LocalBringIntoViewSpec provides rememberRowsSpec()) {
            LazyColumn(state = listState, contentPadding = PaddingValues(top = 10.dp, bottom = 120.dp)) {
                if (only == null && playback != null) item(key = "parties") { PartiesNow(api, playback) }
                // The Movies and Shows tabs open with their own Top 10.
                if (only != null && h.top10.isNotEmpty()) item(key = "top10-first") { TopTenRow(h.top10, only, memory, onFocus = { spot.value = it }, onClick = onCard) }
                // The Top 10 comes after what you're watching (or the first row).
                val topAt = maxOf(0, rows.count { it.key == "continue" || it.key == "new-episodes" || it.key == "out-now" } - 1)
                rows.forEachIndexed { i, r ->
                    item(key = r.key) { CardRow(r, memory, onFocus = { spot.value = it }, onClick = onCard) }
                    if (only == null && i == topAt && h.top10.isNotEmpty()) item(key = "top10") { TopTenRow(h.top10, only, memory, onFocus = { spot.value = it }, onClick = onCard) }
                }
            }
        }
    }
}

// The banner: the highlighted title's logo, facts and story, sitting on its
// picture (HomeBackdrop, behind the whole screen) with no card around it.
@Composable
fun Banner(api: CueApi, c: Tile) {
    Box(Modifier.padding(horizontal = Gutter).padding(top = 8.dp).fillMaxWidth().height(226.dp)) {
        // The old words are gone before the new ones arrive, so two titles
        // never show at once however fast the remote moves.
        AnimatedContent(
            targetState = c,
            contentKey = { it.key },
            transitionSpec = {
                (fadeIn(tween(300, delayMillis = 130, easing = Settle)) + slideInVertically(tween(380, delayMillis = 130, easing = Settle)) { it / 10 }) togetherWith
                    fadeOut(tween(110))
            },
            label = "banner-words",
            modifier = Modifier.align(Alignment.BottomStart),
        ) { cc ->
            val logo = rememberLogo(api, cc.kind, cc.tmdbId, cc.logo?.takeIf { it.isNotEmpty() })
            Column(Modifier.padding(start = 2.dp, bottom = 18.dp).width(440.dp)) {
                TitleMark(cc.title, logo, maxWidth = 300.dp, maxHeight = 90.dp, fontSize = 32.sp)
                Gap(10.dp)
                FactsRow(cc, extra = listOfNotNull(if (cc.kind == "tv") "Series" else "Movie", epCode(cc.season, cc.episode)))
                // Away from a show a while: what happened last time instead.
                (cc.recap?.let { "Previously: $it" } ?: cc.overview)?.let {
                    Gap(6.dp)
                    Text(it, fontSize = 12.sp, color = Color(0xD9FFFFFF), maxLines = if (cc.recap != null) 3 else 2, overflow = TextOverflow.Ellipsis, lineHeight = 16.sp)
                }
            }
        }
    }
}

// HomeBackdrop: the highlighted title's picture across the top and right of
// the screen, behind the tabs and the rows, fading into the page to the left
// and below (no blur, which is costly on a TV). Rest on a title and its
// preview clip fades in over the picture, muted (not in Light mode).
@Composable
fun HomeBackdrop(api: CueApi, c: Tile?) {
    Box(Modifier.fillMaxSize().background(Background)) {
        val area = Modifier.align(Alignment.TopEnd).fillMaxWidth(0.82f).fillMaxHeight(0.8f)
        Crossfade(targetState = c?.backdrop, animationSpec = tween(450, easing = Settle), label = "home-backdrop", modifier = area) { u ->
            if (u != null) AsyncImage(model = u, contentDescription = null, contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize())
        }
        if (c != null) key(c.key) { BannerPreview(api, c, area) }
        // Fades: into the page on the left (where the words are), below
        // (where the rows are), and a little at the top for the tabs.
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(0.16f to Background, 0.45f to shade(0.70f), 0.72f to shade(0.00f))))
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0f to shade(0.60f), 0.16f to shade(0.00f), 0.42f to shade(0.00f), 0.74f to shade(0.95f), 0.85f to Background)))
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
    val logo = rememberLogo(api, c.kind, c.tmdbId, c.logo)
    // The page takes the colour of the title's picture.
    val titleExtras = rememberExtras(api, c.kind, c.tmdbId)
    val pageTint = LocalTint.current
    LaunchedEffect(titleExtras?.colour) { titleExtras?.colour?.let { pageTint.value = it } }
    Box(Modifier.fillMaxSize().background(Background).rise(0, 0.dp)) {
        // Rest here a moment and the picture comes alive. It plays under the
        // picture, which fades away once it's really playing.
        var live by remember(card.key) { mutableStateOf(false) }
        val still by animateFloatAsState(if (live) 0f else 1f, tween(900, easing = Settle), label = "title-still")
        // A moment of the episode or film itself, with its sound, from the stream Play
        // will use (so Play then starts at once). The TV's own player draws it,
        // so it's smooth, and there's no YouTube page in the way.
        key(c.kind, c.tmdbId) {
            ClipLayer(api, c.copy(season = t.resumeSeason, episode = t.resumeEpisode, progress = null), Modifier.fillMaxSize(), sound = true) { live = it }
        }
        // The picture fills the screen; the words sit on its darker left.
        AsyncImage(model = c.backdrop, contentDescription = null, contentScale = ContentScale.Crop,
            modifier = Modifier.fillMaxSize().graphicsLayer { alpha = still }.settleIn())
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(0f to shade(0.95f), 0.42f to shade(0.65f), 0.75f to shade(0.10f))))
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.45f to Color.Transparent, 1f to shade(0.95f))))
        CompositionLocalProvider(LocalBringIntoViewSpec provides GentleSpec) {
        // Back up at the buttons: the whole top again, title and all (only
        // bringing the buttons into view left the title above the screen).
        val pageState = androidx.compose.foundation.lazy.rememberLazyListState()
        LazyColumn(state = pageState, contentPadding = PaddingValues(bottom = 120.dp)) {
            item {
                Column(Modifier.padding(start = Gutter, top = 150.dp, bottom = 20.dp).width(520.dp)) {
                    Box(Modifier.rise(0, 24.dp)) { TitleMark(c.title, logo, maxWidth = 380.dp, maxHeight = 120.dp, fontSize = 40.sp) }
                    Gap(14.dp)
                    val length = t.runtime?.takeIf { it > 0 }?.let { if (it >= 60) "${it / 60}h ${it % 60}m" else "${it}m" }
                    Box(Modifier.rise(90)) {
                        FactsRow(c, t.ratings, listOfNotNull(t.certification, length, t.genres.firstOrNull(),
                            if (c.kind == "tv" && t.seasons.size > 0) (if (t.seasons.size == 1) "1 season" else "${t.seasons.size} seasons") else null))
                    }
                    c.overview?.let {
                        Gap(10.dp)
                        Text(it, fontSize = 12.sp, color = Color(0xD9FFFFFF), maxLines = 3, overflow = TextOverflow.Ellipsis, lineHeight = 17.sp,
                            modifier = Modifier.rise(170))
                    }
                    t.theaterNote?.let { note ->
                        Gap(8.dp)
                        Text("$note · usually a camera recording for now", fontSize = 11.sp, fontWeight = FontWeight.SemiBold, color = Color(0xFFF5C518))
                    }
                    if (t.directors.isNotEmpty()) {
                        Gap(8.dp)
                        Row(Modifier.rise(230)) {
                            Text(if (c.kind == "tv") "Created by  " else "Directed by  ", fontSize = 10.sp, color = Dim)
                            Text(t.directors.take(2).joinToString(", "), fontSize = 10.sp, color = Color(0xE6FFFFFF))
                        }
                    }
                    Gap(18.dp)
                    // Two buttons with words at most (Play, Watch on a service); the
                    // rest are icons, named under the row while highlighted.
                    var caption by remember(t) { mutableStateOf<String?>(null) }
                    val ctx = androidx.compose.ui.platform.LocalContext.current
                    var service by remember(c.key) { mutableStateOf<ServiceApp?>(null) }
                    LaunchedEffect(c.key) {
                        val p = runCatching { api.obj("/api/watch/providers/${c.kind}/${c.tmdbId}?region=US") }.getOrNull()
                        service = p?.let { ServiceApps.pick(ctx, it) }
                    }
                    Row(Modifier.rise(290).onFocusChanged { if (it.hasFocus) scope.launch { pageState.animateScrollToItem(0) } },
                        horizontalArrangement = Arrangement.spacedBy(10.dp), verticalAlignment = Alignment.CenterVertically) {
                        val resumeTv = c.kind == "tv" && t.resumeSeason != null && t.resumeEpisode != null
                        val started = (t.resumePosition ?: 0.0) > 0
                        val notOut = t.notOut
                        var reminding by remember(t) { mutableStateOf(t.reminding) }
                        if (notOut != null) {
                            // Not out yet: Remind me, and when it's coming.
                            Pill(if (reminding) "Reminding you" else "Remind me", primary = true,
                                icon = if (reminding) app.cue.tv.R.drawable.ic_cue_check else app.cue.tv.R.drawable.ic_cue_calendar,
                                modifier = Modifier.focusRequester(playFocus)) {
                                val next = !reminding
                                scope.launch { if (runCatching { api.setRemind(c.kind, c.tmdbId, next) }.isSuccess) reminding = next }
                            }
                        } else Pill(
                            if (started) "Resume" else "Play",
                            primary = true,
                            small = if (resumeTv) epCode(t.resumeSeason, t.resumeEpisode) else null,
                            icon = app.cue.tv.R.drawable.ic_cue_play,
                            modifier = Modifier.focusRequester(playFocus),
                        ) {
                            if (c.kind == "tv") {
                                val s = t.resumeSeason ?: season.takeIf { it > 0 } ?: 1
                                playback.playTitle("tv", c.tmdbId, s, t.resumeEpisode ?: 1)
                            } else {
                                playback.playTitle("movie", c.tmdbId, 0, 0)
                            }
                        }
                        // On a service this profile has: its own app, one press.
                        service?.let { sv ->
                            Pill("Watch on ${sv.name}") {
                                if (!ServiceApps.open(ctx, sv.key, c.title)) caption = "Couldn't open ${sv.name} on this TV."
                            }
                        }
                        val say: (String?) -> Unit = { caption = it }
                        if (t.theaterNote != null) IconAction(if (reminding) "Reminding you when it streams" else "Remind me when it streams",
                            icon = if (reminding) app.cue.tv.R.drawable.ic_cue_check else app.cue.tv.R.drawable.ic_cue_calendar, on = reminding, onFocus = say) {
                            val next = !reminding
                            scope.launch { if (runCatching { api.setRemind(c.kind, c.tmdbId, next) }.isSuccess) { reminding = next; caption = if (next) "Reminding you when it streams" else "Remind me when it streams" } }
                        }
                        IconAction(if (inList) "In My List" else "Add to My List",
                            icon = if (inList) app.cue.tv.R.drawable.ic_cue_check else app.cue.tv.R.drawable.ic_cue_add, on = inList, onFocus = say) {
                            scope.launch {
                                val on = !inList
                                if (runCatching { api.setInList(c.kind, c.tmdbId, on) }.isSuccess) { inList = on; caption = if (on) "In My List" else "Add to My List" }
                            }
                        }
                        // Thumbs: "Not for me" keeps it off Home.
                        var thumb by remember(t) { mutableStateOf(t.thumb) }
                        IconAction(if (thumb == "up") "Liked" else "Like", glyph = "👍", on = thumb == "up", onFocus = say) {
                            val next = if (thumb == "up") "" else "up"
                            scope.launch { if (runCatching { api.setThumb(c.kind, c.tmdbId, next) }.isSuccess) { thumb = next; caption = if (next == "up") "Liked" else "Like" } }
                        }
                        IconAction(if (thumb == "down") "Not for me (it stays off Home)" else "Not for me", glyph = "👎", on = thumb == "down", onFocus = say) {
                            val next = if (thumb == "down") "" else "down"
                            scope.launch { if (runCatching { api.setThumb(c.kind, c.tmdbId, next) }.isSuccess) { thumb = next; caption = if (next == "down") "Not for me (it stays off Home)" else "Not for me" } }
                        }
                        if (started || (c.kind == "tv" && t.resumeEpisode != null && (t.resumeEpisode != 1 || t.resumeSeason != t.seasons.firstOrNull()?.number))) {
                            var off by remember(t) { mutableStateOf(false) }
                            IconAction(if (off) "Hidden from Continue watching" else "Hide from Continue watching", glyph = "✕", on = off, onFocus = say) {
                                scope.launch { if (runCatching { api.hideFromContinue(c.kind, c.tmdbId) }.isSuccess) { off = true; caption = "Hidden from Continue watching" } }
                            }
                        }
                    }
                    // The highlighted icon's name (or when it's out, for a title not out yet).
                    Text(caption ?: t.notOut ?: "", fontSize = 11.sp, color = Dim, maxLines = 1, modifier = Modifier.padding(start = 4.dp, top = 8.dp).height(16.dp))
                }
            }
            if (c.kind == "tv" && t.seasons.isNotEmpty()) {
                item {
                    LazyRow(Modifier.keepRowFocus(), contentPadding = PaddingValues(horizontal = Gutter, vertical = 8.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        items(t.seasons.distinctBy { it.number }, key = { it.number }) { s ->
                            ChoiceTab(s.name, selected = s.number == season) { season = s.number }
                        }
                    }
                }
                item {
                    LazyRow(Modifier.keepRowFocus(), contentPadding = PaddingValues(horizontal = Gutter, vertical = 16.dp), horizontalArrangement = Arrangement.spacedBy(18.dp)) {
                        items(episodes.distinctBy { "${it.season}-${it.episode}" }, key = { "${it.season}-${it.episode}" }) { e ->
                            EpisodeCard(e) { playback.playTitle("tv", c.tmdbId, e.season, e.episode) }
                        }
                    }
                }
            }
            // The cast: picking someone shows what else they're in, just below.
            if (t.people.isNotEmpty()) {
                item {
                    var picked by remember(t) { mutableStateOf<CastPerson?>(null) }
                    var withThem by remember(t) { mutableStateOf<List<Tile>>(emptyList()) }
                    LaunchedEffect(picked?.id) {
                        val p = picked ?: return@LaunchedEffect
                        withThem = emptyList()
                        withThem = runCatching { api.personCredits(p.id) }.getOrDefault(emptyList()).filter { it.key != c.key }
                    }
                    Column {
                        Text("Cast", fontSize = 14.sp, fontWeight = FontWeight.SemiBold, color = Color(0xF2FFFFFF), modifier = Modifier.padding(start = Gutter + 2.dp, top = 6.dp))
                        LazyRow(Modifier.keepRowFocus(), contentPadding = PaddingValues(horizontal = Gutter, vertical = 14.dp), horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                            items(t.people.distinctBy { it.id }, key = { it.id }) { p -> PersonCard(p, selected = picked?.id == p.id) { picked = p } }
                        }
                        val p = picked
                        if (p != null && withThem.isNotEmpty()) {
                            CardRow(Shelf("person-${p.id}", "More with ${p.name}", withThem.take(24)), memory) { m -> openTitle(m) }
                        }
                    }
                }
            }
            // The film series it's part of, in order.
            if (t.series.size > 1) {
                item {
                    CardRow(Shelf("series", t.seriesName ?: "In this series", t.series), memory) { m -> openTitle(m) }
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
}

@Composable
fun EpisodeCard(e: Episode, onClick: () -> Unit) {
    val accent = LocalAccent.current
    var focused by remember { mutableStateOf(false) }
    val shape = RoundedCornerShape(CardCorner)
    Column(Modifier.width(220.dp).zIndex(if (focused) 1f else 0f)) {
        Card(
            onClick = onClick,
            modifier = Modifier.size(220.dp, 124.dp).lift(focused, 1.08f, shape).onFocusChanged { focused = it.isFocused },
            shape = CardDefaults.shape(shape),
            scale = CardDefaults.scale(focusedScale = 1f),
            border = CardDefaults.border(focusedBorder = Border(BorderStroke(1.dp, Color(0x59FFFFFF)), shape = shape)),
        ) {
            Box(Modifier.fillMaxSize()) {
                Art(sharpStill(e.still), Modifier.fillMaxSize(), e.title)
                Chip("E${e.episode}", Modifier.align(Alignment.BottomStart).padding(start = 8.dp, bottom = 10.dp))
                e.progress?.takeIf { it > 0 && !e.finished }?.let { p ->
                    Box(Modifier.align(Alignment.BottomStart).fillMaxWidth().height(3.dp).background(Color(0x55FFFFFF)))
                    Box(Modifier.align(Alignment.BottomStart).fillMaxWidth(p.toFloat().coerceIn(0f, 1f)).height(3.dp).background(accent))
                }
                if (e.finished) {
                    Text("✓", color = Color.Black, fontSize = 12.sp, fontWeight = FontWeight.Bold,
                        modifier = Modifier.align(Alignment.TopEnd).padding(6.dp).clip(CircleShape).background(accent).padding(horizontal = 6.dp))
                }
                Sheen(focused)
            }
        }
        Text("${e.episode}. ${e.title}", fontSize = 12.sp, fontWeight = FontWeight.SemiBold, color = Color.White, maxLines = 1,
            overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = 9.dp))
        e.overview?.let { Text(it, fontSize = 10.sp, color = Dim, maxLines = 2, overflow = TextOverflow.Ellipsis, lineHeight = 13.sp) }
    }
}

// ---- Search and My List ----

@Composable
fun MyListScreen(api: CueApi, memory: FocusMemory, takeFocus: Boolean, tookFocus: () -> Unit, onCard: (Tile) -> Unit) {
    var items by remember { mutableStateOf<List<Tile>?>(null) }
    LaunchedEffect(Unit) {
        items = runCatching { api.home().rows.firstOrNull { it.key == "my-list" }?.items ?: emptyList() }.getOrDefault(emptyList())
        delay(80)
        if (takeFocus) items?.firstOrNull()?.let { if (memory.restore("list/${it.key}")) tookFocus() }
    }
    val list = items
    when {
        list == null -> Centered { Text("Loading…", color = Dim) }
        list.isEmpty() -> Centered { Text("Nothing here yet. Add titles with My List on their page.", color = Dim) }
        else -> PosterGrid(list, memory, "list", onCard)
    }
}

@Composable
fun PosterGrid(items: List<Tile>, memory: FocusMemory, prefix: String, onCard: (Tile) -> Unit, sidePadding: Dp = Gutter) {
    LazyVerticalGrid(
        columns = GridCells.Adaptive(118.dp),
        contentPadding = PaddingValues(start = sidePadding, end = Gutter, top = 20.dp, bottom = 20.dp),
        horizontalArrangement = Arrangement.spacedBy(18.dp),
        verticalArrangement = Arrangement.spacedBy(22.dp),
    ) {
        items(items.distinctBy { it.key }, key = { it.key }) { c -> PosterCard(c, memory, "$prefix/${c.key}", onClick = onCard) }
    }
}

// ---- Live TV ----

// Live TV: the game that matters most at the top (LiveHero: both teams, a
// countdown or the score, Featured beside it), then the groups down the
// left and On now: a card per channel, two to a row. Moving down into the
// channels folds the hero away; back to the top row brings it again. A
// channel plays in the player, where up and down change channel. Without
// games, the highlighted channel shows at the top instead.
// The banner for the channel the highlight is on: reads it itself, so only
// the banner is drawn again as the highlight moves.
@Composable
private fun SpotBanner(api: CueApi, channel: () -> Channel?) {
    channel()?.let { LiveBanner(api, it) }
}

@Composable
fun LiveScreen(api: CueApi, playback: Playback, openWeb: (String) -> Unit) {
    var live by remember { mutableStateOf<LiveChannels?>(null) }
    var problem by remember { mutableStateOf("") }
    var group by remember { mutableStateOf("") } // "★" favourites, "*" every topic, else a topic's key
    var country by remember { mutableStateOf<String?>(null) } // "" is every country
    var picking by remember { mutableStateOf(false) } // the column lists the countries
    var find by remember { mutableStateOf("") } // a game no channel was matched to: channels showing it
    var board by remember { mutableStateOf(SportsBoard(null, emptyList(), emptyList())) }
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
    val real = remember(l) { l.channels.filter { !it.divider && !it.copy }.sortedWith(compareBy({ it.rank }, { if (it.num > 0) it.num else Int.MAX_VALUE })) }
    // A channel the list gives no country shows in every one.
    val here = remember(real, place) { if (place.isEmpty()) real else real.filter { it.country == place || it.country.isEmpty() } }
    val topicCounts = remember(here) { here.groupingBy { it.topic }.eachCount() }
    val favorites = remember(l) { l.channels.filter { it.favorite } }
    val shown = remember(l, here, group, find) {
        when {
            find.isNotEmpty() -> real.filter { it.name.contains(find, true) || it.now?.title?.contains(find, true) == true }
            group == "★" -> favorites
            group == "*" || group.isEmpty() -> here
            else -> here.filter { it.topic == group }
        }.distinctBy { it.id }.take(400)
    }
    val countryName = l.countries.firstOrNull { it.first == place }?.second ?: "Every country"
    // Back closes the country list, or the search for a game, first.
    BackHandler(enabled = picking) { picking = false }
    BackHandler(enabled = find.isNotEmpty() && !picking) { find = "" }
    val hasGames = board.big != null || board.now.isNotEmpty() || board.soon.isNotEmpty()
    // Where the highlight is, held outside composition: reading it here would
    // rebuild the whole screen on every press of the remote.
    val spot = remember { mutableStateOf<Channel?>(null) }
    val gridState = androidx.compose.foundation.lazy.rememberLazyListState()
    // The row of channels the highlight is on (-1: none, or the hero): the
    // hero shows while it's on the first row or above, so Up from there
    // reaches it. (Only crossing that line changes anything.)
    val focusRow = remember { mutableIntStateOf(-1) }
    val atTop by remember { androidx.compose.runtime.derivedStateOf { focusRow.intValue <= 0 } }
    Column(Modifier.fillMaxSize()) {
        androidx.compose.animation.AnimatedVisibility(
            visible = atTop,
            enter = androidx.compose.animation.expandVertically(tween(320, easing = Settle)) + fadeIn(tween(320)),
            exit = androidx.compose.animation.shrinkVertically(tween(280, easing = Settle)) + fadeOut(tween(200)),
        ) {
            Box(Modifier.padding(top = 14.dp)) {
                if (hasGames) LiveHero(board) { g ->
                    val ch = g.channelId?.let { id -> l.channels.firstOrNull { it.id == id } }
                    // No channel matched: every channel whose name or show has the team in it.
                    if (ch != null) playback.playLive(ch, shown) else find = g.home.name
                }
                else SpotBanner(api) { spot.value ?: shown.firstOrNull() }
            }
        }
        Row(Modifier.fillMaxSize().padding(top = 16.dp)) {
            LazyColumn(Modifier.width(214.dp), contentPadding = PaddingValues(start = Gutter, bottom = 60.dp), verticalArrangement = Arrangement.spacedBy(4.dp)) {
                item {
                    Text("CHANNELS", fontSize = 10.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.5.sp, color = Dim,
                        modifier = Modifier.padding(start = 6.dp, bottom = 6.dp))
                }
                if (picking) {
                    // The countries, most channels first; then every country.
                    items(l.countries + Triple("", "Every country", real.size), key = { "c-" + it.first }) { (code, name, n) ->
                        ChoiceTab("$name  ·  $n", selected = code == place, modifier = Modifier.fillMaxWidth()) {
                            country = code
                            api.liveCountry = code
                            picking = false
                            find = ""
                            if (group != "★") group = "*"
                        }
                    }
                } else {
                    if (favorites.isNotEmpty()) item(key = "★") {
                        ChoiceTab("Favorites  ·  ${favorites.size}", selected = group == "★" && find.isEmpty(), modifier = Modifier.fillMaxWidth()) { group = "★"; find = "" }
                    }
                    if (l.countries.isNotEmpty()) item(key = "country") {
                        ChoiceTab("$countryName  ›", selected = false, modifier = Modifier.fillMaxWidth()) { picking = true }
                    }
                    item(key = "*") {
                        ChoiceTab("All topics  ·  ${here.size}", selected = group == "*" && find.isEmpty(), modifier = Modifier.fillMaxWidth()) { group = "*"; find = "" }
                    }
                    items(l.topics.filter { (topicCounts[it.first] ?: 0) > 0 }, key = { "t-" + it.first }) { (key, name) ->
                        ChoiceTab("$name  ·  ${topicCounts[key] ?: 0}", selected = group == key && find.isEmpty(), modifier = Modifier.fillMaxWidth()) { group = key; find = "" }
                    }
                }
                item {
                    Gap(10.dp)
                    Pill("Full guide") { openWeb("/watch/live") }
                }
            }
            CompositionLocalProvider(LocalBringIntoViewSpec provides PivotSpec) {
                LazyColumn(Modifier.fillMaxSize(), state = gridState, contentPadding = PaddingValues(start = 18.dp, end = Gutter, top = 2.dp, bottom = 60.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    item(key = "head") {
                        Row(verticalAlignment = Alignment.Bottom) {
                            Text(if (find.isNotEmpty()) "Showing “$find”" else "On now", fontFamily = Type.display(), fontSize = 18.sp, fontWeight = FontWeight.Bold, color = Color.White)
                            HGap(10.dp)
                            Text(if (find.isNotEmpty()) "${shown.size} channels, every country" else "${shown.size} channels live", fontSize = 11.sp, color = Dim, modifier = Modifier.padding(bottom = 2.dp))
                        }
                    }
                    itemsIndexed(shown.chunked(2), key = { _, row -> row.first().id }) { i, row ->
                        Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                            row.forEach { ch ->
                                ChannelTile(api, ch, Modifier.weight(1f), onFocus = { spot.value = ch; focusRow.intValue = i }) { playback.playLive(ch, shown) }
                            }
                            if (row.size == 1) Box(Modifier.weight(1f))
                        }
                    }
                }
            }
        }
    }
}

// The highlighted channel at the top of Live TV: its logo, what's on now
// (with a red LIVE pill, the time and how far through), and what's next.
// The words fade and rise in as the highlight moves, as on Home.
@Composable
fun LiveBanner(api: CueApi, ch: Channel) {
    val shape = RoundedCornerShape(PanelCorner)
    Box(
        Modifier.padding(horizontal = Gutter).padding(top = 8.dp).fillMaxWidth().height(140.dp)
            .clip(shape).background(Color(0x991B1D22)).border(1.dp, Color(0x14FFFFFF), shape),
    ) {
        AnimatedContent(
            targetState = ch,
            contentKey = { it.id },
            transitionSpec = {
                (fadeIn(tween(380, delayMillis = 100, easing = Settle)) + slideInVertically(tween(480, delayMillis = 100, easing = Settle)) { it / 6 }) togetherWith
                    fadeOut(tween(150))
            },
            label = "live-banner",
        ) { c ->
            Row(Modifier.fillMaxSize().padding(horizontal = 26.dp), verticalAlignment = Alignment.CenterVertically) {
                // The channel's logo, then (once the remote rests) the channel itself, live.
                Box(Modifier.size(176.dp, 99.dp).clip(RoundedCornerShape(CardCorner)).background(Color(0x14FFFFFF)), contentAlignment = Alignment.Center) {
                    ChannelLogo(api, c, Modifier.fillMaxSize().padding(14.dp))
                    LivePreview(api, c, Modifier.fillMaxSize())
                }
                HGap(24.dp)
                Column(Modifier.weight(1f)) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Chip("LIVE", bg = Color(0xFFE5484D))
                        HGap(10.dp)
                        Text((if (c.num > 0) "${c.num}  " else "") + c.name, fontSize = 12.sp, color = Dim, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                    Gap(6.dp)
                    Text(c.now?.title ?: "No guide for this channel", fontFamily = Type.display(), fontSize = 22.sp, fontWeight = FontWeight.Bold,
                        color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    c.now?.let { now ->
                        Gap(8.dp)
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            Text("${clockTime(now.start)} – ${clockTime(now.stop)}", fontSize = 11.sp, color = Dim)
                            HGap(12.dp)
                            Box(Modifier.width(220.dp).height(3.dp).clip(RoundedCornerShape(2.dp)).background(Color(0x33FFFFFF))) {
                                Box(Modifier.fillMaxWidth(now.progress()).fillMaxHeight().background(Color(0xFFE5484D)))
                            }
                        }
                    }
                }
                c.next?.let { n ->
                    HGap(20.dp)
                    Column(Modifier.width(220.dp)) {
                        Text("Next · ${clockTime(n.start)}", fontSize = 10.sp, color = Dim)
                        Text(n.title, fontSize = 13.sp, color = Color(0xE6FFFFFF), maxLines = 2, overflow = TextOverflow.Ellipsis)
                    }
                }
            }
        }
    }
}

fun clockTime(ms: Long): String = java.text.DateFormat.getTimeInstance(java.text.DateFormat.SHORT).format(java.util.Date(ms))

// A channel's logo, fetched through Cue (it needs the sign-in), or its
// first letters when it has none.
@Composable
fun ChannelLogo(api: CueApi, ch: Channel, modifier: Modifier = Modifier) {
    Box(modifier, contentAlignment = Alignment.Center) {
        Text(ch.name.take(3).uppercase(), fontSize = 12.sp, fontWeight = FontWeight.Bold, color = Dim)
        ch.logo?.let { path ->
            AsyncImage(
                model = ImageRequest.Builder(androidx.compose.ui.platform.LocalContext.current)
                    .data(api.server + path)
                    .addHeader("Cookie", api.cookie)
                    .addHeader("X-Cue-Profile", api.profileToken)
                    .build(),
                contentDescription = null,
                modifier = Modifier.fillMaxSize(),
            )
        }
    }
}

// A channel: its logo, what's on now (and how far through) and next. It
// lifts and catches the light when highlighted, like the posters.
@Composable
fun ChannelCard(api: CueApi, ch: Channel, onFocus: () -> Unit = {}, onClick: () -> Unit) {
    var focused by remember { mutableStateOf(false) }
    val shape = RoundedCornerShape(CardCorner)
    Card(
        onClick = onClick,
        modifier = Modifier.fillMaxWidth().height(72.dp).zIndex(if (focused) 1f else 0f)
            .lift(focused, 1.025f, shape)
            .onFocusChanged {
                focused = it.isFocused
                if (it.isFocused) onFocus()
            },
        shape = CardDefaults.shape(shape),
        scale = CardDefaults.scale(focusedScale = 1f),
        colors = CardDefaults.colors(containerColor = Color(0x991B1D22), focusedContainerColor = Color(0xCC2A2D34)),
        border = CardDefaults.border(focusedBorder = Border(BorderStroke(1.dp, Color(0x59FFFFFF)), shape = shape)),
    ) {
        Box(Modifier.fillMaxSize()) {
            Row(Modifier.fillMaxSize().padding(horizontal = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                Box(Modifier.size(96.dp, 54.dp).clip(RoundedCornerShape(10.dp)).background(Color(0x14FFFFFF))) {
                    ChannelLogo(api, ch, Modifier.fillMaxSize().padding(6.dp))
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
                        Box(Modifier.padding(top = 5.dp).width(240.dp).height(3.dp).clip(RoundedCornerShape(2.dp)).background(Color(0x33FFFFFF))) {
                            Box(Modifier.fillMaxWidth(now.progress()).fillMaxHeight().background(Color(0xFFE5484D)))
                        }
                    }
                }
                ch.next?.let { n ->
                    Column(Modifier.width(220.dp)) {
                        Text("Next · ${clockTime(n.start)}", fontSize = 10.sp, color = Dim)
                        Text(n.title, fontSize = 12.sp, color = Dim, maxLines = 1, overflow = TextOverflow.Ellipsis)
                    }
                }
            }
            Sheen(focused)
        }
    }
}

// ---- The profile ----

@Composable
fun ProfileScreen(api: CueApi, switchProfile: () -> Unit, changeServer: () -> Unit, openWeb: (String) -> Unit, openSettings: () -> Unit = {}) {
    var managing by remember { mutableStateOf(false) }
    if (managing) {
        ManageProfilesScreen(api) { managing = false }
        return
    }
    val ctx = androidx.compose.ui.platform.LocalContext.current
    val light = LightMode.on(ctx)
    Centered {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Box(Modifier.size(96.dp).clip(CircleShape).background(LocalAccent.current), contentAlignment = Alignment.Center) {
                AvatarFace(api.profileAvatar, api.profileName, 40.sp)
            }
            Gap(12.dp)
            Text(api.profileName, fontSize = 20.sp, fontWeight = FontWeight.Bold, color = Color.White)
            Text(api.server, fontSize = 12.sp, color = Dim)
            Gap(20.dp)
            // Just the few things people come here for; the rest (profiles,
            // the server, Light mode, the tour, the web version) is in Settings.
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                if (AppUpdate.offer != null) Pill("Update Cue", primary = true) { AppUpdate.showing = true }
                Pill("Switch profile", primary = AppUpdate.offer == null, onClick = switchProfile)
                Pill("Settings", onClick = openSettings)
                Pill("Your year in Cue") { openWeb("/watch/year") }
            }
        }
    }
}

// Watch parties going on now (party.go): someone in the household started
// one; OK joins it, at the same moment, with nothing to type or open.
@Composable
fun PartiesNow(api: CueApi, playback: Playback) {
    var list by remember { mutableStateOf<List<org.json.JSONObject>>(emptyList()) }
    LaunchedEffect(Unit) {
        while (true) {
            val a = runCatching { api.arr("/api/parties") }.getOrNull()
            if (a != null) list = (0 until a.length()).mapNotNull { a.optJSONObject(it) }
            delay(20_000)
        }
    }
    if (list.isEmpty()) return
    Column(Modifier.padding(start = 48.dp, end = 48.dp, bottom = 14.dp), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        list.forEach { p ->
            val t = p.optJSONObject("title") ?: return@forEach
            val host = p.optJSONObject("host")?.optString("name") ?: "Someone"
            val what = t.optString("title") + if (t.optString("kind") == "tv" && t.optInt("season") > 0) " · S${t.optInt("season")}:E${t.optInt("episode")}" else ""
            val shape = RoundedCornerShape(16.dp)
            Card(
                onClick = {
                    playback.playTitle(t.optString("kind"), t.optInt("tmdbId"), t.optInt("season", 1), t.optInt("episode", 1), null, party = p.optString("code"))
                },
                modifier = Modifier.fillMaxWidth(),
                shape = CardDefaults.shape(shape),
                scale = CardDefaults.scale(focusedScale = 1.02f),
                colors = CardDefaults.colors(containerColor = tint().copy(alpha = 0.16f), focusedContainerColor = tint().copy(alpha = 0.3f)),
                border = CardDefaults.border(focusedBorder = Border(BorderStroke(2.dp, Color.White), shape = shape)),
            ) {
                Row(Modifier.padding(horizontal = 18.dp, vertical = 12.dp), verticalAlignment = Alignment.CenterVertically) {
                    val faces = p.optJSONArray("faces")
                    Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                        for (i in 0 until (faces?.length() ?: 0)) {
                            val f = faces?.optJSONObject(i) ?: continue
                            Box(Modifier.size(32.dp).clip(CircleShape).background(avatarColor(f.optString("avatar"))), contentAlignment = Alignment.Center) {
                                AvatarFace(f.optString("avatar"), f.optString("name"), 14.sp)
                            }
                        }
                    }
                    Column(Modifier.weight(1f).padding(start = 14.dp)) {
                        Text("● Watch party · ${p.optInt("members")} watching", fontSize = 12.sp, color = Dim)
                        Text("$host is watching $what", fontSize = 16.sp, fontWeight = FontWeight.SemiBold, color = Color.White, maxLines = 1)
                    }
                    Text("Join", fontSize = 15.sp, fontWeight = FontWeight.Bold, color = Color.White)
                }
            }
        }
    }
}

// ---- Coming soon ----

// Coming soon: films not out to stream yet, apart from everything you can
// watch now. Down the side the day it streams; then its picture, name,
// when, and what it's about, with Remind me (Cue says when it's out) and
// Info (its page).
@Composable
fun ComingSoonScreen(api: CueApi, onCard: (Tile) -> Unit) {
    var items by remember { mutableStateOf<List<Tile>?>(null) }
    // New episodes of the shows you watch, coming in the next two weeks:
    // a row at the top, each card saying the day.
    var week by remember { mutableStateOf<List<Tile>>(emptyList()) }
    val memory = remember { FocusMemory() }
    LaunchedEffect(Unit) { items = runCatching { api.comingSoon() }.getOrDefault(emptyList()) }
    LaunchedEffect(Unit) {
        week = runCatching { api.thisWeek() }.getOrDefault(emptyList()).map { (t, day) ->
            t.copy(episodeTitle = listOfNotNull(dayName(day), t.episodeTitle).joinToString(" · "))
        }
    }
    val list = items
    when {
        list == null -> Centered { Text("Loading…", color = Dim) }
        list.isEmpty() && week.isEmpty() -> Centered { Text("Nothing announced right now.", color = Dim) }
        else -> LazyColumn(
            contentPadding = PaddingValues(top = 18.dp, bottom = 40.dp),
            verticalArrangement = Arrangement.spacedBy(26.dp),
        ) {
            if (week.isNotEmpty()) item(key = "week") { CardRow(Shelf("new-episodes", "New episodes coming", week), memory, onClick = onCard) }
            items(list.distinctBy { it.key }, key = { it.key }) { c -> Box(Modifier.padding(horizontal = Gutter)) { ComingSoonItem(api, c, onCard) } }
        }
    }
}

@Composable
private fun ComingSoonItem(api: CueApi, c: Tile, onCard: (Tile) -> Unit) {
    val scope = rememberCoroutineScope()
    var reminding by remember(c.key) { mutableStateOf(c.reminding) }
    val months = listOf("JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC")
    val parts = c.outDate?.take(10)?.split('-')?.mapNotNull { it.toIntOrNull() }?.takeIf { it.size == 3 }
    Row(verticalAlignment = Alignment.Top) {
        Column(Modifier.width(54.dp).padding(top = 4.dp), horizontalAlignment = Alignment.CenterHorizontally) {
            if (parts != null) {
                Text(months[parts[1] - 1], fontSize = 11.sp, fontWeight = FontWeight.Bold, color = Dim, letterSpacing = 1.sp)
                Text("${parts[2]}", fontSize = 26.sp, fontWeight = FontWeight.Bold, color = Color.White)
            } else {
                Text("SOON", fontSize = 11.sp, fontWeight = FontWeight.Bold, color = Dim, letterSpacing = 1.sp)
            }
        }
        HGap(14.dp)
        Art(c.backdrop, Modifier.width(300.dp).height(169.dp).clip(RoundedCornerShape(10.dp)))
        HGap(20.dp)
        Column(Modifier.weight(1f)) {
            Text(c.title, fontFamily = Type.display(), fontSize = 20.sp, fontWeight = FontWeight.Bold, color = Color.White, maxLines = 2)
            Gap(6.dp)
            Text(c.outDate?.let { "Streaming ${shortDay(it)}" } ?: "Streaming date not announced yet", fontSize = 12.sp, fontWeight = FontWeight.SemiBold, color = Color.White)
            c.overview?.let {
                Gap(6.dp)
                Text(it, fontSize = 11.sp, color = Dim, maxLines = 3, overflow = TextOverflow.Ellipsis, lineHeight = 16.sp)
            }
            Gap(12.dp)
            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                Pill(if (reminding) "Reminding you" else "Remind me", primary = !reminding,
                    icon = if (reminding) app.cue.tv.R.drawable.ic_cue_check else app.cue.tv.R.drawable.ic_cue_calendar) {
                    val next = !reminding
                    scope.launch { if (runCatching { api.setRemind(c.kind, c.tmdbId, next) }.isSuccess) reminding = next }
                }
                Pill("Info") { onCard(c) }
            }
        }
    }
}

// A cast member: their photo, name and part. Selected, it's ringed in the
// accent colour (their titles show in the row below).
@Composable
fun PersonCard(p: CastPerson, selected: Boolean, onClick: () -> Unit) {
    val accent = LocalAccent.current
    var focused by remember { mutableStateOf(false) }
    val shape = RoundedCornerShape(14.dp)
    Column(Modifier.width(96.dp).zIndex(if (focused) 1f else 0f)) {
        Card(
            onClick = onClick,
            modifier = Modifier.size(96.dp, 116.dp).lift(focused, 1.08f, shape).onFocusChanged { focused = it.isFocused },
            shape = CardDefaults.shape(shape),
            scale = CardDefaults.scale(focusedScale = 1f),
            border = CardDefaults.border(
                border = if (selected) Border(BorderStroke(2.dp, accent), shape = shape) else Border.None,
                focusedBorder = Border(BorderStroke(if (selected) 2.dp else 1.dp, if (selected) accent else Color(0x59FFFFFF)), shape = shape),
            ),
        ) {
            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                if (p.photo != null) Art(p.photo, Modifier.fillMaxSize(), p.name)
                else Text(p.name.take(1), fontSize = 30.sp, fontWeight = FontWeight.Bold, color = Dim)
                Sheen(focused)
            }
        }
        Text(p.name, fontSize = 11.sp, fontWeight = FontWeight.SemiBold, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.padding(top = 8.dp))
        p.character?.let { Text(it, fontSize = 10.sp, color = Dim, maxLines = 1, overflow = TextOverflow.Ellipsis) }
    }
}

// The next big game at the top of Sports: both teams' logos, names, records
// and the score (live) or kickoff time, and the channel.
@Composable
fun BigGameCard(g: BigGame) {
    val shape = RoundedCornerShape(PanelCorner)
    Column(
        Modifier.fillMaxWidth().clip(shape).background(Color(0x991B1D22)).border(1.dp, Color(0x1FFFFFFF), shape).padding(horizontal = 20.dp, vertical = 14.dp),
    ) {
        Text(g.tag, fontSize = 10.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.5.sp, color = if (g.live) Color(0xFFFF5A6E) else LocalAccent.current)
        Gap(8.dp)
        if (g.away != null && g.home != null) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                GameSideView(g.away, Modifier.weight(1f))
                Text(if (g.live) "–" else "at", fontSize = 14.sp, fontWeight = FontWeight.Bold, color = Dim, modifier = Modifier.padding(horizontal = 14.dp))
                GameSideView(g.home, Modifier.weight(1f))
            }
        } else {
            Text(g.title, fontSize = 18.sp, fontWeight = FontWeight.Bold, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
        Gap(6.dp)
        Text(g.detail, fontSize = 12.sp, color = if (g.live) Color(0xFFFF5A6E) else Dim, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

@Composable
private fun GameSideView(s: GameSide, modifier: Modifier = Modifier) {
    Row(modifier, verticalAlignment = Alignment.CenterVertically) {
        Box(Modifier.size(44.dp), contentAlignment = Alignment.Center) {
            if (s.logo != null) coil.compose.AsyncImage(model = s.logo, contentDescription = s.name,
                contentScale = androidx.compose.ui.layout.ContentScale.Fit, modifier = Modifier.fillMaxSize())
            else Text(s.name.take(3).uppercase(), fontSize = 12.sp, fontWeight = FontWeight.Bold, color = Color.White)
        }
        HGap(10.dp)
        Column(Modifier.weight(1f)) {
            Text(s.name, fontSize = 15.sp, fontWeight = FontWeight.Bold, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
            if (s.record.isNotEmpty()) Text(s.record, fontSize = 10.sp, color = Dim)
        }
        if (s.score.isNotEmpty()) Text(s.score, fontSize = 22.sp, fontWeight = FontWeight.Black, color = Color.White, modifier = Modifier.padding(start = 8.dp))
    }
}

// ---- Sports ----

// Sports: the next big game on top, then games on now and coming up, with
// the teams' logos and the score or kickoff time (from ESPN). It needs no TV
// provider; with Live TV set up, picking a game plays the channel it's on.
@Composable
fun SportsScreen(api: CueApi, playback: Playback) {
    var board by remember { mutableStateOf<SportsBoard?>(null) }
    var channels by remember { mutableStateOf<List<Channel>>(emptyList()) }
    var note by remember { mutableStateOf("") }
    LaunchedEffect(Unit) {
        channels = runCatching { api.channels() }.getOrNull()?.takeIf { it.configured }?.channels ?: emptyList()
        while (true) {
            runCatching { api.sportsBoard() }.onSuccess { board = it }.onFailure { if (board == null) note = it.message ?: "Couldn't load the games." }
            delay(60_000)
        }
    }
    val b = board
    if (b == null) {
        Centered { Text(note.ifEmpty { "Loading the games…" }, color = Dim) }
        return
    }
    if (b.big == null && b.now.isEmpty() && b.soon.isEmpty()) {
        Centered { Text("No games on now or in the next day.", color = Dim) }
        return
    }
    val byId = remember(channels) { channels.associateBy { it.id } }
    val withGames = remember(b, channels) { (listOfNotNull(b.big) + b.now + b.soon).mapNotNull { g -> g.channelId?.let { byId[it] } }.distinctBy { it.id } }
    val watch: (GameRow) -> Unit = { g ->
        val ch = g.channelId?.let { byId[it] }
        if (ch != null) playback.playLive(ch, withGames.ifEmpty { listOf(ch) })
        else note = if (g.where.startsWith("On ")) "${g.away.name} at ${g.home.name} is ${g.where.lowercase().replaceFirstChar { it.uppercase() }}." else "This game isn't on any of your channels."
    }
    LazyColumn(contentPadding = PaddingValues(top = 18.dp, bottom = 60.dp), verticalArrangement = Arrangement.spacedBy(18.dp)) {
        b.big?.let { g ->
            item(key = "big") {
                Box(Modifier.padding(horizontal = Gutter)) {
                    Card(onClick = { watch(g) }, shape = CardDefaults.shape(RoundedCornerShape(PanelCorner)), scale = CardDefaults.scale(focusedScale = 1.02f),
                        colors = CardDefaults.colors(containerColor = Color.Transparent, focusedContainerColor = Color.Transparent),
                        border = CardDefaults.border(focusedBorder = Border(BorderStroke(2.dp, LocalAccent.current), shape = RoundedCornerShape(PanelCorner)))) {
                        BigGameCard(BigGame(
                            (if (g.live) "LIVE" else if (g.favorite) "YOUR TEAM" else "NEXT BIG GAME") + " · " + g.league,
                            g.away, g.home, "", listOf(g.detail, g.where).filter { it.isNotEmpty() }.joinToString(" · "), g.live))
                    }
                }
            }
        }
        if (note.isNotEmpty()) item(key = "note") { Text(note, color = Dim, fontSize = 12.sp, modifier = Modifier.padding(horizontal = Gutter)) }
        for ((label, list) in listOf("On now" to b.now, "Coming up" to b.soon)) {
            if (list.isEmpty()) continue
            item(key = label) {
                Column {
                    Text(label.uppercase(), fontSize = 11.sp, fontWeight = FontWeight.Bold, letterSpacing = 1.5.sp, color = Dim, modifier = Modifier.padding(start = Gutter + 2.dp))
                    LazyRow(Modifier.keepRowFocus(), contentPadding = PaddingValues(horizontal = Gutter, vertical = 12.dp), horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                        items(list.distinctBy { it.key }, key = { it.key }) { g -> GameCard(g) { watch(g) } }
                    }
                }
            }
        }
    }
}

@Composable
private fun GameCard(g: GameRow, onClick: () -> Unit) {
    val shape = RoundedCornerShape(CardCorner)
    val red = Color(0xFFFF5A6E)
    Card(
        onClick = onClick,
        modifier = Modifier.width(250.dp),
        shape = CardDefaults.shape(shape),
        scale = CardDefaults.scale(focusedScale = 1.05f),
        colors = CardDefaults.colors(containerColor = Color(0x991B1D22), focusedContainerColor = Color(0xCC2A2D33)),
        border = CardDefaults.border(
            border = Border(BorderStroke(1.dp, if (g.live) red.copy(alpha = 0.5f) else if (g.favorite) Color(0x8CFFD166) else Color(0x1FFFFFFF)), shape = shape),
            focusedBorder = Border(BorderStroke(2.dp, LocalAccent.current), shape = shape),
        ),
    ) {
        Column(Modifier.padding(14.dp)) {
            Text((if (g.favorite) "★ " else "") + g.league, fontSize = 10.sp, fontWeight = FontWeight.Bold, color = Dim, maxLines = 1, overflow = TextOverflow.Ellipsis)
            Gap(8.dp)
            for (side in listOf(g.away, g.home)) {
                Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(vertical = 3.dp)) {
                    Box(Modifier.size(26.dp), contentAlignment = Alignment.Center) {
                        if (side.logo != null) coil.compose.AsyncImage(model = side.logo, contentDescription = side.name,
                            contentScale = androidx.compose.ui.layout.ContentScale.Fit, modifier = Modifier.fillMaxSize())
                        else Text(side.name.take(3).uppercase(), fontSize = 9.sp, fontWeight = FontWeight.Bold, color = Color.White)
                    }
                    HGap(8.dp)
                    Text(side.name, fontSize = 13.sp, fontWeight = FontWeight.SemiBold, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f))
                    if (side.score.isNotEmpty()) Text(side.score, fontSize = 15.sp, fontWeight = FontWeight.Black, color = Color.White)
                }
            }
            Gap(6.dp)
            Text(g.detail, fontSize = 11.sp, color = if (g.live) red else Dim, maxLines = 1)
            if (g.where.isNotEmpty()) Text(g.where, fontSize = 10.sp, color = Dim, maxLines = 1, overflow = TextOverflow.Ellipsis)
        }
    }
}
