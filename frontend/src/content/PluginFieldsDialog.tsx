// SPDX-License-Identifier: Apache-2.0

import { Button, Dialog, Stack, Text } from '@gophenberg/frontend-sdk'
import { __, sprintf } from '@wordpress/i18n'
import { useState } from 'react'

import type { ContentField } from './types'
import type { FieldGroup } from './groups'

/**
 * Renders the fields a group holds as a plain list, sub fields nested under their container.
 * @param props - The fields to list.
 * @returns The list element.
 */
function FieldList(props: { fields: ContentField[] }) {
	return (
		<ul>
			{props.fields.map((field) => (
				<li key={field.key} aria-label={field.label}>
					<Stack direction="column" gap="xs">
						<Text>{field.label}</Text>
						<Text variant="body-sm">{field.key}</Text>
						{field.fields.length > 0 && <FieldList fields={field.fields} />}
					</Stack>
				</li>
			))}
		</ul>
	)
}

/**
 * Renders the dialog showing the fields of a group a plugin declared, with nothing to change.
 * @param props - The group.
 * @returns The trigger and the dialog it opens.
 */
export function PluginFieldsDialog(props: { held: FieldGroup; origin: string }) {
	const [open, setOpen] = useState(false)
	return (
		<>
			<Button variant="outline" onClick={() => setOpen(true)}>
				{__('Fields', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={setOpen}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>
							{sprintf(__('Fields of %(group)s', 'gophenberg'), { group: props.held.title })}
						</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Stack direction="column" gap="md">
							<Text variant="body-sm">
								{sprintf(
									__('The %(plugin)s plugin declared these fields, so they change only in its code.', 'gophenberg'),
									{ plugin: props.origin },
								)}
							</Text>
							<FieldList fields={props.held.fields} />
						</Stack>
					</Dialog.Content>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}
