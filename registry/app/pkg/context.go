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

package pkg

import (
	v2 "github.com/distribution/distribution/v3/registry/api/v2"
)

type BaseInfo struct {
	ParentID       int64
	RootIdentifier string
	RootParentID   int64
}

type ArtifactInfo struct {
	*BaseInfo
	RegIdentifier string
	Image         string
}

type RegistryInfo struct {
	*ArtifactInfo
	Reference  string
	Digest     string
	Tag        string
	URLBuilder *v2.URLBuilder
	Path       string
}

func (r *RegistryInfo) SetReference(ref string) {
	r.Reference = ref
}

func (a *ArtifactInfo) SetRepoKey(key string) {
	a.RegIdentifier = key
}
