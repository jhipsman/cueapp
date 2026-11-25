# Changelog

All notable user-facing changes to Cue are recorded here. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
version numbers follow [Semantic Versioning](https://semver.org/). Each
release has its own dated section. Changes that aren't released yet collect
under [Unreleased]. How releases are cut: [docs/RELEASING.md](docs/RELEASING.md).

## [Unreleased]

### Added
- **Watch on Netflix (and Disney+, Max, Prime Video…).** Tick the services you pay for in Settings › Streaming quality › My services; a title on one of them gets a Watch on button. On Fire TV / Google TV it opens the service's own app at the title (where the app takes a search) or at its home; on the web, the service's site. Cue doesn't fetch those titles ahead of time on release day, so the Premiumize allowance goes on the rest.
- **Day-one releases: Get it ready.** On the day something comes out, the versions of it that have just appeared often aren't in Premiumize's cloud yet, so Play found nothing to stream. Now Play offers **Get it ready** (on Fire TV / Google TV it just starts when you press Play): Cue sends the best version to Premiumize, shows how far it's got, and plays it the moment it's ready, usually a few minutes for a popular new release; your phone hears too, so you can walk away. And on release day Cue does it by itself, every hour, for new episodes of the shows you're watching and the films you asked to be reminded of, so they're ready before you sit down.
- **Hide adult content** (Settings › Streaming › Family, on by default): adult Live TV channels and groups (XXX, For Adults, 18+, the adult networks) are left out of the channel list, the guide, Sports, Live TV search, recordings and playing, and adult releases out of release search. YouTube searches use moderate SafeSearch, and TMDB movie and show searches ask without adult titles.
- **Settings on Fire TV / Google TV**, at the bottom of the side menu above your profile, laid out like Settings on the web: **Account and server** (switch or manage profiles, change server, the web version), **Playback** (subtitles a film starts with, surround sound, Light mode), **Problems** (the last crashes on this TV and whether Cue has them, every TV's open crashes when signed in as an administrator, and a QR code to open Problems on your phone) and **About and updates** (the app's and the server's versions, and Check for an update). Profile has a Settings button too.
- **Crash alerts and Settings > System > Problems.** When the Fire TV / Google TV app crashes, or a page of the website fails to draw, Cue keeps the crash (the same crash on any TV or build is one line with a count) and an administrator's phone with alerts on hears about a new kind of crash once, and again if one marked fixed comes back. Problems lists them with the devices, versions and stack trace, and **Copy for Claude** copies everything needed to get it fixed in a Claude Code session.
- **Ask Cue on Fire TV / Google TV**, in the side menu under Search: ask for something to watch with one of the ideas (one press), by typing with the TV's keyboard, or out loud where the TV lets apps listen (Google TV). The answer and its picks show beside it; asking again carries on the conversation, and "Play X" opens X's page.
- **Browse**, on the web, Fire TV / Google TV and Apple TV: a page for each streaming service (its Top 10 today, what's new, its originals, its best rated, and its genres), collections across every service (Critically acclaimed, Family movie night, Under 90 minutes, Hidden gems, plus Holiday movies from mid-November and Halloween horror in October), and twenty film series in the order they came out. Coming soon and the Top 10 live in Browse too. Everything shown keeps to a kid profile's age limit.

### Changed
- **The web app is one colour, like the TV app.** Buttons, chips, the highlight and the glow behind each page now use the profile's colour (Cue's teal unless the profile picked another) everywhere, instead of a different colour per section (purple on Discover and a movie's page, green on Library, and so on).
- **Fire TV / Google TV: a shorter control bar in the player.** Play, back and forward 10 seconds, then Next episode (shows), Other version (when there is one), Subtitles, Sound and More. Sound holds the audio track, Surround, Volume boost and Listen on phone (four look-alike note icons before); More holds the speed (0.5× to 2×), Wrong video, Watch party and Report a problem.
- **Fewer buttons.** On a title page, only Play (or Remind me) and Watch on a service have words; My List, 👍, 👎, Remind me when it streams and Remove from Continue Watching are small round icons, named under the row on the TV while highlighted and when pointed at on the web. A title not out yet says when as a line, not a button. The TV's Profile page keeps Switch profile, Settings and Your year in Cue; the rest is in Settings.
- **Search understands what you mean**, on the web, Fire TV / Google TV and Apple TV. Not just titles named like it: a person ("tom hanks", "nolan") shows what they're in or made; a description ("comedy", "funny 90s movies", "best sci-fi shows on netflix", "new comedies", "short animated films", "anime") browses for exactly that; a topic ("zombies", "heist movies", "christmas", "time travel") finds titles about it. Each answer is its own row, under a heading saying how it was read. A misspelling ("breakng bad") finds the well-known title it's a letter or two from, and says so.
- **Fire TV / Google TV: a smoother side menu.** Opening it no longer resizes it frame by frame (which slower Google TV boxes couldn't keep up with); the names fade in. Moving down it, each page now just fades in instead of sliding up, so the screen no longer bounces at every step.
- **New look: graphite and white.** Cue's default colours are now neutral graphite greys with white highlights and buttons, and no glow. Profiles that chose their own colours keep them; Graphite and Cue white are in Colors for anyone who wants to switch.
- **Live TV, redesigned** (web, Fire TV / Google TV): the game that matters most leads the page as an event, with both teams large (logo, city, name, record) over their colours, a countdown to the start (or the score, live), the venue and the channel it's on. Beside it, Featured moves through the next games. Below, On now shows a card per channel (logo, what's on, a line of its story, time left, star), sorted A–Z or by number, with a channel search; Guide still shows the full grid.
- **Channels without the mess.** Live TV sorts your provider's list by Cue's own channel catalog, not the provider's groups: plain names ("ESPN", not "US ★ ESPN FHD ◉"), a country to pick (it starts on the US when your list has it) and topics (General, News, Sports, Kids, Movies, Documentary, Music, Local, 24/7…). The same channel in HD, FHD and backup shows once, the best picture first; if it doesn't play, the player moves on to its copies by itself. Dividers like "##### UK BACKUP #####" are gone. On the web and Fire TV / Google TV.
- **Fire TV / Google TV: a shorter side menu.** Search, Home, Movies, Shows, Live TV and Browse, plus your profile. Ask Cue opens from a button at the top of Search (Back returns to Search), the games lead Live TV, and My List is a tile in Browse.
- **The cloud server picks up a new Cue within minutes.** It checks every 5 minutes instead of once a night, and restarts Cue only when there is a new version. A server set up earlier switches over when the installer is run again. (This replaces the SSH deploy workflow, which needed keys copied between the server and GitHub.)
- **The web app opens like the TV app.** Its loading screen is now the TV's opening logo: the Cue mark grows and turns into place with a soft glow breathing behind it, and "cue" slides in beside it, in the same teal and on the same background as the TV (the profile's colour once signed in). It plays once per page load, and no longer flashes light on a computer set to light mode.
- Fire TV / Google TV: in Settings, a section's page fades in, rising a little, instead of swapping in at once.
- **A livelier demo on the front page.** Instead of one clip, the app plays scene by scene (Home with its trailer, a poster previewing as it's hovered, a title's page, the player, Live TV), cross-fading from one to the next, with the scenes named underneath and a soft glow of what's playing behind the screen, which settles flat as it scrolls into view. No mouse pointer in any of it, and the ad is gone, so the page is about 10 MB lighter.
- **A cleaner front page.** The page someone sees before signing in is now just the headline, the app playing (with Watch the ad on it) and how to get Cue on each screen. The features grid, the poster wall and the 3D phone are gone, and so is three.js, so the page loads less.
- **Ask Cue picks better.** It now thinks of the titles that fit from what it knows about film and TV (not only the most popular titles in a genre), looks them all up at once with their running time, genres and rating, and keeps to everything asked for: length, decade, mood, who's watching, a service, and "like X" (with what people who liked X also liked). Broad asks favour the best rated over the most popular. It runs on Claude Sonnet by default (Haiku and Opus are still in Settings › Movie info), and never answers with a blank reply.
- **Fire TV / Google TV opens straight to Home.** Signed in already, the app no longer holds on the logo while it checks with the server: it opens on the Home you saw last (kept on the TV), then refreshes it; the Cue logo still plays as it opens, over Home loading underneath. Only a first start, or a sign-in the server has dropped, waits.
- Fire TV / Google TV: Back from a title opened in Browse returns to the service, collection or film series it came from, on the same title (it went back to Browse's front page).
- Fire TV / Google TV: moving down the side menu past Browse no longer pulls the highlight into Browse; a service's page, a collection or a film series opens with the highlight on its first title.
- Fire TV / Google TV: the side menu opens a little more gently (260 ms) and closes quicker (180 ms), and opening a title or going Back fades over 320 ms, as the motion rulebook asks.
- **Fire TV / Google TV: a side menu instead of the tabs along the top.** Search, Home, Shows, Movies, Sports, Live TV, My List and Browse are icons down the left, with your profile at the bottom; moving onto it opens it with the names. On the web, Browse takes Coming soon's place in the top bar (Coming soon is in Browse and in More).

### Security
- Updated two libraries Cue uses indirectly for newly published advisories: OpenTelemetry to 1.47.0 (GO-2026-6974) and Gorilla WebSocket to its fixed commit (GO-2026-6278).
- Releases are signed with a new update key, held only by the project owner. **Update now** installs only releases signed with it.

### Fixed
- Web: the loading screen no longer logs a Content Security Policy error on every page, and its logo no longer starts over as the app loads.
- **Fire TV / Google TV: rows bounced while moving along them** (the Top 10 most). The highlighted poster grows a little, and the page re-lined the row up around it at every press, nudging everything up and back down; it now leaves a row that's already in place alone. A test on the build machine traces the poster frame by frame to keep it that way.
- **Fire TV / Google TV: More Like This.** Opening a title from More Like This (or a series or a cast row) now goes on top of the one you were on, so Back returns to it, and coming back up to the buttons brings the whole top of the page back into view, title and all.
- A film already on a streaming service is out, even when TMDB's dates list only its premiere (some streaming originals were labelled In theaters).
- **Colours didn't change to graphite on the web** for profiles that had the old default (teal on midnight) saved. A profile still on exactly the old default now follows the new one; colours someone picked are kept.
- **Live TV looked like a wall of 24/7 channels.** Channels now start in a Top channels order: the big networks first (ABC, CBS, NBC, FOX, ESPN, CNN, HBO…), then the rest, with 24/7 loops of one show, events and adult channels last (A–Z and by number are still there). Names lose any symbols a provider decorates them with (● ⬤ ❖ ★…) and a doubled "24/7", and the provider's groups read "US · ABC".
- **Picking the game up top (or a channel from Sports or a search) didn't move the list to it.** The channel now shows in the list, scrolled to and marked as playing, switching the country or topic if those picked didn't include it.
- **Fire TV / Google TV: nothing was highlighted after Back from Ask Cue.** Search now keeps putting the highlight back on Ask Cue's button until it lands (a TV can refuse the first try just as Ask Cue closes), and stops as soon as it has, so pressing on to the menu isn't pulled back.
- **Live TV's channel sorter could hide channels.** A channel whose name doesn't say a country now shows under every country (it used to disappear once a country was picked), and a list that mostly doesn't say starts on Every country. Only true duplicates (the same channel in HD, FHD or as a backup) are folded into one, and Live TV says how many; they're still used automatically when their channel won't play.
- **Fire TV / Google TV: smoother scrolling.** Moving the highlight on Home and Live TV no longer rebuilds the whole screen at each press of the remote, and Live TV's Featured progress line no longer lays the card out again every frame. The emulator check on GitHub now also records how many frames each screen drops.
- **Fire TV / Google TV: Back did nothing on the side menu after Ask Cue.** Leaving Ask Cue put the highlight back on its button a moment later, even when Back had already gone to the menu, so the next Back was lost. Also: the open side menu now dims the page behind it (no more page showing through beside the names), moving right from Search lands on Ask Cue, and Ask Cue's picks run to the edge of the screen like every row.
- **Sports picked the wrong channels.** A game is now matched first to a channel whose guide names both teams around kickoff (the event channels lists carry for games on ESPN+, Peacock, Prime, League Pass, NHL Network…), then to a channel named for the game, then to the network's own American channel by its plain name, never its news or sister channels, another country's, or (for USA Network) every channel starting with "USA". A game no channel was matched to says **Find it on Live TV** and searches every channel's name and what's on for the team, instead of saying it's only on a streaming service.
- Fire TV / Google TV: no longer crashes when a remote button is pressed at the moment a row is settling its highlight ("Dispatching intercepted soft keyboard event while focus system is invalidated"), seen on a Fire TV with Android 9. The app moves to Compose 1.8, where such a press is skipped instead of crashing.
- **Fire TV / Google TV crashing while moving between rows** ("Release should only be called once", sent in by the TV's crash reports): the rows' memory of which card had the highlight came from Compose's focusRestorer, which on some TVs releases a hold on that card twice and closes the app. Cue now keeps that memory itself, with no hold; coming back into a row still lands on the card you left.
- **Sports showed only TV-guide games, never ESPN's** (no team logos or scores): ESPN started refusing how Cue asked for the games (several days at once, "400 Bad Request"). Cue now asks for each day on its own (today and tomorrow, as ESPN counts days), falling back to ESPN's plain "today" if that is refused too.
- **Sports**: an ESPN game whose details arrive in an unexpected shape is now kept (with that detail left blank) instead of dropped, which on some servers dropped every game and left only the TV-guide fallback. When ESPN can't be used (whether or not the TV guide has games), the page now says why ("ESPN listed 40 games: 38 finished…", or which games couldn't be read). In the TV-guide fallback, titles lose the providers' small "ᴸᶦᵛᵉ" tags, and real matchups come before shows about sport.
- **Movies and Shows tabs**: the Top 10 row now has ten films (or ten shows) on every service, instead of the two or three left after hiding the other kind, so every service has its tab again (web, Fire TV / Google TV). The service tiles and decades below it have headings ("Browse films by service", "By decade"), each service tile shows its logo with its name, and the service tabs above the Top 10 are all the same size, with the chosen one ringed.
- **Sports**: when ESPN can't be reached, the page no longer sits on "Loading the games…": it shows games from your TV guide straight away and says why ESPN didn't answer. Guide games read better too: the league on its own line, then the matchup ("Milwaukee Bucks at Miami Heat"), and channel names without the provider's clutter ("ESPN", not "US ★ ESPN HD ◉"). A game shown on two channels, or in two languages, is listed once.
- Fire TV / Google TV: moving through the menus no longer crashes the app when a list holds the same title, person, channel or game twice (a cast member in two roles, a channel listed twice). Signing out with no connection and a dropped connection while waiting for a sign-in code no longer close the app either.
- Fire TV / Google TV: if the app does crash, it sends the error to your Cue server the next time it opens, so the cause can be found: `docker compose logs cue | grep "tv app crashed"`.
- **Sports couldn't be found without Live TV**: it lived inside Live TV, which only shows with a TV provider set up. Sports is now its own section: **Sports** in the top bar (and More on a phone) on the web, a **Sports** tab on Fire TV / Google TV and on Apple TV. Without a TV provider it shows the games and scores; with one, picking a game plays the channel it's on.
- **Next episode on the TV played the same episode again** (from the second Next on, or when an episode ended). During the short fade into the next episode the old one was still loaded and near its end, so the TV prepared the episode it was about to play as the one after it. Nothing is prepared during the change now, and a prepared next episode is only kept while the episode it follows is the one playing.
- **Next episode played the same episode again in a watch party.** Right after Next, the party could still be saying the old episode for a moment, and the player "followed the party" back to it. The TV and browser now ignore word of the episode they just left for a few seconds.
- **Team logos**: Sports shows each team's logo on the web (ESPN's image server is now allowed) and on the TV's Sports banner, with the score; a team ESPN's scoreboard gives no logo gets its usual logo from ESPN.
- **Skip intro showed up only near the end of the intro** for shows whose intro Cue learned from your own skips (no TheIntroDB times): what it learned was where your jump started, often part-way through the intro. A learned intro's button now shows at least 50 seconds before the intro ends, and learning the same intro again keeps the earliest start seen.
- **Sound behind the picture in a converted video started part-way in** (joining a watch party, resuming, jumping, in a browser that can't play the file's own sound): a copied picture can only start at a keyframe, a few seconds before the time asked for, while the remade sound started exactly there. Both now start at that keyframe, and the player is told where it is (`GET /api/play/convert/start`), so its timeline matches too.
- **The watch party panel is much smaller**: on the TV a small card in the top corner that shows on news and hides itself after a few seconds (it was a third of the screen); in a browser a slimmer side panel (280 px), on a phone held sideways a narrow strip, upright a short sheet under the picture.
- **Fire TV: the picture could freeze while the sound played on.** The player now notices (no new frame for four seconds while playing) and restarts the TV's video decoder where the sound is; if it keeps happening it suggests Other version. In a watch party, a TV sending surround sound on as it is no longer catches up by changing speed (which that sound can't do, so the picture stalled); it makes small jumps instead.
- **Next episode on Fire TV and Google TV is seamless**: the player finds the next episode in the background near the end, then moves on in place with a short fade (from Next, the countdown at the credits, or the episode ending), instead of going back to Home and through a second "Finding a version" screen.
- **Loading on the TV shows the Cue logo**, gently breathing, instead of a spinning blue ring.
- **Nothing above PG-13 plays by itself**: preview clips, Home's banner and the screensavers now only ever use titles rated up to PG-13 or TV-14 (R, TV-MA, NC-17 and unrated titles never), on top of the horror, anime and adult rules. And a version far from the title's length (another video with the same name) is skipped by Play and never used for a clip.
- **An old preview clip kept playing after it was replaced**: browsers kept each clip for a day. Clips are now checked with Cue each time (and their address changes when one is made again).
- **The wrong video for a film with a common name**: an adult video named like a film ("Anastasia.2020.XXX...") matched it by title and year, so Play and its preview clip could show it, as happened with the children's film Anastasia (2020). Releases named like adult videos (XXX, studio and site names) or filed under a site's XXX category never match a film or show now, for Play and library downloads alike. Every preview clip made before is thrown away and made again.
- **Preview clips of erotic films**: titles TMDB tags as erotic, softcore, or for nudity or sex scenes now count as adult, so they get no preview clip and stay off the banner and screensavers (before, only TMDB's adult flag and NC-17 did).
- **Nowhere to add the OpenSubtitles key in streaming-only mode**: the server settings now have **Movie info and subtitles > Subtitles** with just the key (the TV app reads the subtitles inside the file; a browser can't, so it finds them on OpenSubtitles), and the player's message says where to go.
- **Watch party: a phone that fell a few seconds behind stayed behind.** A phone whose video Cue converts now jumps straight to the party's moment when that part is already converted (on an iPhone the conversion runs well ahead), instead of waiting for a gap of 20 seconds and restarting the conversion; it restarts only for a point not made yet, from 12 seconds out.
- **The Fire TV player looks like the web one**: bare white icons with a soft ring on the one the remote is on (some Fire TVs drew the selected button as a plain white circle), no boxed panel, and everything about a fifth smaller.
- **Films not released anywhere yet no longer offer Play**: they're on Coming soon, with the day they stream (from TMDB's release dates). Films **in theaters** still play, tagged **In theaters** ("In theaters now · streaming March 4th"), with camera recordings allowed for them (after anything better) since that's usually all there is, and **Remind me when it streams**. Shows that haven't started say when they start. Films in your own library always play.
- **Watch party: a converted video kept starting over** (the browser or phone showed "Starting…" again and again, and fell further behind). Each catch-up jump restarted the conversion, which on a small server took longer than the gap, so it jumped again. Now a converting player starts where the party will be once it's going, measures how long its start took and aims that far ahead, and doesn't chase the party again for a minute and a half.
- **Fewer "Converting this version…" detours in the browser**: versions whose picture a browser plays as it is (H.264) are now tried before HEVC, x265, 10-bit and AV1 ones (Safari plays HEVC, so it keeps those), and a file the browser surely can't open (AVI and the like, HEVC outside Safari, MKV in Safari) is converted straight away instead of being tried first and waited on.
- Usenet: when one download finished while another was running, the provider could refuse the next connection with "too many connections". Cue now gives a connection's place back only once the provider has hung up.
- Watch parties on an iPhone or iPad: a phone joining no longer lags far behind or restarts its video each time someone presses play or pause. Safari now gets an HEVC picture copied as it is instead of remade (far quicker on a small server), a phone that is converting only jumps when it's more than 15 seconds out (aiming a few seconds ahead for the time it takes to start), and it no longer pulls everyone back to where it is. When the phone won't start the video with sound by itself, it shows **Tap to join the party**.
- **Watch parties got stuck on "Starting…" for shows Cue converts for the browser** (most shows on a phone or PC): every time a converted video loaded, the party lined it up again, which restarted the conversion, over and over. A converted video now jumps only for a real difference (8 seconds while playing), never while it's loading, and joining starts its conversion once, at the party's moment.
- **The tour shows once per account**, not on every new browser or device: whether it's been seen is kept on the account.
- **Settings in Streaming only mode opens on Streaming**, not on Media types (a page that mode hides from the menu), and Media types now says its switches do nothing while Streaming only is on, with a link to change it.
- **No sound in the web player, and versions flicking past one after another**: when a browser shows a version's picture but can't play its sound (Dolby, DTS, TrueHD), Cue now keeps the picture exactly as it is and converts only the sound, from the same moment. Before, it converted only plain H.264 pictures and otherwise jumped to the next version, which usually had the same sound, so it cycled. **No sound? Fix it** in Versions does the same.

### Added
- **Streaming service logos**: the Top 10 tabs, the Top 10 page and the Movies and Shows homes show each service's logo (TMDB's) instead of its name, on the web and the TV.
- **A Top 10 tab on Fire TV / Google TV**: a Top 10 for any service, genre, decade or country, picked with the remote (All / Movies / Shows, the services by logo, genres, decades), with the big numbers beside the posters.
- **Ask Cue**: an assistant (the sparkle in the top bar, and More › Ask Cue). Say what you're in the mood for ("something funny for tonight, under two hours") or ask about a film, a show or a person, and it answers with posters you can play; "play Dune" plays it. It only offers titles from Cue's own lists, so a kids' profile only hears about what it may watch. It uses Claude with the owner's own key (Settings › Movie info › Ask Cue); without a key it stays off. It answers with Claude Haiku by default (about a fifth of a cent a question); Opus can be chosen there for the smartest answers.
- **Cast and people**: a title's page shows its cast with photos; each opens their page (photo, story, and everything else they're in, playable). On the TV, picking someone shows "More with them" under the cast.
- **Top 10 for anything**: a Top 10 page (More › Top 10, or See all on Home's Top 10) by service, country, genre and decade, with the big numbers. The Movies and Shows tabs open with their own Top 10, then a strip of services and decades (web and TV).
- **The liquid glass look**: frosted, see-through surfaces with a bright edge: a floating tab bar on phones, the top bar once you scroll, menus and sheets.
- **A menu that names everything**: More (in the top bar, and the last tab on a phone) lists every section and tool: Coming soon, Top 10, Ask Cue, Your year, Reports, TV remote, Settings.
- **Find a line**: Versions › Find a line in the player: type words someone says and jump to that moment (from the subtitles).
- **Volume boost** up to twice as loud: on the TV (Volume boost in the player), and in a browser when Cue converts the video.
- **Listen on phone**: on the TV, Listen on phone shows a QR code; scan it and the sound plays on the phone, in step with the picture (headphones, late at night), while the TV stays quiet. Press it again for the sound back on the TV.
- **Sports on Live TV, from ESPN**: real schedules and live scores (NFL, NBA, MLB, NHL, college football and basketball, Premier League, Champions League, MLS, WNBA) with team logos and records. The next big game sits on top (bigger leagues, the playoffs and your teams first, sooner beats later), then games on now and coming up. Each game's network is matched to a channel in your Live TV list, so Watch goes straight there; games only on a streaming service say where ("On Prime Video"). **Your teams**: pick teams per profile; their games come first, and with alerts on, the phone hears an hour before kickoff. When ESPN can't be reached, the games come from the TV guide instead. On the TV: a Sports group with the score or kickoff time.
- **Still watching?** After three episodes in a row play on by themselves with nobody pressing anything, the player asks before the fourth (TV and web), so a show doesn't play all night. Not in a watch party.
- **Cue notices problems nobody reports**: when two people (or three plays) move off a title's first version within a week, Cue files a report about it itself and alerts the owner.
- **Previously…**: on Continue Watching, a show you haven't watched for three weeks says what happened in the last episode you saw (under the card on the web, in the banner on the TV).
- **A kids' Home**: a profile limited to PG or below gets its own Home: Family films, Animated films, Kids' shows, Family shows and Cartoons, with the general trending and popular rows (TV-PG grown-up shows like Seinfeld got through the limit there), the Top 10s and anime or horror left off.
- **Friday's week ahead**: with alerts on, Friday brings one alert saying what's coming this week on your shows ("3 new episodes this week: Paper Moons Sunday, …").
- **Genres on the TV's Search**: tabs above the posters (Popular, Action, Comedy, Kids…) before anything's typed.
- **Report a problem**: in the player (Versions on the web, **Report** on the TV), anyone watching picks what's wrong (sound, subtitles, picture, wrong video, keeps stopping, something else) and can add a word. The owner gets an alert, sees them under **Reports** in the profile menu (with a count on the avatar), and answers each with **Mark fixed**, **Can't fix** or **Wrong video: drop this version** (never used for the title again, its clip thrown away), with a reply. Whoever reported it gets an alert and a note on Home.
- **Top 10 on each service**: Home has a row of what's most watched right now on Netflix, Prime Video, Apple TV+, Disney+, Hulu, Max, Peacock and Paramount+ (a tab for each), with big ranked numbers beside the posters, on the web and the TV. Everything in it plays on Cue.
- **Genres on Search**: with nothing typed, Search shows colourful genre tiles (Action, Comedy, Sci-Fi, Kids…) that open that genre's most popular films and shows. Search and Coming soon both have **All / Movies / Shows** pills.
- **New episodes coming** on Coming soon: the next episodes of the shows you watch (recently watched, on My List, or with Remind me) airing in the next two weeks, by day ("Tomorrow", "Friday, March 6th"), on the web and the TV.
- **A greeting on Home** ("Good evening, Sam") with tiles for what's new: new episodes, what's coming, watch parties on now and what to carry on with.
- **Quick actions on a poster**: pointed at with a mouse, a poster shows Play, My List, a thumb and More info along its foot.
- **Wrong video** in the player (web and TV): when what plays isn't the title at all, one press makes Cue never use that version for it again, for the whole household, throws away the preview clip cut from it, and plays the next version.
- **Phone alerts**: Settings > Alerts turns on alerts on a phone or computer for a new episode of a show you're watching, and for titles you asked to be reminded about once they're out. Web push signed with Cue's own key; the push carries no text (the phone asks your Cue what to say). On iPhone, in Cue added to the Home Screen.
- **Send to TV**: a title's page on a phone or computer has **Play on TV** for any Fire TV or Google TV with Cue open; the TV starts it, and the phone becomes its remote (play, pause, back 10 s, forward 30 s, jump, stop). **TV remote** in the avatar menu.
- **Coming soon and Remind me**: films not out yet have their own **Coming soon** section (top bar on the web, a tab on the TV), apart from what you can watch: the day each streams ("March 4th"), its picture and story, and **Remind me**, which puts it under **Out now** at the top of Home once it's out, until you play it. More like this only lists what's out.
- **Film series in order**: a film's page shows the series it belongs to (from TMDB's collections), in release order, on the web and TV apps; at the end of a film the web player offers **Next in the series**.
- **Remove from Continue Watching** and **thumbs**: a title's page can take it off Continue Watching (where you were is kept; watching again brings it back), and a thumb up or down per profile: up makes it a reason for Because you watched, down ("Not for me") keeps it off Home. Web and TV apps.
- **Kids' profiles: an age limit.** Manage profiles can give a profile a limit (G, PG, PG-13 or R; shows' TV ratings are matched to it). Titles above it are left out of Home, search and More like this on that profile, and their pages and Play are refused, on every device.
- **Watch parties play in step, to a fraction of a second**: a player a little behind or ahead catches up by playing 5% faster or slower instead of jumping (web, Fire TV and Google TV); the party waits for everyone with "Waiting for Mom…" while someone's player loads (half a minute at most); and everyone plays the same version, the one every device opens most easily, so the same moment is at the same second everywhere.
- **Watch parties**, like the old Netflix Party: press **Watch party** in the player and everyone in the household sees it at the top of Home and joins with one press (on a Fire TV or Google TV too), at the same moment. Anyone's play, pause or jump happens for everybody with a line in the chat saying who did it; players that drift catch up by themselves, and the party moves on to the next episode together. A chat sits beside the picture on the web, and reactions pop up on everyone's picture with who sent them, drift up and fade away, as in a Teams meeting; on a TV, **React** works with the remote and a QR code opens the chat on your phone. Phones and computers can also join from an invite link.
- **Two-step sign-in**: in Watch, Settings > Sign-in & security turns on a code from an authenticator app after the password (QR code, or open it straight in the app on the phone), with ten one-time backup codes for a lost phone. Each code works once; wrong codes count like failed sign-ins; turning it off or new backup codes need the password. TVs keep signing in with the phone. The server's `reset-password` command also turns it off, for a locked-out owner.
- **A Settings page in Watch** with categories: Account (change password, sign out), Sign-in & security, Profiles & look, Playback (autoplay the next episode, previews, preview sound, streaming quality), Privacy (clear a profile's watch history), TVs & devices (sign in a TV, set one up, **sign out everywhere else**), Family (the owner's) and Help & about. The profile menu is now Switch profile, Settings and Sign out. These settings belong to the account, so they follow it to every device.
- **Locking down a cloud server** in docs/security.md: firewall, SSH keys only, automatic security updates, backups.
- **Family code**: make one code in Settings > Accounts (or choose your own) and tell your family; the sign-in page shows **New here? Create an account**, where the code, a name, a username and a password make their own account. It doesn't run out (a new code or **Turn it off** stops the old one), accounts made with it watch but can't change settings, and wrong codes count like failed sign-ins.
- **Get Cue on your TV, with pictures** (cuetv.me/setup-tv): each step for a Fire TV or a Google TV with a drawing of the TV screen and what to press lit: getting Downloader, allowing it to install apps (and turning on developer options), typing the code, installing, and signing in with a phone. Open to anyone; the texted invite and the front page link to it.
- **Text the invite**: Settings > Accounts > Invite family has **Text it** (opens Messages on a phone with the invite written out: the link, then how to get Cue on a Fire TV or Google TV with Downloader's code), **Share**, and **Copy the whole message**.
- **Manage profiles on the TV** (Fire TV, Google TV): Profile > Manage profiles changes each profile's name, picture and color, adds one or removes one, with the remote (from the main profile; PINs are set on the web).
- **Light mode** (Fire TV, Google TV): still pictures instead of moving previews, and highlighted cards without their soft shadow and sweeping shine (the parts that redraw every frame), for TVs with a slow processor. On by itself on TVs with little memory (most built-in Google TVs); Profile switches it.
- **Sign in the TV with your phone** (Fire TV, Google TV): the sign-in screen shows a QR code and a 6-letter code; scan it with a phone that's signed in (or go to cuetv.me/link and type the code), tap **Sign in the TV**, and the TV is signed in to that account. A code lasts ten minutes and works once. `POST /api/auth/tv-link`, `GET /api/auth/tv-link/{token}`, `POST /api/auth/tv-link/approve`.
- **Profile pictures**: in Manage profiles, each profile can wear a picture on its color (a fox, a unicorn, a game controller, 18 in all) instead of its initial, so everyone finds theirs at a glance: on the web, Fire TV, Google TV and Apple TV. Stored with the color (`"avatar": "teal/fox"`); `GET /api/profiles` lists them as `icons`.
- **A short tour** (web, Fire TV, Google TV): the first time Watch opens, four slides cover what's worth knowing: **Other version** / **Try another version** when a version is blurry, in the wrong language, out of sync or keeps stopping; **Skip intro** and **Next episode**; subtitles; and searching from your phone. Each has a small drawing of the real control, lit. **How Cue works** in the profile menu (web) or on Profile (TV) shows it again.
- **A new Cue logo**: the C with a play button, redrawn: an even ring with round ends and a soft play button set in its middle, beside "cue" in Sora Bold. On the web (header, sign-in, front page, favicon and home-screen icons) and the Fire TV and Google TV app (launcher icon, banner and header).
- **Invite family**: in Settings > Accounts, make an invite link and send it round; each person makes their own account (one that watches with this Cue's Premiumize, add-ons and live TV, and can't see or change its settings) and is signed in. A link works for seven days; a new one ends the old, and it can be turned off. `GET`/`POST`/`DELETE /api/users/invite`, `GET`/`POST /api/join/{token}`.
- **The TV apps open on cuetv.me**: no server address to type on a new Fire TV, Google TV or Apple TV, just sign in. Profile > Change server (Fire TV) and Change on the sign-in page (Apple TV) still point it elsewhere; `-PcueServer=` builds the Fire TV app for another server.
- **A front page for the web** (signed out): Cue's address shows what Cue is, clips of the app playing, the commercial with sound, and how to get it on a Fire TV, a phone or an Apple TV, with Sign in at the top. Signed in, Cue opens straight to Watch as before; a link to anywhere else (a title, the phone-search code) still goes to sign-in and then there.
- **Next episode, starting** (Fire TV, Google TV): when an episode ends or Next episode is pressed, the next episode's picture, name and story fill the screen with "Finding a version…" until it plays, instead of going back to the menus; Back stops it.
- **Search from your phone** (Fire TV, Google TV): Search shows a QR code; scan it and type or say what you want on the phone (its keyboard's mic, or Tap and say it), and the TV searches as you go. This is voice search on a Fire TV, which keeps its remote's microphone for Alexa. `GET`/`PUT /api/watch/remote/{code}`.
- **The TV app updates itself** (Fire TV, Google TV): when a new build is out, Cue says so on opening and on Profile; Update downloads it and Android installs it over the old one, signed in and on your profile. `GET /tv.version` forwards to the newest build's number, published beside the APK.
- **Live TV, channel surfing** (Fire TV, Google TV): rest on a channel and its live picture plays, muted, in the banner at the top of Live TV; in the player, up and down (or channel up and down) change channel in place with a banner of the channel's number, name and what's on, tuning only the channel you stop on.
- **Up next** (Fire TV, Google TV): when an episode ends, the next one fills the screen (its still, the show's logo, its name and story) and plays after a countdown round the Play button; Back stays where you were.
- **Your year in Cue** (web, and from Profile on the TV): the profile's year told in full-screen slides: hours watched, titles and days, the show of the year, the top five shows, the movie of the year and the busiest day and month. `GET /api/watch/year`.
- **Voice search** (Google TV, Android TV): Speak on Search, and the remote's search or mic key opens Search listening.
- **The artwork comes alive**:
  - **Living banner** (web on a computer, Fire TV, Google TV): rest on Home's banner and, after a ring fills, the title's trailer fades in muted behind its logo while the story steps aside; on the web a button turns the sound on. It stops when you move on, scroll away or switch tabs.
  - **Cards with depth**: on the web a card tilts toward the mouse with a glare that follows it; on the TV the highlighted card sways gently in depth. Continue Watching cards float the title's logo in front of the picture.
  - **Colour from the picture** (all apps): Cue reads the main colour of the artwork in view and tints the glow behind the page, the highlight's halo and the progress bars with it.
  - **Screensaver** (web on a computer, Fire TV, Google TV): after three minutes untouched, Cue drifts through your own titles full screen (new episodes, where you stopped, your list, what's trending) with a clock; any button brings you back.
  - **Previews everywhere**: a title's page plays its trailer behind the words after a moment (web, Fire TV, Google TV), and on the web resting the mouse on a card plays its trailer in the card. When a title has no trailer, Cue shows the real thing: on the web, frames from the episode or film fading one into the next; on the TV, a muted clip of it (for something you're watching, from just before where you stopped). `GET /api/thumb` takes `w=1280` for the larger frames.
  - The server answers `GET /api/watch/extras/{kind}/{tmdbId}` with a title's logo, main colour and trailer.
- **Streaming quality, per profile**: the profile menu has Streaming quality, as in apps like Nuvio: the picture to aim for (4K, 1080p, 720p), how big a file may be (Smart: the size streaming services use; up to 10-50 GB for a two-hour film; or any size), Dolby Vision (prefer, allow, avoid), and whether to prefer HDR, skip Remux, prefer HEVC and AV1, and prefer Dolby Digital Plus or Atmos. By default Cue picks a 4K version at a streaming size instead of the biggest file an add-on lists first.
- **TV players**: the Fire TV and Google TV player has a rounded control panel with the time, speeds (0.5x to 2x) and icon buttons; on Apple TV, Apple's own player gets Skip Intro, Next Episode, speeds and Other Version.
- **A frame of the video while scrubbing**: moving along the time bar shows the picture at that point, in the browser (hover or drag) and in the TV app (holding left or right), recordings included. Cue takes the frames from the file itself, every 10 seconds, and keeps them a while.
- **Quicker Play and a Download**: the first title on Continue Watching (and any card pointed at) is looked up ahead, so Play starts at once; the player's Versions menu has Download, to save a version for offline. On the TV, Back returns the highlight to the card you opened.
- **Record Live TV**: Record on any show in the guide (or on now) and Cue records it on the server, even with nothing open, then keeps it 30 days in a new Recordings group, to play in the browser or the TV app.
- **Follow a team or show**: Follow on a Live TV search ("Packers") and every game or episode in the guide gets a reminder by itself.
- **Channel list on the TV**: in the TV app's live player, OK (or left) opens the channel list over the picture, with what's on each; up and down, OK to switch.
- **Subtitles in Watch**: a CC button in the player finds English subtitles on OpenSubtitles for whatever is playing (the one made for that release first), with text size, a dark band behind them, and "Try another" for one that's out of time; the TV app's Subtitles button finds them too, and they show from the start when they're on. Each is downloaded once and kept.
- **Skip intro**: a Skip intro (and Skip recap) button in the browser and the TV app, and the next episode offered when the credits start. Times come from TheIntroDB, and otherwise Cue learns them from what your household skips: jump over an intro once (or press Next during the credits) and the rest of the season gets the button.
- **A smarter home screen**: "Because you watched…" rows from each profile's recent titles, a New episodes row for shows you follow with an episode out in the last two weeks (and a New episode badge on Continue Watching), and title pages show the streaming services a title is on.
- **Plays in any browser**: when a browser can't open a version (MKV or AVI, Xvid or HEVC pictures, Dolby or DTS sound, or silent), Cue converts it as it plays with ffmpeg (now in the image), copying the picture when it can and making English stereo sound; moving through the video carries on from that point.
- **Colors**: each profile picks its own theme in the profile menu: a color (eleven, or any color on a phone or computer) for buttons, focus and the logo, a glow (off, soft or strong) and a background (midnight, black, slate, or tinted with the color). It follows the profile to every device, the TV app's player included.
- **TV app**: **Other version** in the player, and a version whose sound the TV can't play moves on to the next one by itself. Dolby (AC3, E-AC3, TrueHD) and DTS sound now play on any TV, decoded by the app. The player's text is sized by the screen, not the TV's font setting, and it shows the picture's real resolution.
- **Hard-to-find shows**: when the add-ons and torrent sites have nothing, Play looks on the Internet Archive (plays in Cue's player) and, with a free YouTube key in Settings > Streaming, for full episodes and films on YouTube (plays in YouTube's own player). Title pages and the player show free services a title is on, like Tubi, Pluto TV and The Roku Channel. See [docs/watch.md](docs/watch.md#hard-to-find-shows-internet-archive-and-youtube).
- **Live TV catch-up and reminders**: past shows on channels your provider keeps are marked in the guide (with Earlier and Later to move through it) and play from their start, a show on now has From the start, and Remind me on a show that hasn't started pops up a card with Watch when it does, on any device on that profile.
- **Live TV like a TV box**: the channel playing stays in a window over the guide with what's on beside it; flip channels from the guide, go full screen and come back without it stopping. Search finds games and shows on now and coming up.
- **Cue on a cloud server**: one command on a rented server (Ubuntu or Debian) installs Cue with an HTTPS address (your domain, or a free one made from the server's IP), FlareSolverr, and a nightly update, so the TV app and phones work from anywhere. The image is built for Intel and ARM servers on every change. See [docs/cloud.md](docs/cloud.md).
- **Phones**: a tab bar at the bottom (Home, Shows, Movies, Live TV, My List), Cue opens full screen when added to the home screen, and the player has **Open in Infuse** and **Open in VLC** (iPhone) or **Open in VLC** (Android) for videos the phone's browser can't play.
- **Live TV**: add your IPTV provider's Xtream Codes login in Settings > Streaming, and Watch gets a Live TV tab with every channel and its logo, a TV guide grid (now and the next hours, from the provider's guide), the provider's channel groups, and Favourites per profile. Channels play on the TV app straight from the provider, with up and down changing channel, and in browsers and on phones through Cue. See [docs/watch.md](docs/watch.md#live-tv).
- **Streaming only**, on by default: Cue opens on Watch, Settings shrinks to Streaming, Torrent sites, Movie info, Accounts and System, and the library manager is hidden with its automatic searches resting (one switch brings it back). Settings > Streaming gathers Premiumize, stream add-ons, a picture-quality cap (best, 1080p, or 720p to save data) and that switch. See [docs/watch.md](docs/watch.md#streaming-only).
- **Profiles**, Netflix style: one sign-in for the household, **Who's watching?**, and a profile per person with its own Continue Watching, My List and progress. Profiles can be locked with a 4-digit PIN, and only the main profile can change settings or manage profiles. See [docs/watch.md](docs/watch.md#profiles).
- **Stream add-ons**: paste a Stremio add-on link (Comet on ElfHosted, Torrentio, MediaFusion) in Settings > Downloading, and Play asks it for streams first, with its own scrapers and sorting. Links set up with your Premiumize key play straight away; Cue's own search is the fallback. See [docs/watch.md](docs/watch.md#stream-add-ons-comet-torrentio-mediafusion).

### Changed
- Horror films, anime and adult titles no longer show by themselves: they're kept off Home's banner, the preview clips and the screensavers (web, Fire TV and Google TV). You can still find and play them as before.
- **A friendlier, lighter front page**: the demo clips and the phone's "search on the TV" now feature Wildwood instead of Dune (no more grim close-ups), and the poster wall is 24 brighter posters instead of 30. It also loads less: posters are WebP (592 KB to 328 KB), the 3D phone model is 2.3 MB instead of 3.8 MB and is sent compressed (about 1.3 MB), the wall's rows slide as one layer without per-poster shadows and pause once scrolled past, and the commercial downloads only when played.
- **Preview clips are ready sooner**: a clip is cut from a 1080p (or smaller) version, preferring H.264, rather than a 4K one, which is several times less work for a small server (what Play picks is unchanged). Resting on a card for a second on the web starts its clip, so it's often ready when you open the title. A clip for a title you opened and left is still made later instead of dropped. Overnight, each opened title's first few More Like This titles are made too, along with everything on Home.
- **Smoother web app, like the TV**: opening a title grows it into place, Back slides the page you came from in from the left, and switching sections rises softly. The player fades up from black and fades back out when you leave; its controls glide in and out, play and pause bloom softly, its menus open from their button, and buttons dip when pressed. Reduced-motion settings get plain fades.
- **Previews play once**, then the title's picture comes back (web and TV), instead of looping.
- **The screensaver shows things you haven't watched too** (TV and web): trending titles, "because you watched", and the rest of Home, mixed in with your own (where you stopped, new episodes, your list) and shuffled each time.
- **Home on the web, full-bleed**: the banner is no longer a card in the margins; the title's picture runs edge to edge under the header and fades into the page to the left and below, with the first rows sitting over its faded foot, as on a title's page and in the TV app.
- **The TV app follows your theme** (Fire TV, Google TV): the profile's Colors from Watch (its colour, glow and background: Midnight, Black, Slate or Tinted) now colour the whole app, the backgrounds, panels, fades and highlights too, and a change made on the web shows the next time Cue comes to the front.
- **Opening the TV app**: the Cue logo grows into place with a soft glow and "cue" slides in beside it (it was the word in plain text), then fades into sign-in or Home; each of sign-in, "Who's watching?" and Home fades into the next.
- **Cleaner buttons on Fire TV and Google TV**: every button the same frosted glass with white words; only the one the remote is on turns solid white and grows a touch (no coloured ring). Seasons and channel groups are plain tabs, the chosen one with a short line under it, so the highlight and the choice never look alike.
- **Smoother everywhere on Fire TV and Google TV**: opening a title zooms it in over Home and Back zooms it away; the player fades in and out; its controls glide up as they appear; pausing dims the picture and the pause mark pops in softly instead of snapping on.
- **New buttons** (Fire TV, Google TV): frosted glass with a fine edge and real icons (Play, My List); the one the remote is on turns white, lifts and gets a soft ring of light in the profile's colour.
- **Home on Fire TV and Google TV, as on a title's page**: the highlighted title's picture fills the screen behind the tabs and rows, fading into the page, with its logo and story on it, instead of a banner in a card. The blurred picture behind Home is gone too (costly on a TV).
- **A faster TV app**: it's now optimized when built (R8), and installs Compose's ready-made startup profiles itself, which an APK installed outside the app store otherwise goes without, so screens and scrolling are smooth from the first use.
- **Previews are ready before you open a title**: everything Home shows gets its clip made while nobody is using Cue (no one signed in has done anything for half an hour: overnight, mostly), one after another while the server is quiet. Clips are 12 seconds now (twice as quick to make, about 2–3 MB each), so one not made yet appears sooner.
- The cloud image built from every change is for Intel/AMD servers only, so it's ready in about half the time; releases still build for ARM too.
- A family member's menu in Watch shows **Your account** (their own name and password) instead of Settings.
- **Downloader code 3575368** (cuetv.me/tv.apk) on the front page's Fire TV steps, so the app installs by typing a short number.
- **Front page, what's in it**: the parts that make Cue different: every movie and show with no subscriptions, searching the TV from your phone (the 3D phone typing "Dune"), trying another version of a bad copy, skipping the intro, live TV with a guide, your year, and every screen it runs on.
- **Sign out** is in Watch's profile menu (it was only in the library manager's).
- **A 3D phone on the front page**: a real 3D model of a phone with Cue playing on its screen, turning gently toward the pointer (three.js, loaded only on that page; a drawn phone shows until it's ready, or where there's no WebGL or less motion is asked for). The security policy now lets a page read its own `blob:` data (`connect-src 'self' blob:`), which the model's packed pictures need.
- **Preview clips stay out of the way** on small servers, and the one you're looking at comes first: clips are made one at a time at the lowest priority with the cheapest encoding; the title someone has just opened goes to the front, ones they've left are dropped, and the ones Home prepares ahead of time wait for a quiet server (load under 70% of its cores, at most 12 an hour per core). Pages wait longer for a clip being made (up to three minutes on the web).
- **Cue's CPU** on the server page now includes the programs Cue runs (ffmpeg converting or cutting previews), so it shows what Cue really uses.
- Posters (the tall movie and show cards) no longer play a preview when the mouse rests on them; the wide Continue Watching cards still do.
- **A simpler front page** (web): just the headlines, the clips, the ad and how to get it: no explaining lines under each part, and no example family profiles.
- **Home's banner comes alive on the TV** (Fire TV, Google TV): rest on a title and its preview clip (the small one your server cuts) fades in, muted, while the story steps aside. A TV asks only for the title it rests on, and the server makes at most a few at once.
- **Switch profile** (Fire TV, Google TV) always shows "Who's watching?", even with one profile, and it now has **Sign out**, so someone else can sign in with their own account on that TV.
- **Previews are Cue's own** (web): the banner, a title's page and a card the mouse rests on play a short clip of the title itself, cut by your server and kept, instead of a YouTube trailer, so no play buttons or YouTube titles show and they start at once. Home's banner and first Continue Watching titles are prepared ahead.
- **Sound on a title's page** (web, Fire TV, Google TV): as on Netflix, opening a show or movie plays its preview with sound, rising gently (a button in the corner mutes it on the web); Home's banner and cards stay quiet.
- **A cleaner, smaller player** (Fire TV, Google TV): the technical line under the title (frames, Mbps, loaded ahead, decoder) is gone, leaving the episode and picture size, and the bottom bar is about a quarter smaller, with icons sized by the screen so a Fire TV no longer draws them larger.
- **Continue Watching and new episodes open their page** instead of playing as soon as they're chosen, in every app; Resume is there, with the episodes.
- **Live TV in the new look, everywhere**: on the web, a glow of the profile's colour behind the page, rounded frosted panels, the groups as pills, programmes as rounded tiles that light up, a red LIVE pill and a red line at now; on Fire TV and Google TV, a banner for the highlighted channel (what's on, how far through, what's next) and channel cards that lift and catch the light like the posters.
- **Moving between sections**: on the web, each section fades in as it rises, a line slides under the section you're on, and the phone's tabs pop when tapped. On phones and tablets a tapped card no longer stays lifted (it dips while pressed instead).
- **Apple TV**: titles drawn with their logo artwork on Home's banner and title pages, with IMDb, Rotten Tomatoes and Metacritic scores and who directed or created it.
- **Watch on the web, the same look and motion**: titles drawn with their logo artwork; Home's banner a rounded card, its picture glowing softly behind the page and drifting slowly; names under every card, with the episode's code on Continue Watching pictures; a title's page with IMDb, Rotten Tomatoes and Metacritic scores, who directed or created it, and pill buttons with a frosted back that turn white when pointed at; a card's picture lifts on a soft curve with a deeper shadow and a light that sweeps across it once, its name easing out of the way; a page's words fade in as they rise, one after another. Devices set to reduce motion get fades only.
- **Fire TV and Google TV app, a new look**: titles are drawn with their own logo artwork; Home has a rounded banner card that follows the highlight, with the picture of what's highlighted glowing softly behind the whole screen; Continue Watching cards carry the episode's code on the picture and the show and episode's name underneath; a title's page fills the screen with its picture, IMDb, Rotten Tomatoes and Metacritic scores, who directed or created it, and pill buttons that turn white when highlighted; motion as on Apple TV: a highlighted card springs toward you with a deepening shadow and a light that sweeps across it once, lists glide into place, the banner's words fade and rise in while its picture drifts slowly, a title's page settles in from a slight zoom with its parts arriving one after another, and pages cross-fade between tabs; one 48dp margin and the same corners and gaps on every screen; Cue's fonts throughout. Cue looks up the logos on TMDB (`GET /api/watch/logo/{kind}/{tmdbId}`, and `logoUrl` on title pages and the Home banner).
- **No more version switching mid-show**: Cue still tries the next version by itself when one won't start, but once the picture has played it never switches on its own. A stream that drops is picked up where it was (twice at most), then the player says so and leaves Other version to you. In the browser and the Fire TV / Google TV app.
- **The last Mediarium names are now Cue**, and nothing you set up stops working:
  - Settings in the container's environment are `CUE_...` (for example `CUE_PAUSE_AUTOMATION`); the old `MEDIARIUM_...` names still work. Scripts after imports get both `CUE_...` and `MEDIARIUM_...` variables.
  - The compose files, guides and app store packages use `cue` for the service and container, `/opt/cue` and `cue.service` (with `/etc/cue/cue.env`) for a Linux install, and `cue_<version>_<os>_<arch>.tar.gz` for the release files. An existing container called `mediarium` keeps running as it is.
  - The recycle bin in each library folder is `.cue-trash`; an old `.mediarium-trash` is moved into it the first time it is opened, with everything in it. Saved backups are `cue-backup-*.zip`; old `mediarium-backup-*.zip` files are still listed, kept and restored. Downloads paused before the update carry on.
  - You stay signed in: the sign-in cookie is now `cue_session`, and the old one works until you next sign in. This browser's own settings (theme, sidebar, library view, reader and player) carry over.
  - Plex, Jellyfin and Emby still see the same device, and calendar apps the same events, so nothing has to be signed in or added again there.
  - **Prometheus metrics** are renamed from `mediarium_*` to `cue_*` (`cue_up` and so on). Update any dashboards or alerts that use the old names.
- **Updates and installs come from Cue**: the update check and **Update now** look only at Cue's own releases (`jhipsman/cueapp`), signed with Cue's own key, and the install guides and compose files use Cue's image, `ghcr.io/jhipsman/cue`. Builds of `main` are also published as `ghcr.io/jhipsman/cue:edge`.
- **Mediarium is now Cue**, with a new icon (a C around a play button). Settings, the library, your downloads and everything else carry over as they are.

### Added
- **Watch**, a streaming-service view of everything: a banner and rows of artwork (Continue Watching, My List, trending, popular, genres), search across every movie and show, title pages with backdrops, IMDb and Rotten Tomatoes ratings, cast and More like this, and for shows a season picker with episode thumbnails and descriptions. Play works on any title, in your library or not, resumes where you stopped (saved per account), and offers the next episode at the end. Open it with **Watch** at the top of the page. See [docs/watch.md](docs/watch.md).
- **IMDb ratings** in Watch, with a free OMDb key in Settings > Info, lists and subtitles.
- **Play.** With Premiumize set up, a **Play** button on every movie and episode starts it straight away, streamed from Premiumize, with nothing downloaded to your server. Cue searches your indexers, ranks the releases with the title's quality profile, checks which ones Premiumize already has in one go, and plays the best of those. If a version won't play, the next one starts by itself. For apps, the same answer comes from `GET /api/play/movies/{id}` and `GET /api/play/series/{id}/{season}/{episode}`. See [docs/downloads.md](docs/downloads.md#play-streaming-from-premiumize).
- **Premiumize.me as a downloader.** Add your Premiumize API key in Settings > Downloading > Usenet and torrents, under Cloud downloader, and torrents (or Usenet, or both, your choice) are downloaded by Premiumize on its own servers instead of by the built-in client. Releases Premiumize already has are ready at once. Cue then fetches the finished files over HTTPS, picks up where it stopped after a pause, and removes the transfer and its files from your Premiumize cloud once they are home. Torrents fetched this way need no VPN and don't seed. See [docs/downloads.md](docs/downloads.md#premiumize).

### Fixed
- Progress bars (Continue Watching) are in the profile's theme colour on the web and the TV; they took the colour of the picture, which mostly came out orange whatever theme was chosen.
- Back from a title opened from More Like This goes straight back where you were (Fire TV, Google TV), instead of through every title on the way.
- Home on Fire TV and Google TV: a highlighted row's title ("Trending movies") no longer slides under the banner, and rows and posters glide for a shorter time, which stutters less on slow TVs.
- Previews play the **English** sound when a version has several languages (they took whichever came first, sometimes a dub); clips made before are made again. On Fire TV and Google TV a title page's moment prefers English too.
- Family members (not administrators) can add, change and remove their own account's profiles, pictures included, from its main profile; it said only an administrator could.
- The tour on Fire TV and Google TV: the remote now moves between its buttons instead of the screen behind it.
- **Smoother edges on the web**: cards draw their faint edge inside the picture (a bright ring outside it showed stair-steps and a seam as cards lifted), and rounded panels keep their background inside their border, so corners no longer show a lighter, stepped halo.
- **Trailers on the web start sooner and never show YouTube's buttons**: they begin loading half a second after the banner settles (not two and a half), sit a little further zoomed past YouTube's title bar, and hide whenever YouTube pauses, ends or reloads, when it shows its play button. On the TV a title's page starts its clip after 0.8 s instead of 2.
- **Sharper pictures on the TV**: Home's banner loads a full-width picture (it was a 780-pixel one stretched almost twice over) and a show's episode tiles load 780-pixel stills instead of 300-pixel ones, so pixels no longer show. Every card has a faint smoothed edge, so rounded corners don't look stair-stepped as cards lift.
- **Subtitles on the TV can be turned off**: Subtitles now opens a list straight away (Off, the video's own subtitles, Find English subtitles), chosen with one press, and the choice carries on to the next episode. Before, pressing it went off to find English subtitles, so subtitles already showing couldn't be switched off, and a video's "default" subtitles switched themselves on every episode (forced ones, for lines in another language, still show).
- **Fire TV, Google TV feel smoother**:
  - A highlighted card glides up instead of bouncing, no longer sways, and its shadow doesn't redraw every frame. A change of colour no longer rebuilds every card on screen.
  - Home's banner holds still and changes only once the highlight rests, and its old title has faded before the new one shows, so two titles never overlap.
  - Moving along the tabs no longer builds each page as it passes, and pages no longer overlap as they change (My List's "Nothing here yet" showing over Home).
- **Previews on the TV**: a title's page plays a muted moment of the episode or film itself, in the TV's own player, instead of a YouTube trailer (which showed YouTube's logo, half-filled the picture or went black on Fire TV). It also gets the stream ready, so Play starts sooner.
- **A title's page on the TV** opens with its logo, scores and story in view: it no longer scrolls Play to the top of the screen (cutting the title off), and scrolls only as far as the highlight needs.
- **Cue's screensaver on Fire TV** stays on: the TV is kept awake while it shows, so the Fire TV's own screensaver no longer replaces it. After an hour Cue lets the TV sleep as usual.
- **Search on the TV** is redesigned: the field and the ways in on the left (Speak, the phone code, recent searches), and what's popular until something is typed.
- **Fire TV, Google TV**: the controls no longer pop up by themselves near the end of an episode. From the credits (or the last 30 seconds) a small **Next episode** button shows in the corner for a few seconds instead, like Skip intro; OK plays the next episode, and it stays in the controls.
- **Next episode offered too early** (all apps): an episode's credits are now taken to be four minutes at most. A credits time further back (Next once pressed with 9 minutes to go, or TheIntroDB's time for another cut) is no longer learned or used, and one already saved is ignored, so the offer no longer comes up mid-scene.
- **Fire TV, Google TV**: the banner's preview is smooth (the picture fades away over it instead of the video fading in), shows the whole frame in a 16:9 window instead of a cropped strip, and a title page's preview is no longer cut off. The no-trailer clip no longer shows through the app as a hole while it loads.

## [2.1.9] - 2026-10-07

### Fixed
- **Banner trailers showed YouTube's "Video player configuration error" (153)**: the page sends no referrer, and YouTube's player won't start without one. The trailer now sends the site's address only, and shows only once YouTube says it's playing; one that can't play leaves the banner's picture as it is.
- **Skip intro no longer gets stuck**: it shows for a few seconds as the intro starts and then gets out of the way (it comes back with the controls, or when the mouse moves), so OK pauses again; intro and recap times over three minutes long are treated as wrong and ignored.
- **The next episode no longer starts early**: at the credits (whose times from TheIntroDB or learned skips can be minutes off) the player now only offers the next episode, as a button (Fire TV / Google TV) or an Up next card (web); it plays by itself only once the episode has really ended.
- **OK pauses and plays on Fire TV and Google TV** with the controls showing too: the highlight rests on the time bar, where OK used to do nothing, so pausing meant moving over to the pause button.
- **Phone rows**: on Continue Watching a movie (no episode line under it) sat lower than the shows beside it, and on touch screens the rows' scroll arrows stayed showing as dark strips over the cards once a row was touched. Cards now keep their own height with the picture at the top, and the arrows are left to mice. The New episode badge is a small chip like the episode's code.
- Adding a media server by pasting the address from your browser (`https://host/web/#/home` for Jellyfin, `.../web/index.html#!/home` for Emby and Plex) failed. The part that only opens the server's web app is now dropped, so it becomes `https://host`; an address with a folder before it, like `https://host/jellyfin/web/#/home`, becomes `https://host/jellyfin` (suggested by msholly on GitHub, #29).

## [2.1.8] - 2026-10-07

### Fixed
- Torrent sites added through Prowlarr or Jackett as a **Usenet** indexer (an easy mix-up, since both kinds of feed look the same) failed with "couldn't read the NZB file". Results now say for themselves that they are torrents, from what the feed sends, so they download as torrents whichever tab the indexer was added on. If a torrent file or magnet link still reaches the Usenet downloader, the error now says to add that indexer again on the Torrent tab, and the release is no longer blocklisted for it (reported by Apostol6 on GitHub, #25).

## [2.1.7] - 2026-10-07

### Added
- **Import existing** can show just one kind of row in the review table: Unmatched, Check match, Matched or Already in library, with a count on each (asked for by PauloJf on GitHub, #24).

### Fixed
- Scanning a folder again after importing it showed titles you had matched by hand as unmatched, and never recognised shows as already imported. A scan now recognises files already in the library by their location and lists them as **Already in library** under the title they belong to (reported by PauloJf on GitHub, #24).
- The check for folders Cue can't write to inside a library folder no longer looks inside the filesystem root or system folders, and remembers fewer results at once.
- In the setup wizard, a site setting with a checkbox (1337x's "Disable sorting", for example) stretched the checkbox across the form and squeezed its label into a column one letter wide (reported by PauloJf on GitHub, #23).

## [2.1.6] - 2026-10-07

### Added
- Paste your WireGuard `.conf` file (or choose it) under **Settings > Downloading > VPN protection** and every field is filled in for you, including the preshared key, DNS servers and allowed IPs. Windscribe is in the provider list (asked for by PauloJf on GitHub, #21).

### Fixed
- The VPN form had no field for a WireGuard preshared key, so configs that use one (Windscribe's, some self-hosted servers) never connected. It's there now, and stored encrypted like the private key (reported by PauloJf on GitHub, #21).

## [2.1.5] - 2026-10-07

### Added
- File names understand Radarr's and Sonarr's tokens, so a format copied from either works as it is, for example `{Movie CleanTitle} ({Release Year}) - {Custom Formats}{ - Edition Tags}`. That includes text inside the braces that only shows when there's a value (`{ - Edition Tags}`, `{[Quality Full]}`), dotted names (`{Movie.CleanTitle}`), Plex's `{edition-{Edition Tags}}`, quality, media info and ids (asked for by u/Wiwer on Reddit).
- A file name builder under **Custom** naming: start from a ready-made format (including *Detailed, like Radarr and Sonarr* and *Plex with editions*) or paste your own, click tokens to add them, and see a preview of both a movie and an episode as you type. Episodes now have their own custom format.
- The preview says what's wrong with a format (an unknown token, a missing brace, no season or episode) while you type.

### Fixed
- Usenet releases posted with only PAR2 recovery volumes (`.vol01.par2` and so on) and no main `.par2` failed with "no PAR2 files to repair it" and were blocklisted, though they could be repaired. Cue now repairs from one of the volumes (reported by TryToTilt on GitHub, #20).

## [2.1.4] - 2026-10-07

### Fixed
- Obfuscated Usenet releases (random file names) failed PAR2 repair and were blocklisted, even when nothing was wrong with them. Cue now gives the files their real names back from the PAR2 data before repairing, and hands par2 every file in the download (reported by u/Wiwer on Reddit).
- A failed repair now says in one line why ("it needs 12 more recovery blocks than the release has") instead of pages of par2 progress output.
- A download is no longer started when Cue can't write to the folder the file will go in; the reason names the folder. Before, the file was downloaded in full and then failed to import (reported by u/Wiwer on Reddit).
- Setup, Settings and the dashboard warn about folders inside Movies and TV that Cue can't write to, such as ones another app created under a different user, with an example path.
- Obfuscated releases with many PAR2 files are verified once, not once per file.

## [2.1.3] - 2026-10-06

### Fixed
- A Torznab or Newznab address pasted with `/api` on the end, as Prowlarr and Jackett show it (for example `http://192.168.1.10:9696/1/api`), didn't work: Cue added its own `/api` and got Prowlarr's web page back. Both forms work now, and so does an address pasted with `?t=...&apikey=...` after it.

## [2.1.2] - 2026-10-06

### Fixed
- Sites from the definition list (UTOPIA, for example) that write their dates differently from what their definition expects failed every search with "could not read the results … dateparse". Such a date is now read as any common date format, as Prowlarr does.
- Adding Prowlarr's main address (or any web page) as a Torznab indexer gave a puzzling "parse newznab response … `<html>`" error. It now says the address gave back a web page, and how to copy an indexer's feed address from Prowlarr.

## [2.1.1] - 2026-10-06

### Fixed
- Outside your download hours, Activity said "Started downloading" for a grab that was in fact waiting for the hours to come round.
- Scripts after imports: a script stopped at its time limit now takes everything it started with it, and anything it leaves running in the background is stopped when it ends. **Try it** answers straight away when a run after an import is still going instead of waiting behind it.
- Subtitle timing: "Put the original back" could bring back an older, different subtitle after a new one was downloaded; and checking for the original left a file open.
- Books that aren't out yet no longer use up the automatic searches meant for missing books, a book announced for a later year is no longer searched for straight away, and a followed series keeps its books' release dates up to date when one is postponed.
- Dates such as "out on" and "unaired" use your own time zone, not UTC, so they no longer flip a day early or late in the evening.
- Audiobooks: chapters of very long books (over about 60 hours) are read again, and a damaged M4B can no longer keep the server busy.
- A Kindle book is no longer converted twice when the library folder is reached through a link.
- Kindle books: a damaged or hostile MOBI/AZW3 file could make the converter use up all memory and stop Cue; sizes are now capped. Books with many internal links (footnotes, big omnibus editions) convert many times faster.
- Daily shows were almost never found: the year in an air date ("Show.2024.03.15") was taken for the show's year and the release was turned down.
- Anime and daily shows: an automatic grab of one episode was queued as a whole season, which blocked the rest of the season. "Show S2 - 05" names are now read as one episode, a "12.5" recap is no longer taken for episode 12, and a site tag in front of a release name no longer replaces the real release group.
- Shows moved over from Sonarr keep Sonarr's series type, and shows from Overseerr or Jellyseerr requests get theirs guessed like any other.
- Specials: 2.1.0 let you switch a special on, but Cue couldn't download it, so it stayed missing and used up searches. Specials are now listed for reference only, with no Search button or Monitored box, until they can be looked for properly.
- Family accounts that have to ask could still add titles without a request: by picking a release in Search for something not in the library, or by following a book series or an author. Both now need "Add titles themselves" (the Follow switches are hidden without it).
- Requests: Approve, Decline and taking a request back no longer step on each other when clicked at the same time; two titles with the same name (Dune 1984 and 2021) are no longer treated as one request; a book request needs ebook or audiobook; a request for a media type that has since been switched off waits instead of being marked approved; and a music request for "Only this album" now adds and looks for that album when approved.
- Cleanup rules (off unless you switched them on) could remove things you still wanted. Now: "nobody watched" counts from when a movie arrived, not from when you first asked for it; a title downloaded again after it was watched is kept; nothing runs unless every media server answered; a media server that doesn't know when something was played no longer makes it look watched in 1970; cleaning an episode removes only that file, never the show's folder; a file with several episodes goes only when all of them match; a problem reading tags stops the run; the oldest go first; and "Remove these now" removes exactly what the preview showed.
- Security: sign-in through a reverse proxy believed the user-name header from any address on your home network, so another device could sign in as any account by sending it. It now needs your proxy's own address and believes the header only from there. If you use it, open Settings > Accounts through your proxy and save it once more.
- The Kindle converter ignores impossible positions inside a book instead of trusting them (found by GitHub's code scanning; no effect on 64-bit systems).

### Changed
- The standard Docker image runs a tiny init process (tini) first, like the full image already did, so processes a post-import script leaves behind are always cleaned up.
- Updated source-map-js, a library used only while building the web interface, for a security fix. The app itself is unchanged.

## [2.1.0] - 2026-10-06

### Added
- Run a script after each import: put your own script in the config folder's `scripts` folder and pick it under Settings > System. It is told what was imported in `CUE_*` variables, runs one at a time with a time limit, and can be tried from the page. Off until you pick one, and it can't be chosen or run with an API key.
- Subtitles made for your exact file: searches send the video's fingerprint, so a subtitle timed against that very file is marked and picked first. With automatic downloading on, Cue also looks again for a month and swaps an earlier pick for one made for the file when it turns up. Hand-picked or edited subtitles are never touched.
- Watch statistics: with What's been watched on, the Statistics page shows how much of your movies and episodes get played, the space held by what nobody has watched, and the most watched and latest watched titles.
- Specials: a show's season 0 is listed as Specials, unmonitored to start with, so you can ask for the ones you want. Existing shows get theirs at the next refresh.
- Anime and daily shows: a show's page has Episode numbering (seasons, anime episode numbers counted from the start, or air dates), set by itself when the show is added. Releases like "[Group] Show - 105" and "Show.2024.03.15" are searched for, picked and imported to the right episode.
- Subtitle timing: move a subtitle earlier or later, or line it up automatically with another subtitle of the same title that is in time (frame-rate drift included), from the Files panel. The original is kept, and Put the original back undoes it.
- Download hours: let new downloads start only between two hours (for example at night). Outside them they wait in line and start by themselves; a running download finishes. Settings > Downloading > Usenet and torrents.
- Permissions for basic users: choose, when adding or editing the account, which types of media they can add, and whether they can add titles themselves, pick releases, start searches and change monitoring, retry downloads, get subtitles and play, read and listen. Everything is on by default.
- Requests: a basic user who can't add titles themselves sends a request instead. Admins approve (the title is added for them, as they chose) or decline it with a note under Activity > Requests, and can be told about new ones by notification.
- What's been watched: Cue can read play counts from Plex, Jellyfin and Emby every six hours and show them on movie and show pages. Off until you switch it on under Settings > Connections > Media servers.
- Cleanup rules: remove movies or episodes watched a while ago, or movies nobody watched long after they were added, with a preview first, tags that keep a title safe, the recycle bin, and every removal in Activity. Off by default.
- Book series. A book's page shows the series it belongs to, every book in it in reading order, and a **Follow this series** switch that adds the missing books now and new ones as they appear. Books that aren't out yet show their release date, and Cue waits for that day before looking for them.
- Books on the calendar: books with a release date show up on Upcoming > Calendar and in the calendar feed, with a Books filter. On Wanted, a book that isn't out yet shows the day it comes out instead of Search now.
- Audiobook details: who reads it and how long it runs, from Audnexus (Audible's catalogue), on the book page and in the player. The full unabridged reading is preferred over translations and dramatisations.
- Chapters inside one-file audiobooks: an M4B's own chapter marks show in the player's list, with previous and next chapter, a slider per chapter and End of chapter on the sleep timer.
- Kindle books (MOBI, AZW, AZW3) open in the built-in reader: Cue's own converter turns them into EPUB the first time, in well under a second, and keeps the copy in its cache folder without touching your library.
- An optional Hardcover token (Settings > Info, lists and subtitles) for more book series and release dates. Without one, Open Library is used, as before.
- Sign in through your reverse proxy: a proxy that does the login itself (Authelia, Authentik, Cloudflare Access) can pass the user name on in a header, and Cue signs that account in. Off by default, only believed from a trusted proxy, and only for accounts that exist. Set it under Settings > Accounts.
- Email notifications have a **From name** box: the name your emails show as coming from. It starts as Cue, so emails no longer show the part before the @ (such as "nas").

### Changed
- The repository is tidier: the compose files, the container start script and `.env.example` are in `docker/`, and the contributing, security and conduct guides are in `.github/`. The install command now downloads `docker/docker-compose.yml`; the file itself is unchanged.
- Updated the libraries Cue is built with (SQLite driver, Go time package, Vite, oxlint).
- The Synology guide has a step-by-step section for reaching Cue from outside your home with DSM's reverse proxy, and the Synology, Unraid and QNAP guides no longer call ebooks and audiobooks unfinished.

### Fixed
- Tick boxes in settings forms (such as the subtitle languages) sit beside their names again instead of above them.

## [2.0.0] - 2026-10-05

Version 2 adds ebooks and audiobooks, with a reader and player built in, and tags for movies and shows.

### Upgrading

- **More movie and TV folders are gone.** If you added extra folders under Settings > Library, the titles in them stay in your library, but Cue no longer renames, replaces or deletes their files there. Move them into your main movies or TV folder and use Import to pick them up. Tags (for example Kids or 4K) are the way to keep them apart now.
- For scripts using the API: adding a movie or show no longer takes a `rootPath`, and the settings `library.movies_extra_paths` and `library.tv_extra_paths` are removed.
- Nothing else needs doing. The database updates itself on the first start, and your compose file keeps working. To use books, switch them on under Settings > Media types; Cue suggests a folder inside the one you already map and can create it.

### Added
- Ebooks and audiobooks. Switch them on under Settings > Media types, find a book by title or author (book details come from Open Library), pick ebook, audiobook or both, and Cue searches your indexers, downloads the best release (EPUB first for ebooks, M4B first for audiobooks) and files it as Author/Title (Year).
- Discover's eBooks & Audiobooks tab: trending and popular books from Open Library, the most read classics, fantasy, science fiction and mysteries, and a browse by genre, year and order. A banner on each cover says whether an eBook, an audiobook or both have been published, and Add lights up when you point at a book.
- Every book has an info page (description, subjects, editions, Add, more by the author), also from the search box.
- Import the ebooks and audiobooks you already have: Cue reads the folder, recognises Author/Title folders (and Calibre's and Audiobookshelf's layouts), and adds them where they are.
- Cue Books: a built-in reader and audiobook player that opens in its own window and installs as an app of its own. Read EPUBs a page at a time (contents, text size, light, sepia and dark pages, swipe and keys), open PDFs, and listen with chapters, 30-second skips, speed, a sleep timer and lock-screen controls. Your place is saved and carries on across devices.
- A book's page shows more by its author, and you can follow an author so their new books are added by themselves.
- Connect Audiobookshelf and Kavita under Media servers: new books show up there straight away, and a book's page links to it ("Listen in Audiobookshelf", "Read in Kavita").
- The search box at the top finds books, and Upcoming > Wanted lists the books still missing, with Search now.
- Tags on movies and shows (Kids, 4K, anything): set them on a title's page, when adding, or on a whole selection; filter the Library by tag; and every tag shows up as a collection on Plex, Jellyfin and Emby. A tidy way to keep kids' films or 4K apart in one folder.
- A library folder that doesn't exist yet can be created from Settings and from the setup wizard, and when your other folders share one mapped folder (like /data) Cue offers to use and create /data/Ebooks, /data/Audiobooks or /data/Music for you, so no compose change is needed.

### Changed
- The header holds the small links that were at the bottom of the sidebar: the eBooks/Audiobooks Player, your media servers (named Plex, Jellyfin and so on) and, for administrators, the support heart. The search box sits next to the page title. On narrower screens the links show as icons.
- Settings > Media types has bigger cards, each with a short list of what that kind of media does.
- The dashboard's server details now sit in a slim panel inside the greeting card (the greeting takes a third, the server two thirds), instead of a separate strip.

### Removed
- More movie and TV folders. Each kind of media has one folder again, and tags sort it instead (see Upgrading above).
- The Windows and macOS programs. Releases now carry the Linux program only, next to the Docker image. On a Windows PC or a Mac, run Cue in Docker Desktop.

### Fixed
- The "Cue is slow to answer" notice no longer appears when the wait is for an outside service (TMDB, Open Library, MusicBrainz, your indexers). Those lists show their own loading placeholders, and a service that doesn't answer is named in the message.
- A book with a short title is no longer matched to a longer one that contains it ("It" picked up "If It Bleeds"): the title has to stand on its own in the release name.
- Once a book has downloaded, its earlier failed tries leave the queue, as they already did for movies.
- Ebook and Audiobook in the add-book window show a tick when chosen.
- The reader and the audiobook player no longer leave an empty strip down the right edge of the window, and the search box hint is shorter so it fits next to long page titles.

## [1.4.1] - 2026-10-04

### Fixed
- The "Cue is slow to answer" notice no longer appears after the phone or the tab was asleep: a request that waited while the page was out of sight does not count as slow.
- A failed download in Activity has **Why did this happen, and what can I do?**: a plain explanation of the failure (for example a release that is no longer complete on your Usenet provider) and what to try, such as another release, a quality profile that accepts more qualities, or a second Usenet provider.
- Failed tries of a movie or episode clear themselves from Activity once it has been downloaded another way.
- On the dashboard, each recently added title says where it stands: In library, Downloading, Some episodes, Waiting for a release or Not monitored.
- The dashboard's server details are a full-width strip under the greeting with a steady height, so the Movies, TV and Music cards have the whole row and the page no longer jumps as the numbers change.
- The lines under "Coming up" on the dashboard open their movie, show or album.
- In the Library, **Import** and **Add** (shorter names) sit together at the end of the toolbar row instead of Add wrapping onto its own line.

## [1.4.0] - 2026-10-04

A big release about control and safety, with everything kept simple: a recycle bin, nightly backups, speed and space limits, more ways to find and import titles, and many everyday pages reworked.

### Added
- **More than one library folder**: add extra movie and TV folders in Settings > Library, and choose which one a title goes in when you add it.
- **Rename existing files** (Settings > Library) renames files already in the library to match the naming preset, after showing every change.
- A **Statistics** page: library size, titles per quality, and downloads per month.
- Discover has **Select** to pick many titles and add them all at once, and **Not interested** (the x on a poster) to never see a title there again, with a list to undo it.
- **Quiet hours** for notifications: everyday messages wait until the morning; problems are still sent at once.
- **Search releases** (from the Search page) asks all your indexers for a release by name and downloads the one you pick; the movie or show is added to your library if needed.
- **Import a file by hand** (from Activity) lists the video files in the downloads folder with a guess of what each is, and puts the one you choose into your library as a movie or an episode.
- The series page folds its seasons (the one that needs attention stays open), has a row of season buttons to jump around, a **Missing episodes only** switch, and shows the next episode and when it airs.
- The Wanted page folds episodes of the same show into one line, has a search box, **Search all**, and shows 50 at a time.
- The calendar can be filtered (movies, TV, music, not downloaded yet), and **Add to your calendar app** gives a private address to subscribe to in Google, Apple or Outlook calendars.
- The History tab has a search box, **Show older lines**, and links each line to its title.
- The Library has a button to reverse the sort order, and remembers the sort.
- Indexers have a priority (Preferred, Normal, Last resort): between two equally good releases the preferred indexer wins. Each card shows how its last test went, and **Test all** tests them all at once.
- Releases far too small for the quality they claim (fakes and samples) are skipped by automatic searches, and a quality profile can set the **Largest download** in GB. Profiles can be duplicated.
- A torrent that gets no data for 2 hours is treated as stalled: it is stopped, blocklisted, and another release is tried.
- A download speed limit for Usenet and torrents together, optionally only between two hours (for example full speed at night), and a free-space floor: with less than 5 GB free in the downloads folder no new download starts until there is room. Both are in the new **Speed and space** box under Settings > Downloading.
- Automatic backups. Every night, and before every update, Cue saves a backup in `/config/backups` and keeps the newest 7. Settings > System > Backup lists them with a Download button, has **Back up now**, and lets you switch the nightly backup off or keep more.
- A recycle bin. Removing a title with its files now moves them to a hidden folder in the same library folder for 7 days. **Activity > Recycle bin** shows them, with **Put back** and **Delete now**. The number of days is under Settings > System > Clean up (0 deletes straight away, as before).

### Changed
- Notification emails have a new look: wider, with the full Cue logo on top, a big poster on one side, and the facts stacked on the other with bold titles. The logo travels inside the email, so nothing is loaded from the web except the poster, which is now sharper.
- You can delete an API key after you have revoked it. Revoked keys no longer pile up in the list.
- The test button on an email notification sends a real-looking email, using a title from your library, so you see the design you will get.
- When the update check fails, the Updates box now says why (for example that GitHub is limiting requests).
- An address that does not exist in the app shows a "That page doesn't exist" page with links, instead of jumping to the dashboard.
- Cue can be added to a phone's home screen and opens like an app.
- Finished library imports have a "Dismiss all" button when there is more than one.
- The Audiobooks and Ebooks tiles are gone from the dashboard until they are ready.
- The dashboard numbers no longer count up when your system asks for reduced motion.

## [1.3.1] - 2026-10-01

### Fixed
- A release that can never be completed no longer wastes a whole download. If articles are missing on every Usenet server and the release has too few PAR2 repair files to rebuild them (or none at all), Cue stops straight away, blocklists that release and moves on to the next one. Before, it downloaded everything and only then found out it could not be repaired.
- Logs and errors groups the same problem for the same movie, show or album on one line for 24 hours, even when different downloads caused it. A title that keeps failing is one line with a count, not a page of lines. The log keeps at most 2000 problems now (it was 5000).
- Emails no longer end with "This message is about...". They end with one plain line that says where to choose which emails you get.
- On Discover and on a title's page, the rows of posters no longer leave a big empty gap before "Load more".

## [1.3.0] - 2026-09-30

First public release.

### Movies and TV
- Add a movie or a show from search or Discover. Cue finds the best release on your indexers, downloads it, unpacks it, names it and files it in your library.
- Shows come with their full episode list. You can monitor a whole show, a season or a single episode, and season packs and multi-episode releases are understood.
- Every title has its own activity log that says what was searched, what was skipped and why, and what happened to the download.
- The Upcoming page has a calendar of releases and episodes, and a Wanted list of what is still missing.
- Discover has trending, popular and coming-soon rows, "More like your library" with a switch for older titles, browsing by genre, year and order, and importing a public Trakt list.
- In the Library, administrators can press Select and change many titles at once: monitor, better versions, quality profile, where to download from, search now, or remove.

### Music
- Music is off until you switch it on under Settings > Media types.
- Add artists from MusicBrainz, follow them, and download their albums. Music has its own quality profiles and its own Discover page.
- Import an existing music folder without moving or renaming anything.

### Downloading and the download line
- Cue has its own Usenet downloader (several providers, backup providers, PAR2 repair, unpacking) and its own torrent client. There is no download program to connect.
- Downloads wait in a line. **Downloads at the same time** sets how many run together (1 to 5, one by default), and what you start yourself goes before what the automatic searches add.
- Downloads can be paused, resumed and stopped, one at a time or all at once. A paused download stays paused after a restart.
- Torrents can go through a built-in WireGuard VPN with a kill switch. It needs no special container permissions. The VPN comes back by itself after a restart, says plainly whether it is really connected, and torrents wait for it instead of going out without it.
- Settings shows whether the torrent port is open and whether another peer has reached you.
- Indexers can be Newznab or Torznab sites, or sites from the community definition list, with support for the FlareSolverr helper for Cloudflare checks. The `-full` image has the helper built in.
- A daily clean-up removes leftovers from the downloads folder. Your library is never touched. You choose how many days finished downloads and activity are kept.

### Importing an existing library
- **Import existing** finds movies, shows and music you already have and adds them without moving, renaming or deleting anything.
- Imported titles start unmonitored and set to leave your files alone, so nothing is downloaded by surprise. The report has buttons to start monitoring afterwards.
- Details fill in on the server in the background, so you can leave the page. A banner shows the progress.
- **Move from other apps** reads Radarr, Sonarr, Prowlarr, SABnzbd, NZBGet, Jackett, NZBHydra2, Overseerr or Jellyseerr, Ombi, Bazarr, Medusa and SickChill, and only reads them.

### Quality and languages
- Quality profiles say which releases are acceptable, where to stop upgrading, and which other profiles to try if nothing is found. Five profiles come built in.
- Looking for better versions after a download is off unless you switch it on.
- An audio language setting decides which language tags in release names are taken (English unless you change it).
- Subtitles are off until you switch them on under Settings > Info, lists and subtitles > Subtitles. Subtitles that come inside a download are always kept.
- File names follow a preset for Plex, Jellyfin or Emby, Kodi, a short style, or your own pattern.

### Media servers
- Connect Plex, Jellyfin or Emby. Cue can find servers on your network, help you sign in, and ask the server to scan just the new folder after each import (or the whole library, when the server does not know that folder).
- Title pages get a "Watch in" link to your server.

### Notifications
- Send messages by email, ntfy, Gotify, Pushover, Slack, Discord, Telegram or a webhook, and choose which events each one is told about.

### Accounts and security
- Passwords, API keys and tokens are only ever sent to the address they were saved with, stored encrypted, and removed from the log before you see or copy it.
- Downloaded archives are checked before anything is unpacked, and update files are checked against a signature before they are installed.
- The first account is the administrator. Family members can get basic accounts that can find and add titles but cannot change settings.
- Every account can make API keys. Saved passwords and keys are stored encrypted, and sign-in attempts are rate limited.
- [docs/security.md](docs/security.md) has a checklist and examples for putting Cue behind a reverse proxy.
- A backup and restore of your settings and database is built in (Settings > System > Server and backup).

### Updates and restart
- Cue checks GitHub once a day for a new version and shows a notice with what is new. You can turn the check off.
- **Update now** installs a new release in the Docker image after checking its signature. Installing overnight and pushing an update through the API are both off unless you switch them on.
- **Restart Cue** and **Restart with automation paused** are on the Server and backup page. Cue also restarts itself if it stops answering.
- Setting `CUE_PAUSE_AUTOMATION` starts it in safe mode, with no automatic searching, downloading or refreshing.

### Logs and errors
- Settings > System > Logs and errors lists what went wrong in plain words, with what to try. Repeats are counted on one line and problems are kept for 30 days.
- **Copy for support** puts a report on your clipboard, with passwords and keys left out.

### Install and platforms
- Runs in Docker on 64-bit Linux, Synology, Unraid, QNAP and Raspberry Pi (`amd64` and `arm64`). Images are published as `ghcr.io/jhipsman/cue` with the tags `latest`, `1.3.0` and `1.3`, and the same with `-full`.
- A first-run wizard asks what you want to manage (movies, TV, music), checks your folders live, and helps you connect your indexers, Usenet provider, media player and media server.
- Audiobooks and ebooks are not ready yet. Windows and macOS installers and one-click app store entries are still to come.

### Support the project
- A small "Support Cue" link sits at the bottom of the sidebar for administrators. One click hides it.

### Documentation
- Step-by-step guides for Synology, Unraid, QNAP and Linux, and pages for each part of the app, are in [docs/](docs/README.md).
- The pages for the HTTP API, environment variables, stored settings and problem codes are generated from the code.
