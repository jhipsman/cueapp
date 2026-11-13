# Changelog

All notable user-facing changes to Cue are recorded here. The format
follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and
version numbers follow [Semantic Versioning](https://semver.org/). Each
release has its own dated section. Changes that aren't released yet collect
under [Unreleased]. How releases are cut: [docs/RELEASING.md](docs/RELEASING.md).

## [Unreleased]

### Security
- Releases are signed with a new update key, held only by the project owner. **Update now** installs only releases signed with it.

### Added
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
