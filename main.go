/*
Copyright (c) 2025 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/innabox/fulfillment-service/internal"
	"github.com/innabox/fulfillment-service/internal/cmd"
	"github.com/innabox/fulfillment-service/internal/exit"
)

func main() {
	// Create a context:
	ctx := context.Background()

	// Create the tool:
	tool, err := internal.NewTool().
		AddArgs(os.Args...).
		SetIn(os.Stdin).
		SetOut(os.Stdout).
		SetErr(os.Stderr).
		AddCommand(cmd.NewDevCommand).
		AddCommand(cmd.NewStartCommand).
		AddCommand(cmd.NewVersionCommand).
		Build()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	// Run the tool:
	err = tool.Run(ctx)
	if err != nil {
		exitErr, ok := err.(exit.Error)
		if ok {
			os.Exit(exitErr.Code())
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", err.Error())
			os.Exit(1)
		}
	}
}
