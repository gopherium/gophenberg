// SPDX-License-Identifier: Apache-2.0

import { Field, Textarea } from '@wordpress/ui'

const DEFAULT_ROWS = 4

/**
 * Renders a textarea carrying its own label.
 * @param props - The label, the value, the words under it, whether it is read only, and the handler for a change.
 * @returns The labelled textarea.
 */
export function TextareaControl({
	label,
	value,
	description,
	readOnly,
	onValueChange,
}: {
	label: string
	value: string
	description?: string
	readOnly?: boolean
	onValueChange: (value: string) => void
}) {
	return (
		<Field.Root>
			<Field.Label>{label}</Field.Label>
			<Field.Control
				render={<Textarea rows={DEFAULT_ROWS} value={value} readOnly={readOnly} onValueChange={onValueChange} />}
			/>
			{description === undefined ? null : <Field.Description>{description}</Field.Description>}
		</Field.Root>
	)
}
