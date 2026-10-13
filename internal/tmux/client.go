package tmux

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

type PaneInfo struct {
	ID      string
	Process string
	Title   string
	PID     string
	Command string // current foreground command name
	Dead    bool   // true when pane process has exited (remain-on-exit)
	// Direct panes run their command as the pane process, so Dead alone
	// reports exit. Older panes typed it into an interactive shell instead.
	Direct bool
}

type runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg != "" {
			return "", fmt.Errorf("%w: %s", err, msg)
		}
		return "", err
	}
	return stdout.String(), nil
}

type Client struct {
	bin    string
	runner runner
}

func NewClient(bin string) *Client {
	if strings.TrimSpace(bin) == "" {
		bin = "tmux"
	}
	return &Client{bin: bin, runner: execRunner{}}
}

func (c *Client) EnsureTmux(ctx context.Context) error {
	if _, err := exec.LookPath(c.bin); err != nil {
		return fmt.Errorf("tmux is required but was not found in PATH")
	}
	_, err := c.runner.Run(ctx, c.bin, "-V")
	if err != nil {
		return fmt.Errorf("tmux not available: %w", err)
	}
	return nil
}

func (c *Client) EnsureSession(ctx context.Context, session string) error {
	_, err := c.runner.Run(ctx, c.bin, "has-session", "-t", SessionTarget(session))
	if err == nil {
		return nil
	}
	if !isMissingTmuxTarget(err) {
		return fmt.Errorf("check tmux session %q: %w", session, err)
	}
	_, err = c.runner.Run(ctx, c.bin, "new-session", "-d", "-s", session)
	if err != nil {
		return fmt.Errorf("create tmux session %q: %w", session, err)
	}
	return nil
}

func (c *Client) HasWindow(ctx context.Context, session, window string) (bool, error) {
	_, err := c.runner.Run(ctx, c.bin, "list-windows", "-t", WindowTarget(session, window))
	if err == nil {
		return true, nil
	}
	if isMissingTmuxTarget(err) {
		return false, nil
	}
	return false, fmt.Errorf("check window %q in session %q: %w", window, session, err)
}

// SessionTarget returns an exact-match tmux target for session. Without the
// "=" prefix tmux accepts a unique prefix or pattern match, which could pick a
// different wts session whose name merely starts with session. The trailing
// ":" matters: set-option and show-option reject "=name" without it.
func SessionTarget(session string) string {
	return "=" + session + ":"
}

// WindowTarget returns the tmux target for a window inside a specific session.
// Both parts use tmux's "=" exact-match prefix so a missing session or window
// fails instead of resolving against the current session or a prefix match.
// Every session/window lookup must go through this helper: a bare window name
// is resolved against the client's current session, not the worktree's.
func WindowTarget(session, window string) string {
	return SessionTarget(session) + "=" + window
}

// FindWindowSessions returns every session, in tmux order, that contains a
// window named exactly window. It is used to locate worktree windows created
// under another wts session name.
func (c *Client) FindWindowSessions(ctx context.Context, window string) ([]string, error) {
	output, err := c.runner.Run(ctx, c.bin, "list-windows", "-a", "-F", "#{session_name}\t#{window_name}")
	if err != nil {
		if isMissingTmuxTarget(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list tmux windows: %w", err)
	}
	return parseWindowSessions(output, window), nil
}

func parseWindowSessions(output, window string) []string {
	var sessions []string
	seen := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSuffix(line, "\r")
		session, name, ok := strings.Cut(line, "\t")
		if !ok || name != window || session == "" || seen[session] {
			continue
		}
		seen[session] = true
		sessions = append(sessions, session)
	}
	return sessions
}

// placeholderCommand holds a new pane open until it is configured and the real
// command replaces it. Two arguments make tmux execute it directly rather than
// through the user's default shell.
var placeholderCommand = []string{"cat", "-"}

// ListWindows returns the window names in session, or none if it is missing.
func (c *Client) ListWindows(ctx context.Context, session string) ([]string, error) {
	output, err := c.runner.Run(ctx, c.bin, "list-windows", "-t", SessionTarget(session), "-F", "#{window_name}")
	if err != nil {
		if isMissingTmuxTarget(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("list windows in session %q: %w", session, err)
	}
	var names []string
	for _, name := range strings.Split(output, "\n") {
		if name = strings.TrimSuffix(name, "\r"); name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

func (c *Client) StartWindowCommand(ctx context.Context, session, window, dir, shell, command string, env map[string]string, paneTitle string) error {
	exists, err := c.HasWindow(ctx, session, window)
	if err != nil {
		return err
	}
	if exists {
		if _, err := c.runner.Run(ctx, c.bin, "kill-window", "-t", WindowTarget(session, window)); err != nil {
			return fmt.Errorf("reset existing window %q: %w", window, err)
		}
	}

	args := append([]string{"new-window", "-d", "-t", SessionTarget(session), "-n", window, "-c", dir, "-P", "-F", "#{pane_id}", "--"}, placeholderCommand...)
	paneID, err := c.runner.Run(ctx, c.bin, args...)
	if err != nil {
		return fmt.Errorf("create window %q: %w", window, err)
	}
	if err := c.launch(ctx, strings.TrimSpace(paneID), dir, shell, command, env, paneTitle); err != nil {
		_, _ = c.runner.Run(ctx, c.bin, "kill-window", "-t", WindowTarget(session, window))
		return fmt.Errorf("start command in window %q: %w", window, err)
	}
	return nil
}

// launch configures a placeholder pane and replaces its process with command.
// The command is passed to tmux as arguments, so it never goes through the
// user's interactive shell: its quoting rules, rc files, and history.
func (c *Client) launch(ctx context.Context, paneID, dir, shell, command string, env map[string]string, paneTitle string) error {
	if paneID == "" {
		return fmt.Errorf("tmux did not report the new pane")
	}
	if err := c.hardenPane(ctx, paneID, paneTitle); err != nil {
		return fmt.Errorf("configure pane: %w", err)
	}
	if _, err := c.runner.Run(ctx, c.bin, "respawn-pane", "-k", "-t", paneID, "-c", dir, "--", shell, "-lc", buildPayload(command, env)); err != nil {
		return fmt.Errorf("run command: %w", err)
	}
	return nil
}

func (c *Client) StopWindow(ctx context.Context, session, window string, timeout time.Duration) error {
	exists, err := c.HasWindow(ctx, session, window)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	panes, err := c.ListPanes(ctx, session, window)
	if err != nil {
		if isMissingTmuxTarget(err) {
			return nil
		}
		return err
	}
	for _, pane := range panes {
		if c.ProcessExited(ctx, pane) {
			continue
		}
		if _, err := c.runner.Run(ctx, c.bin, "send-keys", "-t", pane.ID, "C-c"); err != nil {
			if isMissingTmuxTarget(err) {
				continue
			}
			return fmt.Errorf("interrupt pane %q in window %q: %w", pane.ID, window, err)
		}
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		alive, err := c.HasWindow(ctx, session, window)
		if err != nil {
			return err
		}
		if !alive {
			return nil
		}
		exited, err := c.windowProcessesExited(ctx, session, window)
		if err != nil {
			return err
		}
		if exited {
			break
		}
		if err := waitForPoll(ctx, 200*time.Millisecond); err != nil {
			return err
		}
	}

	if _, err := c.runner.Run(ctx, c.bin, "kill-window", "-t", WindowTarget(session, window)); err != nil && !isMissingTmuxTarget(err) {
		return fmt.Errorf("force kill window %q: %w", window, err)
	}
	return nil
}

func (c *Client) windowProcessesExited(ctx context.Context, session, window string) (bool, error) {
	panes, err := c.ListPanes(ctx, session, window)
	if err != nil {
		if isMissingTmuxTarget(err) {
			return true, nil
		}
		return false, err
	}
	if len(panes) == 0 {
		return false, nil
	}
	for _, pane := range panes {
		if !c.ProcessExited(ctx, pane) {
			return false, nil
		}
	}
	return true, nil
}

// ProcessExited reports whether a pane's managed process has finished. Direct
// panes die with their process. Older panes ran it from an interactive shell,
// so the process finished once that shell is in the foreground with no child.
func (c *Client) ProcessExited(ctx context.Context, pane PaneInfo) bool {
	return ProcessExited(pane, func(pid string) bool { return c.PaneExitedByPID(ctx, pid) })
}

// ProcessExited is the pane exit rule shared with callers that check child
// processes through another backend.
func ProcessExited(pane PaneInfo, childless func(pid string) bool) bool {
	switch {
	case pane.Dead:
		return true
	case pane.Direct:
		return false
	case !IsShellCommand(pane.Command):
		return false
	default:
		return pane.PID == "" || childless(pane.PID)
	}
}

func (c *Client) SetSessionOption(ctx context.Context, session, key, value string) error {
	if value == "" {
		_, err := c.runner.Run(ctx, c.bin, "set-option", "-t", SessionTarget(session), "-q", "-u", key)
		if err != nil {
			return fmt.Errorf("unset tmux option %q: %w", key, err)
		}
		return nil
	}
	_, err := c.runner.Run(ctx, c.bin, "set-option", "-t", SessionTarget(session), "-q", key, value)
	if err != nil {
		return fmt.Errorf("set tmux option %q: %w", key, err)
	}
	return nil
}

func (c *Client) GetSessionOption(ctx context.Context, session, key string) (string, error) {
	value, err := c.runner.Run(ctx, c.bin, "show-option", "-t", SessionTarget(session), "-v", key)
	if err != nil {
		message := strings.ToLower(err.Error())
		if isMissingTmuxTarget(err) || strings.Contains(message, "invalid option") || strings.Contains(message, "unknown option") {
			return "", nil
		}
		return "", fmt.Errorf("read tmux option %q: %w", key, err)
	}
	// show-option adds one newline; the option itself may end in whitespace.
	return strings.TrimSuffix(value, "\n"), nil
}

func (c *Client) CapturePane(ctx context.Context, session, window string, lines int) (string, error) {
	if lines <= 0 {
		lines = 200
	}
	output, err := c.runner.Run(ctx, c.bin, "capture-pane", "-p", "-t", WindowTarget(session, window), "-S", fmt.Sprintf("-%d", lines))
	if err != nil {
		return "", fmt.Errorf("capture pane for %q: %w", window, err)
	}
	return output, nil
}

func (c *Client) Attach(ctx context.Context, session, window, paneID string) error {
	if _, err := c.runner.Run(ctx, c.bin, "select-window", "-t", WindowTarget(session, window)); err != nil {
		return fmt.Errorf("select window %q: %w", window, err)
	}
	if paneID != "" {
		if _, err := c.runner.Run(ctx, c.bin, "select-pane", "-t", paneID); err != nil {
			return fmt.Errorf("select pane %q: %w", paneID, err)
		}
	}
	if os.Getenv("TMUX") != "" {
		if _, err := c.runner.Run(ctx, c.bin, "switch-client", "-t", SessionTarget(session)); err != nil {
			return fmt.Errorf("switch tmux client to %q: %w", session, err)
		}
		return nil
	}
	cmd := exec.CommandContext(ctx, c.bin, "attach-session", "-t", SessionTarget(session))
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("attach tmux session %q: %w", session, err)
	}
	return nil
}

func (c *Client) SplitWindowCommand(ctx context.Context, session, window, dir, shell, command string, env map[string]string, paneTitle string) error {
	target := WindowTarget(session, window)
	// Splitting halves the active pane, so balance the layout around each
	// split; otherwise a detached 80x24 window has no room after a few panes.
	c.tileWindow(ctx, target)
	args := append([]string{"split-window", "-d", "-t", target, "-c", dir, "-P", "-F", "#{pane_id}", "--"}, placeholderCommand...)
	paneID, err := c.runner.Run(ctx, c.bin, args...)
	if err != nil {
		return fmt.Errorf("split window %q: %w", window, err)
	}
	paneID = strings.TrimSpace(paneID)
	c.tileWindow(ctx, target)

	if err := c.launch(ctx, paneID, dir, shell, command, env, paneTitle); err != nil {
		_, _ = c.runner.Run(ctx, c.bin, "kill-pane", "-t", paneID)
		return fmt.Errorf("start %q: %w", paneTitle, err)
	}
	return nil
}

// tileWindow is best effort: a failed layout only matters if a later split
// lacks space, and that split reports its own error.
func (c *Client) tileWindow(ctx context.Context, target string) {
	_, _ = c.runner.Run(ctx, c.bin, "select-layout", "-t", target, "tiled")
}

// hardenPane configures a pane for reliable identity tracking across
// arbitrary tmux configurations. It sets the pane title, the @wts_process
// per-pane option (which cannot be overwritten by the shell, unlike pane_title),
// and enables remain-on-exit so panes survive process exit and logs remain
// accessible.
func (c *Client) hardenPane(ctx context.Context, target, paneTitle string) error {
	if paneTitle == "" {
		return nil
	}
	// Set pane title before the shell starts — the shell will likely
	// overwrite pane_title via escape sequences, but @wts_process persists.
	if _, err := c.runner.Run(ctx, c.bin, "select-pane", "-t", target, "-T", paneTitle); err != nil {
		return fmt.Errorf("set title: %w", err)
	}
	if processName := ProcessFromPaneTitle(paneTitle); processName != "" {
		if _, err := c.runner.Run(ctx, c.bin, "set-option", "-p", "-t", target, "-q", PaneProcessOptionKey(), processName); err != nil {
			return fmt.Errorf("set process identity: %w", err)
		}
	}
	// Keep pane alive after process exit so we can still read logs and identity.
	if _, err := c.runner.Run(ctx, c.bin, "set-option", "-p", "-t", target, "-q", "remain-on-exit", "on"); err != nil {
		return fmt.Errorf("enable remain-on-exit: %w", err)
	}
	if _, err := c.runner.Run(ctx, c.bin, "set-option", "-p", "-t", target, "-q", PaneLaunchOptionKey(), directLaunch); err != nil {
		return fmt.Errorf("mark direct launch: %w", err)
	}
	return nil
}

func (c *Client) ListPanes(ctx context.Context, session, window string) ([]PaneInfo, error) {
	output, err := c.runner.Run(ctx, c.bin, "list-panes", "-t", WindowTarget(session, window), "-F", "#{pane_id}\t#{@wts_process}\t#{pane_title}\t#{pane_pid}\t#{pane_current_command}\t#{pane_dead}\t#{@wts_launch}")
	if err != nil {
		return nil, fmt.Errorf("list panes for %q: %w", window, err)
	}
	return parsePaneList(output), nil
}

func parsePaneList(output string) []PaneInfo {
	var panes []PaneInfo
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 7)
		if len(parts) < 4 {
			continue
		}
		info := PaneInfo{
			ID:      parts[0],
			Process: parts[1],
			Title:   parts[2],
			PID:     parts[3],
		}
		if len(parts) >= 5 {
			info.Command = parts[4]
		}
		if len(parts) >= 6 {
			info.Dead = parts[5] == "1"
		}
		if len(parts) >= 7 {
			info.Direct = parts[6] == directLaunch
		}
		panes = append(panes, info)
	}
	return panes
}

func (c *Client) StopPane(ctx context.Context, paneID string, timeout time.Duration) error {
	// Get the pane state before sending interrupt. A dead pane can be removed
	// immediately, while a live pane is allowed to return to its shell first.
	pid, exited, err := c.paneState(ctx, paneID)
	if err != nil {
		if isMissingTmuxTarget(err) {
			return nil
		}
		return fmt.Errorf("read pane %q state: %w", paneID, err)
	}

	if !exited {
		if _, err := c.runner.Run(ctx, c.bin, "send-keys", "-t", paneID, "C-c"); err != nil {
			if isMissingTmuxTarget(err) {
				return nil
			}
			return fmt.Errorf("interrupt pane %q: %w", paneID, err)
		}
	}

	if !exited {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			_, exited, err = c.paneState(ctx, paneID)
			if err != nil {
				if isMissingTmuxTarget(err) {
					return nil
				}
				return fmt.Errorf("poll pane %q state: %w", paneID, err)
			}
			if exited || (pid != "" && c.PaneExitedByPID(ctx, pid)) {
				break
			}
			if err := waitForPoll(ctx, 200*time.Millisecond); err != nil {
				return err
			}
		}
	}

	// Kill the pane (harmless if already gone).
	if _, err := c.runner.Run(ctx, c.bin, "kill-pane", "-t", paneID); err != nil && !isMissingTmuxTarget(err) {
		return fmt.Errorf("kill pane %q: %w", paneID, err)
	}
	return nil
}

func (c *Client) paneState(ctx context.Context, paneID string) (pid string, exited bool, err error) {
	// list-panes -t <pane> expands to every pane in that pane's window. Use
	// display-message so the format is evaluated for exactly paneID.
	output, err := c.runner.Run(ctx, c.bin, "display-message", "-p", "-t", paneID, "#{pane_pid}\t#{pane_dead}\t#{pane_current_command}\t#{@wts_launch}")
	if err != nil {
		return "", false, err
	}
	line := output
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	parts := strings.SplitN(line, "\t", 4)
	if len(parts) < 3 {
		return "", false, fmt.Errorf("unexpected pane state %q", line)
	}
	pane := PaneInfo{
		ID:      paneID,
		PID:     strings.TrimSpace(parts[0]),
		Dead:    parts[1] == "1",
		Command: strings.TrimSpace(parts[2]),
	}
	if len(parts) == 4 {
		pane.Direct = parts[3] == directLaunch
	}
	return pane.PID, c.ProcessExited(ctx, pane), nil
}

func (c *Client) CapturePaneByID(ctx context.Context, paneID string, lines int) (string, error) {
	if lines <= 0 {
		lines = 200
	}
	output, err := c.runner.Run(ctx, c.bin, "capture-pane", "-p", "-t", paneID, "-S", fmt.Sprintf("-%d", lines))
	if err != nil {
		return "", fmt.Errorf("capture pane %q: %w", paneID, err)
	}
	return output, nil
}

func (c *Client) PaneExitedByPID(ctx context.Context, pid string) bool {
	children, err := c.runner.Run(ctx, "pgrep", "-P", pid)
	if err == nil {
		return strings.TrimSpace(children) == ""
	}
	if ctx.Err() != nil {
		return false
	}
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1
}

func isMissingTmuxTarget(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "error connecting to") && strings.Contains(message, "no such file or directory") {
		return true
	}
	for _, fragment := range []string{
		"can't find pane",
		"can't find window",
		"can't find session",
		"no such session",
		"no server running",
	} {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

func waitForPoll(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func buildPayload(command string, env map[string]string) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys)+1)
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("export %s=%s", key, shellQuote(env[key])))
	}
	// Commands are shell programs: prefixing exec would discard everything
	// after the first executable in a sequence and break assignments or loops.
	parts = append(parts, command)
	return strings.Join(parts, "; ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
