package src

import (
	"reflect"
	"testing"
)

func TestSplitCommandLinePreservesQuotedArguments(t *testing.T) {
	got, err := splitCommandLine(`-hide_banner -headers "Referer: https://example.test/live page\r\nOrigin: https://example.test\r\n" -i [URL] -f mpegts pipe:1`)
	if err != nil {
		t.Fatalf("splitCommandLine returned error: %v", err)
	}

	want := []string{
		"-hide_banner",
		"-headers",
		`Referer: https://example.test/live page\r\nOrigin: https://example.test\r\n`,
		"-i",
		"[URL]",
		"-f",
		"mpegts",
		"pipe:1",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitCommandLine() = %#v, want %#v", got, want)
	}
}

func TestSplitCommandLinePreservesVlcSout(t *testing.T) {
	got, err := splitCommandLine(`-I dummy "[URL]" --sout "#std{mux=ts,access=file,dst=-}" :http-user-agent="Threadfin Test"`)
	if err != nil {
		t.Fatalf("splitCommandLine returned error: %v", err)
	}

	want := []string{
		"-I",
		"dummy",
		"[URL]",
		"--sout",
		"#std{mux=ts,access=file,dst=-}",
		":http-user-agent=Threadfin Test",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitCommandLine() = %#v, want %#v", got, want)
	}
}

func TestSplitCommandLineRejectsUnterminatedQuote(t *testing.T) {
	if _, err := splitCommandLine(`-i "[URL]`); err == nil {
		t.Fatal("splitCommandLine() error = nil, want unterminated quote error")
	}
}
