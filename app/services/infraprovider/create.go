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
	"fmt"

	"github.com/harness/gitness/infraprovider"
	"github.com/harness/gitness/types"

	"github.com/rs/zerolog/log"
)

func (c *Service) CreateTemplate(
	ctx context.Context,
	template *types.InfraProviderTemplate,
) error {
	return c.infraProviderTemplateStore.Create(ctx, template)
}

func (c *Service) CreateInfraProvider(
	ctx context.Context,
	infraProviderConfig *types.InfraProviderConfig,
) error {
	err := c.tx.WithTx(ctx, func(ctx context.Context) error {
		err := c.createConfig(ctx, infraProviderConfig)
		if err != nil {
			return fmt.Errorf("could not create the config: %q %w", infraProviderConfig.Identifier, err)
		}
		configID := infraProviderConfig.ID
		err = c.createResources(ctx, infraProviderConfig.Resources, configID)
		if err != nil {
			return fmt.Errorf("could not create the resources: %v %w", infraProviderConfig.Resources, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to complete txn for the infraprovider %w", err)
	}
	return nil
}

func (c *Service) createConfig(ctx context.Context, infraProviderConfig *types.InfraProviderConfig) error {
	err := c.infraProviderConfigStore.Create(ctx, infraProviderConfig)
	if err != nil {
		return fmt.Errorf("failed to create infraprovider config for : %q %w", infraProviderConfig.Identifier, err)
	}
	return nil
}

func (c *Service) CreateResources(ctx context.Context, resources []types.InfraProviderResource, configID int64) error {
	err := c.tx.WithTx(ctx, func(ctx context.Context) error {
		return c.createResources(ctx, resources, configID)
	})
	if err != nil {
		return fmt.Errorf("failed to complete create txn for the infraprovider resource %w", err)
	}
	return nil
}

func (c *Service) createResources(ctx context.Context, resources []types.InfraProviderResource, configID int64) error {
	for idx := range resources {
		resource := &resources[idx]
		resource.InfraProviderConfigID = configID
		infraProvider, err := c.infraProviderFactory.GetInfraProvider(resource.InfraProviderType)
		if err != nil {
			return fmt.Errorf("failed to fetch infrastructure impl for type : %q %w", resource.InfraProviderType, err)
		}
		if len(infraProvider.TemplateParams()) > 0 {
			err = c.validateTemplates(ctx, infraProvider, *resource)
			if err != nil {
				return err
			}
		}
		err = c.infraProviderResourceStore.Create(ctx, resource)
		if err != nil {
			return fmt.Errorf("failed to create infraprovider resource for : %q %w", resource.UID, err)
		}
	}
	return nil
}

func (c *Service) validate(ctx context.Context, resource *types.InfraProviderResource) error {
	infraProvider, err := c.infraProviderFactory.GetInfraProvider(resource.InfraProviderType)
	if err != nil {
		return fmt.Errorf("failed to fetch infrastructure impl for type : %q %w", resource.InfraProviderType, err)
	}
	if len(infraProvider.TemplateParams()) > 0 {
		err = c.validateTemplates(ctx, infraProvider, *resource)
		if err != nil {
			return err
		}
	}
	return err
}

func (c *Service) validateTemplates(
	ctx context.Context,
	infraProvider infraprovider.InfraProvider,
	res types.InfraProviderResource,
) error {
	templateParams := infraProvider.TemplateParams()
	for _, param := range templateParams {
		key := param.Name
		if res.Metadata[key] != "" {
			templateIdentifier := res.Metadata[key]
			_, err := c.infraProviderTemplateStore.FindByIdentifier(
				ctx, res.SpaceID, templateIdentifier)
			if err != nil {
				log.Warn().Msgf("unable to get template params for ID : %s",
					res.Metadata[key])
			}
		}
	}
	return nil
}
