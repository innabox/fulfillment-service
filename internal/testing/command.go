/*
Copyright (c) 2025 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package testing

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"slices"

	"github.com/innabox/fulfillment-service/internal/logging"
)

// CommandBuilder contains the data and logic needed to create an object that helps execute a command line tool. Don't
// create instances of this type directly, use the NewCommand function instead.
type CommandBuilder struct {
	logger *slog.Logger
	name   string
	args   []string
	dir    string
}

// Command helps manage execute a command line tool. Don't create instances of this type directly, use the NewCommand
// function instead.
type Command struct {
	logger *slog.Logger
	name   string
	args   []string
	dir    string
}

// NewCommand creates a builder that can then be used to configure and create a new command.
func NewCommand() *CommandBuilder {
	return &CommandBuilder{}
}

// SetLogger sets the logger. This is mandatory.
func (b *CommandBuilder) SetLogger(value *slog.Logger) *CommandBuilder {
	b.logger = value
	return b
}

// SetName sets the name of the command. This is mandatory.
func (b *CommandBuilder) SetName(name string) *CommandBuilder {
	b.name = name
	return b
}

// SetArgs sets the arguments for the command. Note that this replaces any previously arguments added with the AddArgs
// /method. Arguments are optional, if not set no arguments will be used.
func (b *CommandBuilder) SetArgs(args ...string) *CommandBuilder {
	b.args = args
	return b
}

// AddArgs adds the arguments for the command. Note that this appends to any previously set arguments with the SetArgs
// method. Arguments are optional, if not set no arguments will be used.
func (b *CommandBuilder) AddArgs(args ...string) *CommandBuilder {
	b.args = append(b.args, args...)
	return b
}

// SetDir sets the directory where the command will be executed. This is optional, if not set the current working
// directory will be used.
func (b *CommandBuilder) SetDir(value string) *CommandBuilder {
	b.dir = value
	return b
}

// Build creates the command object.
func (b *CommandBuilder) Build() (result *Command, err error) {
	// Check parameters:
	if b.logger == nil {
		err = fmt.Errorf("logger is mandatory")
		return
	}
	if b.name == "" {
		err = fmt.Errorf("name is mandatory")
		return
	}

	// Use the current working directory if no directory is set:
	dir := b.dir
	if dir == "" {
		dir, err = os.Getwd()
		if err != nil {
			return
		}
	}

	// Add some details to the logger:
	logger := b.logger.With(
		slog.String("cmd", b.name),
		slog.String("dir", dir),
	)

	// Create and populate the object:
	result = &Command{
		logger: logger,
		name:   b.name,
		args:   slices.Clone(b.args),
		dir:    dir,
	}
	return
}

// Execute executes the commands. It returns an error if the command fails.
func (c *Command) Execute(ctx context.Context) error {
	logger := c.logger.With(
		slog.Any("args", c.args),
	)
	cmd := exec.Command(c.name, c.args...)
	cmd.Dir = c.dir
	err := c.setupLogging(ctx, cmd)
	if err != nil {
		return err
	}
	logger.DebugContext(ctx, "Executing command")
	err = cmd.Run()
	if logger.Enabled(ctx, slog.LevelDebug) {
		var attrs []slog.Attr
		c.appendStateAttrs(cmd, &attrs)
		logger.LogAttrs(ctx, slog.LevelDebug, "Executed command", attrs...)
	}
	return err
}

// Evaluate evalutes the command and returns the standard output and error as byte slices. It returns an error if the
// command fails.
func (c *Command) Evaluate(ctx context.Context) (stdoutBytes, stderrBytes []byte, err error) {
	logger := c.logger.With(
		slog.Any("args", c.args),
	)
	outBuffer := &bytes.Buffer{}
	errBuffer := &bytes.Buffer{}
	cmd := exec.Command(c.name, c.args...)
	cmd.Dir = c.dir
	cmd.Stdout = outBuffer
	cmd.Stderr = errBuffer
	err = c.setupLogging(ctx, cmd)
	if err != nil {
		return
	}
	logger.DebugContext(ctx, "Evaluating command")
	err = cmd.Run()
	if logger.Enabled(ctx, slog.LevelDebug) {
		stdoutBytes = outBuffer.Bytes()
		stderrBytes = errBuffer.Bytes()
		attrs := []slog.Attr{
			slog.String("stdout", string(stdoutBytes)),
			slog.String("stderr", string(stderrBytes)),
		}
		c.appendStateAttrs(cmd, &attrs)
		logger.LogAttrs(ctx, slog.LevelDebug, "Evaluated command", attrs...)
	}
	return
}

func (c *Command) setupLogging(ctx context.Context, cmd *exec.Cmd) error {
	stdoutLogger := c.logger.With(
		slog.String("stream", "stdout"),
	)
	stdoutWriter, err := logging.NewWriter().
		SetLogger(stdoutLogger).
		SetLevel(slog.LevelDebug).
		SetContext(ctx).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create stdout logger for command '%s': %w", c.name, err)
	}
	stderrLogger := c.logger.With(
		slog.String("stream", "stderr"),
	)
	stderrWriter, err := logging.NewWriter().
		SetLogger(stderrLogger).
		SetLevel(slog.LevelDebug).
		SetContext(ctx).
		Build()
	if err != nil {
		return fmt.Errorf("failed to create stderr logger for command '%s': %w", c.name, err)
	}
	if cmd.Stdout != nil {
		cmd.Stdout = io.MultiWriter(cmd.Stdout, stdoutWriter)
	} else {
		cmd.Stdout = stdoutWriter
	}
	if cmd.Stderr != nil {
		cmd.Stderr = io.MultiWriter(cmd.Stderr, stderrWriter)
	} else {
		cmd.Stderr = stderrWriter
	}
	return nil
}

func (c *Command) appendStateAttrs(cmd *exec.Cmd, attrs *[]slog.Attr) {
	if cmd == nil {
		return
	}
	state := cmd.ProcessState
	if state == nil {
		return
	}
	*attrs = append(
		*attrs,
		slog.Int("code", state.ExitCode()),
		slog.Int("pid", state.Pid()),
	)
}
