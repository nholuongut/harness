import { Container, Layout, Text } from '@harnessio/uicore'
import React from 'react'
import { Cloud } from 'iconoir-react'
import { Menu, MenuItem } from '@blueprintjs/core'
import { useFormikContext } from 'formik'
import { useStrings } from 'framework/strings'
import type { OpenapiCreateGitspaceRequest } from 'services/cde'
import { CDECustomDropdown } from '../CDECustomDropdown/CDECustomDropdown'

const SelectInfraProviderType = () => {
  const { getString } = useStrings()

  const { values, setFieldValue: onChange } = useFormikContext<OpenapiCreateGitspaceRequest>()

  const infraProviders = [
    { label: 'HARNESS GCP', value: 'HARNESS_GCP' },
    { label: 'HARNESS OVH', value: 'HARNESS_OVH' }
  ]

  const selectedInfraProvider = infraProviders.find(item => item.value === values?.metadata?.infraProvider)

  return (
    <Container>
      <CDECustomDropdown
        label={
          <Layout.Horizontal spacing={'small'} flex={{ alignItems: 'center', justifyContent: 'flex-start' }}>
            <Layout.Vertical>
              <Text font={'normal'}>{selectedInfraProvider?.label || getString('cde.infraProvider')}</Text>
            </Layout.Vertical>
          </Layout.Horizontal>
        }
        leftElement={
          <Layout.Horizontal>
            <Cloud height={20} width={20} style={{ marginRight: '8px', alignItems: 'center' }} />
            <Layout.Vertical spacing="small">
              <Text>Infra Provider Type</Text>
              <Text font="small">Your Gitspace will run on the selected infra provider type</Text>
            </Layout.Vertical>
          </Layout.Horizontal>
        }
        menu={
          <Menu>
            {infraProviders.map(({ label, value }) => {
              return (
                <MenuItem
                  key={label}
                  active={label === selectedInfraProvider?.label}
                  text={<Text font={{ size: 'normal', weight: 'bold' }}>{label.toUpperCase()}</Text>}
                  onClick={() => {
                    onChange('metadata.infraProvider', value)
                    onChange('metadata.region', undefined)
                    onChange('resource_identifier', undefined)
                  }}
                />
              )
            })}
          </Menu>
        }
      />
    </Container>
  )
}

export default SelectInfraProviderType
