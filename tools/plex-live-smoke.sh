#!/usr/bin/env bash
set -euo pipefail

PLEX_URL=${PLEX_URL:-http://127.0.0.1:32400}
PLEX_CONTAINER=${PLEX_CONTAINER:-plex}
PLEX_DVR_ID=${PLEX_DVR_ID:-20}
PLEX_CLIENT_ID=${PLEX_CLIENT_ID:-threadfin-live-tv-smoke}
PLEX_PRODUCT=${PLEX_PRODUCT:-Threadfin Smoke Test}
CHANNEL=${1:-${PLEX_CHANNEL_ID:-202}}
TOKEN=${PLEX_TOKEN:-}

if [ -z "$TOKEN" ]; then
  TOKEN=$(docker exec "$PLEX_CONTAINER" cat "/config/Library/Application Support/Plex Media Server/Preferences.xml" | sed -n 's/.*PlexOnlineToken="\([^"]*\)".*/\1/p')
fi

if [ -z "$TOKEN" ]; then
  echo "PLEX_TOKEN is required, or PLEX_CONTAINER must point to a Plex container with Preferences.xml." >&2
  exit 1
fi

curl -sS -X POST -o /tmp/plex_tune.xml \
  -H "Accept: application/xml" \
  -H "X-Plex-Client-Identifier: $PLEX_CLIENT_ID" \
  -H "X-Plex-Product: $PLEX_PRODUCT" \
  -H "X-Plex-Version: 1" \
  -H "X-Plex-Platform: curl" \
  "$PLEX_URL/livetv/dvrs/$PLEX_DVR_ID/channels/$CHANNEL/tune?X-Plex-Token=$TOKEN"

curl -sS "$PLEX_URL/livetv/sessions?X-Plex-Token=$TOKEN" -o /tmp/plex_sessions.xml
SESSION=$(grep -o "/livetv/sessions/[^\"]*" /tmp/plex_sessions.xml | head -1 | cut -d/ -f4)
INDEX="$PLEX_URL/livetv/sessions/$SESSION/$PLEX_CLIENT_ID/index.m3u8?offset=-1.000000&X-Plex-Token=$TOKEN"

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
      /*) url="$PLEX_URL$seg" ;;
      *) url="$PLEX_URL/livetv/sessions/$SESSION/$PLEX_CLIENT_ID/$seg" ;;
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
curl -sS "$PLEX_URL/livetv/sessions?X-Plex-Token=$TOKEN" -o /tmp/plex_sessions_after.xml
grep -o 'maxOffsetAvailable="[^"]*"\|videoDecision="[^"]*"\|audioDecision="[^"]*"\|protocol="[^"]*"' /tmp/plex_sessions_after.xml | head -20
curl -sS -X DELETE "$PLEX_URL/media/grabbers/operations/$CHANNEL-$PLEX_CLIENT_ID?X-Plex-Token=$TOKEN" >/dev/null || true
sleep 2
curl -sS "$PLEX_URL/livetv/sessions?X-Plex-Token=$TOKEN" | grep -o 'MediaContainer size="[0-9]*"' | head -1
