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
 * Returns the identities a gallery value holds.
 * @param value - The value the buffer holds under the field key.
 * @returns The identities, empty when the field holds none.
 */
export function galleryHeld(value: unknown): number[] {
	if (!Array.isArray(value)) {
		return []
	}
	return value.filter((held): held is number => typeof held === 'number')
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
 * Renders the control filling a media field that holds many items.
 * @param props - The field, the identities held, and what to do with a change.
 * @returns The control element.
 */
export function GalleryField(props: {
	field: ContentField
	value: number[]
	onChange: (value: number[] | null) => void
}) {
	return (
		<Stack direction="column" gap="xs">
			<FieldLabel field={props.field} />
			{props.value.map((id) => (
				<Stack key={id} direction="row" gap="sm">
					<Text>{sprintf(__('Media %(id)d', 'gophenberg'), { id })}</Text>
					<Button
						variant="outline"
						size="compact"
						onClick={() => {
							const kept = props.value.filter((held) => held !== id)
							props.onChange(kept.length > 0 ? kept : null)
						}}
					>
						{sprintf(__('Remove Media %(id)d', 'gophenberg'), { id })}
					</Button>
				</Stack>
			))}
			<MediaLibraryPicker
				onSelect={(picked) => {
					const chosen = pickedMedia(picked)
					if (chosen !== null && !props.value.includes(chosen)) {
						props.onChange([...props.value, chosen])
					}
				}}
				onClose={() => {}}
				render={({ open }) => (
					<Button variant="outline" onClick={open}>
						{sprintf(__('Add to %(field)s', 'gophenberg'), { field: props.field.label })}
					</Button>
				)}
			/>
		</Stack>
	)
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
