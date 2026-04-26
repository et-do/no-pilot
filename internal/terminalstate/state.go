package terminalstate

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

const maxOutputBytes = 64 * 1024

// LastCommand captures the most recent command executed through no-pilot's
// execute toolset and the latest output text associated with it.
type LastCommand struct {
	Command string
	Output  string
}

// StartOptions configures how a terminal session is created.
type StartOptions struct {
	Cwd string
	Env []string
}

// Snapshot is a stable external view of a terminal session.
type Snapshot struct {
	ID          string
	Command     string
	Cwd         string
	Env         []string
	Output      string
	OutputBytes int
	Running     bool
	ExitCode    int
	HasExitCode bool
	DurationMS  int64
	StartedAtMS int64
}

type session struct {
	mu          sync.RWMutex
	id          string
	command     string
	cwd         string
	env         []string
	cmd         *exec.Cmd
	stdin       io.WriteCloser
	readers     sync.WaitGroup
	output      []byte
	running     bool
	exitCode    int
	hasExitCode bool
	duration    time.Duration
	startedAt   time.Time
	done        chan struct{}
}

var (
	mu         sync.RWMutex
	sessions   = map[string]*session{}
	lastID     string
	manualLast *LastCommand
)

// Store records a synthetic last-command snapshot. Intended for tests.
func Store(command, output string) {
	mu.Lock()
	defer mu.Unlock()
	manual := LastCommand{Command: command, Output: output}
	manualLast = &manual
	lastID = ""
}

// Start launches a new tracked terminal session.
func Start(command string) (Snapshot, error) {
	return StartWithOptions(command, StartOptions{})
}

// StartWithOptions launches a new tracked terminal session with per-session
// command context (working directory and extra environment variables).
func StartWithOptions(command string, opts StartOptions) (Snapshot, error) {
	cmd := exec.Command("sh", "-c", command)
	if opts.Cwd != "" {
		cmd.Dir = opts.Cwd
	}
	if len(opts.Env) > 0 {
		cmd.Env = append(os.Environ(), opts.Env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return Snapshot{}, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Snapshot{}, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return Snapshot{}, fmt.Errorf("stderr pipe: %w", err)
	}

	s := &session{
		id:        newID(),
		command:   command,
		cwd:       opts.Cwd,
		env:       append([]string(nil), opts.Env...),
		cmd:       cmd,
		stdin:     stdin,
		running:   true,
		startedAt: time.Now(),
		done:      make(chan struct{}),
	}

	mu.Lock()
	sessions[s.id] = s
	lastID = s.id
	manualLast = nil
	mu.Unlock()

	if err := cmd.Start(); err != nil {
		mu.Lock()
		delete(sessions, s.id)
		if lastID == s.id {
			lastID = ""
		}
		mu.Unlock()
		return Snapshot{}, fmt.Errorf("start command: %w", err)
	}

	s.readers.Add(2)
	go s.capture(stdout)
	go s.capture(stderr)
	go s.wait()

	return s.snapshot(), nil
}

// Wait blocks until a session exits or the timeout elapses. When timeout is 0,
// it waits indefinitely.
func Wait(id string, timeout time.Duration) (Snapshot, bool, error) {
	s, ok := getSession(id)
	if !ok {
		return Snapshot{}, false, fmt.Errorf("terminal %q not found", id)
	}

	if timeout == 0 {
		<-s.done
		return s.snapshot(), true, nil
	}

	select {
	case <-s.done:
		return s.snapshot(), true, nil
	case <-time.After(timeout):
		return s.snapshot(), false, nil
	}
}

// GetOutput returns the current snapshot for a session.
func GetOutput(id string) (Snapshot, bool) {
	s, ok := getSession(id)
	if !ok {
		return Snapshot{}, false
	}
	return s.snapshot(), true
}

// ListSnapshots returns all tracked terminal sessions, newest first.
func ListSnapshots() []Snapshot {
	mu.RLock()
	all := make([]*session, 0, len(sessions))
	for _, s := range sessions {
		all = append(all, s)
	}
	mu.RUnlock()

	out := make([]Snapshot, 0, len(all))
	for _, s := range all {
		out = append(out, s.snapshot())
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].StartedAtMS > out[j].StartedAtMS
	})
	return out
}

// Send writes a line of input to a running terminal session.
func Send(id, input string) (Snapshot, error) {
	s, ok := getSession(id)
	if !ok {
		return Snapshot{}, fmt.Errorf("terminal %q not found", id)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.running {
		return s.snapshotLocked(), fmt.Errorf("terminal %q is not running", id)
	}

	line := "\n"
	if strings.TrimSpace(input) != "" {
		line = input + "\n"
	}
	if _, err := io.WriteString(s.stdin, line); err != nil {
		return s.snapshotLocked(), fmt.Errorf("send to terminal %q: %w", id, err)
	}
	return s.snapshotLocked(), nil
}

// Kill terminates a running terminal session.
func Kill(id string) (Snapshot, error) {
	s, ok := getSession(id)
	if !ok {
		return Snapshot{}, fmt.Errorf("terminal %q not found", id)
	}

	s.mu.RLock()
	running := s.running
	cmd := s.cmd
	s.mu.RUnlock()
	if !running {
		return s.snapshot(), nil
	}
	if err := cmd.Process.Kill(); err != nil {
		return s.snapshot(), fmt.Errorf("kill terminal %q: %w", id, err)
	}
	<-s.done
	return s.snapshot(), nil
}

// LastSnapshot returns the current snapshot of the most recently started
// tracked terminal session, or the latest synthetic test snapshot.
func LastSnapshot() Snapshot {
	mu.RLock()
	last := lastID
	manual := manualLast
	mu.RUnlock()

	if last != "" {
		if snap, ok := GetOutput(last); ok {
			return snap
		}
	}
	if manual != nil {
		return Snapshot{Command: manual.Command, Output: manual.Output}
	}
	return Snapshot{}
}

// Get returns the current last-command snapshot in the legacy shape used by
// read_terminalLastCommand.
func Get() LastCommand {
	snap := LastSnapshot()
	return LastCommand{Command: snap.Command, Output: snap.Output}
}

// Reset clears state and terminates any running sessions. Intended for tests.
func Reset() {
	mu.Lock()
	current := sessions
	sessions = map[string]*session{}
	lastID = ""
	manualLast = nil
	mu.Unlock()

	for _, s := range current {
		s.mu.RLock()
		running := s.running
		cmd := s.cmd
		s.mu.RUnlock()
		if running && cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
}

func getSession(id string) (*session, bool) {
	mu.RLock()
	s, ok := sessions[id]
	mu.RUnlock()
	return s, ok
}

func (s *session) capture(r io.Reader) {
	defer s.readers.Done()
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			s.appendOutput(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func (s *session) appendOutput(chunk []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.output = append(s.output, chunk...)
	if len(s.output) > maxOutputBytes {
		s.output = append([]byte(nil), s.output[len(s.output)-maxOutputBytes:]...)
	}
}

func (s *session) wait() {
	err := s.cmd.Wait()
	s.readers.Wait()
	duration := time.Since(s.startedAt)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
			s.appendOutput([]byte(err.Error()))
		}
	}

	s.mu.Lock()
	s.running = false
	s.hasExitCode = true
	s.exitCode = exitCode
	s.duration = duration
	_ = s.stdin.Close()
	s.mu.Unlock()

	close(s.done)
}

func (s *session) snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotLocked()
}

func (s *session) snapshotLocked() Snapshot {
	return Snapshot{
		ID:          s.id,
		Command:     s.command,
		Cwd:         s.cwd,
		Env:         append([]string(nil), s.env...),
		Output:      string(s.output),
		OutputBytes: len(s.output),
		Running:     s.running,
		ExitCode:    s.exitCode,
		HasExitCode: s.hasExitCode,
		DurationMS:  s.duration.Milliseconds(),
		StartedAtMS: s.startedAt.UnixMilli(),
	}
}

func newID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("terminal-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(raw[:])
}