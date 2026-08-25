package src

import (
	"reflect"
	"testing"
)

func TestNextBackupNumberSkipsMissingBackups(t *testing.T) {
	stream := ThisStream{
		BackupChannel2: &BackupStream{URL: "http://backup-2.example.test/stream"},
		BackupChannel3: &BackupStream{URL: "http://backup-3.example.test/stream"},
	}

	next, ok := nextBackupNumber(stream, 0)
	if !ok || next != 2 {
		t.Fatalf("nextBackupNumber(..., 0) = %d, %t; want 2, true", next, ok)
	}

	next, ok = nextBackupNumber(stream, 2)
	if !ok || next != 3 {
		t.Fatalf("nextBackupNumber(..., 2) = %d, %t; want 3, true", next, ok)
	}

	next, ok = nextBackupNumber(stream, 3)
	if ok || next != 0 {
		t.Fatalf("nextBackupNumber(..., 3) = %d, %t; want 0, false", next, ok)
	}
}

func TestSelectThirdPartyStreamURLFallsBackToPrimaryForMissingBackup(t *testing.T) {
	stream := ThisStream{
		URL:            "http://primary.example.test/stream",
		BackupChannel2: &BackupStream{URL: "http://backup-2.example.test/stream"},
	}

	gotURL, selectedBackup := selectThirdPartyStreamURL(stream, true, 1)
	if gotURL != stream.URL || selectedBackup != 0 {
		t.Fatalf("selectThirdPartyStreamURL missing backup = %q, %d; want primary, 0", gotURL, selectedBackup)
	}

	gotURL, selectedBackup = selectThirdPartyStreamURL(stream, true, 2)
	if gotURL != stream.BackupChannel2.URL || selectedBackup != 2 {
		t.Fatalf("selectThirdPartyStreamURL backup 2 = %q, %d; want backup 2", gotURL, selectedBackup)
	}
}

func TestBuildThirdPartyArgsForFFmpeg(t *testing.T) {
	oldSettings := Settings
	t.Cleanup(func() { Settings = oldSettings })

	Settings.UserAgent = "Threadfin Test"
	playlist := Playlist{
		HttpProxyIP:     "127.0.0.1",
		HttpProxyPort:   "8888",
		HttpUserReferer: "https://referer.example.test/path",
		HttpUserOrigin:  "https://origin.example.test",
	}

	got, err := buildThirdPartyArgs("FFMPEG", `-hide_banner -i "[URL]" -f mpegts pipe:1`, "https://source.example.test/live token", playlist)
	if err != nil {
		t.Fatalf("buildThirdPartyArgs returned error: %v", err)
	}

	want := []string{
		"-user_agent",
		"Threadfin Test",
		"-http_proxy",
		"http://127.0.0.1:8888",
		"-headers",
		"Referer: https://referer.example.test/path\r\nOrigin: https://origin.example.test\r\n",
		"-hide_banner",
		"-i",
		"https://source.example.test/live token",
		"-f",
		"mpegts",
		"pipe:1",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildThirdPartyArgs() = %#v, want %#v", got, want)
	}
}

func TestBuildThirdPartyArgsForVLC(t *testing.T) {
	oldSettings := Settings
	t.Cleanup(func() { Settings = oldSettings })

	Settings.UserAgent = "Threadfin Test"
	playlist := Playlist{
		HttpProxyIP:     "127.0.0.1",
		HttpProxyPort:   "8888",
		HttpUserReferer: "https://referer.example.test/path",
	}

	got, err := buildThirdPartyArgs("VLC", `-I dummy "[URL]" --sout "#std{mux=ts,access=file,dst=-}"`, "https://source.example.test/live token", playlist)
	if err != nil {
		t.Fatalf("buildThirdPartyArgs returned error: %v", err)
	}

	want := []string{
		"-I",
		"dummy",
		"https://source.example.test/live token",
		":http-user-agent=Threadfin Test",
		":http-referrer=https://referer.example.test/path",
		":http-proxy=127.0.0.1:8888",
		"--sout",
		"#std{mux=ts,access=file,dst=-}",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildThirdPartyArgs() = %#v, want %#v", got, want)
	}
}

func TestResolveThirdPartyEngineForceHTTP(t *testing.T) {
	oldSettings := Settings
	t.Cleanup(func() { Settings = oldSettings })

	Settings.FFmpegPath = "/usr/bin/ffmpeg"
	Settings.FFmpegOptions = "-i [URL] pipe:1"
	Settings.FFmpegForceHttp = true

	got, forcedHTTP, supported := resolveThirdPartyEngine(Playlist{Buffer: "ffmpeg"}, "https://source.example.test/live")
	if !supported {
		t.Fatal("resolveThirdPartyEngine supported = false, want true")
	}
	if !forcedHTTP {
		t.Fatal("resolveThirdPartyEngine forcedHTTP = false, want true")
	}
	if got.URL != "http://source.example.test/live" {
		t.Fatalf("resolveThirdPartyEngine URL = %q, want forced HTTP URL", got.URL)
	}
	if got.Path != Settings.FFmpegPath || got.Options != Settings.FFmpegOptions || got.BufferType != "FFMPEG" {
		t.Fatalf("resolveThirdPartyEngine config = %#v", got)
	}
}

func TestResolveThirdPartyEngineRejectsUnsupportedBuffer(t *testing.T) {
	_, _, supported := resolveThirdPartyEngine(Playlist{Buffer: "unknown"}, "https://source.example.test/live")
	if supported {
		t.Fatal("resolveThirdPartyEngine supported = true, want false")
	}
}
