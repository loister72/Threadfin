package src

import "strings"

type ChannelIdentity struct {
	SourceID string
	TvgID    string
	URL      string
}

func NewChannelIdentityFromM3U(channel M3UChannelStructXEPG) ChannelIdentity {
	return ChannelIdentity{
		SourceID: strings.TrimSpace(channel.FileM3UID),
		TvgID:    strings.TrimSpace(channel.TvgID),
		URL:      strings.TrimSpace(channel.URL),
	}
}

func NewChannelIdentityFromXEPG(channel XEPGChannelStruct) ChannelIdentity {
	return ChannelIdentity{
		SourceID: strings.TrimSpace(channel.FileM3UID),
		TvgID:    strings.TrimSpace(channel.TvgID),
		URL:      strings.TrimSpace(channel.URL),
	}
}

func (identity ChannelIdentity) Key() string {
	hashInput := identity.URL + identity.SourceID
	if identity.TvgID != "" {
		hashInput = identity.URL + identity.TvgID + identity.SourceID
	}

	return getMD5(hashInput)
}
