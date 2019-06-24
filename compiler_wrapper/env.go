package main

import (
	"bytes"
	"io"
	"os"
	"syscall"
)

type env interface {
	getenv(key string) string
	environ() []string
	getwd() string
	stdout() io.Writer
	stderr() io.Writer
	run(cmd *command, stdout io.Writer, stderr io.Writer) error
	exec(cmd *command) error
}

type processEnv struct {
	wd string
}

func newProcessEnv() (env, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, wrapErrorwithSourceLocf(err, "failed to read working directory")
	}
	return &processEnv{wd: wd}, nil
}

var _ env = (*processEnv)(nil)

func (env *processEnv) getenv(key string) string {
	return os.Getenv(key)
}

func (env *processEnv) environ() []string {
	return os.Environ()
}

func (env *processEnv) getwd() string {
	return env.wd
}

func (env *processEnv) stdout() io.Writer {
	return os.Stdout
}

func (env *processEnv) stderr() io.Writer {
	return os.Stderr
}

func (env *processEnv) exec(cmd *command) error {
	execCmd := newExecCmd(env, cmd)
	return syscall.Exec(execCmd.Path, execCmd.Args, execCmd.Env)
}

func (env *processEnv) run(cmd *command, stdout io.Writer, stderr io.Writer) error {
	execCmd := newExecCmd(env, cmd)
	execCmd.Stdout = stdout
	execCmd.Stderr = stderr
	return execCmd.Run()
}

type commandRecordingEnv struct {
	env
	cmdResults []*commandResult
}
type commandResult struct {
	cmd      *command
	stdout   string
	stderr   string
	exitCode int
}

var _ env = (*commandRecordingEnv)(nil)

func (env *commandRecordingEnv) exec(cmd *command) error {
	// Note: We will only get here if the exec failed,
	// e.g. when the underlying command could not be called.
	// In this case, we don't compare commands, so nothing to record.
	return env.exec(cmd)
}

func (env *commandRecordingEnv) run(cmd *command, stdout io.Writer, stderr io.Writer) error {
	stdoutBuffer := &bytes.Buffer{}
	stderrBuffer := &bytes.Buffer{}
	err := env.env.run(cmd, io.MultiWriter(stdout, stdoutBuffer), io.MultiWriter(stderr, stderrBuffer))
	if exitCode, ok := getExitCode(err); ok {
		env.cmdResults = append(env.cmdResults, &commandResult{
			cmd:      cmd,
			stdout:   stdoutBuffer.String(),
			stderr:   stderrBuffer.String(),
			exitCode: exitCode,
		})
	}
	return err
}
