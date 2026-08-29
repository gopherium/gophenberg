// SPDX-License-Identifier: Apache-2.0

import { Text } from '@gophenberg/frontend-sdk'

import type { ContentField } from './types'

/**
 * Renders the name of a field the panel builds by hand, with the instructions it carries.
 * @param props - The field to name.
 * @returns The name and instructions.
 */
export function FieldLabel({ field }: { field: ContentField }) {
	const instructions = field.settings.instructions
	return (
		<>
			<Text variant="body-sm">{field.label}</Text>
			{typeof instructions === 'string' && instructions !== '' && (
				<Text variant="body-sm">{instructions}</Text>
			)}
		</>
	)
}
