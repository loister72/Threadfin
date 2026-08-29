#!/usr/bin/env bash
set -euo pipefail

PLEX_URL=${PLEX_URL:-http://127.0.0.1:32400}
PLEX_CONTAINER=${PLEX_CONTAINER:-plex}
THREADFIN_CONTAINER=${THREADFIN_CONTAINER:-threadfin}
TOKEN=${PLEX_TOKEN:-}

if [ -z "$TOKEN" ]; then
  TOKEN=$(docker exec "$PLEX_CONTAINER" cat "/config/Library/Application Support/Plex Media Server/Preferences.xml" | sed -n 's/.*PlexOnlineToken="\([^"]*\)".*/\1/p')
fi

if [ -z "$TOKEN" ]; then
  echo "PLEX_TOKEN is required, or PLEX_CONTAINER must point to a Plex container with Preferences.xml." >&2
  exit 1
fi

curl -sS "$PLEX_URL/?X-Plex-Token=$TOKEN" -o /tmp/plex_root.xml
echo "[plex]"
grep -o 'version="[^"]*"\|platform="[^"]*"\|platformVersion="[^"]*"\|machineIdentifier="[^"]*"\|myPlexSubscription="[^"]*"\|friendlyName="[^"]*"' /tmp/plex_root.xml | head -20

curl -sS "$PLEX_URL/livetv/dvrs?X-Plex-Token=$TOKEN" -o /tmp/plex_dvrs_now.xml
echo "[dvr]"
grep -o 'key="[^"]*"\|uuid="[^"]*"\|uri="[^"]*"\|status="[^"]*"\|state="[^"]*"\|deviceId="[^"]*"\|make="[^"]*"\|model="[^"]*"\|tuners="[^"]*"\|lineupTitle="[^"]*"' /tmp/plex_dvrs_now.xml | head -60

echo "[threadfin]"
docker inspect "$THREADFIN_CONTAINER" --format 'image={{.Config.Image}} status={{.State.Status}} started={{.State.StartedAt}}'
docker logs --since 5m "$THREADFIN_CONTAINER" 2>&1 | grep -E 'Version:|GitHub:|Git Branch:|Updates have been disabled|Tuner \\(Plex / Emby\\):|XEPG Channels:|Streaming Status:|FFMPEG log:' | tail -80
