// SPDX-License-Identifier: Apache-2.0

import { SelectControl } from '@gophenberg/frontend-sdk'
import { __ } from '@wordpress/i18n'

import { chosenOf } from './select'
import type { BacklinkSources } from './groups'

/**
 * Renders the pickers naming the group and the relation a backlinks field reads.
 * @param props - The sources on offer and what to report when one is picked.
 * @returns The pickers element.
 */
export function SourceOfLinks(props: {
	sources: BacklinkSources
	onGroup: (key: string) => void
	onField: (key: string) => void
}) {
	const { groups, fields, group, field } = props.sources
	return (
		<>
			<SelectControl
				label={__('Reads from', 'gophenberg')}
				items={groups}
				value={group}
				onValueChange={(item) => props.onGroup(chosenOf(item, groups, group).value)}
			/>
			<SelectControl
				label={__('Through', 'gophenberg')}
				items={fields}
				value={field}
				onValueChange={(item) => props.onField(chosenOf(item, fields, field).value)}
			/>
		</>
	)
}
