#!/usr/bin/env bash
set -euo pipefail

MODE=${MODE:-plex}
DURATION=${DURATION:-60}
STALL_SECONDS=${STALL_SECONDS:-12}
OUT_DIR=${OUT_DIR:-}

PLEX_URL=${PLEX_URL:-http://127.0.0.1:32400}
PLEX_CONTAINER=${PLEX_CONTAINER:-plex}
PLEX_DVR_ID=${PLEX_DVR_ID:-20}
PLEX_CLIENT_ID=${PLEX_CLIENT_ID:-threadfin-live-tv-smoke}
PLEX_PRODUCT=${PLEX_PRODUCT:-Threadfin Smoke Test}
PLEX_CHANNEL_ID=${PLEX_CHANNEL_ID:-202}
THREADFIN_STREAM_URL=${THREADFIN_STREAM_URL:-}

usage() {
  cat <<'EOF'
Usage:
  MODE=plex ./tools/plex-live-smoke.sh [channel]
  MODE=threadfin THREADFIN_STREAM_URL=http://threadfin/stream/... ./tools/plex-live-smoke.sh

Environment:
  MODE                 plex or threadfin
  DURATION             seconds to read stream data, default 60
  STALL_SECONDS        max seconds without a new Plex segment, default 12
  OUT_DIR              optional artifact directory
  PLEX_URL             Plex base URL, default http://127.0.0.1:32400
  PLEX_CONTAINER       container used to discover Plex token, default plex
  PLEX_TOKEN           optional Plex token
  PLEX_DVR_ID          Plex DVR id, default 20
  PLEX_CHANNEL_ID      Plex channel id, default 202
  THREADFIN_STREAM_URL direct Threadfin stream URL for MODE=threadfin
EOF
}

now() {
  date +%s
}

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

emit_json() {
  local status="$1"
  local reason="$2"
  local elapsed="$3"
  local segments="$4"
  local bytes="$5"
  local errors="$6"
  local stalls="$7"
  local startup_latency="$8"
  local session="${9:-}"

  printf '{'
  printf '"status":"%s",' "$(json_escape "$status")"
  printf '"reason":"%s",' "$(json_escape "$reason")"
  printf '"mode":"%s",' "$(json_escape "$MODE")"
  printf '"channel":"%s",' "$(json_escape "$PLEX_CHANNEL_ID")"
  printf '"duration_seconds":%s,' "$DURATION"
  printf '"elapsed_seconds":%s,' "$elapsed"
  printf '"segments":%s,' "$segments"
  printf '"bytes":%s,' "$bytes"
  printf '"errors":%s,' "$errors"
  printf '"stalls":%s,' "$stalls"
  printf '"startup_latency_seconds":%s' "$startup_latency"
  if [ -n "$session" ]; then
    printf ',"plex_session":"%s"' "$(json_escape "$session")"
  fi
  printf ',"artifact_dir":"%s"' "$(json_escape "$OUT_DIR")"
  printf '}\n'
}

make_out_dir() {
  if [ -n "$OUT_DIR" ]; then
    mkdir -p "$OUT_DIR"
  else
    OUT_DIR=$(mktemp -d "${TMPDIR:-/tmp}/threadfin-smoke.XXXXXX")
  fi
}

append_plex_token() {
  local url="$1"
  local token="$2"

  case "$url" in
    *\?*) printf '%s&X-Plex-Token=%s' "$url" "$token" ;;
    *) printf '%s?X-Plex-Token=%s' "$url" "$token" ;;
  esac
}

get_plex_token() {
  local token="${PLEX_TOKEN:-}"
  if [ -z "$token" ]; then
    token=$(docker exec "$PLEX_CONTAINER" cat "/config/Library/Application Support/Plex Media Server/Preferences.xml" | sed -n 's/.*PlexOnlineToken="\([^"]*\)".*/\1/p')
  fi

  if [ -z "$token" ]; then
    echo "PLEX_TOKEN is required, or PLEX_CONTAINER must point to a Plex container with Preferences.xml." >&2
    exit 2
  fi

  printf '%s' "$token"
}

redact_file() {
  local file="$1"
  local token="$2"

  if [ -f "$file" ] && [ -n "$token" ]; then
    SMOKE_REDACT_TOKEN="$token" perl -0pi -e 's/\Q$ENV{SMOKE_REDACT_TOKEN}\E/REDACTED/g' "$file"
  fi
}

run_threadfin_smoke() {
  if [ -z "$THREADFIN_STREAM_URL" ]; then
    echo "THREADFIN_STREAM_URL is required when MODE=threadfin." >&2
    exit 2
  fi

  local start elapsed bytes curl_code
  start=$(now)

  set +e
  curl -fsS --max-time "$DURATION" "$THREADFIN_STREAM_URL" -o "$OUT_DIR/threadfin-stream.ts" 2>"$OUT_DIR/threadfin-curl.err"
  curl_code=$?
  set -e

  elapsed=$(( $(now) - start ))
  if [ -f "$OUT_DIR/threadfin-stream.ts" ]; then
    bytes=$(wc -c < "$OUT_DIR/threadfin-stream.ts" | tr -d ' ')
  else
    bytes=0
  fi

  if [ "$bytes" -gt 0 ] && { [ "$curl_code" -eq 0 ] || [ "$curl_code" -eq 28 ]; }; then
    emit_json pass ok "$elapsed" 0 "$bytes" 0 0 0 ""
    exit 0
  fi

  emit_json fail "threadfin_stream_failed_curl_${curl_code}" "$elapsed" 0 "$bytes" 1 0 0 ""
  exit 1
}

run_plex_smoke() {
  local token session index part_key start first_segment_at last_segment_at last_segment
  local elapsed segments bytes errors stalls startup_latency

  token=$(get_plex_token)

  curl -fsS -X POST \
    -H "Accept: application/xml" \
    -H "X-Plex-Client-Identifier: $PLEX_CLIENT_ID" \
    -H "X-Plex-Product: $PLEX_PRODUCT" \
    -H "X-Plex-Version: 1" \
    -H "X-Plex-Platform: curl" \
    "$(append_plex_token "$PLEX_URL/livetv/dvrs/$PLEX_DVR_ID/channels/$PLEX_CHANNEL_ID/tune" "$token")" \
    -o "$OUT_DIR/plex-tune.xml"
  redact_file "$OUT_DIR/plex-tune.xml" "$token"

  part_key=$(grep -o 'key="/livetv/sessions/[^"]*index\.m3u8[^"]*"' "$OUT_DIR/plex-tune.xml" | head -1 | sed 's/^key="//; s/"$//' || true)
  if [ -n "$part_key" ]; then
    session=$(printf '%s' "$part_key" | cut -d/ -f4)
  fi

  if curl -fsS "$(append_plex_token "$PLEX_URL/livetv/sessions" "$token")" -o "$OUT_DIR/plex-sessions.xml"; then
    redact_file "$OUT_DIR/plex-sessions.xml" "$token"
  fi
  if [ -z "$session" ] && [ -f "$OUT_DIR/plex-sessions.xml" ]; then
    session=$(grep -o "/livetv/sessions/[^\"]*" "$OUT_DIR/plex-sessions.xml" | head -1 | cut -d/ -f4 || true)
  fi
  if [ -z "$session" ]; then
    emit_json fail no_plex_session 0 0 0 1 0 0 ""
    exit 1
  fi

  if [ -n "$part_key" ]; then
    index="$PLEX_URL$part_key"
  else
    index="$PLEX_URL/livetv/sessions/$session/$PLEX_CLIENT_ID/index.m3u8?offset=-1.000000"
  fi
  start=$(now)
  first_segment_at=0
  last_segment_at=$start
  last_segment=""
  segments=0
  bytes=0
  errors=0
  stalls=0

  while [ $(( $(now) - start )) -lt "$DURATION" ]; do
    if ! curl -fsS "$(append_plex_token "$index" "$token")" -o "$OUT_DIR/plex-live-index.m3u8" 2>>"$OUT_DIR/plex-curl.err"; then
      errors=$((errors + 1))
      sleep 1
      continue
    fi
    redact_file "$OUT_DIR/plex-live-index.m3u8" "$token"

    local segment segment_url segment_file size current
    segment=$(awk '!/^#/ && NF {x=$0} END{gsub(/\r$/, "", x); print x}' "$OUT_DIR/plex-live-index.m3u8")
    current=$(now)

    if [ -n "$segment" ] && [ "$segment" != "$last_segment" ]; then
      case "$segment" in
        http://*|https://*) segment_url="$segment" ;;
        /*) segment_url="$PLEX_URL$segment" ;;
        *) segment_url="$PLEX_URL/livetv/sessions/$session/$PLEX_CLIENT_ID/$segment" ;;
      esac

      segment_file="$OUT_DIR/plex-live-segment-$segments.ts"
      if curl -fsS "$(append_plex_token "$segment_url" "$token")" -o "$segment_file" 2>>"$OUT_DIR/plex-curl.err"; then
        size=$(wc -c < "$segment_file" | tr -d ' ')
        bytes=$((bytes + size))
        segments=$((segments + 1))
        last_segment="$segment"
        last_segment_at=$current
        if [ "$first_segment_at" -eq 0 ]; then
          first_segment_at=$current
        fi
      else
        errors=$((errors + 1))
      fi
    elif [ $(( current - last_segment_at )) -ge "$STALL_SECONDS" ]; then
      stalls=$((stalls + 1))
      last_segment_at=$current
    fi

    sleep 1
  done

  elapsed=$(( $(now) - start ))
  if [ "$first_segment_at" -gt 0 ]; then
    startup_latency=$(( first_segment_at - start ))
  else
    startup_latency=0
  fi

  if curl -fsS "$(append_plex_token "$PLEX_URL/livetv/sessions" "$token")" -o "$OUT_DIR/plex-sessions-after.xml"; then
    redact_file "$OUT_DIR/plex-sessions-after.xml" "$token"
  fi
  curl -fsS -X DELETE "$(append_plex_token "$PLEX_URL/media/grabbers/operations/$PLEX_CHANNEL_ID-$PLEX_CLIENT_ID" "$token")" >/dev/null || true

  if [ "$segments" -gt 0 ] && [ "$bytes" -gt 0 ] && [ "$stalls" -eq 0 ] && [ "$errors" -eq 0 ]; then
    emit_json pass ok "$elapsed" "$segments" "$bytes" "$errors" "$stalls" "$startup_latency" "$session"
    exit 0
  fi

  emit_json fail stream_unhealthy "$elapsed" "$segments" "$bytes" "$errors" "$stalls" "$startup_latency" "$session"
  exit 1
}

if [ "${1:-}" = "-h" ] || [ "${1:-}" = "--help" ]; then
  usage
  exit 0
fi

if [ "${1:-}" != "" ]; then
  PLEX_CHANNEL_ID="$1"
fi

make_out_dir

case "$MODE" in
  plex) run_plex_smoke ;;
  threadfin) run_threadfin_smoke ;;
  *)
    echo "Unsupported MODE: $MODE" >&2
    usage >&2
    exit 2
    ;;
esac
