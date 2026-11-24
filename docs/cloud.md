# Cue on a cloud server

Run Cue on a small rented server instead of your computer, and the Fire TV app and phones reach it from anywhere, at an `https://` address, with your computer off.

What runs where:

- **Movies and shows** stream from Premiumize straight to the TV or phone. The server only finds them, so it needs little bandwidth.
- **Live TV** on the Fire TV app plays straight from your IPTV provider. In a browser or on a phone it plays through the server, so that uses the server's bandwidth while you watch.

## 1. Rent a server

Any server with Ubuntu 24.04 (or Debian 12), 2 GB of memory or more, works. Some good choices:

| | Size | About | Notes |
|---|---|---|---|
| [Hetzner Cloud](https://www.hetzner.com/cloud), US (Ashburn or Hillsboro) | CPX21: 3 CPUs, 4 GB | about $10 a month | 1 TB of traffic a month included. Sign-up can ask for ID. |
| [Hetzner Cloud](https://www.hetzner.com/cloud), Europe | CX22: 2 CPUs, 4 GB | about €4 a month | 20 TB of traffic included. |
| [DigitalOcean](https://www.digitalocean.com) | Basic, 2 GB | $12 a month | Simple to use, easy sign-up. |
| [Vultr](https://www.vultr.com) | Cloud Compute, 2 GB | $10–12 a month | Many US locations. |
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

That address is made from the server's IP by [sslip.io](https://sslip.io), so you need no domain. For a short one of your own (like `cuetv.me`), buy it (Cloudflare, Porkbun or Namecheap; about $10–15 a year), and in its DNS add an **A record** for `@` with the server's IP (on Cloudflare, set it to **DNS only**, the grey cloud). To have `www.cuetv.me` work too, add the same for `www`. Wait a few minutes, then run the installer again with the name:

```sh
curl -fsSL https://raw.githubusercontent.com/jhipsman/cueapp/HEAD/deploy/cloud/install.sh | sudo CUE_HOST=cuetv.me bash
```

Everything you set up stays. Typing just `cuetv.me` then opens Cue (and Watch, once signed in), on phones, computers and in the Fire TV app; `www.cuetv.me` forwards to it. The old sslip.io address stops working, so sign in again on each device, and in the Fire TV app use **Change server** (in your avatar's menu).

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

The server fetches the latest Cue every night at 4:17 (server time) and restarts it only when there is a new version. Until Cue's first release, "latest" is the newest build of `main`; after that it is the newest release. To keep following every `main` build, put `CUE_IMAGE=ghcr.io/jhipsman/cue:edge` in `/opt/cue/.env`. To update now:

```sh
cd /opt/cue && docker compose pull && docker compose up -d
```

Everything Cue keeps (settings, accounts, profiles) is in `/opt/cue/config`. Copy that folder to back it up.

### Update on every merge

GitHub can update the server itself a few minutes after each change to `main`: once the "Cloud image" build finishes, the **Deploy to cloud server** workflow connects to the server and it pulls the new image. It uses an SSH key that can only run the update (no shell, nothing else), and it checks the server's identity first. Set it up once:

1. **On the server** (DigitalOcean: the droplet's **Console**), as root, paste:

   ```sh
   cat > /opt/cue/update.sh <<'EOF'
   #!/bin/sh
   # Run by GitHub after each build of main: pull the newest Cue, restart it if it changed.
   set -e
   cd /opt/cue
   docker compose pull -q
   docker compose up -d --remove-orphans
   docker image prune -f >/dev/null
   docker compose ps
   EOF
   chmod 755 /opt/cue/update.sh
   ssh-keygen -q -t ed25519 -N "" -C cue-deploy -f /root/cue-deploy
   mkdir -p /root/.ssh && chmod 700 /root/.ssh
   echo "restrict,command=\"/opt/cue/update.sh\" $(cat /root/cue-deploy.pub)" >> /root/.ssh/authorized_keys
   chmod 600 /root/.ssh/authorized_keys
   echo "----- DROPLET_SSH_KEY -----"; cat /root/cue-deploy
   echo "----- DROPLET_HOST_KEY -----"; cat /etc/ssh/ssh_host_ed25519_key.pub
   ```

2. **On GitHub**, in the repository's **Settings > Secrets and variables > Actions**, add three repository secrets with **New repository secret**:
   - `DROPLET_HOST`: the server's IP address (or its name, like `cuetv.me`, if its DNS points straight at the server).
   - `DROPLET_SSH_KEY`: everything printed under `DROPLET_SSH_KEY`, from `-----BEGIN OPENSSH PRIVATE KEY-----` to `-----END OPENSSH PRIVATE KEY-----`.
   - `DROPLET_HOST_KEY`: the one line printed under `DROPLET_HOST_KEY` (it starts with `ssh-ed25519`).

   (Add `DROPLET_USER` too only if you log in as someone other than `root`.)

3. **Back on the server**, delete the key file: `rm /root/cue-deploy /root/cue-deploy.pub`. GitHub has the only copy now.

4. **Try it**: in the repository's **Actions** tab, open **Deploy to cloud server**, press **Run workflow**, and watch the run: it ends by listing Cue's containers. From then on it runs by itself after every merge.

To stop it, delete the line ending in `cue-deploy` from `/root/.ssh/authorized_keys` on the server (and the secrets on GitHub). The nightly update keeps running either way.

## If something isn't right

- **The address doesn't open, or says the certificate is wrong**: ports 80 and 443 must be open to the internet (in the provider's firewall, or Oracle's security list). The certificate comes on the first visit and can take a minute.
- **Slow with 2 GB of memory**: FlareSolverr (for torrent sites behind Cloudflare) runs a hidden Chrome. Resize the server to 4 GB from its page; nothing is lost.
- **"unauthorized" or "denied" while downloading Cue's image**: the image on GitHub is private. On GitHub, open your profile, **Packages**, **cue**, **Package settings**, and under **Danger zone** choose **Change visibility > Public**. Then run the installer again.
- **Torrent sites don't answer from the server**: some sites block servers in data centers. Comet (Settings > Streaming) works regardless and is the better source anyway.
- **Live TV channels don't play in a browser but do on the TV**: some IPTV providers refuse connections from data centers. The TV app plays straight from the provider at home, so it isn't affected.
- **Logs**: `cd /opt/cue && docker compose logs --tail 100 cue`.
