// SPDX-License-Identifier: Apache-2.0

import { Button, Stack, Text } from '@gophenberg/frontend-sdk'

/**
 * Returns the word the header shows for the state of the buffer.
 * @param dirty - Whether the buffer holds unsaved changes.
 * @param saving - Whether a save is in flight.
 * @returns The state to show.
 */
function stateOf(dirty: boolean, saving: boolean): string {
	if (saving) {
		return 'Saving'
	}
	return dirty ? 'Unsaved changes' : 'Saved'
}

/**
 * Renders the editor header with the save control and the state of the buffer.
 * @param props - The buffer state and the save handler.
 * @returns The header element.
 */
export function EditorHeader({
	dirty,
	saving,
	onSave,
}: {
	dirty: boolean
	saving: boolean
	onSave: () => void
}) {
	return (
		<Stack direction="row" gap="md" align="center" justify="space-between">
			<Text>{stateOf(dirty, saving)}</Text>
			<Button disabled={!dirty || saving} onClick={onSave}>
				Save draft
			</Button>
		</Stack>
	)
}
