// SPDX-License-Identifier: Apache-2.0

import { CheckboxControl, InputControl, Stack } from '@gophenberg/frontend-sdk'
import { __, sprintf } from '@wordpress/i18n'

import { FieldLabel } from './FieldLabel'
import type { ContentField } from './types'

/** The three parts a link field holds. */
export interface LinkValue {
	url: string
	title: string
	new_tab: boolean
}

/**
 * Returns the link a stored value holds, its parts blank where the value names none.
 * @param value - The value the buffer holds under the field key.
 * @returns The link.
 */
export function linkHeld(value: unknown): LinkValue {
	const held = (typeof value === 'object' && value !== null ? value : {}) as Record<string, unknown>
	return {
		url: typeof held.url === 'string' ? held.url : '',
		title: typeof held.title === 'string' ? held.title : '',
		new_tab: held.new_tab === true,
	}
}

/**
 * Returns the link to store, or nothing when neither its address nor its title was written.
 * @param link - The link as the control holds it.
 * @returns The link, or null.
 */
export function linkToStore(link: LinkValue): LinkValue | null {
	return link.url === '' && link.title === '' ? null : link
}

/**
 * Renders the three inputs a link field is filled in through.
 * @param props - The field, the link held, and what to do with a change.
 * @returns The control element.
 */
export function LinkField(props: {
	field: ContentField
	value: LinkValue
	onChange: (value: LinkValue | null) => void
}) {
	const named = { field: props.field.label }
	/**
	 * Carries the link with one part changed.
	 * @param part - The part of the link to change.
	 */
	function carry(part: Partial<LinkValue>) {
		props.onChange(linkToStore({ ...props.value, ...part }))
	}
	return (
		<Stack direction="column" gap="xs">
			<FieldLabel field={props.field} />
			<InputControl
				label={sprintf(__('%(field)s address', 'gophenberg'), named)}
				autoComplete="off"
				value={props.value.url}
				onValueChange={(url) => carry({ url })}
			/>
			<InputControl
				label={sprintf(__('%(field)s title', 'gophenberg'), named)}
				autoComplete="off"
				value={props.value.title}
				onValueChange={(title) => carry({ title })}
			/>
			<CheckboxControl
				__nextHasNoMarginBottom
				label={sprintf(__('Open %(field)s in a new tab', 'gophenberg'), named)}
				checked={props.value.new_tab}
				onChange={(newTab) => carry({ new_tab: newTab })}
			/>
		</Stack>
	)
}
