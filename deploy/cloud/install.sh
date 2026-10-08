#!/usr/bin/env bash
# Puts Cue on a fresh cloud server (Ubuntu or Debian), with HTTPS:
#
#   curl -fsSL https://raw.githubusercontent.com/jhipsman/cueapp/HEAD/deploy/cloud/install.sh | sudo bash
#
# It installs Docker if it's missing, sets Cue up in /opt/cue, gets an HTTPS
# address (your own domain, or a free one made from the server's IP), starts
# it, and updates it every night. Run it again any time: it keeps your
# settings and only updates what it set up.
#
# Choices, all optional, as environment variables before `bash`:
#   CUE_HOST=cue.example.com   your own domain (point its DNS at this server first)
#   TZ=America/New_York        the time zone for the TV guide and logs
set -euo pipefail

REPO_RAW="${CUE_REPO_RAW:-https://raw.githubusercontent.com/jhipsman/cueapp/HEAD}"
DIR=/opt/cue

say() { printf '\n\033[1;36m==>\033[0m %s\n' "$*"; }
die() { printf '\n\033[1;31mProblem:\033[0m %s\n' "$*" >&2; exit 1; }
trap 'printf "\n\033[1;31mProblem:\033[0m the installer stopped at line %s. Send this to whoever is helping you.\n" "$LINENO" >&2' ERR

[ "$(id -u)" = 0 ] || die "Run this as root: put sudo before bash."
command -v curl >/dev/null || { apt-get update -qq && apt-get install -y -qq curl; }

# ---- Docker ----
if ! command -v docker >/dev/null || ! docker compose version >/dev/null 2>&1; then
  say "Installing Docker (a few minutes)"
  curl -fsSL https://get.docker.com | sh
fi
systemctl enable --now docker >/dev/null 2>&1 || true

# ---- Files ----
say "Setting Cue up in $DIR"
mkdir -p "$DIR/config"
cd "$DIR"
curl -fsSL "$REPO_RAW/deploy/cloud/docker-compose.yml" -o docker-compose.yml
curl -fsSL "$REPO_RAW/deploy/cloud/Caddyfile" -o Caddyfile

# ---- The address ----
# Kept from an earlier run unless CUE_HOST says otherwise.
old_host=""
if [ -f .env ]; then old_host="$(sed -n 's/^CUE_HOST=//p' .env | head -n1)"; fi
host="${CUE_HOST:-$old_host}"
if [ -z "$host" ]; then
  ip="$(curl -fsS4 --max-time 10 https://api.ipify.org || curl -fsS4 --max-time 10 https://ifconfig.me || true)"
  [ -n "$ip" ] || die "Couldn't find this server's public IP address. Run again with CUE_HOST=your.domain."
  # sslip.io answers any name with the IP written in it, so this works with
  # no domain of your own, and Let's Encrypt gives it a certificate.
  host="cue-${ip//./-}.sslip.io"
fi
# (Each step here must succeed even when there's no .env yet: a failed
# test inside $(...) would end the script silently under set -e.)
tz="${TZ:-}"
if [ -z "$tz" ] && [ -f .env ]; then tz="$(sed -n 's/^TZ=//p' .env | head -n1)"; fi
if [ -z "$tz" ]; then tz="$(timedatectl show -p Timezone --value 2>/dev/null || true)"; fi
[ -n "$tz" ] || tz=Etc/UTC
cat > .env <<EOT
CUE_HOST=$host
TZ=$tz
EOT

# ---- Firewall ----
if command -v ufw >/dev/null && ufw status 2>/dev/null | grep -q "Status: active"; then
  say "Opening ports 80 and 443 in the firewall"
  ufw allow 80/tcp >/dev/null
  ufw allow 443/tcp >/dev/null
  ufw allow 443/udp >/dev/null
fi

# ---- Start ----
say "Downloading and starting Cue"
docker compose pull -q || die "Couldn't download Cue's image. If it says 'unauthorized' or 'denied', make the cue package public on GitHub (see docs/cloud.md)."
docker compose up -d --remove-orphans

# ---- Nightly update ----
cat > /etc/cron.d/cue-update <<'EOT'
# Cue: fetch the latest version every night and restart only if it changed.
17 4 * * * root cd /opt/cue && docker compose pull -q && docker compose up -d --remove-orphans && docker image prune -f >/dev/null
EOT
chmod 644 /etc/cron.d/cue-update

# ---- Wait for it ----
say "Waiting for Cue to start"
for _ in $(seq 1 60); do
  if docker compose exec -T cue wget -q -T 3 -O /dev/null http://127.0.0.1:8264/api/version 2>/dev/null; then
    break
  fi
  sleep 2
done

code="$(docker compose logs cue 2>/dev/null | sed -n 's/.*setup_code=\([A-Z0-9-]*\).*/\1/p' | tail -n1)"
say "Cue is running"
cat <<EOT

  Open:  https://$host

  The first visit may take a minute while the HTTPS certificate is issued.
EOT
if [ -n "$code" ]; then
  cat <<EOT
  If Cue asks for a setup code to create your account, it's:  $code
EOT
fi
cat <<EOT

  Fire TV app: enter https://$host as the server address.
  Phone: open https://$host in Safari or Chrome and add it to the home screen.

  Cue updates itself every night. Its settings are in $DIR/config.
EOT
