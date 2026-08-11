// SPDX-License-Identifier: Apache-2.0

import { render, screen, waitFor } from '@testing-library/react'
import { dispatch, select } from '@wordpress/data'
import { beforeAll, expect, test } from 'vitest'

import {
	BlockEditorProvider,
	BlockList,
	ShortcutProvider,
	SlotFillProvider,
	apiFetchAttempts,
	clearApiFetchAttempts,
	installApiFetchGuard,
	parse,
	registerCuratedBlocks,
	serialize,
} from '../editor'

const PLACED_IMAGE =
	'<!-- wp:image {"id":7,"sizeSlug":"large"} -->\n' +
	'<figure class="wp-block-image size-large">' +
	'<img src="/media/2026/08/harbor-1024x640.jpg" alt="Boats at sunrise" class="wp-image-7"/>' +
	'</figure>\n' +
	'<!-- /wp:image -->'

// blockEditorStore names the store the block editor registers itself under.
const blockEditorStore = 'core/block-editor'

interface BlockEditorSelectors {
	isBlockSelected: (clientId: string) => boolean
	getBlocks: () => { attributes: { id?: number } }[]
}

beforeAll(async () => {
	registerCuratedBlocks()
	await Promise.resolve()
}, 120000)

test('keeps a placed image whole while the attachment probe is refused', async () => {
	installApiFetchGuard()
	clearApiFetchAttempts()
	const blocks = parse(PLACED_IMAGE)
	const onDefaultRegistry = { useSubRegistry: false } as object
	render(
		<SlotFillProvider>
			<ShortcutProvider>
				<BlockEditorProvider
					value={blocks}
					onInput={() => {}}
					onChange={() => {}}
					settings={{}}
					{...onDefaultRegistry}
				>
					<BlockList />
				</BlockEditorProvider>
			</ShortcutProvider>
		</SlotFillProvider>,
	)
	await screen.findByAltText('Boats at sunrise')

	const editor = select(blockEditorStore) as unknown as BlockEditorSelectors
	;(dispatch(blockEditorStore) as { selectBlock: (clientId: string) => void }).selectBlock(
		blocks[0].clientId,
	)
	await waitFor(() => {
		expect(editor.isBlockSelected(blocks[0].clientId)).toBe(true)
	})
	await waitFor(() => {
		expect(apiFetchAttempts().length).toBeGreaterThan(0)
	})
	await new Promise((settle) => setTimeout(settle, 50))

	const held = editor.getBlocks()
	expect(held[0].attributes.id).toBe(7)
	expect(serialize(held as Parameters<typeof serialize>[0])).toContain('"id":7')
	expect(screen.getByAltText('Boats at sunrise')).toBeInTheDocument()
})
