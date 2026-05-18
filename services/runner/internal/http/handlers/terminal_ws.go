package handlers

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"gitgym/services/runner/internal/engine"
	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/go-chi/chi/v5"
)

func TerminalWebSocket(workRoot string, manager *engine.TerminalManager) http.HandlerFunc {
	if manager == nil {
		manager = engine.NewTerminalManager()
	}

	return func(w http.ResponseWriter, r *http.Request) {
		workspaceID := chi.URLParam(r, "workspaceID")
		workspacePath, err := resolveWorkspacePath(workRoot, workspaceID)
		if err != nil {
			writeWorkspaceError(w, err)
			return
		}

		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		var closeOnce sync.Once
		var writeMu sync.Mutex
		closeConn := func(code websocket.StatusCode, reason string) {
			closeOnce.Do(func() {
				_ = conn.Close(code, reason)
			})
		}
		writeFrame := func(message engine.TerminalServerMessage) error {
			writeMu.Lock()
			defer writeMu.Unlock()

			return wsjson.Write(ctx, conn, message)
		}

		session, err := manager.Acquire(ctx, workspacePath, workspaceID)
		if err != nil {
			closeConn(websocket.StatusInternalError, "terminal unavailable")
			return
		}

		if err := writeFrame(engine.TerminalServerMessage{
			Type: "ready",
			Cols: 120,
			Rows: 30,
		}); err != nil {
			return
		}

		streamDone := make(chan error, 1)
		commandTracker := newSubmittedCommandTracker()
		completionTracker := newCommandCompletionTracker(commandTracker)
		go func() {
			streamErr := session.ReadLoop(ctx, func(chunk []byte) error {
				output, completions := completionTracker.ingestOutput(string(chunk))
				if output != "" {
					if err := writeFrame(engine.TerminalServerMessage{
						Type: "output",
						Data: output,
					}); err != nil {
						return err
					}
				}
				for _, completion := range completions {
					if err := writeFrame(engine.TerminalServerMessage{
						Type:   "status",
						Phase:  "running",
						Detail: completion.command,
					}); err != nil {
						return err
					}
					if err := writeFrame(engine.TerminalServerMessage{
						Type:     "exit",
						ExitCode: &completion.exitCode,
					}); err != nil {
						return err
					}
				}
				return nil
			})
			switch {
			case streamErr == nil:
				output, completions := completionTracker.flush()
				if output != "" {
					_ = writeFrame(engine.TerminalServerMessage{
						Type: "output",
						Data: output,
					})
				}
				for _, completion := range completions {
					_ = writeFrame(engine.TerminalServerMessage{
						Type:   "status",
						Phase:  "running",
						Detail: completion.command,
					})
					_ = writeFrame(engine.TerminalServerMessage{
						Type:     "exit",
						ExitCode: &completion.exitCode,
					})
				}
				if exitCode, ok := terminalExitCode(session.Wait()); ok {
					for _, completion := range commandTracker.finalizePending(exitCode) {
						_ = writeFrame(engine.TerminalServerMessage{
							Type:   "status",
							Phase:  "running",
							Detail: completion.command,
						})
						_ = writeFrame(engine.TerminalServerMessage{
							Type:     "exit",
							ExitCode: &completion.exitCode,
						})
					}
				}
				closeConn(websocket.StatusNormalClosure, "")
			case !errors.Is(streamErr, context.Canceled):
				closeConn(websocket.StatusInternalError, "terminal stream failed")
			}
			streamDone <- streamErr
		}()
		defer func() {
			cancel()
			<-streamDone
		}()

		for {
			var message engine.TerminalClientMessage
			if err := wsjson.Read(ctx, conn, &message); err != nil {
				return
			}

			switch message.Type {
			case "input":
				commandTracker.ingest(message.Data)
				if err := session.WriteInput(message.Data); err != nil {
					closeConn(websocket.StatusInternalError, "terminal unavailable")
					return
				}
			case "resize":
				if err := session.Resize(message.Cols, message.Rows); err != nil {
					closeConn(websocket.StatusInternalError, "terminal unavailable")
					return
				}
			case "ping":
			}
		}
	}
}

type submittedCommandTracker struct {
	mu             sync.Mutex
	pending        []rune
	pendingCommand string
	inEscapeBranch bool
	promptOwned    bool
	commands       []string
}

func newSubmittedCommandTracker() *submittedCommandTracker {
	return &submittedCommandTracker{promptOwned: true}
}

func (t *submittedCommandTracker) ingest(input string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	trackCommands := t.promptOwned || strings.TrimSpace(t.pendingCommand) != ""
	sawCarriageReturn := false

	for _, char := range input {
		if sawCarriageReturn {
			sawCarriageReturn = false
			if char == '\n' {
				continue
			}
		}

		if t.inEscapeBranch {
			if char >= 0x40 && char <= 0x7E {
				t.inEscapeBranch = false
			}
			continue
		}

		switch char {
		case 0x1B:
			t.inEscapeBranch = true
		case '\r':
			sawCarriageReturn = true
			t.submitPendingLine(trackCommands)
		case '\n':
			t.submitPendingLine(trackCommands)
		case '\b', 0x7F:
			if len(t.pending) > 0 {
				t.pending = t.pending[:len(t.pending)-1]
			}
		default:
			if unicode.IsPrint(char) {
				t.pending = append(t.pending, char)
			}
		}
	}
}

func (t *submittedCommandTracker) completeCommand(exitCode int) (commandCompletion, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.commands) == 0 {
		t.promptOwned = true
		return commandCompletion{}, false
	}

	if !isCompleteTopLevelCommand(t.commands[0]) {
		t.promptOwned = false
		return commandCompletion{}, false
	}

	completion := commandCompletion{
		command:  t.commands[0],
		exitCode: exitCode,
	}
	t.commands = t.commands[1:]
	t.promptOwned = len(t.commands) == 0 && strings.TrimSpace(t.pendingCommand) == ""
	return completion, true
}

func (t *submittedCommandTracker) finalizePending(exitCode int) []commandCompletion {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.commands) == 0 {
		t.promptOwned = true
		return nil
	}

	completions := make([]commandCompletion, 0, len(t.commands))
	for _, command := range t.commands {
		completions = append(completions, commandCompletion{
			command:  command,
			exitCode: exitCode,
		})
	}

	t.commands = nil
	t.promptOwned = true
	t.pending = t.pending[:0]
	t.pendingCommand = ""
	return completions
}

func (t *submittedCommandTracker) submitPendingLine(trackCommands bool) {
	line := strings.TrimSpace(string(t.pending))
	t.pending = t.pending[:0]

	if !trackCommands {
		return
	}

	if line == "" {
		if strings.TrimSpace(t.pendingCommand) == "" {
			return
		}
		line = ""
	}

	command := line
	if strings.TrimSpace(t.pendingCommand) != "" {
		if command == "" {
			command = t.pendingCommand + "\n"
		} else {
			command = t.pendingCommand + "\n" + command
		}
	}

	if strings.TrimSpace(command) == "" {
		t.pendingCommand = ""
		return
	}

	if isCompleteTopLevelCommand(command) {
		t.commands = append(t.commands, command)
		t.pendingCommand = ""
		t.promptOwned = false
		return
	}

	t.pendingCommand = command
	t.promptOwned = false
}

func terminalExitCode(err error) (int, bool) {
	if err == nil {
		return 0, true
	}

	var closeErr websocket.CloseError
	if errors.As(err, &closeErr) {
		return 0, false
	}

	return 0, false
}

var terminalCommandExitLine = regexp.MustCompile("^" + regexp.QuoteMeta(engine.TerminalCommandExitMarker) + ":(-?\\d+)\\r?$")
var terminalContinuationPromptLine = regexp.MustCompile("^" + regexp.QuoteMeta(engine.TerminalContinuationPromptMarker) + "\\r?$")
var terminalANSISequence = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

type commandCompletionTracker struct {
	mu            sync.Mutex
	pendingOutput string
	commands      *submittedCommandTracker
}

type commandCompletion struct {
	command  string
	exitCode int
}

func newCommandCompletionTracker(commands *submittedCommandTracker) *commandCompletionTracker {
	return &commandCompletionTracker{commands: commands}
}

func (t *commandCompletionTracker) ingestOutput(chunk string) (string, []commandCompletion) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.pendingOutput += chunk
	return t.drainLocked(false)
}

func (t *commandCompletionTracker) flush() (string, []commandCompletion) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.drainLocked(true)
}

func (t *commandCompletionTracker) drainLocked(flushAll bool) (string, []commandCompletion) {
	var output strings.Builder
	completions := make([]commandCompletion, 0)

	for {
		newline := strings.IndexByte(t.pendingOutput, '\n')
		if newline < 0 {
			break
		}

		line := t.pendingOutput[:newline+1]
		t.pendingOutput = t.pendingOutput[newline+1:]

		if exitCode, ok := parseTerminalCommandExitLine(line); ok {
			if completion, ok := t.commands.completeCommand(exitCode); ok {
				completions = append(completions, completion)
			}
			continue
		}
		if isTerminalContinuationPromptLine(line) {
			continue
		}

		output.WriteString(line)
	}

	if flushAll || !couldBeExitMarkerFragment(t.pendingOutput) {
		output.WriteString(t.pendingOutput)
		t.pendingOutput = ""
	}

	return output.String(), completions
}

func parseTerminalCommandExitLine(line string) (int, bool) {
	normalized := normalizeTerminalMarkerLine(line)

	match := terminalCommandExitLine.FindStringSubmatch(normalized)
	if len(match) != 2 {
		return 0, false
	}

	exitCode, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}

	return exitCode, true
}

func isTerminalContinuationPromptLine(line string) bool {
	return terminalContinuationPromptLine.MatchString(normalizeTerminalMarkerLine(line))
}

func normalizeTerminalMarkerLine(line string) string {
	normalized := strings.TrimSuffix(line, "\n")
	normalized = strings.TrimSuffix(normalized, "\r")
	return terminalANSISequence.ReplaceAllString(normalized, "")
}

func couldBeExitMarkerFragment(value string) bool {
	if value == "" {
		return false
	}

	return strings.HasPrefix(engine.TerminalCommandExitMarker, value)
}

func isCompleteTopLevelCommand(command string) bool {
	return isCompleteTopLevelCommandForShell(detectShellDialect(), command)
}

type shellDialect string

const (
	posixShellDialect      shellDialect = "posix"
	powershellShellDialect shellDialect = "powershell"
)

func detectShellDialect() shellDialect {
	if runtime.GOOS == "windows" {
		return powershellShellDialect
	}

	return posixShellDialect
}

func isCompleteTopLevelCommandForShell(shell shellDialect, command string) bool {
	if strings.TrimSpace(command) == "" {
		return false
	}

	if hasTrailingContinuation(shell, strings.TrimSpace(command)) {
		return false
	}

	if hasUnclosedDelimitedSections(shell, command) {
		return false
	}

	if hasUnclosedShellBlock(shell, command) {
		return false
	}

	return true
}

func hasTrailingContinuation(shell shellDialect, command string) bool {
	lines := strings.Split(command, "\n")
	if len(lines) == 0 {
		return false
	}

	last := strings.TrimSpace(lines[len(lines)-1])
	if last == "" {
		return false
	}

	suffixes := []string{"|", "&&", "||"}
	if shell == powershellShellDialect {
		suffixes = append([]string{"`"}, suffixes...)
	} else {
		suffixes = append([]string{"\\", "`"}, suffixes...)
	}

	for _, suffix := range suffixes {
		if strings.HasSuffix(last, suffix) {
			return true
		}
	}

	return false
}

func hasUnclosedDelimitedSections(shell shellDialect, command string) bool {
	braceDepth := 0
	parenDepth := 0
	bracketDepth := 0
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	runes := []rune(command)
	for index := 0; index < len(runes); index++ {
		char := runes[index]

		if escaped {
			escaped = false
			continue
		}

		switch char {
		case '\\':
			if shell != powershellShellDialect && !inSingleQuote {
				escaped = true
			}
		case '`':
			if shell == powershellShellDialect && !inSingleQuote {
				escaped = true
			}
		case '\'':
			if !inDoubleQuote {
				if shell == powershellShellDialect && inSingleQuote && index+1 < len(runes) && runes[index+1] == '\'' {
					index++
					continue
				}
				inSingleQuote = !inSingleQuote
			}
		case '"':
			if !inSingleQuote {
				inDoubleQuote = !inDoubleQuote
			}
		default:
			if inSingleQuote || inDoubleQuote {
				continue
			}
			switch char {
			case '{':
				braceDepth++
			case '}':
				braceDepth--
			case '(':
				parenDepth++
			case ')':
				parenDepth--
			case '[':
				bracketDepth++
			case ']':
				bracketDepth--
			}
		}
	}

	return inSingleQuote ||
		inDoubleQuote ||
		braceDepth > 0 ||
		parenDepth > 0 ||
		bracketDepth > 0
}

func hasUnclosedShellBlock(shell shellDialect, command string) bool {
	if shell == powershellShellDialect {
		return false
	}

	blockDepth := 0

	for _, line := range strings.Split(command, "\n") {
		normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(line)), " "))
		if normalized == "" {
			continue
		}

		switch {
		case normalized == "fi" || strings.HasPrefix(normalized, "fi "):
			blockDepth--
		case normalized == "done" || strings.HasPrefix(normalized, "done "):
			blockDepth--
		case normalized == "esac" || strings.HasPrefix(normalized, "esac "):
			blockDepth--
		}

		switch {
		case normalized == "then" || strings.HasSuffix(normalized, " then"):
			blockDepth++
		case normalized == "do" || strings.HasSuffix(normalized, " do"):
			blockDepth++
		case strings.HasSuffix(normalized, " in"):
			blockDepth++
		}
	}

	return blockDepth > 0
}
