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

package oci

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	usercontroller "github.com/harness/gitness/app/api/controller/user"
	"github.com/harness/gitness/app/auth/authn"
	"github.com/harness/gitness/app/auth/authz"
	corestore "github.com/harness/gitness/app/store"
	urlprovider "github.com/harness/gitness/app/url"
	"github.com/harness/gitness/registry/app/api/controller/metadata"
	"github.com/harness/gitness/registry/app/api/openapi/contracts/artifact"
	"github.com/harness/gitness/registry/app/dist_temp/dcontext"
	"github.com/harness/gitness/registry/app/dist_temp/errcode"
	"github.com/harness/gitness/registry/app/pkg"
	"github.com/harness/gitness/registry/app/pkg/commons"
	"github.com/harness/gitness/registry/app/pkg/docker"

	v2 "github.com/distribution/distribution/v3/registry/api/v2"
	"github.com/opencontainers/go-digest"
	"github.com/rs/zerolog/log"
)

func NewHandler(
	controller *docker.Controller, spaceStore corestore.SpaceStore, tokenStore corestore.TokenStore,
	userCtrl *usercontroller.Controller, authenticator authn.Authenticator, urlProvider urlprovider.Provider,
	authorizer authz.Authorizer, ociRelativeURL bool,
) *Handler {
	return &Handler{
		Controller:     controller,
		SpaceStore:     spaceStore,
		TokenStore:     tokenStore,
		UserCtrl:       userCtrl,
		Authenticator:  authenticator,
		URLProvider:    urlProvider,
		Authorizer:     authorizer,
		OCIRelativeURL: ociRelativeURL,
	}
}

type Handler struct {
	Controller     *docker.Controller
	SpaceStore     corestore.SpaceStore
	TokenStore     corestore.TokenStore
	UserCtrl       *usercontroller.Controller
	Authenticator  authn.Authenticator
	URLProvider    urlprovider.Provider
	Authorizer     authz.Authorizer
	OCIRelativeURL bool
}

type routeType string

const (
	Manifests            routeType = "manifests"            // /v2/:registry/:image/manifests/:reference.
	Blobs                routeType = "blobs"                // /v2/:registry/:image/blobs/:digest.
	BlobsUploadsSession  routeType = "blob-uploads-session" // /v2/:registry/:image/blobs/uploads/:session_id.
	Tags                 routeType = "tags"                 // /v2/:registry/:image/tags/list.
	Referrers            routeType = "referrers"            // /v2/:registry/:image/referrers/:digest.
	Invalid              routeType = "invalid"              // Invalid route.
	MinSizeOfURLSegments           = 5

	APIPartManifest = "manifests"
	APIPartBlobs    = "blobs"
	APIPartUpload   = "uploads"
	APIPartTag      = "tags"
	APIPartReferrer = "referrers"
	// Add other route types here.
)

func getRouteType(url string) routeType {
	url = strings.Trim(url, "/")
	segments := strings.Split(url, "/")
	if len(segments) < MinSizeOfURLSegments {
		return Invalid
	}
	typ := segments[len(segments)-2]
	switch typ {
	case APIPartManifest:
		return Manifests
	case APIPartBlobs:
		if segments[len(segments)-1] == APIPartUpload {
			return BlobsUploadsSession
		}
		return Blobs
	case APIPartUpload:
		return BlobsUploadsSession
	case APIPartTag:
		return Tags
	case APIPartReferrer:
		return Referrers
	}
	return Invalid
}

func GetQueryParamMap(queryParams url.Values) map[string]string {
	queryMap := make(map[string]string)
	for key, values := range queryParams {
		if len(values) > 0 {
			queryMap[key] = values[0]
		}
	}
	return queryMap
}

// ExtractPathVars extracts registry, image, reference, digest and tag from the path
// Path format: /v2/:rootSpace/:registry/:image/manifests/:reference (for ex:
// /v2/myRootSpace/reg1/alpine/blobs/sha256:a258b2a6b59a7aa244d8ceab095c7f8df726f27075a69fca7ad8490f3f63148a).
func ExtractPathVars(path string, paramMap map[string]string) (rootIdentifier, registry, image, ref, dgst, tag string) {
	path = strings.Trim(path, "/")
	segments := strings.Split(path, "/")
	rootIdentifier = segments[1]
	registry = segments[2]
	image = strings.Join(segments[3:len(segments)-2], "/")
	typ := getRouteType(path)

	switch typ {
	case Manifests:
		ref = segments[len(segments)-1]
		_, err := digest.Parse(ref)
		if err != nil {
			tag = ref
		} else {
			dgst = ref
		}
	case Blobs:
		dgst = segments[len(segments)-1]
	case BlobsUploadsSession:
		if segments[len(segments)-1] != APIPartUpload && segments[len(segments)-2] == APIPartUpload {
			image = strings.Join(segments[3:len(segments)-3], "/")
			ref = segments[len(segments)-1]
		}
		if _, ok := paramMap["digest"]; ok {
			dgst = paramMap["digest"]
		}
	case Tags:
		// do nothing.
	case Referrers:
		dgst = segments[len(segments)-1]
	case Invalid:
		log.Warn().Msgf("Invalid route: %s", path)
	default:
		log.Warn().Msgf("Unknown route type: %s", typ)
	}

	log.Debug().Msgf(
		"For path: %s, rootIdentifier: %s, registry: %s, image: %s, ref: %s, dgst: %s, tag: %s",
		path, rootIdentifier, registry, image, ref, dgst, tag,
	)

	return rootIdentifier, registry, image, ref, dgst, tag
}

func handleErrors(ctx context.Context, errors errcode.Errors, w http.ResponseWriter) {
	if !commons.IsEmpty(errors) {
		_ = errcode.ServeJSON(w, errors)
		docker.LogError(errors)
		log.Ctx(ctx).Error().Errs("OCI errors", errors).Msgf("Error occurred")
	} else if status, ok := ctx.Value("http.response.status").(int); ok && status >= 200 && status <= 399 {
		dcontext.GetResponseLogger(ctx, log.Info()).Msg("response completed")
	}
}

func (h *Handler) GetRegistryInfo(r *http.Request, remoteSupport bool) (pkg.RegistryInfo, error) {
	ctx := r.Context()
	queryParams := r.URL.Query()
	path := r.URL.Path
	paramMap := GetQueryParamMap(queryParams)
	rootIdentifier, registryIdentifier, image, ref, dgst, tag := ExtractPathVars(path, paramMap)
	if err := metadata.ValidateIdentifier(rootIdentifier); err != nil {
		return pkg.RegistryInfo{}, err
	}

	rootSpace, err := h.SpaceStore.FindByRefCaseInsensitive(ctx, rootIdentifier)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("Root space not found: %s", rootIdentifier)
		return pkg.RegistryInfo{}, errcode.ErrCodeRootNotFound
	}

	registry, err := h.Controller.RegistryDao.GetByRootParentIDAndName(ctx, rootSpace.ID, registryIdentifier)
	if err != nil {
		log.Ctx(ctx).Error().Msgf(
			"registry %s not found for root: %s. Reason: %s", registryIdentifier, rootSpace.Identifier, err,
		)
		return pkg.RegistryInfo{}, errcode.ErrCodeRegNotFound
	}
	_, err = h.SpaceStore.Find(r.Context(), registry.ParentID)
	if err != nil {
		log.Ctx(ctx).Error().Msgf("Parent space not found: %d", registry.ParentID)
		return pkg.RegistryInfo{}, errcode.ErrCodeParentNotFound
	}

	info := &pkg.RegistryInfo{
		ArtifactInfo: &pkg.ArtifactInfo{
			BaseInfo: &pkg.BaseInfo{
				RootIdentifier: rootIdentifier,
				RootParentID:   rootSpace.ID,
				ParentID:       registry.ParentID,
			},
			RegIdentifier: registryIdentifier,
			Image:         image,
		},
		Reference:  ref,
		Digest:     dgst,
		Tag:        tag,
		URLBuilder: v2.NewURLBuilderFromRequest(r, h.OCIRelativeURL),
		Path:       r.URL.Path,
	}

	log.Ctx(ctx).Info().Msgf("Dispatch: URI: %s", path)
	if commons.IsEmpty(rootSpace.Identifier) {
		log.Ctx(ctx).Error().Msgf("ParentRef not found in context")
		return pkg.RegistryInfo{}, errcode.ErrCodeParentNotFound
	}

	if commons.IsEmpty(registryIdentifier) {
		log.Ctx(ctx).Warn().Msgf("registry not found in context")
		return pkg.RegistryInfo{}, errcode.ErrCodeRegNotFound
	}

	if !commons.IsEmpty(info.Image) && !commons.IsEmpty(info.Tag) {
		flag, err2 := MatchArtifactFilter(registry.AllowedPattern, registry.BlockedPattern, info.Image+":"+info.Tag)
		if !flag || err2 != nil {
			return pkg.RegistryInfo{}, errcode.ErrCodeDenied
		}
	}

	if registry.Type == artifact.RegistryTypeUPSTREAM && !remoteSupport {
		log.Ctx(ctx).Warn().Msgf("Remote registryIdentifier %s not supported", registryIdentifier)
		return pkg.RegistryInfo{}, errcode.ErrCodeDenied
	}

	return *info, nil
}
