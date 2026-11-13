@file:OptIn(
    androidx.tv.material3.ExperimentalTvMaterial3Api::class,
    androidx.compose.ui.ExperimentalComposeUiApi::class,
    androidx.compose.foundation.ExperimentalFoundationApi::class,
)

package app.cue.tv.home

import androidx.activity.compose.BackHandler
import androidx.compose.ui.input.key.KeyEventType
import androidx.compose.ui.input.key.key
import androidx.compose.ui.input.key.onPreviewKeyEvent
import androidx.compose.ui.input.key.type
import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.Crossfade
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.togetherWith
import androidx.compose.ui.zIndex
import androidx.compose.animation.core.tween
import androidx.compose.animation.core.animateFloatAsState
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
import androidx.compose.ui.focus.focusRestorer
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
    // Switch profile asked for: "Who's watching?" stays up even with one profile.
    var switching by remember { mutableStateOf(false) }
    val rootScope = rememberCoroutineScope()
    var problem by remember { mutableStateOf("") }
    var accent by remember { mutableStateOf(accentOf(api.accent)) }
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
            if (e == androidx.lifecycle.Lifecycle.Event.ON_RESUME) { inFront = true; lastInput = System.currentTimeMillis() }
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
            when (stage) {
                Stage.Checking -> Centered { Text("Cue", fontSize = 48.sp, fontWeight = FontWeight.Black, color = Teal) }
                Stage.SignIn -> SignInScreen(api, problem, changeServer) { stage = Stage.Profiles }
                Stage.Profiles -> ProfilesScreen(api, pickAlone = !switching, signOut = {
                    rootScope.launch {
                        api.signOut()
                        switching = false
                        stage = Stage.SignIn
                    }
                }) {
                    switching = false
                    accent = accentOf(api.accent)
                    stage = Stage.Ready
                }
                // The tour, once on a new TV (after the update offer), in
                // place of the screens so the remote only moves within it.
                Stage.Ready -> if (Tour.showing && !(AppUpdate.showing && AppUpdate.offer != null)) {
                    TourScreen { Tour.done(ctx) }
                } else MainScreen(api, playback, openWeb, changeServer) {
                    api.forgetProfile()
                    switching = true
                    stage = Stage.Profiles
                }
            }
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
            } catch (e: CueApi.Problem) {
                if (e.code == 410) link = null // ran out: a new code
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
private val avatarEmoji = mapOf(
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

// ---- The tabs ----

private val tabs = listOf("Home", "Shows", "Movies", "Live TV", "Search", "My List")

@Composable
fun MainScreen(api: CueApi, playback: Playback, openWeb: (String) -> Unit, changeServer: () -> Unit, switchProfile: () -> Unit) {
    var tab by remember { mutableIntStateOf(0) }
    val open = remember { mutableStateListOf<Tile>() } // titles opened, one over another
    val memories = remember { List(tabs.size + 1) { FocusMemory() } }
    val titleMemory = remember { FocusMemory() }
    val tabFocus = remember { List(tabs.size + 1) { FocusRequester() } }
    var tabsFocused by remember { mutableStateOf(false) }
    // Whether a page may take the highlight when it shows: on opening the
    // app and on coming back from a title, but not while the remote moves
    // along the tabs (each tab's page shows as the highlight passes it).
    var pageTakesFocus by remember { mutableStateOf(true) }
    BackHandler(enabled = open.isNotEmpty()) {
        open.removeAt(open.lastIndex)
        pageTakesFocus = true
    }
    // Back in a page goes up to the tabs; on the tabs, to Home; on Home's
    // tab it leaves the app.
    BackHandler(enabled = open.isEmpty() && !tabsFocused) { runCatching { tabFocus[tab].requestFocus() } }
    BackHandler(enabled = open.isEmpty() && tabsFocused && tab != 0) {
        tab = 0
        runCatching { tabFocus[0].requestFocus() }
    }
    // A card opens its page, Continue Watching and new episodes too: Resume
    // (or Play) is there, with the episodes, so nothing starts by surprise.
    val onCard: (Tile) -> Unit = { c -> open.add(c) }
    val tookFocus = { pageTakesFocus = false }
    // The remote's search (or mic) key: Search, listening straight away.
    var listenNow by remember { mutableStateOf(false) }
    // The picture glowing behind the tabs and the page (Home's highlight).
    var ambient by remember { mutableStateOf<String?>(null) }
    val setAmbient: (String?) -> Unit = { ambient = it }

    var page by remember { mutableIntStateOf(0) }
    LaunchedEffect(tab) {
        if (page == tab) return@LaunchedEffect
        if (tabsFocused) delay(240)
        page = tab
    }

    if (open.isNotEmpty()) {
        val t = open.last()
        TitleScreen(api, playback, t, titleMemory) { c -> open.add(c) }
        return
    }
    Box(Modifier.fillMaxSize().onPreviewKeyEvent {
        if (it.type == KeyEventType.KeyDown && (it.key == androidx.compose.ui.input.key.Key.Search || it.key == androidx.compose.ui.input.key.Key.VoiceAssist)) {
            tab = 4
            listenNow = true
            true
        } else {
            false
        }
    }) {
        AmbientBackground(if (page <= 2) ambient else null)
        Column(Modifier.fillMaxSize()) {
            Row(Modifier.fillMaxWidth().padding(start = Gutter, end = Gutter, top = 12.dp, bottom = 2.dp), verticalAlignment = Alignment.CenterVertically) {
                androidx.compose.foundation.Image(
                    androidx.compose.ui.res.painterResource(app.cue.tv.R.drawable.ic_cue_mark), contentDescription = null,
                    modifier = Modifier.size(22.dp),
                )
                HGap(6.dp)
                Text("cue", fontFamily = Type.display(), fontSize = 20.sp, fontWeight = FontWeight.Bold, color = Color.White)
                HGap(22.dp)
                // Coming up to the tabs lands on the tab showing, not whichever
                // is nearest, so it doesn't change page by itself.
                TabRow(
                    selectedTabIndex = tab,
                    modifier = Modifier
                        .focusProperties { enter = { tabFocus[tab] } }
                        .onFocusChanged {
                            tabsFocused = it.hasFocus
                            if (it.hasFocus) pageTakesFocus = false
                        },
                ) {
                    tabs.forEachIndexed { i, name ->
                        Tab(selected = i == tab, onFocus = { tab = i }, modifier = Modifier.focusRequester(tabFocus[i])) {
                            Text(name, fontSize = 12.sp, fontWeight = FontWeight.Medium, modifier = Modifier.padding(horizontal = 13.dp, vertical = 5.dp))
                        }
                    }
                    Tab(selected = tab == tabs.size, onFocus = { tab = tabs.size }, modifier = Modifier.focusRequester(tabFocus[tabs.size])) {
                        Box(Modifier.padding(horizontal = 8.dp, vertical = 3.dp).size(22.dp).clip(CircleShape).background(LocalAccent.current),
                            contentAlignment = Alignment.Center) {
                            AvatarFace(api.profileAvatar, api.profileName, 11.sp,
                                letterColor = if (LocalAccent.current.luminance() > 0.45f) Color.Black else Color.White)
                        }
                    }
                }
            }
            // The page follows the highlight once it rests on a tab (passing
            // over one doesn't build its page), and the old page has faded
            // before the new one shows, so two pages never overlap.
            AnimatedContent(
                targetState = page,
                transitionSpec = { fadeIn(tween(220, delayMillis = 90, easing = Settle)) togetherWith fadeOut(tween(90)) },
                label = "tab",
                modifier = Modifier.fillMaxSize(),
            ) { showing ->
                when (showing) {
                    0 -> HomeScreen(api, null, memories[0], pageTakesFocus, tookFocus, setAmbient, onCard)
                    1 -> HomeScreen(api, "tv", memories[1], pageTakesFocus, tookFocus, setAmbient, onCard)
                    2 -> HomeScreen(api, "movie", memories[2], pageTakesFocus, tookFocus, setAmbient, onCard)
                    3 -> LiveScreen(api, playback, openWeb)
                    4 -> SearchScreen(api, memories[4], listen = listenNow, listened = { listenNow = false }, onCard = onCard)
                    5 -> MyListScreen(api, memories[5], pageTakesFocus, tookFocus, onCard)
                    else -> ProfileScreen(api, switchProfile, changeServer, openWeb)
                }
            }
        }
    }
}

// ---- Home ----

// Home (and Shows and Movies), laid out like the TV apps: a rounded banner
// card at the top shows whatever the highlight is on (its picture, logo,
// scores and story), its picture glows behind the whole screen, and the
// rows slide up beneath it.
@Composable
fun HomeScreen(api: CueApi, only: String?, memory: FocusMemory, takeFocus: Boolean, tookFocus: () -> Unit,
               setAmbient: (String?) -> Unit, onCard: (Tile) -> Unit) {
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
    // The highlight settles before the banner follows it, so moving quickly
    // along a row doesn't redraw it for every poster passed.
    var settled by remember { mutableStateOf(shown) }
    LaunchedEffect(shown?.key) {
        delay(300)
        settled = shown
    }
    LaunchedEffect(settled?.backdrop) { setAmbient(settled?.backdrop) }
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
        CompositionLocalProvider(LocalBringIntoViewSpec provides PivotSpec) {
            LazyColumn(state = listState, contentPadding = PaddingValues(top = 10.dp, bottom = 120.dp)) {
                items(rows, key = { it.key }) { r ->
                    CardRow(r, memory, onFocus = { spot = it }, onClick = onCard)
                }
            }
        }
    }
}

// The banner card: the picture to the right, fading into the card on the
// left under the logo, the scores and two lines of the story.
@Composable
fun Banner(api: CueApi, c: Tile) {
    val shape = RoundedCornerShape(PanelCorner)
    Box(
        Modifier.padding(horizontal = Gutter).padding(top = 8.dp).fillMaxWidth().height(226.dp)
            .clip(shape).background(Color(0xFF16171B)).border(1.dp, Color(0x14FFFFFF), shape),
    ) {
        // The picture changes with a slow cross-fade. Rest on a title and
        // its preview clip (a small file the server cut, light to decode)
        // fades in over it, muted; the story steps aside while it plays.
        var live by remember(c.key) { mutableStateOf(false) }
        Crossfade(targetState = c.backdrop, animationSpec = tween(450, easing = Settle), label = "banner",
            modifier = Modifier.fillMaxHeight().fillMaxWidth(0.8f).align(Alignment.TopEnd)) { u ->
            if (u != null) AsyncImage(model = u, contentDescription = null, contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize())
        }
        key(c.key) {
            BannerPreview(api, c, Modifier.fillMaxHeight().fillMaxWidth(0.8f).align(Alignment.TopEnd)) { live = it }
        }
        val story by animateFloatAsState(if (live) 0f else 1f, tween(600, easing = Settle), label = "banner-story")
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(0.18f to Color(0xF516171B), 0.48f to Color(0x9916171B), 0.75f to Color.Transparent)))
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.5f to Color.Transparent, 1f to Color(0xCC16171B))))
        // The words fade out, and the new ones fade in as they rise.
        AnimatedContent(
            targetState = c,
            contentKey = { it.key },
            // The old words are gone before the new ones arrive, so two titles
            // never show at once however fast the remote moves.
            transitionSpec = {
                (fadeIn(tween(300, delayMillis = 130, easing = Settle)) + slideInVertically(tween(380, delayMillis = 130, easing = Settle)) { it / 10 }) togetherWith
                    fadeOut(tween(110))
            },
            label = "banner-words",
            modifier = Modifier.align(Alignment.BottomStart),
        ) { cc ->
            val logo = rememberLogo(api, cc.kind, cc.tmdbId, cc.logo?.takeIf { it.isNotEmpty() })
            Column(Modifier.padding(start = 32.dp, bottom = 24.dp).width(400.dp)) {
                TitleMark(cc.title, logo, maxWidth = 280.dp, maxHeight = 78.dp, fontSize = 30.sp)
                Gap(10.dp)
                FactsRow(cc, extra = listOfNotNull(if (cc.kind == "tv") "Series" else "Movie", epCode(cc.season, cc.episode)))
                cc.overview?.let {
                    Gap(6.dp)
                    Text(it, fontSize = 11.sp, color = Color(0xCCFFFFFF), maxLines = 2, overflow = TextOverflow.Ellipsis, lineHeight = 15.sp,
                        modifier = Modifier.graphicsLayer { alpha = story })
                }
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
        Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(0f to Color(0xF20C0D10), 0.42f to Color(0xA60C0D10), 0.75f to Color(0x1A0C0D10))))
        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(0.45f to Color.Transparent, 1f to Color(0xF20C0D10))))
        CompositionLocalProvider(LocalBringIntoViewSpec provides GentleSpec) {
        LazyColumn(contentPadding = PaddingValues(bottom = 120.dp)) {
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
                    if (t.directors.isNotEmpty()) {
                        Gap(8.dp)
                        Row(Modifier.rise(230)) {
                            Text(if (c.kind == "tv") "Created by  " else "Directed by  ", fontSize = 10.sp, color = Dim)
                            Text(t.directors.take(2).joinToString(", "), fontSize = 10.sp, color = Color(0xE6FFFFFF))
                        }
                    }
                    Gap(18.dp)
                    Row(Modifier.rise(290), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        val resumeTv = c.kind == "tv" && t.resumeSeason != null && t.resumeEpisode != null
                        val started = (t.resumePosition ?: 0.0) > 0
                        Pill(
                            if (started) "▶  Resume" else "▶  Play",
                            primary = true,
                            small = if (resumeTv) epCode(t.resumeSeason, t.resumeEpisode) else null,
                            modifier = Modifier.focusRequester(playFocus),
                        ) {
                            if (c.kind == "tv") {
                                val s = t.resumeSeason ?: season.takeIf { it > 0 } ?: 1
                                playback.playTitle("tv", c.tmdbId, s, t.resumeEpisode ?: 1)
                            } else {
                                playback.playTitle("movie", c.tmdbId, 0, 0)
                            }
                        }
                        Pill(if (inList) "✓  My List" else "+  My List") {
                            scope.launch {
                                val on = !inList
                                if (runCatching { api.setInList(c.kind, c.tmdbId, on) }.isSuccess) inList = on
                            }
                        }
                    }
                }
            }
            if (c.kind == "tv" && t.seasons.isNotEmpty()) {
                item {
                    LazyRow(Modifier.focusRestorer(), contentPadding = PaddingValues(horizontal = Gutter, vertical = 8.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                        items(t.seasons, key = { it.number }) { s ->
                            Pill(s.name, primary = s.number == season) { season = s.number }
                        }
                    }
                }
                item {
                    LazyRow(Modifier.focusRestorer(), contentPadding = PaddingValues(horizontal = Gutter, vertical = 16.dp), horizontalArrangement = Arrangement.spacedBy(18.dp)) {
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
    // The highlighted channel, big at the top, as Home's banner is.
    var spot by remember { mutableStateOf<Channel?>(null) }
    val shownSpot = spot ?: shown.firstOrNull()
    Column(Modifier.fillMaxSize()) {
    shownSpot?.let { LiveBanner(api, it) }
    Row(Modifier.fillMaxSize().padding(top = 12.dp)) {
        LazyColumn(Modifier.width(208.dp), contentPadding = PaddingValues(start = Gutter, bottom = 60.dp), verticalArrangement = Arrangement.spacedBy(6.dp)) {
            item {
                Pill("Full guide") { openWeb("/watch/live") }
                Gap(8.dp)
            }
            items(groups, key = { it.first }) { (id, name) ->
                Pill(name, primary = id == group, modifier = Modifier.fillMaxWidth()) { group = id }
            }
        }
        CompositionLocalProvider(LocalBringIntoViewSpec provides PivotSpec) {
            LazyColumn(Modifier.fillMaxSize(), contentPadding = PaddingValues(start = 20.dp, end = Gutter, top = 6.dp, bottom = 60.dp),
                verticalArrangement = Arrangement.spacedBy(10.dp)) {
                items(shown.take(400), key = { it.id }) { ch ->
                    ChannelCard(api, ch, onFocus = { spot = ch }) { playback.playLive(ch, shown) }
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
fun ProfileScreen(api: CueApi, switchProfile: () -> Unit, changeServer: () -> Unit, openWeb: (String) -> Unit) {
    Centered {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Box(Modifier.size(96.dp).clip(CircleShape).background(LocalAccent.current), contentAlignment = Alignment.Center) {
                AvatarFace(api.profileAvatar, api.profileName, 40.sp)
            }
            Gap(12.dp)
            Text(api.profileName, fontSize = 20.sp, fontWeight = FontWeight.Bold, color = Color.White)
            Text(api.server, fontSize = 12.sp, color = Dim)
            Gap(20.dp)
            Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                if (AppUpdate.offer != null) Pill("Update Cue", primary = true) { AppUpdate.showing = true }
                Pill("Switch profile", primary = AppUpdate.offer == null, onClick = switchProfile)
                Pill("Your year in Cue") { openWeb("/watch/year") }
                Pill("How Cue works") { Tour.showing = true }
                Pill("Web version") { openWeb("/watch") }
                Pill("Change server", onClick = changeServer)
            }
        }
    }
}
