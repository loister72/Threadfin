package src

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type ThirdPartyProcess struct {
	cmd    *exec.Cmd
	stdout io.ReadCloser
	stderr io.ReadCloser
	once   sync.Once
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
		cmd:    cmd,
		stdout: stdout,
		stderr: stderr,
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
	return p.stderr
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
