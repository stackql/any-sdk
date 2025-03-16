package local_template_executor

import (
	"bytes"
	"io"
	"os/exec"
)

var (
	_ io.Reader = &bytes.Buffer{}
	_ io.Writer = &bytes.Buffer{}
)

type ExecutionResponse interface {
	GetStdOut() (*bytes.Buffer, bool)
	GetStdErr() (*bytes.Buffer, bool)
	GetExitCode() int
}

type Executor interface {
	Execute() (ExecutionResponse, error)
}

func NewLocalTemplateExecutor(commandName string, commandArgs []string, stdInStream *bytes.Buffer) Executor {
	return &localTemplateExecutor{
		stdInStream:  stdInStream,
		commandName:  commandName,
		commandArgs:  commandArgs,
		stdOutBuffer: &bytes.Buffer{},
		stdErrBuffer: &bytes.Buffer{},
	}
}

type standardExecutionResponse struct {
	stdOut   *bytes.Buffer
	stdErr   *bytes.Buffer
	exitCode int
}

func (ser *standardExecutionResponse) GetStdOut() (*bytes.Buffer, bool) {
	return ser.stdOut, ser.stdOut != nil
}

func (ser *standardExecutionResponse) GetStdErr() (*bytes.Buffer, bool) {
	return ser.stdErr, ser.stdErr != nil
}

func (ser *standardExecutionResponse) GetExitCode() int {
	return ser.exitCode
}

type localTemplateExecutor struct {
	// Path to the template file.
	stdInStream  *bytes.Buffer
	commandName  string
	commandArgs  []string
	stdOutBuffer *bytes.Buffer
	stdErrBuffer *bytes.Buffer
}

func (lt *localTemplateExecutor) Execute() (ExecutionResponse, error) {
	// Execute the template file.
	cmd := exec.Command(lt.commandName, lt.commandArgs...)
	cmd.Stdin = lt.stdInStream
	cmd.Stdout = lt.stdOutBuffer
	cmd.Stderr = lt.stdErrBuffer
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	return &standardExecutionResponse{
		stdOut:   lt.stdOutBuffer,
		stdErr:   lt.stdErrBuffer,
		exitCode: cmd.ProcessState.ExitCode(),
	}, nil
}
