#!/usr/bin/env python3

import json
import sys
import tempfile
import unittest
import xml.etree.ElementTree as ET
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))

from threadfin_guide_enricher import enrich


class GuideEnricherTest(unittest.TestCase):
    def test_enriches_logos_and_builds_plex_safe_xmltv(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            tmp = Path(tmpdir)
            m3u = tmp / "provider.m3u"
            source = tmp / "source.xml"
            output = tmp / "out.xml"
            cache = tmp / "iptv-cache.json"

            m3u.write_text(
                "\n".join(
                    [
                        "#EXTM3U",
                        '#EXTINF:-1 tvg-id="219" tvg-chno="219" tvg-name="FanDuel Sports Southwest" tvg-logo="",FanDuel Sports Southwest',
                        "http://example.invalid/219",
                        '#EXTINF:-1 tvg-id="333" tvg-chno="333" tvg-name="UPtv" tvg-logo="https://provider.example/uptv.png",UPtv',
                        "http://example.invalid/333",
                    ]
                )
                + "\n"
            )
            source.write_text(
                """<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <channel id="src-sports"><display-name>Sports</display-name></channel>
  <programme channel="src-sports" start="20260825000000 +0000" stop="20260825003000 +0000">
    <title></title>
    <sub-title>Pregame</sub-title>
    <episode-num system="xmltv_ns">0.0.</episode-num>
  </programme>
</tv>
""",
                encoding="utf-8",
            )
            cache.write_text(
                json.dumps(
                    {
                        "channels": [
                            {
                                "id": "FanDuelSportsNetworkSouthwestDallasFortWorth.us",
                                "name": "FanDuel Sports Network Southwest/Dallas Fort Worth",
                                "alt_names": ["Bally Sports Southwest/Dallas Fort Worth"],
                                "network": "FanDuel Sports Network",
                                "country": "US",
                            },
                            {
                                "id": "UpTV.us",
                                "name": "Up TV",
                                "alt_names": [],
                                "network": None,
                                "country": "US",
                            },
                        ],
                        "logos": [
                            {
                                "channel": "FanDuelSportsNetworkSouthwestDallasFortWorth.us",
                                "in_use": True,
                                "format": "PNG",
                                "url": "https://logos.example/fanduel.png",
                            },
                            {
                                "channel": "UpTV.us",
                                "in_use": True,
                                "format": "PNG",
                                "url": "https://logos.example/uptv.png",
                            },
                        ],
                    }
                ),
                encoding="utf-8",
            )

            report = enrich(
                {
                    "country": "US",
                    "paths": {
                        "m3u": str(m3u),
                        "source_xmltv": str(source),
                        "output_xmltv": str(output),
                        "iptv_org_cache": str(cache),
                    },
                    "iptv_org_channel_aliases": {
                        "FanDuel Sports Southwest": "FanDuelSportsNetworkSouthwestDallasFortWorth.us",
                        "UPtv": "UpTV.us",
                    },
                    "xmltv_channel_map": {
                        "FanDuel Sports Southwest": "src-sports",
                    },
                },
                allow_network=False,
            )

            self.assertEqual(report["channels"], 2)
            self.assertEqual(report["matched_channels"], 1)
            self.assertEqual(report["logos"]["missing"], 0)
            self.assertIn('tvg-logo="https://logos.example/fanduel.png"', m3u.read_text())

            root = ET.parse(output).getroot()
            channels = root.findall("channel")
            self.assertEqual(len(channels), 2)
            self.assertEqual(channels[0].find("icon").attrib["src"], "https://logos.example/fanduel.png")
            programme = root.find("programme")
            self.assertEqual(programme.attrib["channel"], "219")
            self.assertEqual(programme.findtext("title"), "Pregame")
            self.assertIsNone(programme.find("episode-num"))

    def test_dry_run_reports_changes_without_writing(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            tmp = Path(tmpdir)
            m3u = tmp / "provider.m3u"
            source = tmp / "source.xml"
            output = tmp / "out.xml"
            cache = tmp / "iptv-cache.json"

            original_m3u = "\n".join(
                [
                    "#EXTM3U",
                    '#EXTINF:-1 tvg-id="100" tvg-chno="100" tvg-name="No Logo" tvg-logo="",No Logo',
                    "http://example.invalid/100",
                ]
            ) + "\n"
            m3u.write_text(original_m3u, encoding="utf-8")
            source.write_text(
                """<?xml version="1.0" encoding="UTF-8"?>
<tv>
  <channel id="src-100"><display-name>No Logo</display-name></channel>
  <programme channel="src-100" start="20260825000000 +0000" stop="20260825003000 +0000">
    <title>Test</title>
  </programme>
</tv>
""",
                encoding="utf-8",
            )
            cache.write_text(
                json.dumps(
                    {
                        "channels": [
                            {
                                "id": "NoLogo.us",
                                "name": "No Logo",
                                "alt_names": [],
                                "network": None,
                                "country": "US",
                            }
                        ],
                        "logos": [
                            {
                                "channel": "NoLogo.us",
                                "in_use": True,
                                "format": "PNG",
                                "url": "https://logos.example/no-logo.png",
                            }
                        ],
                    }
                ),
                encoding="utf-8",
            )

            report = enrich(
                {
                    "country": "US",
                    "paths": {
                        "m3u": str(m3u),
                        "source_xmltv": str(source),
                        "output_xmltv": str(output),
                        "iptv_org_cache": str(cache),
                    },
                    "iptv_org_channel_aliases": {"No Logo": "NoLogo.us"},
                    "xmltv_channel_map": {"No Logo": "src-100"},
                },
                allow_network=False,
                dry_run=True,
            )

            self.assertTrue(report["dry_run"])
            self.assertTrue(report["m3u_changed"])
            self.assertTrue(report["xmltv_changed"])
            self.assertEqual(m3u.read_text(encoding="utf-8"), original_m3u)
            self.assertFalse(output.exists())

    def test_reports_unmatched_channels_and_missing_logos(self):
        with tempfile.TemporaryDirectory() as tmpdir:
            tmp = Path(tmpdir)
            m3u = tmp / "provider.m3u"
            source = tmp / "source.xml"
            output = tmp / "out.xml"
            cache = tmp / "iptv-cache.json"

            m3u.write_text(
                "#EXTM3U\n"
                '#EXTINF:-1 tvg-id="999" tvg-chno="999" tvg-name="Missing Metadata" tvg-logo="",Missing Metadata\n'
                "http://example.invalid/999\n",
                encoding="utf-8",
            )
            source.write_text('<?xml version="1.0" encoding="UTF-8"?><tv />\n', encoding="utf-8")
            cache.write_text('{"channels":[],"logos":[]}', encoding="utf-8")

            report = enrich(
                {
                    "country": "US",
                    "paths": {
                        "m3u": str(m3u),
                        "source_xmltv": str(source),
                        "output_xmltv": str(output),
                        "iptv_org_cache": str(cache),
                    },
                },
                allow_network=False,
                dry_run=True,
            )

            self.assertEqual(report["logos"]["missing"], 1)
            self.assertEqual(report["missing_logo_channels"], ["Missing Metadata"])
            self.assertEqual(report["unmatched_channels"], ["Missing Metadata"])


if __name__ == "__main__":
    unittest.main()
