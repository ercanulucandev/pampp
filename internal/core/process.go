package core

import (
	"os"
	"os/exec"
	"time"
)

// process wraps a running child and lets us observe its exit.
type process struct {
	cmd  *exec.Cmd
	done chan struct{}
}

func startProcess(cmd *exec.Cmd, logPath string) (*process, error) {
	var logFile *os.File
	if logPath != "" {
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			logFile = f
			cmd.Stdout = f
			cmd.Stderr = f
		}
	}
	if err := cmd.Start(); err != nil {
		if logFile != nil {
			logFile.Close()
		}
		return nil, err
	}
	p := &process{cmd: cmd, done: make(chan struct{})}
	go func() {
		_ = cmd.Wait()
		if logFile != nil {
			logFile.Close()
		}
		close(p.done)
	}()
	return p, nil
}

func (p *process) alive() bool {
	if p == nil {
		return false
	}
	select {
	case <-p.done:
		return false
	default:
		return true
	}
}

// wait blocks until the process exits or the timeout passes; reports whether it exited.
func (p *process) wait(timeout time.Duration) bool {
	if p == nil {
		return true
	}
	select {
	case <-p.done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func (p *process) kill() {
	if !p.alive() {
		return
	}
	killTree(p.cmd.Process.Pid)
	if !p.wait(3 * time.Second) {
		_ = p.cmd.Process.Kill()
		p.wait(2 * time.Second)
	}
}
