#!/usr/bin/env python3
"""Build Plex-friendly XMLTV and enrich channel logos for Threadfin lineups."""

from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import time
import urllib.request
import xml.etree.ElementTree as ET
from dataclasses import dataclass
from pathlib import Path
from typing import Any


IPTV_ORG_CHANNELS_URL = "https://iptv-org.github.io/api/channels.json"
IPTV_ORG_LOGOS_URL = "https://iptv-org.github.io/api/logos.json"
DEFAULT_CACHE_TTL_SECONDS = 7 * 24 * 60 * 60


@dataclass(frozen=True)
class Channel:
    tvg_id: str
    number: str
    name: str
    logo: str
    extinf: str
    url: str


def normalize(value: str | None) -> str:
    return re.sub(r"[^a-z0-9]+", "", (value or "").lower())


def read_json(path: Path) -> dict[str, Any]:
    with path.open("r", encoding="utf-8") as fh:
        return json.load(fh)


def backup(path: Path) -> None:
    if path.exists():
        shutil.copy2(path, path.with_suffix(path.suffix + f".bak-{int(time.time())}"))


def atomic_write_text(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(path.suffix + ".tmp")
    tmp.write_text(content, encoding="utf-8")
    os.replace(tmp, path)


def parse_extinf_attr(line: str, attr: str) -> str:
    match = re.search(rf'{re.escape(attr)}="([^"]*)"', line)
    return match.group(1) if match else ""


def replace_extinf_attr(line: str, attr: str, value: str) -> str:
    escaped = value.replace('"', "%22")
    pattern = rf'{re.escape(attr)}="[^"]*"'
    replacement = f'{attr}="{escaped}"'
    if re.search(pattern, line):
        return re.sub(pattern, replacement, line)
    return line.replace(",", f' {replacement},', 1)


def parse_m3u(path: Path) -> list[Channel]:
    lines = path.read_text(encoding="utf-8", errors="ignore").splitlines()
    channels: list[Channel] = []
    index = 0
    while index < len(lines):
        line = lines[index]
        if not line.startswith("#EXTINF"):
            index += 1
            continue

        url = ""
        if index + 1 < len(lines) and not lines[index + 1].startswith("#"):
            url = lines[index + 1]

        channels.append(
            Channel(
                tvg_id=parse_extinf_attr(line, "tvg-id") or parse_extinf_attr(line, "channelID"),
                number=parse_extinf_attr(line, "tvg-chno"),
                name=line.rsplit(",", 1)[-1].strip(),
                logo=parse_extinf_attr(line, "tvg-logo"),
                extinf=line,
                url=url,
            )
        )
        index += 2 if url else 1
    return channels


def rewrite_m3u(path: Path, logo_by_name: dict[str, str], dry_run: bool, backup_enabled: bool) -> bool:
    original = path.read_text(encoding="utf-8", errors="ignore")
    output: list[str] = []
    for line in original.splitlines():
        if line.startswith("#EXTINF"):
            name = line.rsplit(",", 1)[-1].strip()
            logo = logo_by_name.get(name)
            if logo:
                line = replace_extinf_attr(line, "tvg-logo", logo)
        output.append(line)

    rewritten = "\n".join(output) + "\n"
    if rewritten == original:
        return False
    if not dry_run:
        if backup_enabled:
            backup(path)
        atomic_write_text(path, rewritten)
    return True


def load_iptv_org(cache_path: Path, ttl_seconds: int, allow_network: bool) -> dict[str, Any]:
    if cache_path.exists() and time.time() - cache_path.stat().st_mtime < ttl_seconds:
        return read_json(cache_path)

    if not allow_network:
        if cache_path.exists():
            return read_json(cache_path)
        return {"channels": [], "logos": []}

    data: dict[str, Any] = {}
    for key, url in {"channels": IPTV_ORG_CHANNELS_URL, "logos": IPTV_ORG_LOGOS_URL}.items():
        with urllib.request.urlopen(url, timeout=30) as response:
            data[key] = json.load(response)

    cache_path.parent.mkdir(parents=True, exist_ok=True)
    atomic_write_text(cache_path, json.dumps(data, separators=(",", ":")))
    return data


def build_iptv_indexes(data: dict[str, Any]) -> tuple[dict[str, list[str]], dict[str, dict[str, Any]], dict[str, str]]:
    names: dict[str, list[str]] = {}
    channels_by_id: dict[str, dict[str, Any]] = {}
    logos_by_channel: dict[str, str] = {}

    for channel in data.get("channels", []):
        channel_id = channel.get("id")
        if not channel_id:
            continue
        channels_by_id[channel_id] = channel
        candidates = [channel.get("name"), channel.get("network"), *(channel.get("alt_names") or [])]
        for candidate in candidates:
            key = normalize(candidate)
            if key:
                names.setdefault(key, []).append(channel_id)

    for logo in data.get("logos", []):
        if not logo.get("in_use"):
            continue
        channel_id = logo.get("channel")
        url = logo.get("url")
        if not channel_id or not url:
            continue
        current = logos_by_channel.get(channel_id)
        is_png = (logo.get("format") or "").upper() == "PNG"
        if current is None or is_png:
            logos_by_channel[channel_id] = url

    return names, channels_by_id, logos_by_channel


def resolve_iptv_channel_id(
    channel: Channel,
    country: str,
    aliases: dict[str, str],
    names: dict[str, list[str]],
    channels_by_id: dict[str, dict[str, Any]],
) -> tuple[str, str]:
    if channel.name in aliases:
        return aliases[channel.name], "alias"

    exact = names.get(normalize(channel.name), [])
    country_matches = [cid for cid in exact if channels_by_id.get(cid, {}).get("country") == country]
    if country_matches:
        return country_matches[0], "exact"
    if exact:
        return exact[0], "exact"

    callsign = re.search(r"\b([A-Z]{3,5})\b", channel.name)
    if callsign:
        call = callsign.group(1)
        for suffix in ("1", "DT1"):
            channel_id = f"{call}TV{suffix}.{country.lower()}"
            if channel_id in channels_by_id:
                return channel_id, "callsign"
        for channel_id, metadata in channels_by_id.items():
            if metadata.get("country") == country and channel_id.startswith(f"{call}TV"):
                return channel_id, "callsign"

    return "", ""


def plex_safe_programme(programme: ET.Element, channel_id: str) -> ET.Element:
    copy = ET.fromstring(ET.tostring(programme, encoding="utf-8"))
    copy.attrib["channel"] = channel_id

    for tag in ("episode-num", "sub-title", "new", "previously-shown"):
        for child in list(copy.findall(tag)):
            copy.remove(child)

    title = copy.find("title")
    if title is None:
        title = ET.Element("title")
        copy.insert(0, title)
    if not (title.text or "").strip():
        title.text = (programme.findtext("sub-title") or "Unknown Airing").strip()

    return copy


def build_xmltv(
    channels: list[Channel],
    source_xmltv_path: Path,
    output_xmltv_path: Path,
    channel_map: dict[str, str],
    logo_by_name: dict[str, str],
    dry_run: bool,
    backup_enabled: bool,
) -> dict[str, Any]:
    source_root = ET.parse(source_xmltv_path).getroot()
    source_channels = {
        channel.attrib.get("id", "")
        for channel in source_root.findall("channel")
        if channel.attrib.get("id")
    }
    programmes_by_channel: dict[str, list[ET.Element]] = {}
    for programme in source_root.findall("programme"):
        programmes_by_channel.setdefault(programme.attrib.get("channel", ""), []).append(programme)

    output = ET.Element("tv")
    programme_copies: list[ET.Element] = []
    matched = 0
    programme_count = 0
    unmatched_channels: list[str] = []

    for channel in channels:
        xml_channel = ET.SubElement(output, "channel", {"id": channel.tvg_id})
        ET.SubElement(xml_channel, "display-name").text = channel.name
        if channel.number:
            ET.SubElement(xml_channel, "display-name").text = channel.number
        logo = logo_by_name.get(channel.name) or channel.logo
        if logo:
            ET.SubElement(xml_channel, "icon", {"src": logo})

        source_id = channel_map.get(channel.name)
        if not source_id or source_id not in source_channels:
            unmatched_channels.append(channel.name)
            continue

        matched += 1
        for programme in programmes_by_channel.get(source_id, []):
            programme_copies.append(plex_safe_programme(programme, channel.tvg_id))
            programme_count += 1

    programme_copies.sort(
        key=lambda p: (
            p.attrib.get("channel", ""),
            p.attrib.get("start", ""),
            p.attrib.get("stop", ""),
            p.findtext("title") or "",
        )
    )
    output.extend(programme_copies)
    ET.indent(output, space="  ")

    xmltv_changed = True
    tmp = output_xmltv_path.with_suffix(output_xmltv_path.suffix + ".tmp")
    output_xmltv_path.parent.mkdir(parents=True, exist_ok=True)
    if not dry_run:
        if backup_enabled:
            backup(output_xmltv_path)
        ET.ElementTree(output).write(tmp, encoding="utf-8", xml_declaration=True)
        os.replace(tmp, output_xmltv_path)
    else:
        existing = output_xmltv_path.read_bytes() if output_xmltv_path.exists() else b""
        ET.ElementTree(output).write(tmp, encoding="utf-8", xml_declaration=True)
        xmltv_changed = tmp.read_bytes() != existing
        tmp.unlink(missing_ok=True)

    return {
        "channels": len(channels),
        "matched_channels": matched,
        "programmes": programme_count,
        "xmltv_changed": xmltv_changed,
        "unmatched_channels": unmatched_channels,
    }


def enrich(
    config: dict[str, Any],
    allow_network: bool,
    dry_run: bool = False,
    backup_enabled: bool = True,
) -> dict[str, Any]:
    paths = config["paths"]
    m3u_path = Path(paths["m3u"])
    source_xmltv_path = Path(paths["source_xmltv"])
    output_xmltv_path = Path(paths["output_xmltv"])
    cache_path = Path(paths.get("iptv_org_cache", output_xmltv_path.parent / "iptv-org-cache.json"))
    country = config.get("country", "US")
    cache_ttl = int(config.get("cache_ttl_seconds", DEFAULT_CACHE_TTL_SECONDS))
    prefer_iptv_for = set(config.get("prefer_iptv_org_logo_for_match_types", ["alias", "callsign"]))

    channels = parse_m3u(m3u_path)
    iptv_data = load_iptv_org(cache_path, cache_ttl, allow_network)
    iptv_names, iptv_channels, iptv_logos = build_iptv_indexes(iptv_data)

    logo_by_name: dict[str, str] = {}
    logo_stats = {"provider": 0, "iptv_org": 0, "missing": 0}
    aliases = config.get("iptv_org_channel_aliases", {})
    missing_logo_channels: list[str] = []
    iptv_org_matches: dict[str, dict[str, str]] = {}

    for channel in channels:
        iptv_id, match_type = resolve_iptv_channel_id(channel, country, aliases, iptv_names, iptv_channels)
        iptv_logo = iptv_logos.get(iptv_id, "")
        logo = channel.logo
        if iptv_logo and (not logo or match_type in prefer_iptv_for):
            logo = iptv_logo
            logo_by_name[channel.name] = logo

        if iptv_logo and logo == iptv_logo:
            logo_stats["iptv_org"] += 1
            iptv_org_matches[channel.name] = {"channel_id": iptv_id, "match_type": match_type}
        elif logo:
            logo_stats["provider"] += 1
        else:
            logo_stats["missing"] += 1
            missing_logo_channels.append(channel.name)

    m3u_changed = rewrite_m3u(m3u_path, logo_by_name, dry_run=dry_run, backup_enabled=backup_enabled)
    xmltv_stats = build_xmltv(
        channels=parse_m3u(m3u_path) if m3u_changed else channels,
        source_xmltv_path=source_xmltv_path,
        output_xmltv_path=output_xmltv_path,
        channel_map=config.get("xmltv_channel_map", {}),
        logo_by_name=logo_by_name,
        dry_run=dry_run,
        backup_enabled=backup_enabled,
    )

    return {
        **xmltv_stats,
        "dry_run": dry_run,
        "m3u_changed": m3u_changed,
        "logos": logo_stats,
        "missing_logo_channels": missing_logo_channels,
        "iptv_org_matches": iptv_org_matches,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description="Build Plex-friendly XMLTV and enrich Threadfin M3U logos.")
    parser.add_argument("--config", required=True, type=Path, help="Path to JSON config.")
    parser.add_argument("--no-network", action="store_true", help="Use the iptv-org cache only.")
    parser.add_argument("--dry-run", action="store_true", help="Calculate changes without writing M3U/XMLTV files.")
    parser.add_argument("--no-backup", action="store_true", help="Do not create timestamped backups before writing.")
    parser.add_argument("--report-json", action="store_true", help="Print machine-readable JSON report.")
    parser.add_argument("--report-file", type=Path, help="Write the JSON report to a file.")
    parser.add_argument("--fail-on-missing-logo", action="store_true", help="Exit 2 if any channel lacks a logo.")
    parser.add_argument(
        "--fail-on-unmatched-channel",
        action="store_true",
        help="Exit 2 if any M3U channel is not mapped to a source XMLTV channel.",
    )
    args = parser.parse_args()

    report = enrich(
        read_json(args.config),
        allow_network=not args.no_network,
        dry_run=args.dry_run,
        backup_enabled=not args.no_backup,
    )
    report_text = json.dumps(report, sort_keys=True)
    if args.report_file:
        atomic_write_text(args.report_file, report_text + "\n")
    if args.report_json:
        print(report_text)
    else:
        print(
            "channels={channels} matched={matched_channels} programmes={programmes} "
            "dry_run={dry_run} m3u_changed={m3u_changed} xmltv_changed={xmltv_changed} logos_provider={provider} "
            "logos_iptv_org={iptv_org} logos_missing={missing}".format(
                **report,
                **report["logos"],
            )
        )

    failed = False
    if args.fail_on_missing_logo and report["logos"]["missing"]:
        failed = True
    if args.fail_on_unmatched_channel and report["unmatched_channels"]:
        failed = True
    return 2 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
