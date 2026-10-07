# Cue for TV

The Fire TV, Google TV and Android TV app. It shows Watch from your Cue server
full screen, moves around with the remote, and plays videos with its own
player (ExoPlayer): MKV and MP4, H.264, HEVC and AV1 as far as the TV can
decode them, 4K and HDR on TVs that show them, and Dolby and DTS sound passed
to the TV or receiver as it is.

## Install on a Fire TV

1. On the Fire TV: Settings > My Fire TV > Developer options > turn on
   **Apps from Unknown Sources** (or **Install unknown apps** > Downloader).
   No Developer options? Settings > My Fire TV > About, then press OK on the
   device name seven times.
2. Install **Downloader** from the Amazon Appstore.
3. In Downloader, enter `http://<your Cue server>:8264/tv.apk` (for example
   `http://192.168.1.20:8264/tv.apk`) and install.
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
