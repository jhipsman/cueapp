# Cue on Apple TV

Live TV from Cue, played by the Apple TV's own player: sign in, pick a
profile, pick a channel. Press and hold a channel to add it to Favorites;
swipe down while watching for what's on, sound and subtitles.

## Installing it (free Apple ID, no paid developer account)

You need a Mac with Xcode (a 2017 MacBook runs macOS Ventura and Xcode 15.2,
from the Mac App Store or developer.apple.com/download/all).

1. Download this repository (Code > Download ZIP on GitHub) and open
   `appletv/CueTV.xcodeproj` in Xcode.
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
and press Run again to renew it.

If Xcode says the Apple TV's tvOS is newer than it supports, your Mac's
Xcode is too old for that Apple TV; OpenCore Legacy Patcher can put a newer
macOS (and Xcode) on an older Mac.
