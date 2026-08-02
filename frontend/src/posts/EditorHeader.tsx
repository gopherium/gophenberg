// SPDX-License-Identifier: Apache-2.0

import {
	Button,
	IconButton,
	SelectControl,
	Stack,
	Text,
	backIcon,
	listViewIcon,
	redoIcon,
	undoIcon,
} from '@gophenberg/frontend-sdk'
import { Inserter } from '@gophenberg/frontend-sdk/editor'
import { Link } from '@tanstack/react-router'

import { DocumentBar } from './DocumentBar'
import { PREVIEW_WIDTHS, previewItem } from './editorSetup'
import type { PreviewWidth } from './editorSetup'
import type { EditorBuffer } from './useEditorBuffer'
import type { EditorViews } from './useEditorViews'

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
 * Returns the preview width a select change asks for.
 * @param item - The item the select reported, or nothing.
 * @param current - The width the canvas renders at.
 * @returns The width to render at.
 */
export function chosenPreview(
	item: { value: string | null } | null,
	current: PreviewWidth,
): PreviewWidth {
	if (item === null || item.value === null) {
		return current
	}
	return item.value as PreviewWidth
}

/**
 * Renders the editor header with its navigation, history and write controls.
 * @param props - The buffer and views the header drives and the type of post it holds.
 * @returns The header element.
 */
export function EditorHeader(
	{ buffer, type, views }: { buffer: EditorBuffer, type: string, views: EditorViews },
) {
	const published = buffer.status === 'published'
	return (
		<div className="gophenberg-editor__header">
			<Stack direction="row" gap="xs" align="center">
				<IconButton
					label="Back to posts"
					icon={backIcon}
					variant="minimal"
					tone="neutral"
					render={<Link to="/posts" />}
				/>
				<Inserter position="bottom right" toggleProps={{ label: 'Add block' }} />
				<IconButton
					label="Undo"
					icon={undoIcon}
					variant="minimal"
					tone="neutral"
					disabled={!buffer.hasUndo}
					onClick={buffer.undo}
				/>
				<IconButton
					label="Redo"
					icon={redoIcon}
					variant="minimal"
					tone="neutral"
					disabled={!buffer.hasRedo}
					onClick={buffer.redo}
				/>
				<IconButton
					label="List view"
					icon={listViewIcon}
					variant="minimal"
					tone="neutral"
					aria-pressed={views.listOpen}
					onClick={views.toggleList}
				/>
			</Stack>
			<DocumentBar title={buffer.title} type={type} />
			<Stack direction="row" gap="sm" align="center">
				<Text>{stateOf(buffer.dirty, buffer.saving)}</Text>
				<SelectControl
					label="Preview"
					hideLabelFromVision
					size="compact"
					items={PREVIEW_WIDTHS}
					value={previewItem(views.preview)}
					onValueChange={(item) => views.setPreview(chosenPreview(item, views.preview))}
				/>
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
