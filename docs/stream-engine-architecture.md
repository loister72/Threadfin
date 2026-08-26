# Stream Engine Architecture

Threadfin exposes IPTV sources to Plex, Emby, and Jellyfin as tuner-like HTTP
streams. The production goal is to make that output boring: clients should see a
stable MPEG-TS response, predictable guide/channel identity, useful errors, and
no provider credentials in logs.

## Current Findings

- Threadfin currently treats FFmpeg and VLC settings as a single text field. That
  is flexible, but it must be parsed like a command line. Plain whitespace
  splitting breaks HTTP headers, User-Agent values, VLC `--sout` chains, and any
  quoted path or filter argument.
- FFmpeg is the best primary stream engine for server-side automation. Its HTTP
  protocol implementation directly supports `user_agent`, `referer`, `headers`,
  `http_proxy`, `rw_timeout`, and reconnect behavior for streamed HTTP inputs.
- VLC should remain a fallback/compatibility engine. Its `standard` stream
  output can mux to `ts` and write to stdout, and its HTTP access module supports
  user-agent, referrer, proxy, and network caching. It is less direct to
  supervise and classify from Threadfin than FFmpeg.
- The Docker image currently patches `/usr/bin/vlc` so VLC can run as root. A
  production image should run Threadfin as a non-root user instead of modifying a
  packaged binary.
- The default FFmpeg command mixes useful live-stream ideas with irrelevant or
  risky flags:
  - `-movflags +faststart` is for MP4-style output and does not help MPEG-TS
    output to `pipe:1`.
  - `-copyts` preserves provider timestamps and may amplify client compatibility
    issues. FFmpeg's MPEG-TS muxer defaults to shifting timestamps to start at 0.
  - `-map 0:v -map 0:a:0` fails streams that are missing video/audio or have
    different stream ordering. Optional maps are safer for unstable IPTV inputs.

## Delivery Model

Threadfin should support three explicit delivery profiles.

| Profile | Use Case | Behavior |
| --- | --- | --- |
| Direct | Provider stream already works in the target client | Proxy bytes with minimal buffering and no transcoder process. |
| Remux | Provider stream is valid but client needs MPEG-TS/HDHR-compatible output | FFmpeg reads the source and writes MPEG-TS to stdout with copied video and compatible audio handling. |
| Transcode | Codec or timestamp problems require normalization | FFmpeg transcodes selected streams with explicit CPU/GPU policy and resource limits. |

Plex-facing Live TV should default to `Remux` for IPTV sources unless direct
play has been verified. The client contract should be "HTTP response containing
continuous MPEG-TS bytes"; Threadfin can buffer before response start, but should
avoid creating avoidable stalls between internal chunks.

## HDHomeRun Profiles

`hdhr-remux` is the default Plex compatibility target. It is a structured FFmpeg
profile that exposes the stream as continuous MPEG-TS, copies the first video
stream, normalizes the first audio stream to AAC stereo 48 kHz, regenerates
timestamps, and re-sends MPEG-TS program tables.

`hdhr-safe` is the stricter compatibility target for picky clients such as Plex
on Android TV. It keeps the same audio/timestamp normalization but uses a deeper
probe window and repeats PAT/PMT more aggressively so a client joining at stream
start sees tuner-like headers quickly.

The old `ffmpeg` setting remains an advanced raw-command override. Product code
should prefer named profiles so behavior is deterministic and testable.

## Guide Preflight

Automation can call the API with `{"cmd":"guide.preflight"}` to inspect the
loaded XEPG state before handing the lineup to Plex. The report is read-only and
flags active visible channels with duplicate guide numbers, duplicate canonical
identities, missing XMLTV mappings, and missing channel logos.

The canonical identity currently preserves existing Threadfin behavior:
`URL + tvg-id + source` when `tvg-id` exists, otherwise `URL + source`. That
keeps existing channel state stable while giving the codebase one place to
improve identity matching later.

## FFmpeg Profile Direction

The remux profile should be assembled as structured arguments, not one opaque
string. A safe baseline to test:

```text
-nostdin
-hide_banner
-loglevel warning
-rw_timeout <microseconds>
-reconnect 1
-reconnect_streamed 1
-reconnect_on_network_error 1
-reconnect_on_http_error 4xx,5xx
-reconnect_delay_max 5
-analyzeduration 1000000
-probesize 1000000
-user_agent <value>
-referer <value>
-headers <value>
-i <url>
-map 0:v:0?
-map 0:a:0?
-sn
-c:v copy
-c:a aac
-b:a 192k
-ac 2
-f mpegts
-fflags +genpts
-flags +global_header
-mpegts_flags resend_headers
-flush_packets 1
-muxdelay 0
-muxpreload 0
pipe:1
```

This is a target profile, not yet a default. It needs regression testing against
real provider samples and Plex Live TV smoke tests before replacing the existing
default.

## VLC Profile Direction

VLC fallback should continue to use a `standard` stream output equivalent to:

```text
-I dummy <url> --sout #std{mux=ts,access=file,dst=-}
```

Threadfin should pass HTTP user-agent, referrer, proxy, and caching options as
separate arguments. The Docker image should stop patching the VLC binary and
instead run as a non-root user.

## Implementation Backlog

- Add a `StreamEngine` abstraction that builds structured args for `direct`,
  `ffmpeg-remux`, `ffmpeg-transcode`, and `vlc-remux`.
- Replace ad hoc `exec.Command` handling with a supervised process helper:
  context cancellation, process group cleanup, stderr ring buffer, exit-code
  classification, and redacted debug output.
- Add deterministic Plex/HDHR smoke tests that tune a channel, read bytes for a
  minimum duration, and report startup time, stalls, stderr summary, and exit
  reason.
- Make first-byte and steady-state buffering separate settings. Startup buffering
  can intentionally wait a few seconds; post-start buffering should be measured
  as a stream-health failure.
- Replace the current internal `.ts` chunk handoff for third-party engines with a
  direct streaming path or a tested ring buffer. Chunked files are a likely place
  for post-start stalls.
- Add provider failure taxonomy: no bytes, early EOF, reconnect exhaustion,
  client disconnected, unsupported codec, bad input URL, and tuner limit.
- Add a non-root Docker runtime path and remove the VLC binary patch.

## Source Notes

- FFmpeg HTTP protocol options are implemented in `libavformat/http.c` and
  documented in `doc/protocols.texi`.
- FFmpeg MPEG-TS muxer behavior and flags are implemented in `libavformat` and
  documented in `doc/muxers.texi`.
- VLC HTTP input options are in `modules/access/http*.c`.
- VLC stdout/file streaming is in `modules/access_output/file.c`; the standard
  stream output module is in `modules/stream_out/standard.c`.
