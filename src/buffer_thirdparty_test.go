package src

import (
	"io"
	"reflect"
	"testing"
	"time"
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

func TestResolveThirdPartyEngineHDHRProfileUsesFFmpeg(t *testing.T) {
	oldSettings := Settings
	t.Cleanup(func() { Settings = oldSettings })

	Settings.FFmpegPath = "/usr/bin/ffmpeg"

	got, forcedHTTP, supported := resolveThirdPartyEngine(Playlist{Buffer: "hdhr-remux"}, "https://source.example.test/live")
	if !supported {
		t.Fatal("resolveThirdPartyEngine supported = false, want true")
	}
	if forcedHTTP {
		t.Fatal("resolveThirdPartyEngine forcedHTTP = true, want false")
	}
	if got.Path != Settings.FFmpegPath {
		t.Fatalf("resolveThirdPartyEngine path = %q, want ffmpeg path", got.Path)
	}
	if got.BufferType != "HDHR-REMUX" {
		t.Fatalf("resolveThirdPartyEngine BufferType = %q, want HDHR-REMUX", got.BufferType)
	}
}

func TestResolveThirdPartyEngineRejectsUnsupportedBuffer(t *testing.T) {
	_, _, supported := resolveThirdPartyEngine(Playlist{Buffer: "unknown"}, "https://source.example.test/live")
	if supported {
		t.Fatal("resolveThirdPartyEngine supported = true, want false")
	}
}

func TestPlaylistBufferDefaultsToGlobal(t *testing.T) {
	oldSettings := Settings
	t.Cleanup(func() { Settings = oldSettings })

	Settings.Buffer = "vlc"

	if got := effectivePlaylistBuffer("default"); got != "vlc" {
		t.Fatalf("effectivePlaylistBuffer(default) = %q, want vlc", got)
	}
	if got := effectivePlaylistBuffer(""); got != "vlc" {
		t.Fatalf("effectivePlaylistBuffer(empty) = %q, want vlc", got)
	}
}

func TestPlaylistBufferProviderOverrideIsExplicit(t *testing.T) {
	oldSettings := Settings
	t.Cleanup(func() { Settings = oldSettings })

	Settings.Buffer = "vlc"
	Settings.Files.M3U = map[string]interface{}{
		"MTEST": map[string]interface{}{
			"buffer":          "ffmpeg",
			"buffer.override": true,
		},
	}

	got, source := getPlaylistBuffer("MTEST")
	if got != "ffmpeg" || source != "provider" {
		t.Fatalf("getPlaylistBuffer override = %q, %q; want ffmpeg, provider", got, source)
	}
}

func TestBuildThirdPartyArgsForHDHRRemux(t *testing.T) {
	oldSettings := Settings
	t.Cleanup(func() { Settings = oldSettings })

	Settings.UserAgent = "Threadfin Test"
	playlist := Playlist{
		HttpProxyIP:     "127.0.0.1",
		HttpProxyPort:   "8888",
		HttpUserReferer: "https://referer.example.test/path",
		HttpUserOrigin:  "https://origin.example.test",
	}

	got, err := buildThirdPartyArgs("HDHR-REMUX", "", "https://source.example.test/live token", playlist)
	if err != nil {
		t.Fatalf("buildThirdPartyArgs returned error: %v", err)
	}

	assertContainsInOrder(t, got, []string{
		"-nostdin",
		"-reconnect_streamed", "1",
		"-reconnect_on_http_error", "4xx,5xx",
		"-fflags", "+genpts+discardcorrupt",
		"-user_agent", "Threadfin Test",
		"-http_proxy", "http://127.0.0.1:8888",
		"-headers", "Referer: https://referer.example.test/path\r\nOrigin: https://origin.example.test\r\n",
		"-i", "https://source.example.test/live token",
		"-map", "0:v:0?",
		"-map", "0:a:0?",
		"-sn",
		"-c:v", "copy",
		"-c:a", "aac",
		"-ar", "48000",
		"-ac", "2",
		"-f", "mpegts",
		"-mpegts_flags", "+resend_headers",
		"pipe:1",
	})
}

func TestBuildThirdPartyArgsForHDHRSafeAddsFrequentTables(t *testing.T) {
	got, err := buildThirdPartyArgs("HDHR-SAFE", "", "https://source.example.test/live", Playlist{})
	if err != nil {
		t.Fatalf("buildThirdPartyArgs returned error: %v", err)
	}

	assertContainsInOrder(t, got, []string{
		"-analyzeduration", "5000000",
		"-probesize", "5000000",
		"-mpegts_flags", "+resend_headers+pat_pmt_at_frames",
		"-pat_period", "0.1",
		"-sdt_period", "0.5",
		"-pcr_period", "20",
		"pipe:1",
	})
}

func assertContainsInOrder(t *testing.T, got []string, want []string) {
	t.Helper()

	next := 0
	for _, value := range got {
		if next < len(want) && value == want[next] {
			next++
		}
	}

	if next != len(want) {
		t.Fatalf("args missing ordered values from %q at index %d; got %#v", want[next], next, got)
	}
}

func TestNormalizeProviderBufferSettingsMigratesLegacyInheritedEngines(t *testing.T) {
	settings := SettingsStruct{}
	settings.Buffer = "vlc"
	settings.Files.M3U = map[string]interface{}{
		"MOLD": map[string]interface{}{"buffer": "ffmpeg"},
		"MDIRECT": map[string]interface{}{
			"buffer": "-",
		},
		"MOVERRIDE": map[string]interface{}{
			"buffer":          "ffmpeg",
			"buffer.override": true,
		},
	}
	settings.Files.HDHR = map[string]interface{}{}

	if !normalizeProviderBufferSettings(&settings) {
		t.Fatal("normalizeProviderBufferSettings changed = false, want true")
	}

	if got := settings.Files.M3U["MOLD"].(map[string]interface{})["buffer"]; got != "default" {
		t.Fatalf("legacy inherited buffer = %q, want default", got)
	}
	if got := settings.Files.M3U["MDIRECT"].(map[string]interface{})["buffer"]; got != "-" {
		t.Fatalf("explicit direct buffer = %q, want -", got)
	}
	if got := settings.Files.M3U["MOVERRIDE"].(map[string]interface{})["buffer"]; got != "ffmpeg" {
		t.Fatalf("explicit override buffer = %q, want ffmpeg", got)
	}
}

func TestNormalizeProviderBufferSaveStoresDefaultForGlobalValue(t *testing.T) {
	oldSettings := Settings
	t.Cleanup(func() { Settings = oldSettings })

	Settings.Buffer = "vlc"
	provider := map[string]interface{}{"buffer": "vlc", "buffer.override": true}

	normalizeProviderBufferSave(provider)

	if got := provider["buffer"]; got != "default" {
		t.Fatalf("provider buffer = %q, want default", got)
	}
	if _, ok := provider["buffer.override"]; ok {
		t.Fatal("buffer.override was kept for default provider buffer")
	}
}

func TestThirdPartyStartupTimeoutUsesConfiguredSeconds(t *testing.T) {
	oldSettings := Settings
	t.Cleanup(func() { Settings = oldSettings })

	Settings.BufferTimeout = 12
	if got := thirdPartyStartupTimeout(); got.Seconds() != 12 {
		t.Fatalf("thirdPartyStartupTimeout = %s, want 12s", got)
	}

	Settings.BufferTimeout = 0
	if got := thirdPartyStartupTimeout(); got != defaultThirdPartyStartupTimeout {
		t.Fatalf("thirdPartyStartupTimeout default = %s, want %s", got, defaultThirdPartyStartupTimeout)
	}

	Settings.BufferTimeout = 0.1
	if got := thirdPartyStartupTimeout(); got != time.Second {
		t.Fatalf("thirdPartyStartupTimeout minimum = %s, want 1s", got)
	}

	Settings.BufferTimeout = 500
	if got := thirdPartyStartupTimeout(); got != maxThirdPartyStartupTimeout {
		t.Fatalf("thirdPartyStartupTimeout maximum = %s, want %s", got, maxThirdPartyStartupTimeout)
	}
}

func TestShouldRestartThirdPartyProcessOnlyAfterActivePrimaryEOF(t *testing.T) {
	if shouldRestartThirdPartyProcess(ThisStream{Status: false}, 0, 0) {
		t.Fatal("inactive startup failure should not restart")
	}

	if !shouldRestartThirdPartyProcess(ThisStream{Status: true}, 0, 0) {
		t.Fatal("active stream without backups should restart")
	}
}

func TestShouldRestartThirdPartyProcessHonorsLimitsAndBackupFailover(t *testing.T) {
	stream := ThisStream{
		Status:         true,
		BackupChannel1: &BackupStream{URL: "http://backup.example.test/stream"},
	}

	if shouldRestartThirdPartyProcess(stream, 0, 0) {
		t.Fatal("stream with backup should use backup failover instead of primary restart")
	}

	if shouldRestartThirdPartyProcess(ThisStream{Status: true}, maxThirdPartyProcessRestarts, 0) {
		t.Fatal("restart limit should stop additional retries")
	}
}

func TestThirdPartySegmentWriterRotatesSegments(t *testing.T) {
	initBufferVFS()

	writer := NewThirdPartySegmentWriter("/stream-test/", 8)
	if err := writer.Reset(); err != nil {
		t.Fatalf("Reset returned error: %v", err)
	}
	if err := writer.CreateCurrent(); err != nil {
		t.Fatalf("CreateCurrent returned error: %v", err)
	}
	if err := writer.OpenCurrent(); err != nil {
		t.Fatalf("OpenCurrent returned error: %v", err)
	}

	if _, err := writer.Write([]byte("abcd")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if !writer.ShouldRotate() {
		t.Fatal("ShouldRotate = false, want true after half-buffer segment")
	}
	if err := writer.Rotate(); err != nil {
		t.Fatalf("Rotate returned error: %v", err)
	}

	if writer.Segment() != 2 {
		t.Fatalf("Segment = %d, want 2", writer.Segment())
	}
	if writer.CurrentSize() != 0 {
		t.Fatalf("CurrentSize = %d, want 0 after rotate", writer.CurrentSize())
	}

	if _, err := writer.Write([]byte("xy")); err != nil {
		t.Fatalf("second Write returned error: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	first, err := readBufferVFSTestFile("/stream-test/1.ts")
	if err != nil {
		t.Fatalf("read first segment: %v", err)
	}
	second, err := readBufferVFSTestFile("/stream-test/2.ts")
	if err != nil {
		t.Fatalf("read second segment: %v", err)
	}

	if string(first) != "abcd" {
		t.Fatalf("first segment = %q, want %q", first, "abcd")
	}
	if string(second) != "xy" {
		t.Fatalf("second segment = %q, want %q", second, "xy")
	}
}

func TestThirdPartySegmentRotateSizeIsCapped(t *testing.T) {
	got := thirdPartySegmentRotateSize(2 * 1024 * 1024)
	if got != maxThirdPartySegmentSize {
		t.Fatalf("thirdPartySegmentRotateSize = %d, want %d", got, maxThirdPartySegmentSize)
	}
}

func TestThirdPartySegmentWriterUsesSmallerStartupSegment(t *testing.T) {
	writer := NewThirdPartySegmentWriter("/stream-test/", 2*1024*1024)

	if got := writer.currentRotateSize(); got != maxThirdPartyStartupSegmentSize {
		t.Fatalf("startup currentRotateSize = %d, want %d", got, maxThirdPartyStartupSegmentSize)
	}

	writer.segment = 2
	if got := writer.currentRotateSize(); got != maxThirdPartySegmentSize {
		t.Fatalf("steady currentRotateSize = %d, want %d", got, maxThirdPartySegmentSize)
	}
}

func TestGetBufTmpFilesReturnsFirstCompleteSegmentAfterFirstRotation(t *testing.T) {
	initBufferVFS()
	writer := NewThirdPartySegmentWriter("/stream-test/", 1024)
	if err := writer.Reset(); err != nil {
		t.Fatalf("reset writer: %v", err)
	}
	if err := writer.CreateCurrent(); err != nil {
		t.Fatalf("create first segment: %v", err)
	}
	if err := writer.OpenCurrent(); err != nil {
		t.Fatalf("open first segment: %v", err)
	}
	if _, err := writer.Write([]byte("first")); err != nil {
		t.Fatalf("write first segment: %v", err)
	}
	if err := writer.Rotate(); err != nil {
		t.Fatalf("rotate to active segment: %v", err)
	}
	defer writer.Close()

	stream := ThisStream{Folder: "/stream-test/"}
	got := getBufTmpFiles(&stream)

	if !reflect.DeepEqual(got, []string{"1.ts"}) {
		t.Fatalf("getBufTmpFiles() = %#v, want first complete segment", got)
	}
}

func readBufferVFSTestFile(filename string) ([]byte, error) {
	f, err := bufferVFS.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return io.ReadAll(f)
}
