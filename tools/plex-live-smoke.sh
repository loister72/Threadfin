#!/usr/bin/env bash
set -euo pipefail

TOKEN=$(docker exec plex cat "/config/Library/Application Support/Plex Media Server/Preferences.xml" | sed -n 's/.*PlexOnlineToken="\([^"]*\)".*/\1/p')
CLIENT=pdgeek-live-tv-smoke
CHANNEL=${1:-202}

curl -sS -X POST -o /tmp/plex_tune.xml \
  -H "Accept: application/xml" \
  -H "X-Plex-Client-Identifier: $CLIENT" \
  -H "X-Plex-Product: PDGeek Smoke Test" \
  -H "X-Plex-Version: 1" \
  -H "X-Plex-Platform: curl" \
  "http://127.0.0.1:32400/livetv/dvrs/20/channels/$CHANNEL/tune?X-Plex-Token=$TOKEN"

curl -sS "http://127.0.0.1:32400/livetv/sessions?X-Plex-Token=$TOKEN" -o /tmp/plex_sessions.xml
SESSION=$(grep -o "/livetv/sessions/[^\"]*" /tmp/plex_sessions.xml | head -1 | cut -d/ -f4)
INDEX="http://127.0.0.1:32400/livetv/sessions/$SESSION/$CLIENT/index.m3u8?offset=-1.000000&X-Plex-Token=$TOKEN"

start=$(date +%s)
last=""
count=0
bytes=0
errors=0

while [ $(( $(date +%s) - start )) -lt 60 ]; do
  if ! curl -sS "$INDEX" -o /tmp/plex_live_index.m3u8; then
    errors=$((errors + 1))
    sleep 1
    continue
  fi

  seg=$(awk '!/^#/ && NF {x=$0} END{print x}' /tmp/plex_live_index.m3u8)
  if [ -n "$seg" ] && [ "$seg" != "$last" ]; then
    case "$seg" in
      http://*|https://*) url="$seg" ;;
      /*) url="http://127.0.0.1:32400$seg" ;;
      *) url="http://127.0.0.1:32400/livetv/sessions/$SESSION/$CLIENT/$seg" ;;
    esac
    case "$url" in
      *\?*) url="$url&X-Plex-Token=$TOKEN" ;;
      *) url="$url?X-Plex-Token=$TOKEN" ;;
    esac

    if ! curl -sS "$url" -o /tmp/plex_live_seg.ts; then
      errors=$((errors + 1))
      sleep 1
      continue
    fi

    sz=$(wc -c < /tmp/plex_live_seg.ts)
    bytes=$((bytes + sz))
    count=$((count + 1))
    last="$seg"
  fi

  sleep 1
done

echo "plex_hls_session=$SESSION channel=$CHANNEL segments=$count bytes=$bytes errors=$errors"
curl -sS "http://127.0.0.1:32400/livetv/sessions?X-Plex-Token=$TOKEN" -o /tmp/plex_sessions_after.xml
grep -o 'maxOffsetAvailable="[^"]*"\|videoDecision="[^"]*"\|audioDecision="[^"]*"\|protocol="[^"]*"' /tmp/plex_sessions_after.xml | head -20
curl -sS -X DELETE "http://127.0.0.1:32400/media/grabbers/operations/$CHANNEL-$CLIENT?X-Plex-Token=$TOKEN" >/dev/null || true
sleep 2
curl -sS "http://127.0.0.1:32400/livetv/sessions?X-Plex-Token=$TOKEN" | grep -o 'MediaContainer size="[0-9]*"' | head -1
