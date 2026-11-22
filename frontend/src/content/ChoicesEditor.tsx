// SPDX-License-Identifier: Apache-2.0

import { Button, InputControl, Stack, Text } from '@gophenberg/frontend-sdk'
import { __ } from '@wordpress/i18n'

import type { ChoicePair } from './types'

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
	return (
		<Stack direction="column" gap="sm">
			<Text variant="body-sm">{__('Choices', 'gophenberg')}</Text>
			{props.pairs.map((pair, at) => (
				<Stack key={at} direction="row" gap="sm">
					<InputControl
						label={__('Value', 'gophenberg')}
						autoComplete="off"
						value={pair.value}
						readOnly={props.readOnly}
						onValueChange={(written) =>
							props.onChange(props.pairs.map((held, row) =>
								row === at ? { ...held, value: written } : held,
							))
						}
					/>
					<InputControl
						label={__('Label', 'gophenberg')}
						autoComplete="off"
						value={pair.label}
						readOnly={props.readOnly}
						onValueChange={(written) =>
							props.onChange(props.pairs.map((held, row) =>
								row === at ? { ...held, label: written } : held,
							))
						}
					/>
					<Button
						variant="outline"
						size="compact"
						disabled={props.readOnly}
						onClick={() => props.onChange(props.pairs.filter((_, row) => row !== at))}
					>
						{__('Remove', 'gophenberg')}
					</Button>
				</Stack>
			))}
			<Button
				variant="outline"
				disabled={props.readOnly}
				onClick={() => props.onChange([...props.pairs, { value: '', label: '' }])}
			>
				{__('Add choice', 'gophenberg')}
			</Button>
		</Stack>
	)
}
