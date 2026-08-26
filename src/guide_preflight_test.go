package src

import "testing"

func TestBuildGuidePreflightReportFindsMappingLogoAndDuplicateIssues(t *testing.T) {
	channels := map[string]interface{}{
		"x-ID.1": XEPGChannelStruct{
			FileM3UID:  "playlist-a",
			TvgID:      "fox-news",
			TvgLogo:    "http://logos.test/fox-news.png",
			URL:        "http://streams.test/fox-news",
			XActive:    true,
			XChannelID: "102",
			XEPG:       "x-ID.1",
			XMapping:   "fox-news",
			XmltvFile:  "guide.xml",
			XName:      "Fox News Channel",
		},
		"x-ID.2": XEPGChannelStruct{
			FileM3UID:  "playlist-a",
			TvgID:      "motortrend",
			URL:        "http://streams.test/motortrend",
			XActive:    true,
			XChannelID: "102",
			XEPG:       "x-ID.2",
			XMapping:   "missing",
			XmltvFile:  "guide.xml",
			XName:      "MotorTrend",
		},
		"x-ID.3": XEPGChannelStruct{
			FileM3UID:    "playlist-a",
			TvgID:        "hidden",
			URL:          "http://streams.test/hidden",
			XActive:      true,
			XChannelID:   "999",
			XHideChannel: true,
			XEPG:         "x-ID.3",
			XName:        "Hidden",
		},
	}
	xmltvMapping := map[string]interface{}{
		"guide.xml": map[string]interface{}{
			"fox-news": map[string]interface{}{"id": "fox-news"},
		},
	}

	report := BuildGuidePreflightReport(channels, xmltvMapping)

	if report.ActiveChannels != 2 {
		t.Fatalf("ActiveChannels = %d, want 2", report.ActiveChannels)
	}
	if report.HiddenChannels != 1 {
		t.Fatalf("HiddenChannels = %d, want 1", report.HiddenChannels)
	}
	if len(report.DuplicateGuideNumber) != 2 {
		t.Fatalf("DuplicateGuideNumber length = %d, want 2", len(report.DuplicateGuideNumber))
	}
	if len(report.MissingGuideMapping) != 1 || report.MissingGuideMapping[0].Name != "MotorTrend" {
		t.Fatalf("MissingGuideMapping = %#v, want MotorTrend issue", report.MissingGuideMapping)
	}
	if len(report.MissingLogo) != 1 || report.MissingLogo[0].Name != "MotorTrend" {
		t.Fatalf("MissingLogo = %#v, want MotorTrend issue", report.MissingLogo)
	}
}

func TestBuildGuidePreflightReportFindsDuplicateIdentity(t *testing.T) {
	channels := map[string]interface{}{
		"x-ID.1": XEPGChannelStruct{
			FileM3UID:  "playlist-a",
			TvgID:      "fox-news",
			URL:        "http://streams.test/fox-news",
			XActive:    true,
			XChannelID: "102",
			XMapping:   "PPV",
			XmltvFile:  "Threadfin Dummy",
			XName:      "Fox News A",
		},
		"x-ID.2": XEPGChannelStruct{
			FileM3UID:  "playlist-a",
			TvgID:      "fox-news",
			URL:        "http://streams.test/fox-news",
			XActive:    true,
			XChannelID: "103",
			XMapping:   "PPV",
			XmltvFile:  "Threadfin Dummy",
			XName:      "Fox News B",
		},
	}

	report := BuildGuidePreflightReport(channels, nil)

	if len(report.DuplicateIdentity) != 2 {
		t.Fatalf("DuplicateIdentity length = %d, want 2", len(report.DuplicateIdentity))
	}
	if len(report.MissingGuideMapping) != 0 {
		t.Fatalf("MissingGuideMapping length = %d, want 0 for Threadfin Dummy PPV", len(report.MissingGuideMapping))
	}
}
