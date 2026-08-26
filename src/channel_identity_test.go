package src

import "testing"

func TestChannelIdentityPreservesExistingHashSemantics(t *testing.T) {
	identity := ChannelIdentity{
		SourceID: "playlist-a",
		TvgID:    "fox-news",
		URL:      "http://example.test/fox-news",
	}

	want := getMD5("http://example.test/fox-news" + "fox-news" + "playlist-a")
	if got := identity.Key(); got != want {
		t.Fatalf("ChannelIdentity.Key() = %q, want %q", got, want)
	}
}

func TestChannelIdentityFallsBackToURLAndSourceWhenTvgIDMissing(t *testing.T) {
	identity := ChannelIdentity{
		SourceID: "playlist-a",
		URL:      "http://example.test/fox-news",
	}

	want := getMD5("http://example.test/fox-news" + "playlist-a")
	if got := identity.Key(); got != want {
		t.Fatalf("ChannelIdentity.Key() = %q, want %q", got, want)
	}
}

func TestChannelIdentityTrimsInputs(t *testing.T) {
	channel := M3UChannelStructXEPG{
		FileM3UID: " playlist-a ",
		TvgID:     " fox-news ",
		URL:       " http://example.test/fox-news ",
	}

	want := getMD5("http://example.test/fox-news" + "fox-news" + "playlist-a")
	if got := NewChannelIdentityFromM3U(channel).Key(); got != want {
		t.Fatalf("NewChannelIdentityFromM3U().Key() = %q, want %q", got, want)
	}
}
