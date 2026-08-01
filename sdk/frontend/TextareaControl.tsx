// SPDX-License-Identifier: Apache-2.0

import { Field, Textarea } from '@wordpress/ui'

const DEFAULT_ROWS = 4

/**
 * Renders a textarea carrying its own label.
 * @param props - The label, the value and the handler for a change.
 * @returns The labelled textarea.
 */
export function TextareaControl({
	label,
	value,
	onValueChange,
}: {
	label: string
	value: string
	onValueChange: (value: string) => void
}) {
	return (
		<Field.Root>
			<Field.Label>{label}</Field.Label>
			<Field.Control
				render={<Textarea rows={DEFAULT_ROWS} value={value} onValueChange={onValueChange} />}
			/>
		</Field.Root>
	)
}
