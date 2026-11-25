// SPDX-License-Identifier: Apache-2.0

import { Fieldset, InputControl } from '@gophenberg/frontend-sdk'
import { RepeatRows } from '@gopherium/godmin'
import { formatNumber } from '@gopherium/gottext'
import { __, sprintf } from '@wordpress/i18n'
import { useState } from 'react'

import type { ChoicePair } from './types'

/**
 * Reports whether two lists hold the same value and label at each place.
 * @param shown - The list the editor shows.
 * @param held - The pairs held.
 * @returns Whether both hold the same pairs.
 */
function samePairs(shown: ChoicePair[], held: ChoicePair[]): boolean {
	return (
		shown.length === held.length &&
		shown.every((pair, at) => pair.value === held[at].value && pair.label === held[at].label)
	)
}

/**
 * Returns the rows the editor shows, the very list it last showed or handed on while the pairs held match it.
 * @param held - The pairs held.
 * @returns The rows and what records a list the editor hands on.
 */
function useShownPairs(held: ChoicePair[]) {
	const [shown, setShown] = useState(held)
	const same = samePairs(shown, held)
	if (!same) {
		setShown(held)
	}
	return { rows: same ? shown : held, keep: setShown }
}

/**
 * Renders the rows editing the value and label pairs a choice field offers.
 * @param props - The pairs held, whether they are read only, and what to call with an edit.
 * @returns The editor element.
 */
export function ChoicesEditor(props: {
	pairs: ChoicePair[]
	readOnly: boolean
	onChange: (pairs: ChoicePair[]) => void
}) {
	const { readOnly } = props
	const shown = useShownPairs(props.pairs)
	const pairs = shown.rows
	const bound = readOnly ? pairs.length : undefined
	return (
		<Fieldset.Root>
			<Fieldset.Legend>{__('Choices', 'gophenberg')}</Fieldset.Legend>
			<RepeatRows
				rows={pairs}
				onChange={(next) => {
					if (!readOnly) {
						shown.keep(next)
						props.onChange(next)
					}
				}}
				blank={(): ChoicePair => ({ value: '', label: '' })}
				renderRow={(pair, update) => (
					<div className="godmin-form__row">
						<InputControl
							className="godmin-form__grow"
							label={__('Value', 'gophenberg')}
							autoComplete="off"
							value={pair.value}
							readOnly={readOnly}
							onValueChange={(value) => update({ ...pair, value })}
						/>
						<InputControl
							label={__('Label', 'gophenberg')}
							autoComplete="off"
							value={pair.label}
							readOnly={readOnly}
							onValueChange={(label) => update({ ...pair, label })}
						/>
					</div>
				)}
				rowLabel={(at) => sprintf(__('Choice %(number)s', 'gophenberg'), { number: formatNumber(at + 1) })}
				labels={{
					add: __('Add choice', 'gophenberg'),
					empty: __('No choices yet.', 'gophenberg'),
					moveUp: __('Move choice up', 'gophenberg'),
					moveDown: __('Move choice down', 'gophenberg'),
					remove: __('Remove choice', 'gophenberg'),
				}}
				min={bound}
				max={bound}
			/>
		</Fieldset.Root>
	)
}
