//  Copyright 2023 Harness, Inc.
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

package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	apiauth "github.com/harness/gitness/app/api/auth"
	"github.com/harness/gitness/app/api/request"
	"github.com/harness/gitness/registry/app/api/openapi/contracts/artifact"
	ml "github.com/harness/gitness/registry/app/manifest/manifestlist"
	"github.com/harness/gitness/registry/app/manifest/ocischema"
	"github.com/harness/gitness/registry/app/manifest/schema2"
	"github.com/harness/gitness/registry/app/pkg/docker"
	"github.com/harness/gitness/registry/types"
	store2 "github.com/harness/gitness/store"
	"github.com/harness/gitness/types/enum"

	"github.com/rs/zerolog/log"
)

func (c *APIController) GetDockerArtifactManifests(
	ctx context.Context,
	r artifact.GetDockerArtifactManifestsRequestObject,
) (artifact.GetDockerArtifactManifestsResponseObject, error) {
	regInfo, err := c.GetRegistryRequestBaseInfo(ctx, "", string(r.RegistryRef))
	if err != nil {
		return artifact.GetDockerArtifactManifests400JSONResponse{
			BadRequestJSONResponse: artifact.BadRequestJSONResponse(
				*GetErrorResponse(http.StatusBadRequest, err.Error()),
			),
		}, nil
	}

	space, err := c.SpaceStore.FindByRef(ctx, regInfo.ParentRef)
	if err != nil {
		return artifact.GetDockerArtifactManifests400JSONResponse{
			BadRequestJSONResponse: artifact.BadRequestJSONResponse(
				*GetErrorResponse(http.StatusBadRequest, err.Error()),
			),
		}, nil
	}

	session, _ := request.AuthSessionFrom(ctx)
	permissionChecks := GetPermissionChecks(space, regInfo.RegistryIdentifier, enum.PermissionRegistryView)
	if err = apiauth.CheckRegistry(
		ctx,
		c.Authorizer,
		session,
		permissionChecks...,
	); err != nil {
		return artifact.GetDockerArtifactManifests403JSONResponse{
			UnauthorizedJSONResponse: artifact.UnauthorizedJSONResponse(
				*GetErrorResponse(http.StatusForbidden, err.Error()),
			),
		}, nil
	}

	image := string(r.Artifact)
	version := string(r.Version)
	artifactMetadata, err := c.TagStore.GetLatestTagMetadata(ctx, regInfo.parentID, regInfo.RegistryIdentifier, image)
	if err != nil {
		return artifactManifestsErrorRs(err), nil
	}
	manifestDetailsList, err := c.ProcessManifest(ctx, regInfo, image, version, artifactMetadata.DownloadCount)
	if err != nil {
		return artifactManifestsErrorRs(err), nil
	}

	return artifact.GetDockerArtifactManifests200JSONResponse{
		DockerManifestsResponseJSONResponse: artifact.DockerManifestsResponseJSONResponse{
			Data: artifact.DockerManifests{
				ImageName: image,
				Version:   version,
				Manifests: &manifestDetailsList,
			},
			Status: artifact.StatusSUCCESS,
		},
	}, nil
}

func (c *APIController) getManifestList(
	ctx context.Context, reqManifest *ml.DeserializedManifestList, registry *types.Registry, image string,
	regInfo *RegistryRequestBaseInfo, downloadCount int64,
) ([]artifact.DockerManifestDetails, error) {
	manifestDetailsList := []artifact.DockerManifestDetails{}
	for _, manifestEntry := range reqManifest.Manifests {
		dgst, err := types.NewDigest(manifestEntry.Digest)
		if err != nil {
			return nil, err
		}
		referencedManifest, err := c.ManifestStore.FindManifestByDigest(ctx, registry.ID, image, dgst)
		if err != nil {
			if errors.Is(err, store2.ErrResourceNotFound) {
				if registry.Type == artifact.RegistryTypeUPSTREAM {
					continue
				}
				return nil, fmt.Errorf("manifest: %s not found", dgst.String())
			}
			return nil, err
		}
		mConfig, err := getManifestConfig(
			ctx, referencedManifest.Configuration.Digest,
			regInfo.RootIdentifier, c.StorageDriver,
		)
		if err != nil {
			return nil, err
		}
		manifestDetailsList = append(manifestDetailsList, getManifestDetails(referencedManifest, mConfig, downloadCount))
	}
	return manifestDetailsList, nil
}

func artifactManifestsErrorRs(err error) artifact.GetDockerArtifactManifestsResponseObject {
	return artifact.GetDockerArtifactManifests500JSONResponse{
		InternalServerErrorJSONResponse: artifact.InternalServerErrorJSONResponse(
			*GetErrorResponse(http.StatusInternalServerError, err.Error()),
		),
	}
}

func getManifestDetails(
	m *types.Manifest, mConfig *manifestConfig, downloadsCount int64) artifact.DockerManifestDetails {
	createdAt := GetTimeInMs(m.CreatedAt)
	size := GetSize(m.TotalSize)

	manifestDetails := artifact.DockerManifestDetails{
		Digest:         m.Digest.String(),
		CreatedAt:      &createdAt,
		Size:           &size,
		DownloadsCount: &downloadsCount,
	}
	if mConfig != nil {
		manifestDetails.OsArch = fmt.Sprintf("%s/%s", mConfig.Os, mConfig.Arch)
	}
	return manifestDetails
}

// ProcessManifest processes a Docker artifact manifest by retrieving the manifest details from the database,
// converting it to the appropriate format, and extracting the necessary information based on the manifest type.
// It handles different types of manifests, including schema2, OCI schema, and manifest lists, and returns a list
// of Docker manifest details.
func (c *APIController) ProcessManifest(
	ctx context.Context,
	regInfo *RegistryRequestBaseInfo,
	image, version string, downloadCount int64,
) ([]artifact.DockerManifestDetails, error) {
	registry, err := c.RegistryRepository.GetByParentIDAndName(ctx, regInfo.parentID, regInfo.RegistryIdentifier)
	if err != nil {
		return nil, err
	}
	t, err := c.TagStore.FindTag(ctx, registry.ID, image, version)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	m, err := c.ManifestStore.Get(ctx, t.ManifestID)
	if err != nil {
		return nil, err
	}
	manifest, err := docker.DBManifestToManifest(m)
	if err != nil {
		return nil, err
	}
	manifestDetailsList := []artifact.DockerManifestDetails{}
	switch reqManifest := manifest.(type) {
	case *schema2.DeserializedManifest:
		mConfig, err := getManifestConfig(ctx, reqManifest.Config().Digest, regInfo.RootIdentifier, c.StorageDriver)
		if err != nil {
			return nil, err
		}
		manifestDetailsList = append(manifestDetailsList, getManifestDetails(m, mConfig, downloadCount))
	case *ocischema.DeserializedManifest:
		mConfig, err := getManifestConfig(ctx, reqManifest.Config().Digest, regInfo.RootIdentifier, c.StorageDriver)
		if err != nil {
			return nil, err
		}
		manifestDetailsList = append(manifestDetailsList, getManifestDetails(m, mConfig, downloadCount))
	case *ml.DeserializedManifestList:
		manifestDetailsList, err = c.getManifestList(ctx, reqManifest, registry, image, regInfo, downloadCount)
		if err != nil {
			return nil, err
		}
	default:
		log.Ctx(ctx).Error().Stack().Err(err).Msgf("Unknown manifest type: %T", manifest)
	}
	return manifestDetailsList, nil
}
