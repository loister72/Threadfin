package src

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

const thirdPartyStderrTailBytes = 8192

type ThirdPartyProcess struct {
	cmd        *exec.Cmd
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	stderrTail *boundedLogBuffer
	once       sync.Once
}

func StartThirdPartyProcess(path string, args []string) (*ThirdPartyProcess, error) {
	cmd := exec.Command(path, args...)
	cmd.Env = append(os.Environ(), "DISPLAY=:0")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &ThirdPartyProcess{
		cmd:        cmd,
		stdout:     stdout,
		stderr:     stderr,
		stderrTail: newBoundedLogBuffer(thirdPartyStderrTailBytes),
	}, nil
}

func (p *ThirdPartyProcess) Stdout() io.Reader {
	if p == nil {
		return strings.NewReader("")
	}
	return p.stdout
}

func (p *ThirdPartyProcess) Stderr() io.Reader {
	if p == nil {
		return strings.NewReader("")
	}
	return io.TeeReader(p.stderr, p.stderrTail)
}

func (p *ThirdPartyProcess) StderrTail() string {
	if p == nil || p.stderrTail == nil {
		return ""
	}

	return RedactLogText(p.stderrTail.String())
}

func (p *ThirdPartyProcess) Terminate() {
	if p == nil || p.cmd == nil {
		return
	}

	p.once.Do(func() {
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		_ = p.cmd.Wait()
	})
}

func RedactCommandArgs(args []string) []string {
	redacted := make([]string, len(args))
	copy(redacted, args)

	for i := range redacted {
		if strings.Contains(redacted[i], "://") {
			redacted[i] = redactStreamURL(redacted[i])
		}
	}

	return redacted
}

var logURLPattern = regexp.MustCompile(`https?://[^\s"'<>]+`)

func RedactLogText(value string) string {
	return logURLPattern.ReplaceAllStringFunc(value, redactStreamURL)
}

type boundedLogBuffer struct {
	mu    sync.Mutex
	limit int
	buf   []byte
}

func newBoundedLogBuffer(limit int) *boundedLogBuffer {
	return &boundedLogBuffer{limit: limit}
}

func (b *boundedLogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buf = append(b.buf, p...)
	if len(b.buf) > b.limit {
		b.buf = append([]byte(nil), b.buf[len(b.buf)-b.limit:]...)
	}

	return len(p), nil
}

func (b *boundedLogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return string(bytes.TrimSpace(b.buf))
}
