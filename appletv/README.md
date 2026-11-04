# Cue on Apple TV

A native Apple TV app for your Cue server, in the look of Apple's own TV apps:

- **Tabs along the top**: Home, Shows, Movies, Live TV, Search, My List and
  the profile.
- **Home**: a banner that moves through featured titles, On Now (live
  channels), Continue Watching and rows of posters that lift and catch the
  light when focused.
- **A title's page**: the artwork big, Play or Resume, My List, a show's
  seasons and episodes, More Like This.
- **Live TV**: a programme guide with channel tiles, shows across a timeline,
  a line at now and LIVE on what's on; press and hold a channel for
  Favorites.
- **Playing in Apple's own player**, with Skip Intro and Skip Recap, Next
  Episode at the credits, speeds and Other Version. Files Apple's player
  can't open (MKV) are converted by your Cue server as they play, keeping
  4K, HDR, Dolby Vision and surround; it resumes where you left off and
  plays the next episode at the end.

## Installing it (free Apple ID, no paid developer account)

You need a Mac with Xcode 15.2 or later and its tvOS platform (Xcode >
Settings > Components). An older Mac can't run the newest Xcode: take the
newest one its macOS allows from developer.apple.com/download/all (Xcode 15.2
on Ventura, 16.2 on Sonoma) rather than the Mac App Store.

1. Clone this repository in Xcode (Integrate > Clone, with a GitHub account
   added under Xcode > Settings > Accounts), or download it (Code > Download
   ZIP on GitHub), and open `appletv/CueTV.xcodeproj`. To see it without an
   Apple TV, pick an Apple TV simulator and press Run.
2. Xcode > Settings > Accounts: add your Apple ID.
3. Click "CueTV" in the left column, then the CueTV target > Signing &
   Capabilities: tick "Automatically manage signing" and pick your
   "(Personal Team)". If Xcode says the bundle identifier is taken, change
   `me.cuetv.appletv` to something of your own, like `me.cuetv.appletv.yourname`.
4. On the Apple TV: Settings > Remotes and Devices > Remote App and Devices.
   With the Mac on the same network, Xcode > Window > Devices and Simulators
   shows the Apple TV: click Pair and type the code the TV shows.
5. Pick the Apple TV at the top of Xcode's window and press Run (▶).

With a free Apple ID the app stops opening after 7 days: open the project
and press Run again to renew it. To update, Integrate > Pull (or download
again) and press Run.

If the Apple TV doesn't appear under Discovered: keep it on the Remote App
and Devices screen, put both on the same network (not a guest network),
restart the Apple TV and quit and reopen Xcode.

If Xcode says the Apple TV's tvOS is newer than it supports, your Mac's
Xcode is too old for that Apple TV; OpenCore Legacy Patcher can put a newer
macOS (and Xcode) on an older Mac.
