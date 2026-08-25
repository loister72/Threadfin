# Threadfin Guide Enricher

`tools/threadfin_guide_enricher.py` converts a provider M3U plus a source XMLTV guide into a Plex-friendly Threadfin lineup. It also enriches channel logos from the public iptv-org channel and logo APIs.

The tool is intended for automation. It has no baked-in personal URLs, Plex tokens, ZIP codes, or channel lists. Those belong in a JSON config outside the repository.

## Inputs

- `paths.m3u`: Threadfin/provider M3U to enrich.
- `paths.source_xmltv`: Source XMLTV guide with listings.
- `paths.output_xmltv`: XMLTV file that Threadfin should serve to Plex.
- `paths.iptv_org_cache`: Optional cache file for iptv-org channel/logo metadata.
- `xmltv_channel_map`: Maps each M3U channel display name to the source XMLTV channel id.
- `iptv_org_channel_aliases`: Optional exact aliases when provider names do not match iptv-org names.

Managed EPG providers such as EPG.best can be used as the source XMLTV data by downloading/exporting their XMLTV into `paths.source_xmltv`. This tool should not contain provider credentials or scrape account-backed services.

## Examples

Dry-run without network access:

```sh
python3 tools/threadfin_guide_enricher.py \
  --config examples/guide-enricher.config.example.json \
  --no-network \
  --dry-run \
  --report-json
```

Scheduled production run with a JSON report:

```sh
python3 tools/threadfin_guide_enricher.py \
  --config /opt/threadfin/guide-enricher.json \
  --report-file /opt/threadfin/reports/guide-enricher.json \
  --fail-on-missing-logo \
  --fail-on-unmatched-channel
```

## Exit Codes

- `0`: Completed and quality gates passed.
- `2`: Completed but a requested quality gate failed.
- Other non-zero values: Runtime failure, such as invalid input files.

## Automation Contract

Use `--dry-run --report-json` in CI to validate a config before applying it. Use `--report-file` in cron or systemd timers so monitoring can ingest the current channel count, matched guide count, missing logos, unmatched channel names, and whether files changed.
