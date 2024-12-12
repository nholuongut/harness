// Copyright 2023 Harness, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/harness/gitness/app/gitspace/orchestrator/devcontainer"
	"github.com/harness/gitness/app/gitspace/orchestrator/template"
	"github.com/harness/gitness/app/gitspace/types"
	"github.com/harness/gitness/types/enum"
)

const templateSupportedOSDistribution = "supported_os_distribution.sh"
const templateVsCodeWebToolsInstallation = "install_tools_vs_code_web.sh"
const templateVsCodeToolsInstallation = "install_tools_vs_code.sh"
const templateSetEnv = "set_env.sh"

func ValidateSupportedOS(
	ctx context.Context,
	exec *devcontainer.Exec,
	gitspaceLogger types.GitspaceLogger,
) error {
	// TODO: Currently not supporting arch, freebsd and alpine.
	// For alpine wee need to install multiple things from
	// https://github.com/microsoft/vscode/wiki/How-to-Contribute#prerequisites
	script, err := template.GenerateScriptFromTemplate(
		templateSupportedOSDistribution, &template.SupportedOSDistributionPayload{
			OSInfoScript: osDetectScript,
		})
	if err != nil {
		return fmt.Errorf("failed to generate scipt to validate supported os distribution from template %s: %w",
			templateSupportedOSDistribution, err)
	}
	gitspaceLogger.Info("Validate supported OSes...")
	err = ExecuteCommandInHomeDirAndLog(ctx, exec, script, true, gitspaceLogger, false)
	if err != nil {
		return fmt.Errorf("error while detecting os distribution: %w", err)
	}
	return nil
}

func InstallTools(
	ctx context.Context,
	exec *devcontainer.Exec,
	ideType enum.IDEType,
	gitspaceLogger types.GitspaceLogger,
) error {
	switch ideType {
	case enum.IDETypeVSCodeWeb:
		err := InstallToolsForVsCodeWeb(ctx, exec, gitspaceLogger)
		if err != nil {
			return err
		}
		return nil
	case enum.IDETypeVSCode:
		err := InstallToolsForVsCode(ctx, exec, gitspaceLogger)
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

func InstallToolsForVsCodeWeb(
	ctx context.Context,
	exec *devcontainer.Exec,
	gitspaceLogger types.GitspaceLogger,
) error {
	script, err := template.GenerateScriptFromTemplate(
		templateVsCodeWebToolsInstallation, &template.InstallToolsPayload{
			OSInfoScript: osDetectScript,
		})
	if err != nil {
		return fmt.Errorf(
			"failed to generate scipt to install tools for vs code web from template %s: %w",
			templateVsCodeWebToolsInstallation, err)
	}

	gitspaceLogger.Info("Installing tools for vs code web inside container")
	gitspaceLogger.Info("Tools installation output...")
	err = ExecuteCommandInHomeDirAndLog(ctx, exec, script, true, gitspaceLogger, false)
	if err != nil {
		return fmt.Errorf("failed to install tools for vs code web: %w", err)
	}
	gitspaceLogger.Info("Successfully installed tools for vs code web")
	return nil
}

func InstallToolsForVsCode(
	ctx context.Context,
	exec *devcontainer.Exec,
	gitspaceLogger types.GitspaceLogger,
) error {
	script, err := template.GenerateScriptFromTemplate(
		templateVsCodeToolsInstallation, &template.InstallToolsPayload{
			OSInfoScript: osDetectScript,
		})
	if err != nil {
		return fmt.Errorf(
			"failed to generate scipt to install tools for vs code from template %s: %w",
			templateVsCodeToolsInstallation, err)
	}

	gitspaceLogger.Info("Installing tools for vs code in container")
	err = ExecuteCommandInHomeDirAndLog(ctx, exec, script, true, gitspaceLogger, false)
	if err != nil {
		return fmt.Errorf("failed to install tools for vs code: %w", err)
	}
	gitspaceLogger.Info("Successfully installed tools for vs code")
	return nil
}

func SetEnv(
	ctx context.Context,
	exec *devcontainer.Exec,
	gitspaceLogger types.GitspaceLogger,
	environment []string,
) error {
	script, err := template.GenerateScriptFromTemplate(
		templateSetEnv, &template.SetEnvPayload{
			EnvVariables: environment,
		})
	if err != nil {
		return fmt.Errorf("failed to generate scipt to set env from template %s: %w",
			templateSetEnv, err)
	}
	gitspaceLogger.Info("Setting env...")
	err = ExecuteCommandInHomeDirAndLog(ctx, exec, script, true, gitspaceLogger, true)
	if err != nil {
		return fmt.Errorf("error while setting env vars: %w", err)
	}
	return nil
}

func ExecuteCommandInHomeDirAndLog(
	ctx context.Context,
	exec *devcontainer.Exec,
	script string,
	root bool,
	gitspaceLogger types.GitspaceLogger,
	verbose bool,
) error {
	// Buffer upto a thousand messages
	outputCh := make(chan []byte, 1000)
	err := exec.ExecuteCmdInHomeDirectoryAsyncStream(ctx, script, root, false, outputCh)
	if err != nil {
		return err
	}
	// Use select to wait for the output and exit status
	for {
		select {
		case output := <-outputCh:
			done, chErr := handleOutputChannel(output, verbose, gitspaceLogger)
			if done {
				return chErr
			}
		case <-ctx.Done():
			// Handle context cancellation or timeout
			return ctx.Err()
		}
	}
}

func handleOutputChannel(output []byte, verbose bool, gitspaceLogger types.GitspaceLogger) (bool, error) {
	// Handle the exit status first
	if strings.HasPrefix(string(output), devcontainer.ChannelExitStatus) {
		// Extract the exit code from the message
		exitCodeStr := strings.TrimPrefix(string(output), devcontainer.ChannelExitStatus)
		exitCode, err := strconv.Atoi(exitCodeStr)
		if err != nil {
			return true, fmt.Errorf("invalid exit status format: %w", err)
		}
		if exitCode != 0 {
			gitspaceLogger.Info("Process Exited with status " + exitCodeStr)
			return true, fmt.Errorf("command exited with non-zero status: %d", exitCode)
		}
		// If exit status is zero, just continue processing
		return true, nil
	}
	// Handle regular command output
	msg := string(output)
	if len(output) > 0 {
		// Log output if verbose or if it's an error
		if verbose || strings.HasPrefix(msg, devcontainer.LoggerErrorPrefix) {
			gitspaceLogger.Info(msg)
		}
	}
	return false, nil
}
