package app.cue.tv.home

import android.content.Context
import android.os.Build
import androidx.compose.animation.core.tween
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.focusGroup
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Border
import androidx.tv.material3.Button
import androidx.tv.material3.ButtonDefaults
import androidx.tv.material3.Text
import app.cue.tv.BuildConfig
import app.cue.tv.CrashLog
import kotlinx.coroutines.launch
import java.text.DateFormat
import java.util.Date

// Settings, its own place in the side menu, laid out like Settings on the
// web: the sections down the left (the one the highlight rests on shows on
// the right), and right from there into its settings. Account and server,
// Playback, Problems (this TV's crashes, and every TV's when signed in as an
// administrator) and About and updates.

private enum class SettingsSection(val title: String, val summary: String) {
    Account("Account and server", "Profile, server, sign-in"),
    Playback("Playback", "Subtitles, sound, previews"),
    Problems("Problems", "Crashes on this TV"),
    About("About and updates", "Version, check for an update"),
}

@OptIn(androidx.compose.ui.ExperimentalComposeUiApi::class)
@Composable
fun SettingsScreen(api: CueApi, switchProfile: () -> Unit, changeServer: () -> Unit, openWeb: (String) -> Unit) {
    var section by remember { mutableStateOf(SettingsSection.Account) }
    val sectionFocus = remember { SettingsSection.entries.map { FocusRequester() } }
    var managing by remember { mutableStateOf(false) }
    if (managing) {
        ManageProfilesScreen(api) { managing = false }
        return
    }
    // Coming in from the side menu, the highlight lands on the section showing.
    Row(Modifier.fillMaxSize().padding(start = 96.dp, end = 48.dp, top = 40.dp, bottom = 32.dp)
        .focusProperties { enter = { sectionFocus[section.ordinal] } }.focusGroup()) {
        Column(Modifier.width(260.dp).fillMaxHeight().focusGroup()) {
            Text("Settings", fontSize = 26.sp, fontWeight = FontWeight.Bold, color = Color.White)
            Gap(20.dp)
            SettingsSection.entries.forEach { s ->
                SectionItem(s, selected = s == section, Modifier.focusRequester(sectionFocus[s.ordinal])) { section = s }
                Gap(6.dp)
            }
        }
        HGap(40.dp)
        // A section's panel fades in, rising a little, as the one before
        // fades out a touch quicker (motion rulebook S3, S5).
        val rise = with(androidx.compose.ui.platform.LocalDensity.current) { 12.dp.roundToPx() }
        androidx.compose.animation.AnimatedContent(
            targetState = section,
            transitionSpec = {
                (androidx.compose.animation.fadeIn(tween(240, delayMillis = 40, easing = Settle)) +
                    androidx.compose.animation.slideInVertically(tween(280, easing = Settle)) { rise }) togetherWith
                    androidx.compose.animation.fadeOut(tween(160, easing = Settle))
            },
            label = "settings-section",
            modifier = Modifier.weight(1f).fillMaxHeight(),
        ) { showing ->
            Box(Modifier.fillMaxSize()) {
                when (showing) {
                    SettingsSection.Account -> AccountSettings(api, switchProfile, changeServer, openWeb) { managing = true }
                    SettingsSection.Playback -> PlaybackSettings()
                    SettingsSection.Problems -> ProblemsSettings(api)
                    SettingsSection.About -> AboutSettings(api)
                }
            }
        }
    }
}

// A section in the list on the left: its name and a line on what's in it.
@Composable
private fun SectionItem(s: SettingsSection, selected: Boolean, modifier: Modifier, onFocus: () -> Unit) {
    val shape = RoundedCornerShape(12.dp)
    Button(
        onClick = onFocus,
        modifier = modifier.fillMaxWidth().onFocusChanged { if (it.isFocused) onFocus() },
        shape = ButtonDefaults.shape(shape),
        scale = ButtonDefaults.scale(focusedScale = 1.02f, pressedScale = 0.98f),
        colors = ButtonDefaults.colors(
            containerColor = if (selected) Color(0x1FFFFFFF) else Color.Transparent,
            contentColor = Color.White,
            focusedContainerColor = Color.White,
            focusedContentColor = Color(0xFF0B0C0F),
            pressedContainerColor = Color(0xFFE6E7EA),
            pressedContentColor = Color(0xFF0B0C0F),
        ),
        contentPadding = PaddingValues(horizontal = 16.dp, vertical = 10.dp),
    ) {
        Column {
            Text(s.title, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, maxLines = 1)
            Text(s.summary, fontSize = 11.sp, color = androidx.tv.material3.LocalContentColor.current.copy(alpha = 0.65f), maxLines = 1)
        }
    }
}

// One setting: its name and a line about it on the left, its value on the
// right. OK changes it (to the next choice, or opens what it names).
@Composable
private fun SettingRow(title: String, detail: String, value: String, onClick: () -> Unit) {
    val shape = RoundedCornerShape(12.dp)
    Button(
        onClick = onClick,
        modifier = Modifier.fillMaxWidth(),
        shape = ButtonDefaults.shape(shape),
        scale = ButtonDefaults.scale(focusedScale = 1.02f, pressedScale = 0.98f),
        colors = ButtonDefaults.colors(
            containerColor = Color(0x14FFFFFF),
            contentColor = Color.White,
            focusedContainerColor = Color.White,
            focusedContentColor = Color(0xFF0B0C0F),
            pressedContainerColor = Color(0xFFE6E7EA),
            pressedContentColor = Color(0xFF0B0C0F),
        ),
        border = ButtonDefaults.border(
            border = Border(BorderStroke(1.dp, Color(0x14FFFFFF)), shape = shape),
            focusedBorder = Border(BorderStroke(0.dp, Color.Transparent), shape = shape),
        ),
        contentPadding = PaddingValues(horizontal = 18.dp, vertical = 12.dp),
    ) {
        Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
            Column(Modifier.weight(1f)) {
                Text(title, fontSize = 15.sp, fontWeight = FontWeight.SemiBold, maxLines = 1)
                if (detail.isNotEmpty()) Text(detail, fontSize = 12.sp, lineHeight = 16.sp,
                    color = androidx.tv.material3.LocalContentColor.current.copy(alpha = 0.7f), maxLines = 2)
            }
            HGap(16.dp)
            Text(value, fontSize = 14.sp, fontWeight = FontWeight.Bold, maxLines = 1)
        }
    }
}

@Composable
private fun Heading(text: String) {
    Text(text, fontSize = 13.sp, fontWeight = FontWeight.Bold, color = Dim, letterSpacing = 0.6.sp)
    Gap(8.dp)
}

// ---- Account and server ----

@Composable
private fun AccountSettings(api: CueApi, switchProfile: () -> Unit, changeServer: () -> Unit, openWeb: (String) -> Unit, manage: () -> Unit) {
    Column(Modifier.focusGroup()) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(Modifier.size(64.dp).clip(CircleShape).background(LocalAccent.current), contentAlignment = Alignment.Center) {
                AvatarFace(api.profileAvatar, api.profileName, 26.sp)
            }
            HGap(16.dp)
            Column {
                Text(api.profileName.ifEmpty { "Profile" }, fontSize = 20.sp, fontWeight = FontWeight.Bold, color = Color.White)
                Text(api.server, fontSize = 12.sp, color = Dim)
            }
        }
        Gap(20.dp)
        Column(verticalArrangement = Arrangement.spacedBy(8.dp)) {
            SettingRow("Switch profile", "Watch as someone else in the household.", "", switchProfile)
            SettingRow("Manage profiles", "Add, rename or remove profiles, and set PINs.", "", manage)
            SettingRow("Change server", "Point this TV at another Cue.", "", changeServer)
            SettingRow("Web version", "Cue's website, here on the TV.", "", { openWeb("/watch") })
        }
    }
}

// ---- Playback ----

// The choices kept on this TV: subtitles (the player keeps the last one
// picked there too), surround sound, and Light mode.
private val subtitleChoices = listOf("" to "As in Watch", "en" to "English", "off" to "Off")

@Composable
private fun PlaybackSettings() {
    val ctx = LocalContext.current
    val player = remember { ctx.getSharedPreferences("player", Context.MODE_PRIVATE) }
    var subs by remember { mutableStateOf(player.getString("subs", "") ?: "") }
    var surround by remember { mutableStateOf(player.getBoolean("surround", true)) }
    var light by remember { mutableStateOf(LightMode.on(ctx)) }
    Column(Modifier.focusGroup(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Heading("PLAYBACK ON THIS TV")
        SettingRow(
            "Subtitles",
            "What a new episode or film starts with. Changing them while watching changes this too.",
            subtitleChoices.firstOrNull { it.first == subs }?.second ?: subs.uppercase(),
        ) {
            val i = subtitleChoices.indexOfFirst { it.first == subs }
            subs = subtitleChoices[(i + 1) % subtitleChoices.size].first
            player.edit().putString("subs", subs).apply()
        }
        SettingRow(
            "Surround sound",
            "5.1 sound where the film has it. Turn off if the sound drops out on this TV.",
            if (surround) "On" else "Off",
        ) {
            surround = !surround
            player.edit().putBoolean("surround", surround).apply()
        }
        SettingRow(
            "Light mode",
            if (light) "Still pictures instead of moving previews, for a smoother TV." else "Moving previews on. Turn on if this TV feels slow.",
            if (light) "On" else "Off",
        ) {
            light = !light
            LightMode.set(ctx, light)
        }
    }
}

// ---- Problems ----

@Composable
private fun ProblemsSettings(api: CueApi) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var refresh by remember { mutableIntStateOf(0) }
    val here = remember(refresh) { CrashLog.history(ctx) }
    val waiting = remember(refresh) { CrashLog.pending(ctx) }
    var sending by remember { mutableStateOf(false) }
    var note by remember { mutableStateOf("") }
    // Every TV's crashes as Cue keeps them: only an administrator may see them.
    var reported by remember { mutableStateOf<List<org.json.JSONObject>?>(null) }
    LaunchedEffect(refresh) {
        reported = runCatching {
            val items = api.obj("/api/system/crashes").optJSONArray("items") ?: org.json.JSONArray()
            (0 until items.length()).mapNotNull { items.optJSONObject(it) }
                .filter { it.optString("app") == "tv" && it.optString("fixedAt").isEmpty() }
        }.getOrNull()
    }
    val qr = remember(api.server) { qrBitmap(api.server.trimEnd('/') + "/settings/problems", 220) }

    LazyColumn(verticalArrangement = Arrangement.spacedBy(8.dp), contentPadding = PaddingValues(bottom = 24.dp)) {
        item {
            Heading("ON THIS TV")
            Text(
                when {
                    here.isEmpty() && !waiting -> "No crashes on this TV."
                    waiting -> "A crash is waiting to be sent to Cue."
                    else -> "The last ${if (here.size == 1) "crash" else "${here.size} crashes"} on this TV. Cue has ${if (here.size == 1) "it" else "them"}."
                },
                fontSize = 14.sp, color = Color.White,
            )
            Gap(6.dp)
        }
        items(here.size) { i ->
            val e = here[i]
            SettingRow(
                e.headline.let(::shortHeadline).take(120),
                "${DateFormat.getDateTimeInstance(DateFormat.MEDIUM, DateFormat.SHORT).format(Date(e.at))} · version ${e.version}",
                if (e.sent) "Sent" else "Waiting",
            ) {}
        }
        if (waiting) item {
            SettingRow("Send now", note.ifEmpty { "Send the waiting crash to Cue, so it shows under Problems on the web." },
                if (sending) "Sending…" else "") {
                if (sending) return@SettingRow
                sending = true
                scope.launch {
                    val ok = CrashLog.sendSaved(ctx, api)
                    note = if (ok) "" else "Couldn't reach Cue. It's sent next time the app opens."
                    sending = false
                    refresh++
                }
            }
        }
        if (here.isNotEmpty()) item {
            SettingRow("Clear this list", "Forget the crashes listed here. Cue keeps its own copy.", "") {
                CrashLog.clearHistory(ctx)
                refresh++
            }
        }
        reported?.let { list ->
            item {
                Gap(14.dp)
                Heading("REPORTED TO CUE FROM EVERY TV")
                Text(if (list.isEmpty()) "No open crashes from the TV app." else "${list.size} open, newest first.", fontSize = 14.sp, color = Color.White)
                Gap(6.dp)
            }
            items(minOf(list.size, 5)) { i ->
                val c = list[i]
                val n = c.optInt("count", 1)
                SettingRow(
                    c.optString("headline").let(::shortHeadline).take(120),
                    listOf(c.optString("device"), c.optString("version").takeIf { it.isNotEmpty() }?.let { "version $it" }).filterNotNull().filter { it.isNotEmpty() }.joinToString(" · "),
                    if (n == 1) "Once" else "$n times",
                ) {}
            }
        }
        item {
            Gap(14.dp)
            Row(verticalAlignment = Alignment.CenterVertically) {
                qr?.let {
                    Box(Modifier.size(112.dp).clip(RoundedCornerShape(8.dp)).background(Color.White).padding(8.dp)) {
                        Image(it.asImageBitmap(), contentDescription = null, modifier = Modifier.fillMaxSize())
                    }
                    HGap(18.dp)
                }
                Column(Modifier.weight(1f)) {
                    Text("On your phone", fontSize = 15.sp, fontWeight = FontWeight.Bold, color = Color.White)
                    Gap(4.dp)
                    Text("Scan to open Problems in Cue's Settings: every crash with its stack trace, and Copy for Claude to get it fixed.",
                        fontSize = 12.sp, color = Dim, lineHeight = 16.sp)
                }
            }
        }
    }
}

// ---- About and updates ----

@Composable
private fun AboutSettings(api: CueApi) {
    val ctx = LocalContext.current
    val scope = rememberCoroutineScope()
    var serverVersion by remember { mutableStateOf("") }
    var checking by remember { mutableStateOf(false) }
    var result by remember { mutableStateOf("") }
    LaunchedEffect(Unit) {
        serverVersion = runCatching { api.obj("/api/version").optString("version") }.getOrDefault("")
    }
    val build = remember { AppUpdate.current(ctx) }
    Column(Modifier.focusGroup(), verticalArrangement = Arrangement.spacedBy(8.dp)) {
        Heading("ABOUT")
        Info("Cue for TV", "Version ${BuildConfig.VERSION_NAME} (build $build)")
        Info("Cue server", listOf(serverVersion.takeIf { it.isNotEmpty() }?.let { "Version $it" }, api.server).filterNotNull().joinToString(" · "))
        Info("This TV", "${Build.MANUFACTURER} ${Build.MODEL} · Android ${Build.VERSION.RELEASE}")
        Gap(10.dp)
        Heading("UPDATES")
        val offer = AppUpdate.offer
        if (offer != null) {
            SettingRow("Update Cue", "Build ${offer.build} is out. It installs over this one; nothing is lost.", "Install") { AppUpdate.showing = true }
        } else {
            SettingRow("Check for an update", result.ifEmpty { "Cue checks by itself when it opens." }, if (checking) "Checking…" else "") {
                if (checking) return@SettingRow
                checking = true
                scope.launch {
                    val newest = AppUpdate.latest(api.server)
                    result = when {
                        newest == null -> "Couldn't ask Cue for the newest build. Try again in a moment."
                        newest > build -> { AppUpdate.offer = UpdateOffer(newest); AppUpdate.showing = true; "" }
                        else -> "This is the newest build."
                    }
                    checking = false
                }
            }
        }
        SettingRow("How Cue works", "The short tour of the remote and the side menu.", "") { Tour.showing = true }
    }
}

@Composable
private fun Info(name: String, value: String) {
    Row(Modifier.fillMaxWidth().padding(vertical = 2.dp)) {
        Text(name, fontSize = 14.sp, color = Dim, modifier = Modifier.width(130.dp))
        Text(value, fontSize = 14.sp, color = Color.White, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

// "java.lang.IllegalStateException: …" reads as "IllegalStateException: …":
// the exception's package goes, its message stays whole.
private fun shortHeadline(h: String): String {
    val m = Regex("^(?:[a-z][\\w$]*\\.)+([A-Z][\\w$]*)").find(h) ?: return h
    return h.replaceRange(m.range, m.groupValues[1])
}
