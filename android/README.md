# Cue for TV

The Fire TV, Google TV and Android TV app. Its screens are native (Compose
for TV, the toolkit Google TV's own apps use) and talk to your Cue server
directly; videos play in its own player (Media3 ExoPlayer).

**Screens**: Who's watching?, then tabs along the top (Home, Shows, Movies,
Live TV, Search, My List and the profile). Home's banner shows whatever
poster the highlight is on and the rows slide beneath it; a title's page
has Play or Resume, My List, a show's seasons and episodes, and More Like
This. Live TV lists the groups and channels with what's on now and next;
the full guide grid opens in the web view (as does Profile > Web version).

**The remote**: Back in a page goes up to the tabs, then to Home, then out.
Coming back to a row or a title puts the highlight where it was.

**The player**: a rounded control panel with the time, speeds (0.5x to 2x),
Next episode, Other version, Audio, Subtitles and Surround. It plays MKV and
MP4, H.264, HEVC, AV1, 4K, HDR and Dolby Vision as far as the TV can,
decodes with the TV's own chip (FFmpeg only for sound it can't decode, such
as DTS), sends Dolby and DTS to a receiver or soundbar as they are (Surround
on, the default) or decodes them on the TV (off), and matches the TV's
refresh rate to the film's, or plays tunneled on TVs that stay at 60 Hz.
Skip Intro, the next episode at the credits, subtitles and frames on the
time bar are there too. Press up while playing for a stats line (picture,
frame rate, the TV's refresh rate, bitrate, buffer, dropped frames, the
decoder and how the sound leaves the TV).

## Install on a Fire TV or Google TV

1. On the Fire TV: Settings > My Fire TV > Developer options > turn on
   **Apps from Unknown Sources** (or **Install unknown apps** > Downloader).
   No Developer options? Settings > My Fire TV > About, then press OK on the
   device name seven times.
   On a Google TV: Settings > Apps > Security & restrictions > Unknown
   sources > turn on **Downloader**.
2. Install **Downloader** from the Amazon Appstore (Fire TV) or the Play
   Store (Google TV).
3. In Downloader, enter your Cue server's address followed by `/tv.apk` (for
   example `https://cue.example.com/tv.apk`, or `http://192.168.1.20:8264/tv.apk`
   at home) and install.
4. Open Cue, enter the server's address, sign in, and pick your profile.

To update, do step 3 again: each new build installs over the last.

## How it is built

GitHub Actions (`.github/workflows/android.yml`) builds it on every change in
this folder and publishes `cue-tv.apk` as the `tv-latest` release. To build it
yourself: Android Studio, or `./gradlew assembleRelease` with the Android SDK.

`app/cue-tv.keystore` signs every build with the same key, so updates
install over the app already there. It is in the repository on purpose, for
a personal, sideloaded app: anyone could sign an APK with it, so only install
Cue from your own server or this repository's releases.
