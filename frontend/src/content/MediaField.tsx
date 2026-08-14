// SPDX-License-Identifier: Apache-2.0

import { Button, Stack, Text } from '@gophenberg/frontend-sdk'

import { MediaLibraryPicker } from '../media/MediaLibraryPicker'
import type { ContentField } from './types'

/** What the control reports when a media field points at nothing. */
const nothingChosen = 'No media chosen'

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
			<Text variant="body-sm">{props.field.label}</Text>
			<Text>{props.value === undefined ? nothingChosen : `Media ${props.value}`}</Text>
			<MediaLibraryPicker
				value={props.value}
				onSelect={(picked) => props.onChange(pickedMedia(picked))}
				onClose={() => {}}
				render={({ open }) => (
					<Button variant="outline" onClick={open}>
						Choose {props.field.label}
					</Button>
				)}
			/>
			{props.value !== undefined && (
				<Button variant="outline" onClick={() => props.onChange(null)}>
					Clear {props.field.label}
				</Button>
			)}
		</Stack>
	)
}
