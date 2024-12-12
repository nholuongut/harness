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
	"net/http"
	"strings"

	apiauth "github.com/harness/gitness/app/api/auth"
	"github.com/harness/gitness/app/api/request"
	"github.com/harness/gitness/app/paths"
	"github.com/harness/gitness/registry/app/api/openapi/contracts/artifact"
	"github.com/harness/gitness/registry/app/common"
	"github.com/harness/gitness/types/enum"
)

func (c *APIController) GetClientSetupDetails(
	ctx context.Context,
	r artifact.GetClientSetupDetailsRequestObject,
) (artifact.GetClientSetupDetailsResponseObject, error) {
	regRefParam := r.RegistryRef
	imageParam := r.Params.Artifact
	tagParam := r.Params.Version

	regInfo, _ := c.GetRegistryRequestBaseInfo(ctx, "", string(regRefParam))

	space, err := c.SpaceStore.FindByRef(ctx, regInfo.ParentRef)
	if err != nil {
		return artifact.GetClientSetupDetails400JSONResponse{
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
		return artifact.GetClientSetupDetails403JSONResponse{
			UnauthorizedJSONResponse: artifact.UnauthorizedJSONResponse(
				*GetErrorResponse(http.StatusForbidden, err.Error()),
			),
		}, nil
	}

	reg, err := c.RegistryRepository.GetByParentIDAndName(ctx, regInfo.parentID, regInfo.RegistryIdentifier)
	if err != nil {
		return artifact.GetClientSetupDetails404JSONResponse{
			NotFoundJSONResponse: artifact.NotFoundJSONResponse(
				*GetErrorResponse(http.StatusNotFound, "registry doesn't exist with this ref"),
			),
		}, err
	}

	if imageParam != nil {
		_, err := c.ImageStore.GetByName(ctx, reg.ID, string(*imageParam))
		if err != nil {
			return artifact.GetClientSetupDetails404JSONResponse{
				NotFoundJSONResponse: artifact.NotFoundJSONResponse(
					*GetErrorResponse(http.StatusNotFound, "image doesn't exist"),
				),
			}, err
		}
		if tagParam != nil {
			_, err := c.TagStore.FindTag(ctx, reg.ID, string(*imageParam), string(*tagParam))
			if err != nil {
				return artifact.GetClientSetupDetails404JSONResponse{
					NotFoundJSONResponse: artifact.NotFoundJSONResponse(
						*GetErrorResponse(http.StatusNotFound, "tag doesn't exist"),
					),
				}, err
			}
		}
	}

	packageType := string(reg.PackageType)

	return artifact.GetClientSetupDetails200JSONResponse{
		ClientSetupDetailsResponseJSONResponse: *c.GenerateClientSetupDetails(
			ctx, packageType, imageParam, tagParam, regInfo.RegistryRef,
		),
	}, nil
}

func (c *APIController) GenerateClientSetupDetails(
	ctx context.Context,
	packageType string,
	image *artifact.ArtifactParam,
	tag *artifact.VersionParam,
	registryRef string,
) *artifact.ClientSetupDetailsResponseJSONResponse {
	session, _ := request.AuthSessionFrom(ctx)
	username := session.Principal.Email
	loginUsernameLabel := "Username: <USERNAME>"
	loginUsernameValue := "<USERNAME>"
	loginPasswordLabel := "Password: *see step 2*"
	blankString := ""
	// Fixme: Use ENUMS
	if packageType == "HELM" {
		header1 := "Login to Helm"
		section1step1Header := "Run this Helm command in your terminal to authenticate the client."
		helmLoginValue := "helm registry login <LOGIN_HOSTNAME>"
		section1step1Commands := []artifact.ClientSetupStepCommand{
			{Label: &blankString, Value: &helmLoginValue},
			{Label: &loginUsernameLabel, Value: &loginUsernameValue},
			{Label: &loginPasswordLabel, Value: &blankString},
		}
		section1step1Type := artifact.ClientSetupStepTypeStatic
		section1step2Header := "For the Password field above, generate an identity token"
		section1step2Type := artifact.ClientSetupStepTypeGenerateToken
		section1 := []artifact.ClientSetupStep{
			{
				Header:   &section1step1Header,
				Commands: &section1step1Commands,
				Type:     &section1step1Type,
			},
			{
				Header: &section1step2Header,
				Type:   &section1step2Type,
			},
		}

		header2 := "Push a version"
		section2step1Header := "Run this Helm push command in your terminal to push a chart in OCI form." +
			" Note: Make sure you add oci:// prefix to the repository URL."
		helmPushValue := "helm push <CHART_TGZ_FILE> oci://<HOSTNAME>/<REGISTRY_NAME>"
		section2step1Commands := []artifact.ClientSetupStepCommand{
			{Label: &blankString, Value: &helmPushValue},
		}
		section2step1Type := artifact.ClientSetupStepTypeStatic
		section2 := []artifact.ClientSetupStep{
			{
				Header:   &section2step1Header,
				Commands: &section2step1Commands,
				Type:     &section2step1Type,
			},
		}

		header3 := "Pull a version"
		section3step1Header := "Run this Helm command in your terminal to pull a specific chart version."
		helmPullValue := "helm pull oci://<HOSTNAME>/<REGISTRY_NAME>/<IMAGE_NAME> --version <TAG>"
		section3step1Commands := []artifact.ClientSetupStepCommand{
			{Label: &blankString, Value: &helmPullValue},
		}
		section3step1Type := artifact.ClientSetupStepTypeStatic
		section3 := []artifact.ClientSetupStep{
			{
				Header:   &section3step1Header,
				Commands: &section3step1Commands,
				Type:     &section3step1Type,
			},
		}
		clientSetupDetails := artifact.ClientSetupDetails{
			MainHeader: "Helm Client Setup",
			SecHeader:  "Follow these instructions to install/use Helm artifacts or compatible packages.",
			Sections: []artifact.ClientSetupSection{
				{
					Header: &header1,
					Steps:  &section1,
				},
				{
					Header: &header2,
					Steps:  &section2,
				},
				{
					Header: &header3,
					Steps:  &section3,
				},
			},
		}

		c.replacePlaceholders(ctx, clientSetupDetails, username, registryRef, image, tag)

		return &artifact.ClientSetupDetailsResponseJSONResponse{
			Data:   clientSetupDetails,
			Status: artifact.StatusSUCCESS,
		}
	}
	header1 := "Login to Docker"
	section1step1Header := "Run this Docker command in your terminal to authenticate the client."
	dockerLoginValue := "docker login <LOGIN_HOSTNAME>"
	section1step1Commands := []artifact.ClientSetupStepCommand{
		{Label: &blankString, Value: &dockerLoginValue},
		{Label: &loginUsernameLabel, Value: &loginUsernameValue},
		{Label: &loginPasswordLabel, Value: &blankString},
	}
	section1step1Type := artifact.ClientSetupStepTypeStatic
	section1step2Header := "For the Password field above, generate an identity token"
	section1step2Type := artifact.ClientSetupStepTypeGenerateToken
	section1 := []artifact.ClientSetupStep{
		{
			Header:   &section1step1Header,
			Commands: &section1step1Commands,
			Type:     &section1step1Type,
		},
		{
			Header: &section1step2Header,
			Type:   &section1step2Type,
		},
	}
	header2 := "Pull an image"
	section2step1Header := "Run this Docker command in your terminal to pull image."
	dockerPullValue := "docker pull <HOSTNAME>/<REGISTRY_NAME>/<IMAGE_NAME>:<TAG>"
	section2step1Commands := []artifact.ClientSetupStepCommand{
		{Label: &blankString, Value: &dockerPullValue},
	}
	section2step1Type := artifact.ClientSetupStepTypeStatic
	section2 := []artifact.ClientSetupStep{
		{
			Header:   &section2step1Header,
			Commands: &section2step1Commands,
			Type:     &section2step1Type,
		},
	}
	header3 := "Retag and Push the image"
	section3step1Header := "Run this Docker command in your terminal to tag the image."
	dockerTagValue := "docker tag <IMAGE_NAME>:<TAG> <HOSTNAME>/<REGISTRY_NAME>/<IMAGE_NAME>:<TAG>"
	section3step1Commands := []artifact.ClientSetupStepCommand{
		{Label: &blankString, Value: &dockerTagValue},
	}
	section3step1Type := artifact.ClientSetupStepTypeStatic
	section3step2Header := "Run this Docker command in your terminal to push the image."
	dockerPushValue := "docker push <HOSTNAME>/<REGISTRY_NAME>/<IMAGE_NAME>:<TAG>"
	section3step2Commands := []artifact.ClientSetupStepCommand{
		{Label: &blankString, Value: &dockerPushValue},
	}
	section3step2Type := artifact.ClientSetupStepTypeStatic
	section3 := []artifact.ClientSetupStep{
		{
			Header:   &section3step1Header,
			Commands: &section3step1Commands,
			Type:     &section3step1Type,
		},
		{
			Header:   &section3step2Header,
			Commands: &section3step2Commands,
			Type:     &section3step2Type,
		},
	}
	clientSetupDetails := artifact.ClientSetupDetails{
		MainHeader: "Docker Client Setup",
		SecHeader:  "Follow these instructions to install/use Docker artifacts or compatible packages.",
		Sections: []artifact.ClientSetupSection{
			{
				Header: &header1,
				Steps:  &section1,
			},
			{
				Header: &header3,
				Steps:  &section3,
			},
			{
				Header: &header2,
				Steps:  &section2,
			},
		},
	}

	c.replacePlaceholders(ctx, clientSetupDetails, username, registryRef, image, tag)

	return &artifact.ClientSetupDetailsResponseJSONResponse{
		Data:   clientSetupDetails,
		Status: artifact.StatusSUCCESS,
	}
}

func (c *APIController) replacePlaceholders(
	ctx context.Context,
	clientSetupDetails artifact.ClientSetupDetails,
	username string,
	regRef string,
	image *artifact.ArtifactParam,
	tag *artifact.VersionParam,
) {
	rootSpace, _, _ := paths.DisectRoot(regRef)
	_, registryName, _ := paths.DisectLeaf(regRef)
	hostname := common.TrimURLScheme(c.URLProvider.RegistryURL(ctx, rootSpace))

	for _, s := range clientSetupDetails.Sections {
		if s.Steps == nil {
			continue
		}
		for _, st := range *s.Steps {
			if st.Commands == nil {
				continue
			}
			for i := range *st.Commands {
				replaceText(username, st, i, hostname, registryName, image, tag)
			}
		}
	}
}

func replaceText(
	username string,
	st artifact.ClientSetupStep,
	i int,
	hostname string,
	repoName string,
	image *artifact.ArtifactParam,
	tag *artifact.VersionParam,
) {
	if username != "" {
		(*st.Commands)[i].Value = stringPtr(strings.ReplaceAll(*(*st.Commands)[i].Value, "<USERNAME>", username))
		(*st.Commands)[i].Label = stringPtr(strings.ReplaceAll(*(*st.Commands)[i].Label, "<USERNAME>", username))
	}
	if hostname != "" {
		(*st.Commands)[i].Value = stringPtr(strings.ReplaceAll(*(*st.Commands)[i].Value, "<HOSTNAME>", hostname))
	}
	if hostname != "" {
		(*st.Commands)[i].Value = stringPtr(strings.ReplaceAll(*(*st.Commands)[i].Value,
			"<LOGIN_HOSTNAME>", common.GetHost(hostname)))
	}
	if repoName != "" {
		(*st.Commands)[i].Value = stringPtr(strings.ReplaceAll(*(*st.Commands)[i].Value, "<REGISTRY_NAME>", repoName))
	}
	if image != nil {
		(*st.Commands)[i].Value = stringPtr(strings.ReplaceAll(*(*st.Commands)[i].Value, "<IMAGE_NAME>", string(*image)))
	}
	if tag != nil {
		(*st.Commands)[i].Value = stringPtr(strings.ReplaceAll(*(*st.Commands)[i].Value, "<TAG>", string(*tag)))
	}
}

func stringPtr(s string) *string {
	return &s
}
