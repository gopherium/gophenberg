// SPDX-License-Identifier: Apache-2.0

import {
	Button,
	IconButton,
	Stack,
	Text,
	backIcon,
	redoIcon,
	undoIcon,
} from '@gophenberg/frontend-sdk'
import { Inserter } from '@gophenberg/frontend-sdk/editor'
import { Link } from '@tanstack/react-router'

import { DocumentBar } from './DocumentBar'
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
 * @param props - The buffer the header drives and the type of post it holds.
 * @returns The header element.
 */
export function EditorHeader({ buffer, type }: { buffer: EditorBuffer, type: string }) {
	const published = buffer.status === 'published'
	return (
		<div className="gophenberg-editor__header">
			<Stack direction="row" gap="xs" align="center">
				<IconButton label="Back to posts" icon={backIcon} render={<Link to="/posts" />} />
				<Inserter position="bottom right" toggleProps={{ label: 'Add block' }} />
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
			<DocumentBar title={buffer.title} type={type} />
			<Stack direction="row" gap="sm" align="center">
				<Text>{stateOf(buffer.dirty, buffer.saving)}</Text>
				<Button variant="outline" disabled={!buffer.dirty || buffer.saving} onClick={buffer.save}>
					Save draft
				</Button>
				<Button disabled={buffer.saving} onClick={buffer.publish}>
					{published ? 'Update' : 'Publish'}
				</Button>
			</Stack>
		</div>
	)
}
