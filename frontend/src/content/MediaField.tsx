// SPDX-License-Identifier: Apache-2.0

import { Button, Stack, Text } from '@gophenberg/frontend-sdk'
import { __, sprintf } from '@wordpress/i18n'

import { MediaLibraryPicker } from '../media/MediaLibraryPicker'
import { FieldLabel } from './FieldLabel'
import type { ContentField } from './types'

/**
 * Returns the media identity a stored value holds.
 * @param value - The value the buffer holds under the field key.
 * @returns The identity, or nothing when the field holds none.
 */
export function mediaHeld(value: unknown): number | undefined {
	return typeof value === 'number' ? value : undefined
}

/**
 * Returns the identity a picked attachment carries.
 * @param picked - What the library reported.
 * @returns The identity, or nothing when it reported none.
 */
export function pickedMedia(picked: unknown): number | null {
	if (Array.isArray(picked)) {
		return pickedMedia(picked[0])
	}
	if (typeof picked === 'object' && picked !== null && 'id' in picked) {
		const held = (picked as { id: unknown }).id
		return typeof held === 'number' ? held : null
	}
	return null
}

/**
 * Renders the control pointing a media field at a stored upload.
 * @param props - The field, the identity held, and what to do with a choice.
 * @returns The control element.
 */
export function MediaField(props: {
	field: ContentField
	value: number | undefined
	onChange: (value: number | null) => void
}) {
	return (
		<Stack direction="column" gap="xs">
			<FieldLabel field={props.field} />
			<Text>
				{props.value === undefined
					? __('No media chosen', 'gophenberg')
					: sprintf(__('Media %(id)d', 'gophenberg'), { id: props.value })}
			</Text>
			<MediaLibraryPicker
				value={props.value}
				onSelect={(picked) => props.onChange(pickedMedia(picked))}
				onClose={() => {}}
				render={({ open }) => (
					<Button variant="outline" onClick={open}>
						{sprintf(__('Choose %(field)s', 'gophenberg'), { field: props.field.label })}
					</Button>
				)}
			/>
			{props.value !== undefined && (
				<Button variant="outline" onClick={() => props.onChange(null)}>
					{sprintf(__('Clear %(field)s', 'gophenberg'), { field: props.field.label })}
				</Button>
			)}
		</Stack>
	)
}
