# Cue on a cloud server

Run Cue on a small rented server instead of your computer, and the Fire TV app and phones reach it from anywhere, at an `https://` address, with your computer off.

What runs where:

- **Movies and shows** stream from Premiumize straight to the TV or phone. The server only finds them, so it needs little bandwidth.
- **Live TV** on the Fire TV app plays straight from your IPTV provider. In a browser or on a phone it plays through the server, so that uses the server's bandwidth while you watch.

## 1. Rent a server

Any server with Ubuntu 24.04 (or Debian 12), 2 GB of memory or more, works. Some good choices:

| | Size | About | Notes |
|---|---|---|---|
| [Hetzner Cloud](https://www.hetzner.com/cloud) | CX22: 2 CPUs, 4 GB | €4 a month | 20 TB of traffic included. US and EU locations. |
| [DigitalOcean](https://www.digitalocean.com) | Basic, 2 GB | $12 a month | Simple to use. |
| [Oracle Cloud](https://www.oracle.com/cloud/free/) | Ampere A1, up to 4 CPUs and 24 GB | free | Free for good, but sign-up can be fussy and capacity is sometimes unavailable. Open ports 80 and 443 in the instance's security list as well. |

When you create it, choose **Ubuntu 24.04**, and add your SSH key or set a root password. Note its **IP address**.

## 2. Run the installer

Connect to it (on Windows, open PowerShell; on a Mac, Terminal):

```sh
ssh root@YOUR.SERVER.IP
```

Then paste this and press Enter:

```sh
curl -fsSL https://raw.githubusercontent.com/jhipsman/cueapp/HEAD/deploy/cloud/install.sh | sudo bash
```

It takes a few minutes: it installs Docker, sets Cue up in `/opt/cue`, gives it an HTTPS address, and starts it. At the end it prints the address, like:

```
  Open:  https://cue-203-0-113-7.sslip.io
```

That address is made from the server's IP by [sslip.io](https://sslip.io), so you need no domain. To use your own (like `cue.example.com`), point its DNS **A record** at the server's IP, then run the installer again with it:

```sh
curl -fsSL https://raw.githubusercontent.com/jhipsman/cueapp/HEAD/deploy/cloud/install.sh | sudo CUE_HOST=cue.example.com bash
```

To set the time zone the TV guide uses, add `TZ=America/New_York` (or yours) the same way.

## 3. Set Cue up

Open the address in your browser. Because the visit comes from the internet, Cue asks for a **setup code** before it lets anyone create the account; the installer printed it (or run `cd /opt/cue && docker compose logs cue | grep setup_code`).

Then, as on your computer: create your account, then **Settings > Streaming** for Premiumize, Comet, and Live TV, and **Settings > Movie info** for the TMDB key. This is a new Cue, so it starts empty: profiles and My List from your computer don't come across.

## 4. Point your devices at it

- **Fire TV app**: on its first screen (or **Change server** in its menu), enter the `https://...` address.
  Installing the app on a new Fire TV: Downloader app, then `https://YOUR-ADDRESS/tv.apk`.
- **iPhone**: open the address in Safari, sign in, press **Share**, then **Add to Home Screen**. Cue opens full screen from its icon, like an app.
- **Android phone**: open it in Chrome, then **⋮ > Add to Home screen** (or **Install app**).

## Keeping it up to date

The server fetches the latest Cue every night at 4:17 (server time) and restarts it only when there is a new version. To update now:

```sh
cd /opt/cue && docker compose pull && docker compose up -d
```

Everything Cue keeps (settings, accounts, profiles) is in `/opt/cue/config`. Copy that folder to back it up.

## If something isn't right

- **The address doesn't open, or says the certificate is wrong**: ports 80 and 443 must be open to the internet (in the provider's firewall, or Oracle's security list). The certificate comes on the first visit and can take a minute.
- **"unauthorized" or "denied" while downloading Cue's image**: the image on GitHub is private. On GitHub, open your profile, **Packages**, **cue**, **Package settings**, and under **Danger zone** choose **Change visibility > Public**. Then run the installer again.
- **Torrent sites don't answer from the server**: some sites block servers in data centers. Comet (Settings > Streaming) works regardless and is the better source anyway.
- **Live TV channels don't play in a browser but do on the TV**: some IPTV providers refuse connections from data centers. The TV app plays straight from the provider at home, so it isn't affected.
- **Logs**: `cd /opt/cue && docker compose logs --tail 100 cue`.
