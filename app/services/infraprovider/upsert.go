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

package infraprovider

import (
	"context"
	"errors"
	"fmt"

	"github.com/harness/gitness/store"
	"github.com/harness/gitness/types"

	"github.com/rs/zerolog/log"
)

func (c *Service) UpsertInfraProvider(
	ctx context.Context,
	infraProviderConfig *types.InfraProviderConfig,
) error {
	err := c.tx.WithTx(ctx, func(ctx context.Context) error {
		space, err := c.spaceStore.FindByRef(ctx, infraProviderConfig.SpacePath)
		if err != nil {
			return err
		}
		return c.upsertConfig(ctx, space, infraProviderConfig)
	})
	if err != nil {
		return fmt.Errorf("failed to complete txn for the infraprovider %w", err)
	}
	return nil
}

func (c *Service) upsertConfig(
	ctx context.Context,
	space *types.Space,
	infraProviderConfig *types.InfraProviderConfig,
) error {
	providerConfigInDB, err := c.Find(ctx, space, infraProviderConfig.Identifier)
	if errors.Is(err, store.ErrResourceNotFound) {
		if err = c.createConfig(ctx, infraProviderConfig); err != nil {
			return fmt.Errorf("could not create the config: %q %w", infraProviderConfig.Identifier, err)
		}
		log.Info().Msgf("created new infraconfig %s", infraProviderConfig.Identifier)
		providerConfigInDB, err = c.Find(ctx, space, infraProviderConfig.Identifier)
	} else if err != nil {
		infraProviderConfig.ID = providerConfigInDB.ID
		if err = c.updateConfig(ctx, infraProviderConfig); err != nil {
			return fmt.Errorf("could not update the config: %q %w", infraProviderConfig.Identifier, err)
		}
		log.Info().Msgf("updated infraconfig %s", infraProviderConfig.Identifier)
	}
	if err != nil {
		return err
	}
	if err = c.UpsertResources(ctx, infraProviderConfig.Resources, providerConfigInDB.ID, space.ID); err != nil {
		return err
	}
	return nil
}

func (c *Service) UpsertResources(
	ctx context.Context,
	resources []types.InfraProviderResource,
	configID int64,
	spaceID int64,
) error {
	for idx := range resources {
		resource := &resources[idx]
		resource.InfraProviderConfigID = configID
		resource.SpaceID = spaceID
		if err := c.validate(ctx, resource); err != nil {
			return err
		}
		_, err := c.infraProviderResourceStore.FindByIdentifier(ctx, resource.SpaceID, resource.UID)
		if errors.Is(err, store.ErrResourceNotFound) {
			if err = c.infraProviderResourceStore.Create(ctx, resource); err != nil {
				return fmt.Errorf("failed to create infraprovider resource for : %q %w", resource.UID, err)
			}
			log.Info().Msgf("created new resource %s/%s", resource.InfraProviderConfigIdentifier, resource.UID)
		} else {
			if err = c.UpdateResource(ctx, *resource); err != nil {
				log.Info().Msgf("updated resource %s/%s", resource.InfraProviderConfigIdentifier, resource.UID)
				return fmt.Errorf("could not update the resources: %v %w", resource.UID, err)
			}
		}
	}
	return nil
}
