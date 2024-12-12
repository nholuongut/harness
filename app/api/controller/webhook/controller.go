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

package webhook

import (
	"context"
	"fmt"

	apiauth "github.com/harness/gitness/app/api/auth"
	"github.com/harness/gitness/app/auth"
	"github.com/harness/gitness/app/auth/authz"
	"github.com/harness/gitness/app/services/webhook"
	"github.com/harness/gitness/app/store"
	"github.com/harness/gitness/encrypt"
	"github.com/harness/gitness/errors"
	"github.com/harness/gitness/types"
	"github.com/harness/gitness/types/enum"
)

type Controller struct {
	authorizer     authz.Authorizer
	spaceStore     store.SpaceStore
	repoStore      store.RepoStore
	webhookService *webhook.Service
	encrypter      encrypt.Encrypter
	preprocessor   Preprocessor
}

func NewController(
	authorizer authz.Authorizer,
	spaceStore store.SpaceStore,
	repoStore store.RepoStore,
	webhookService *webhook.Service,
	encrypter encrypt.Encrypter,
	preprocessor Preprocessor,
) *Controller {
	return &Controller{
		authorizer:     authorizer,
		spaceStore:     spaceStore,
		repoStore:      repoStore,
		webhookService: webhookService,
		encrypter:      encrypter,
		preprocessor:   preprocessor,
	}
}

func (c *Controller) getRepoCheckAccess(ctx context.Context,
	session *auth.Session, repoRef string, reqPermission enum.Permission) (*types.Repository, error) {
	if repoRef == "" {
		return nil, errors.InvalidArgument("A valid repository reference must be provided.")
	}

	repo, err := c.repoStore.FindByRef(ctx, repoRef)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo: %w", err)
	}

	if err = apiauth.CheckRepo(ctx, c.authorizer, session, repo, reqPermission); err != nil {
		return nil, fmt.Errorf("failed to verify authorization: %w", err)
	}

	return repo, nil
}

func (c *Controller) getSpaceCheckAccess(
	ctx context.Context,
	session *auth.Session,
	spaceRef string,
	permission enum.Permission,
) (*types.Space, error) {
	space, err := c.spaceStore.FindByRef(ctx, spaceRef)
	if err != nil {
		return nil, fmt.Errorf("parent space not found: %w", err)
	}

	scope := &types.Scope{SpacePath: space.Path}
	resource := &types.Resource{Type: enum.ResourceTypeRepo}
	err = apiauth.Check(ctx, c.authorizer, session, scope, resource, permission)
	if err != nil {
		return nil, fmt.Errorf("auth check failed: %w", err)
	}

	return space, nil
}
