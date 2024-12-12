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
import { useHistory } from 'react-router-dom'

import { useStrings } from '@ar/frameworks/strings'
import { useParentComponents, useRoutes } from '@ar/hooks'
import { RepositoryDetailsTab } from '@ar/pages/repository-details/constants'
import { PermissionIdentifier, ResourceType } from '@ar/common/permissionTypes'

import type { ArtifactActionProps } from './types'

export default function EditRepositoryMenuItem({ repoKey }: ArtifactActionProps): JSX.Element {
  const { getString } = useStrings()
  const { RbacMenuItem } = useParentComponents()
  const history = useHistory()
  const routes = useRoutes()

  const handleOpenRepository = (): void => {
    history.push(
      routes.toARRepositoryDetails({
        repositoryIdentifier: repoKey,
        tab: RepositoryDetailsTab.CONFIGURATION
      })
    )
  }

  return (
    <>
      <RbacMenuItem
        icon="edit"
        text={getString('artifactList.table.actions.editRepository')}
        onClick={handleOpenRepository}
        permission={{
          resource: {
            resourceType: ResourceType.ARTIFACT_REGISTRY,
            resourceIdentifier: repoKey
          },
          permission: PermissionIdentifier.EDIT_ARTIFACT_REGISTRY
        }}
      />
    </>
  )
}
