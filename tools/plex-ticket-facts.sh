#!/usr/bin/env bash
set -euo pipefail

TOKEN=$(docker exec plex cat "/config/Library/Application Support/Plex Media Server/Preferences.xml" | sed -n 's/.*PlexOnlineToken="\([^"]*\)".*/\1/p')

curl -sS "http://127.0.0.1:32400/?X-Plex-Token=$TOKEN" -o /tmp/plex_root.xml
echo "[plex]"
grep -o 'version="[^"]*"\|platform="[^"]*"\|platformVersion="[^"]*"\|machineIdentifier="[^"]*"\|myPlexSubscription="[^"]*"\|friendlyName="[^"]*"' /tmp/plex_root.xml | head -20

curl -sS "http://127.0.0.1:32400/livetv/dvrs?X-Plex-Token=$TOKEN" -o /tmp/plex_dvrs_now.xml
echo "[dvr]"
grep -o 'key="[^"]*"\|uuid="[^"]*"\|uri="[^"]*"\|status="[^"]*"\|state="[^"]*"\|deviceId="[^"]*"\|make="[^"]*"\|model="[^"]*"\|tuners="[^"]*"\|lineupTitle="[^"]*"' /tmp/plex_dvrs_now.xml | head -60

echo "[threadfin]"
docker inspect threadfin --format 'image={{.Config.Image}} status={{.State.Status}} started={{.State.StartedAt}}'
docker logs --since 5m threadfin 2>&1 | grep -E 'Version:|GitHub:|Git Branch:|Updates have been disabled|Tuner \\(Plex / Emby\\):|XEPG Channels:|Streaming Status:|FFMPEG log:' | tail -80
