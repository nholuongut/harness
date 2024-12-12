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

package scm

import (
	"context"

	"github.com/harness/gitness/types"
)

type Provider interface {
	ResolveCredentials(ctx context.Context, gitspaceConfig types.GitspaceConfig) (*ResolvedCredentials, error)

	GetFileContent(
		ctx context.Context,
		gitspaceConfig types.GitspaceConfig,
		filePath string,
		credentials *ResolvedCredentials,
	) ([]byte, error)

	ListRepositories(
		ctx context.Context,
		filter *RepositoryFilter,
		credentials *ResolvedCredentials,
	) ([]Repository, error)

	ListBranches(
		ctx context.Context,
		filter *BranchFilter,
		credentials *ResolvedCredentials,
	) ([]Branch, error)

	GetBranchURL(spacePath string, repoURL string, branch string) (string, error)
}
