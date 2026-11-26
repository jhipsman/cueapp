# Cue for LG TVs (webOS)

Cue's own app for LG TVs: its screens live in the app, made for the remote,
like the Fire TV app's. It has:

- **Signing in** by scanning a code with your phone (or typing the code at
  `<your Cue>/link`), or with your username and password; two-step sign-in
  works too. Then **who's watching**, with profile PINs (the remote's
  number keys type them).
- **The side menu**: Search, Home, Movies, Shows, and your profile (to
  switch). Back from a page goes to the menu; Back twice on Home leaves Cue.
- **Home, Movies and Shows**: the highlighted title's picture behind the
  screen with its story, the rows sliding up beneath (Continue watching, a
  Top 10, what's trending, My List). Each row remembers where you were.
- **Search**, with what's trending before you type.
- **Title pages**: Play or Resume, My List, a show's seasons and episodes
  (starting on the one you're on), More like this.
- **The player**, on the TV's own player (MKV, HEVC and Dolby sound play;
  Cue picks versions with sound the TV plays): OK pauses, left and right
  skip 10 seconds, the remote's play/pause/stop/fast-forward buttons work. It
  starts where you left off and keeps your place; a version that won't play
  falls back to Cue's converted copy, then the next version; at a show's
  end, the next episode.

Coming next: Live TV (the guide, with the channel playing as you browse),
then Browse, Ask Cue, Settings and Watch party.

It signs in with its own session, sent in a header (the app's pages are
files on the TV, so Cue's cookie can't be used), and talks to your Cue
directly; live channels play straight from your provider.

LG only lets you install your own apps in **Developer Mode**, which lasts 50
hours at a time: open the Developer Mode app on the TV now and then and
press **Extend** (or it resets and the app has to be installed again).

## Install it (once)

1. **Get the app.** Download `cue-lg.ipk` from the repository's
   [lg-latest release](../../releases/tag/lg-latest) (CI builds it from every
   change to `lgtv/` on main). Or build it yourself: `./lgtv/build.sh`
   (needs Node) makes `lgtv/dist/app.cue.tv_<version>_all.ipk`.
2. **Turn on Developer Mode on the TV.**
   - Make a free LG developer account at https://webostv.developer.lge.com
     (Sign up, top right).
   - On the TV, open the LG Content Store, search **Developer Mode**, install
     it and open it. Sign in with that account.
   - Switch **Dev Mode Status** on; the TV restarts. Open Developer Mode again
     and switch **Key Server** on. Note the TV's **IP address** shown there.
3. **Send Cue to the TV**, from a computer on the same network. The easy way
   is **webOS Dev Manager** (a free desktop app:
   https://github.com/webosbrew/dev-manager-desktop/releases):
   - Add Device → enter the TV's IP address → it asks for the passphrase the
     Developer Mode app shows; type it in.
   - Apps → Install → pick `cue-lg.ipk`.

   Or with LG's command-line tools (`npm install -g @webos-tools/cli`):
   ```
   ares-setup-device --add lgtv -i "host=<TV IP>" -i "port=9922" -i "username=prisoner"
   ares-novacom --device lgtv --getkey        # type the passphrase from the TV
   ares-install --device lgtv cue-lg.ipk
   ```
4. **Open Cue** from the TV's apps list (it's in the row at the bottom, or
   under Apps). Type your server's address, the one you open in a browser,
   then sign in with your phone.

## Updating

Install the new `cue-lg.ipk` the same way (step 3); you stay signed in.

## Building and testing

```
cd lgtv
npm ci
npm run build      # dist-app/: the app as the TV gets it
npm test           # walks it with the remote against a stand-in Cue
./build.sh         # dist/*.ipk
```

To try it on a computer, open `dist-app/index.html` in Chrome (as a file,
the way the TV opens it; Cue answers only that): arrow keys, Enter, and
Escape for Back.

## Keeping Developer Mode on

Open the Developer Mode app every few days and press **Extend** (it shows the
hours left). If it lapses, turn it on again and repeat step 3.
