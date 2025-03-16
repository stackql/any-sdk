package local_template_executor

import (
	"bytes"
	"io"
	"os/exec"
	"text/template"
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
	Execute(map[string]any) (ExecutionResponse, error)
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

func (lt *localTemplateExecutor) Execute(context map[string]any) (ExecutionResponse, error) {
	// Execute the template file.
	cmdTpl, cmdTplErr := template.New("letter").Parse(lt.commandName)
	if cmdTplErr != nil {
		return nil, cmdTplErr
	}
	var cmdBuffer bytes.Buffer
	err := cmdTpl.Execute(&cmdBuffer, context)
	if err != nil {
		return nil, err
	}
	cmdString := cmdBuffer.String()
	var commandStrArgs []string
	for _, arg := range lt.commandArgs {
		cmdTpl, cmdTplErr = template.New("letter").Parse(arg)
		if cmdTplErr != nil {
			return nil, cmdTplErr
		}
		cmdBuffer.Reset()
		err = cmdTpl.Execute(&cmdBuffer, context)
		if err != nil {
			return nil, err
		}
		commandStrArgs = append(commandStrArgs, cmdBuffer.String())
	}
	cmd := exec.Command(cmdString, commandStrArgs...)
	// cmd.Stdin = lt.stdInStream
	cmd.Stdout = lt.stdOutBuffer
	cmd.Stderr = lt.stdErrBuffer
	err = cmd.Run()
	if err != nil {
		return nil, err
	}
	return &standardExecutionResponse{
		stdOut:   lt.stdOutBuffer,
		stdErr:   lt.stdErrBuffer,
		exitCode: cmd.ProcessState.ExitCode(),
	}, nil
}
