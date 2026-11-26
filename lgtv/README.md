# Cue for LG TVs (webOS)

The LG app opens your Cue server's Watch pages laid out for the TV: the
remote's arrows move between buttons, OK picks, Back goes back, and the
play/pause buttons work the video. Videos and live channels play in the
TV's own web engine (Cue picks the browser-friendly version, as it does for
a computer). It asks for your server's address once; afterwards, press OK or
Down while it says "Opening…" to change it.

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
   under Apps). Type your server's address, the one you open in a browser.

## Updating

Most changes are on your server, so the app picks them up by itself. Only a
change to `lgtv/` needs the app installed again (step 3).

## Keeping Developer Mode on

Open the Developer Mode app every few days and press **Extend** (it shows the
hours left). If it lapses, turn it on again and repeat step 3.
