// SPDX-License-Identifier: Apache-2.0

import {
	BlockCanvas,
	BlockEditorProvider,
	BlockList,
	CANVAS_STYLES,
	ShortcutProvider,
	SlotFillProvider,
	registerCuratedBlocks,
} from '@gophenberg/frontend-sdk/editor'
import { useState } from 'react'

import { EDITOR_SETTINGS, SPIKE_BLOCKS } from './editorSetup'

registerCuratedBlocks()

/**
 * Renders the post editor.
 * @returns The editor screen element.
 */
export function EditorScreen() {
	const [blocks, setBlocks] = useState<unknown[]>(SPIKE_BLOCKS)
	return (
		<SlotFillProvider>
			<ShortcutProvider>
				<BlockEditorProvider
					value={blocks}
					onInput={setBlocks}
					onChange={setBlocks}
					settings={EDITOR_SETTINGS}
				>
					<BlockCanvas height="100vh" styles={CANVAS_STYLES}>
						<BlockList />
					</BlockCanvas>
				</BlockEditorProvider>
			</ShortcutProvider>
		</SlotFillProvider>
	)
}
