package collect

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

const DefaultTimeout = 30 * time.Second

type Runner struct {
	Timeout time.Duration
	Dir     string
	Env     []string
}

type Result struct {
	Command  []string      `json:"command"`
	Stdout   string        `json:"stdout"`
	Stderr   string        `json:"stderr"`
	ExitCode int           `json:"exit_code"`
	Duration time.Duration `json:"duration"`
	TimedOut bool          `json:"timed_out"`
}

type CommandError struct {
	Result Result
	Err    error
}

func (e *CommandError) Error() string {
	if e.Result.TimedOut {
		return fmt.Sprintf("%s timed out after %s", e.Result.Command[0], e.Result.Duration.Truncate(time.Millisecond))
	}
	if e.Result.ExitCode >= 0 {
		return fmt.Sprintf("%s exited with code %d", e.Result.Command[0], e.Result.ExitCode)
	}
	return fmt.Sprintf("%s failed: %v", e.Result.Command[0], e.Err)
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

func (r Runner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	result := Result{
		Command:  append([]string{name}, args...),
		ExitCode: -1,
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.CommandContext(cmdCtx, name, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = r.Dir
	if len(r.Env) > 0 {
		cmd.Env = r.Env
	}

	start := time.Now()
	err := cmd.Run()
	result.Duration = time.Since(start)
	result.Stdout = stdout.String()
	result.Stderr = stderr.String()

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if errors.Is(cmdCtx.Err(), context.DeadlineExceeded) {
		result.TimedOut = true
	}
	if err != nil {
		return result, &CommandError{
			Result: result,
			Err:    err,
		}
	}

	return result, nil
}
