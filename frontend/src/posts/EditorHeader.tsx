// SPDX-License-Identifier: Apache-2.0

import { Button, IconButton, Stack, Text, redoIcon, undoIcon } from '@gophenberg/frontend-sdk'
import { Link } from '@tanstack/react-router'

import type { EditorBuffer } from './useEditorBuffer'

/**
 * Returns the word the header shows for the state of the buffer.
 * @param dirty - Whether the buffer holds unsaved changes.
 * @param saving - Whether a write is in flight.
 * @returns The state to show.
 */
function stateOf(dirty: boolean, saving: boolean): string {
	if (saving) {
		return 'Saving'
	}
	return dirty ? 'Unsaved changes' : 'Saved'
}

/**
 * Renders the editor header with its navigation, history and write controls.
 * @param props - The buffer the header drives.
 * @returns The header element.
 */
export function EditorHeader({ buffer }: { buffer: EditorBuffer }) {
	const published = buffer.status === 'published'
	return (
		<Stack direction="row" gap="md" align="center" justify="space-between">
			<Stack direction="row" gap="xs" align="center">
				<Link to="/posts">Back to posts</Link>
				<IconButton
					label="Undo"
					icon={undoIcon}
					disabled={!buffer.hasUndo}
					onClick={buffer.undo}
				/>
				<IconButton
					label="Redo"
					icon={redoIcon}
					disabled={!buffer.hasRedo}
					onClick={buffer.redo}
				/>
			</Stack>
			<Stack direction="row" gap="sm" align="center">
				<Text>{stateOf(buffer.dirty, buffer.saving)}</Text>
				<Button variant="outline" disabled={!buffer.dirty || buffer.saving} onClick={buffer.save}>
					Save draft
				</Button>
				<Button disabled={buffer.saving} onClick={buffer.publish}>
					{published ? 'Update' : 'Publish'}
				</Button>
			</Stack>
		</Stack>
	)
}
