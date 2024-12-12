/*
 * Copyright 2024 Harness, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import React from 'react'
import { noop } from 'lodash-es'

import { useParentComponents } from '@ar/hooks'
import { useStrings } from '@ar/frameworks/strings'
import { PermissionIdentifier, ResourceType } from '@ar/common/permissionTypes'
import type { UpstreamProxyActionProps } from './type'

export default function ScanUpstreamProxy({ data }: UpstreamProxyActionProps): JSX.Element {
  const { getString } = useStrings()
  const { RbacMenuItem } = useParentComponents()
  return (
    <RbacMenuItem
      icon="sto-grey"
      text={getString('actions.scan')}
      onClick={noop}
      permission={{
        resource: {
          resourceType: ResourceType.ARTIFACT_REGISTRY,
          resourceIdentifier: data.identifier
        },
        permission: PermissionIdentifier.EDIT_ARTIFACT_REGISTRY
      }}
    />
  )
}
