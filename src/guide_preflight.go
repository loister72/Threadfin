package src

import "sort"

type GuidePreflightReport struct {
	ActiveChannels       int                    `json:"activeChannels"`
	HiddenChannels       int                    `json:"hiddenChannels"`
	DuplicateGuideNumber []GuidePreflightIssue  `json:"duplicateGuideNumbers,omitempty"`
	DuplicateIdentity    []GuidePreflightIssue  `json:"duplicateIdentities,omitempty"`
	MissingGuideMapping  []GuidePreflightIssue  `json:"missingGuideMappings,omitempty"`
	MissingLogo          []GuidePreflightIssue  `json:"missingLogos,omitempty"`
	Channels             []GuidePreflightRecord `json:"channels"`
}

type GuidePreflightRecord struct {
	XEPG        string `json:"xepg"`
	Name        string `json:"name"`
	GuideNumber string `json:"guideNumber"`
	Identity    string `json:"identity"`
	XMLTVFile   string `json:"xmltvFile"`
	Mapping     string `json:"mapping"`
	Logo        string `json:"logo,omitempty"`
	HasMapping  bool   `json:"hasMapping"`
	HasLogo     bool   `json:"hasLogo"`
}

type GuidePreflightIssue struct {
	XEPG        string `json:"xepg"`
	Name        string `json:"name"`
	GuideNumber string `json:"guideNumber,omitempty"`
	Identity    string `json:"identity,omitempty"`
	Details     string `json:"details"`
}

func BuildGuidePreflightReport(channels map[string]interface{}, xmltvMapping map[string]interface{}) GuidePreflightReport {
	report := GuidePreflightReport{
		Channels: make([]GuidePreflightRecord, 0, len(channels)),
	}
	guideNumbers := make(map[string]GuidePreflightRecord)
	identities := make(map[string]GuidePreflightRecord)

	for id, value := range channels {
		var channel XEPGChannelStruct
		if err := remarshalJSON(value, &channel); err != nil {
			continue
		}
		if !channel.XActive {
			continue
		}
		if channel.XHideChannel {
			report.HiddenChannels++
			continue
		}

		record := buildGuidePreflightRecord(id, channel, xmltvMapping)
		report.ActiveChannels++
		report.Channels = append(report.Channels, record)

		if record.GuideNumber != "" {
			if existing, ok := guideNumbers[record.GuideNumber]; ok {
				report.DuplicateGuideNumber = append(report.DuplicateGuideNumber,
					preflightIssue(existing, "duplicate guide number"),
					preflightIssue(record, "duplicate guide number"),
				)
			} else {
				guideNumbers[record.GuideNumber] = record
			}
		}

		if record.Identity != "" {
			if existing, ok := identities[record.Identity]; ok {
				report.DuplicateIdentity = append(report.DuplicateIdentity,
					preflightIssue(existing, "duplicate channel identity"),
					preflightIssue(record, "duplicate channel identity"),
				)
			} else {
				identities[record.Identity] = record
			}
		}

		if !record.HasMapping {
			report.MissingGuideMapping = append(report.MissingGuideMapping, preflightIssue(record, "missing XMLTV mapping"))
		}
		if !record.HasLogo {
			report.MissingLogo = append(report.MissingLogo, preflightIssue(record, "missing channel logo"))
		}
	}

	sort.Slice(report.Channels, func(i, j int) bool {
		return report.Channels[i].GuideNumber < report.Channels[j].GuideNumber
	})
	sortGuideIssues(report.DuplicateGuideNumber)
	sortGuideIssues(report.DuplicateIdentity)
	sortGuideIssues(report.MissingGuideMapping)
	sortGuideIssues(report.MissingLogo)

	return report
}

func buildGuidePreflightRecord(id string, channel XEPGChannelStruct, xmltvMapping map[string]interface{}) GuidePreflightRecord {
	name := channel.XName
	if name == "" {
		name = channel.TvgName
	}
	if name == "" {
		name = channel.Name
	}

	record := GuidePreflightRecord{
		XEPG:        id,
		Name:        name,
		GuideNumber: channel.XChannelID,
		Identity:    NewChannelIdentityFromXEPG(channel).Key(),
		XMLTVFile:   channel.XmltvFile,
		Mapping:     channel.XMapping,
		Logo:        channel.TvgLogo,
		HasLogo:     channel.TvgLogo != "",
	}

	if record.XMLTVFile == "Threadfin Dummy" && record.Mapping != "" && record.Mapping != "-" {
		record.HasMapping = true
		return record
	}

	if record.XMLTVFile == "" || record.XMLTVFile == "-" || record.Mapping == "" || record.Mapping == "-" {
		return record
	}

	if fileChannels, ok := xmltvMapping[record.XMLTVFile].(map[string]interface{}); ok {
		_, record.HasMapping = fileChannels[record.Mapping]
	}

	return record
}

func preflightIssue(record GuidePreflightRecord, details string) GuidePreflightIssue {
	return GuidePreflightIssue{
		XEPG:        record.XEPG,
		Name:        record.Name,
		GuideNumber: record.GuideNumber,
		Identity:    record.Identity,
		Details:     details,
	}
}

func sortGuideIssues(issues []GuidePreflightIssue) {
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].GuideNumber == issues[j].GuideNumber {
			return issues[i].Name < issues[j].Name
		}
		return issues[i].GuideNumber < issues[j].GuideNumber
	})
}
