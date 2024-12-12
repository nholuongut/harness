// Source: https://github.com/distribution/distribution

// Copyright 2014 https://github.com/distribution/distribution Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dcontext

import (
	"context"

	"github.com/rs/zerolog/log"
)

type versionKey struct{}

func (versionKey) String() string { return "version" }

// WithVersion stores the application version in the context. The new context
// gets a logger to ensure log messages are marked with the application
// version.
func WithVersion(ctx context.Context, version string) context.Context {
	ctx = context.WithValue(ctx, versionKey{}, version)
	// push a new logger onto the stack
	return WithLogger(ctx, GetLogger(ctx, log.Info(), versionKey{}))
}

// GetVersion returns the application version from the context. An empty
// string may returned if the version was not set on the context.
func GetVersion(ctx context.Context) string {
	return GetStringValue(ctx, versionKey{})
}
