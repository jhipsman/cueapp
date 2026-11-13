package app.cue.tv.home

import androidx.activity.compose.BackHandler
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.OutlinedTextField
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.tv.material3.Border
import androidx.tv.material3.Card
import androidx.tv.material3.CardDefaults
import androidx.tv.material3.Text
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

// Manage profiles on the TV, as on the web: each profile's name, picture
// and color, a new one, or one removed. Only from the account's main profile
// (the server says so otherwise). PINs are set on the web.

private val avatarColors = listOf("teal", "red", "blue", "purple", "orange", "green", "pink", "yellow")
private const val MAX_PROFILES = 6

@Composable
fun ManageProfilesScreen(api: CueApi, close: () -> Unit) {
    var list by remember { mutableStateOf<List<Profile>?>(null) }
    var editing by remember { mutableStateOf<Profile?>(null) } // id 0: a new one
    var problem by remember { mutableStateOf("") }
    var reload by remember { mutableStateOf(0) }
    LaunchedEffect(reload) {
        list = try {
            api.profiles()
        } catch (e: Exception) {
            problem = e.message ?: "Couldn't load the profiles."
            emptyList()
        }
    }
    BackHandler { if (editing != null) editing = null else close() }
    val e = editing
    if (e != null) {
        ProfileEditor(api, e, canDelete = e.id != 0 && !e.main) {
            editing = null
            reload++
        }
        return
    }
    val first = remember { FocusRequester() }
    LaunchedEffect(list) { if (!list.isNullOrEmpty()) { delay(100); runCatching { first.requestFocus() } } }
    Centered {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text("Manage profiles", fontSize = 30.sp, fontWeight = FontWeight.Bold, color = Color.White)
            Gap(28.dp)
            Row(horizontalArrangement = Arrangement.spacedBy(28.dp)) {
                list.orEmpty().forEachIndexed { i, p ->
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Card(
                            onClick = { problem = ""; editing = p },
                            modifier = Modifier.size(110.dp).let { if (i == 0) it.focusRequester(first) else it },
                            shape = CardDefaults.shape(RoundedCornerShape(16.dp)),
                            scale = CardDefaults.scale(focusedScale = 1.1f),
                            border = CardDefaults.border(focusedBorder = Border(BorderStroke(3.dp, Color.White), shape = RoundedCornerShape(16.dp))),
                        ) {
                            Box(Modifier.fillMaxSize().background(avatarColor(p.avatar)), contentAlignment = Alignment.Center) {
                                AvatarFace(p.avatar, p.name, 44.sp)
                            }
                        }
                        Gap(10.dp)
                        Text(p.name, color = Color.White, fontSize = 15.sp)
                        Text("Edit", color = Dim, fontSize = 11.sp)
                    }
                }
                if ((list?.size ?: MAX_PROFILES) < MAX_PROFILES) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Card(
                            onClick = { problem = ""; editing = Profile(0, "", "teal/fox", false, null) },
                            modifier = Modifier.size(110.dp),
                            shape = CardDefaults.shape(RoundedCornerShape(16.dp)),
                            scale = CardDefaults.scale(focusedScale = 1.1f),
                            colors = CardDefaults.colors(containerColor = Color(0x1AFFFFFF)),
                            border = CardDefaults.border(focusedBorder = Border(BorderStroke(3.dp, Color.White), shape = RoundedCornerShape(16.dp))),
                        ) {
                            Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
                                Text("+", fontSize = 48.sp, color = Color.White)
                            }
                        }
                        Gap(10.dp)
                        Text("Add profile", color = Color.White, fontSize = 15.sp)
                    }
                }
            }
            if (problem.isNotEmpty()) {
                Gap(16.dp)
                Text(problem, color = Color(0xFFFF8A80), fontSize = 13.sp)
            }
            Gap(28.dp)
            Pill("Done", onClick = close)
        }
    }
}

@Composable
private fun ProfileEditor(api: CueApi, start: Profile, canDelete: Boolean, done: () -> Unit) {
    var name by remember { mutableStateOf(start.name) }
    var color by remember { mutableStateOf(start.avatar.substringBefore('/').ifEmpty { "teal" }) }
    var icon by remember { mutableStateOf(start.avatar.substringAfter('/', "")) }
    var problem by remember { mutableStateOf("") }
    var busy by remember { mutableStateOf(false) }
    val scope = rememberCoroutineScope()
    val avatar = if (icon.isNotEmpty()) "$color/$icon" else color
    fun save(work: suspend () -> Unit) {
        if (busy) return
        busy = true
        problem = ""
        scope.launch {
            try {
                work()
                done()
            } catch (e: Exception) {
                problem = e.message ?: "Couldn't save the profile."
            }
            busy = false
        }
    }
    Centered {
        Row(horizontalArrangement = Arrangement.spacedBy(48.dp), verticalAlignment = Alignment.CenterVertically) {
            // The profile as it will look.
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Box(Modifier.size(150.dp).clip(RoundedCornerShape(22.dp)).background(avatarColor(avatar)), contentAlignment = Alignment.Center) {
                    AvatarFace(avatar, name.ifBlank { "?" }, 60.sp)
                }
                Gap(12.dp)
                Text(name.ifBlank { "New profile" }, fontSize = 20.sp, fontWeight = FontWeight.Bold, color = Color.White)
            }
            Column(Modifier.width(560.dp)) {
                OutlinedTextField(name, { name = it.take(30) }, label = { androidx.compose.material3.Text("Name") }, singleLine = true,
                    colors = fieldColors(), modifier = Modifier.fillMaxWidth().leaveWithArrows())
                Gap(16.dp)
                Text("Picture", fontSize = 13.sp, color = Dim)
                Gap(8.dp)
                val pics = listOf("") + avatarEmoji.keys
                pics.chunked(10).forEach { row ->
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.padding(bottom = 8.dp)) {
                        row.forEach { key ->
                            Choice(selected = key == icon, size = 46.dp, onClick = { icon = key }) {
                                if (key.isEmpty()) Text(name.take(1).uppercase().ifEmpty { "A" }, fontSize = 18.sp, fontWeight = FontWeight.Bold, color = Color.White)
                                else Text(avatarEmoji[key] ?: "", fontSize = 22.sp)
                            }
                        }
                    }
                }
                Gap(8.dp)
                Text("Color", fontSize = 13.sp, color = Dim)
                Gap(8.dp)
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                    avatarColors.forEach { c ->
                        Choice(selected = c == color, size = 40.dp, round = true, onClick = { color = c }) {
                            Box(Modifier.fillMaxSize().background(avatarColor(c)))
                        }
                    }
                }
                if (problem.isNotEmpty()) {
                    Gap(12.dp)
                    Text(problem, color = Color(0xFFFF8A80), fontSize = 13.sp)
                }
                Gap(20.dp)
                Row(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    Pill(if (busy) "Saving…" else "Save", primary = true) {
                        if (name.isBlank()) {
                            problem = "Give the profile a name."
                            return@Pill
                        }
                        save {
                            if (start.id == 0) api.addProfile(name.trim(), avatar) else api.updateProfile(start.id, name.trim(), avatar)
                        }
                    }
                    Pill("Cancel", onClick = done)
                    if (canDelete) Pill("Delete profile") { save { api.removeProfile(start.id) } }
                }
            }
        }
    }
}

// A picture or color to pick: a ring round the one chosen, white when the
// remote is on it.
@Composable
private fun Choice(selected: Boolean, size: Dp, round: Boolean = false, onClick: () -> Unit, content: @Composable () -> Unit) {
    val shape = if (round) CircleShape else RoundedCornerShape(12.dp)
    Card(
        onClick = onClick,
        modifier = Modifier.size(size).let { if (selected) it.border(2.dp, Teal, shape) else it },
        shape = CardDefaults.shape(shape),
        scale = CardDefaults.scale(focusedScale = 1.15f),
        colors = CardDefaults.colors(containerColor = Color(0x14FFFFFF)),
        border = CardDefaults.border(focusedBorder = Border(BorderStroke(3.dp, Color.White), shape = shape)),
    ) {
        Box(Modifier.fillMaxSize(), contentAlignment = Alignment.Center) { content() }
    }
}
