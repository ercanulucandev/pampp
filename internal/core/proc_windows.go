package core

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	createNoWindow         = 0x08000000
	createNewConsole       = 0x00000010
	createBreakawayFromJob = 0x01000000
)

// InitJobObject puts the current process into a job that kills every child
// (nginx, php-cgi, mariadb...) when PAMPP exits, even after a crash.
func InitJobObject() {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE | windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK,
		},
	}
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		windows.CloseHandle(job)
		return
	}
	_ = windows.AssignProcessToJobObject(job, windows.CurrentProcess())
	// The handle is intentionally kept open for the lifetime of the process.
}

// hiddenCmd builds a command that runs without a console window.
func hiddenCmd(exe string, args ...string) *exec.Cmd {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
	return cmd
}

// detachedCmd builds a command that survives PAMPP exiting (terminals, editors).
func detachedCmd(exe string, args ...string) *exec.Cmd {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNewConsole | createBreakawayFromJob}
	return cmd
}

func killTree(pid int) {
	_ = hiddenCmd("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid)).Run()
}

// shellOpen opens a file, folder or URL with its default handler.
func shellOpen(target string) error {
	return windows.ShellExecute(0, windows.StringToUTF16Ptr("open"), windows.StringToUTF16Ptr(target), nil, nil, windows.SW_SHOWNORMAL)
}

// runElevated starts exe with a UAC prompt.
func runElevated(exe string, args string) error {
	return windows.ShellExecute(0, windows.StringToUTF16Ptr("runas"), windows.StringToUTF16Ptr(exe),
		windows.StringToUTF16Ptr(args), nil, windows.SW_HIDE)
}

func portFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

func waitPort(port int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			c.Close()
			return true
		}
		time.Sleep(150 * time.Millisecond)
	}
	return false
}

// portOwner tries to name the process listening on a port, e.g. "httpd.exe (PID 1234)".
func portOwner(port int) string {
	out, err := hiddenCmd("netstat", "-ano", "-p", "TCP").Output()
	if err != nil {
		return ""
	}
	suffix := ":" + strconv.Itoa(port)
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) < 5 || !strings.EqualFold(f[3], "LISTENING") || !strings.HasSuffix(f[1], suffix) {
			continue
		}
		pid := f[4]
		name := pid
		if o, err := hiddenCmd("tasklist", "/FI", "PID eq "+pid, "/FO", "CSV", "/NH").Output(); err == nil {
			parts := strings.Split(strings.TrimSpace(string(o)), ",")
			if len(parts) > 0 && strings.HasPrefix(parts[0], `"`) {
				name = strings.Trim(parts[0], `"`)
			}
		}
		return fmt.Sprintf("%s (PID %s)", name, pid)
	}
	return ""
}

func portBusyError(port int) error {
	if owner := portOwner(port); owner != "" {
		return fmt.Errorf("port %d kullanımda: %s", port, owner)
	}
	return fmt.Errorf("port %d kullanımda", port)
}

func vcRedistInstalled() bool {
	sys := os.Getenv("SystemRoot")
	if sys == "" {
		sys = `C:\Windows`
	}
	return exists(sys+`\System32\vcruntime140.dll`) && exists(sys+`\System32\vcruntime140_1.dll`)
}
