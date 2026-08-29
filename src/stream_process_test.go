package src

import (
	"runtime"
	"strings"
	"testing"
)

func TestRedactCommandArgsRemovesURLSecrets(t *testing.T) {
	got := RedactCommandArgs([]string{
		"-i",
		"https://user:pass@example.test/live/channel.ts?token=secret",
		"-headers",
		"Referer: https://example.test/path?token=secret\r\n",
	})

	for _, value := range got {
		if strings.Contains(value, "secret") || strings.Contains(value, "user:pass") {
			t.Fatalf("RedactCommandArgs leaked secret in %#v", got)
		}
	}

	if got[1] != "https://example.test/live/channel.ts" {
		t.Fatalf("redacted URL = %q, want query/user stripped", got[1])
	}
}

func TestRedactLogTextRemovesURLSecrets(t *testing.T) {
	got := RedactLogText("ffmpeg opened https://user:pass@example.test/live/channel.ts?token=secret and failed")

	if strings.Contains(got, "secret") || strings.Contains(got, "user:pass") {
		t.Fatalf("RedactLogText leaked secret in %q", got)
	}
	if !strings.Contains(got, "https://example.test/live/channel.ts") {
		t.Fatalf("RedactLogText = %q, want sanitized URL", got)
	}
}

func TestBoundedLogBufferKeepsTail(t *testing.T) {
	buffer := newBoundedLogBuffer(5)

	if _, err := buffer.Write([]byte("hello")); err != nil {
		t.Fatalf("write hello: %v", err)
	}
	if _, err := buffer.Write([]byte(" world")); err != nil {
		t.Fatalf("write world: %v", err)
	}

	if got := buffer.String(); got != "world" {
		t.Fatalf("boundedLogBuffer.String() = %q, want tail", got)
	}
}

func TestThirdPartyProcessTerminateIsIdempotent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}

	process, err := StartThirdPartyProcess("sh", []string{"-c", "sleep 5"})
	if err != nil {
		t.Fatalf("StartThirdPartyProcess returned error: %v", err)
	}

	process.Terminate()
	process.Terminate()
}
