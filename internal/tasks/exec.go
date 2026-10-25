package tasks

import (
	"context"
	"fmt"
	"time"
)

// ToolTimeout is the spec-9.4 tool timeout (tasks.tool_timeout, 120s): a
// tool call that runs past it detaches into a background handle the layer
// can poll, read, or cancel — it is not an error.
const ToolTimeout = 120 * time.Second

// ExecFn runs a command and returns its combined output — the sandbox's
// ExecWithTimeout is adapted into this shape so the handle machinery is
// testable without a jail.
type ExecFn func(ctx context.Context, command string) (string, error)

// Handle is a running detached exec (kind='exec_handle' task row).
type Handle struct {
	TaskID  string
	Command string
}

// ExecOrHandle runs cmd; if it finishes inside timeout the result comes
// back directly. Otherwise the call returns a Handle and the exec keeps
// running in its own goroutine until it ends or is canceled.
func (m *Manager) ExecOrHandle(ctx context.Context, exec ExecFn, command string, timeout time.Duration) (result string, h *Handle, err error) {
	if timeout <= 0 {
		timeout = ToolTimeout
	}
	done := make(chan struct{})
	var res string
	var runErr error
	execCtx, cancel := context.WithCancel(context.Background())
	go func() {
		defer close(done)
		res, runErr = exec(execCtx, command)
	}()
	select {
	case <-done:
		cancel()
		return res, nil, runErr
	case <-time.After(timeout):
		t, serr := m.Submit(context.Background(), Task{
			Kind: "exec_handle", Layer: 6, State: StateRunning, Payload: command})
		if serr != nil {
			cancel()
			return "", nil, serr
		}
		go func() {
			defer cancel() // detached exec holds its ctx until done
			<-done
			if runErr != nil {
				_ = m.Fail(context.Background(), t.ID, runErr)
			} else {
				_ = m.Complete(context.Background(), t.ID)
			}
			_ = m.Checkpoint(context.Background(), t.ID, 1, []byte(res))
		}()
		return "", &Handle{TaskID: t.ID, Command: command}, nil
	}
}

// HandleStatus returns the detached exec's task state.
func (m *Manager) HandleStatus(ctx context.Context, taskID string) (string, error) {
	t, err := m.Poll(ctx, taskID)
	if err != nil {
		return "", err
	}
	return t.State, nil
}

// HandleRead returns the detached exec's captured output (checkpoint blob).
func (m *Manager) HandleRead(ctx context.Context, taskID string) (string, error) {
	_, blob, err := m.LatestCheckpoint(ctx, taskID)
	if err != nil {
		return "", fmt.Errorf("tasks: no output for handle %s yet", taskID)
	}
	return string(blob), nil
}

// HandleCancel marks a detached exec canceled. The goroutine's exec still
// finishes but the task row reflects the cancel intent.
func (m *Manager) HandleCancel(ctx context.Context, taskID string) error {
	return m.Cancel(ctx, taskID)
}
