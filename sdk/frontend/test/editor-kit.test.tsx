// SPDX-License-Identifier: Apache-2.0

import { createBlock, getDefaultBlockName, setDefaultBlockName } from '@wordpress/blocks'
import type { Block } from '@wordpress/blocks'
import { useDispatch } from '@wordpress/data'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { closeSmall, cloudUpload, drawerLeft, drawerRight } from '@wordpress/icons'
import { cleanup, render, screen, waitFor, within } from '@testing-library/react'
import { useEffect } from 'react'
import { beforeAll, expect, onTestFinished, test } from 'vitest'

import * as sdk from '../index'
import { BLOCK_EDITOR_STORE, BlockEditorProvider, BlockToolbar, Button, registerCuratedBlocks } from '../editor'

beforeAll(() => {
	registerCuratedBlocks()
}, 120000)

/**
 * Returns two paragraphs for the toolbar to work over.
 * @returns The blocks.
 */
function paragraphs(): Block[] {
	return [
		createBlock('core/paragraph', { content: 'The first paragraph the toolbar test wrote.' }),
		createBlock('core/paragraph', { content: 'The second paragraph the toolbar test wrote.' }),
	] as Block[]
}

/**
 * Selects a block in the editor around it once mounted.
 * @param props - The block to select, none to leave the selection empty.
 * @returns Nothing.
 */
function Selection({ block }: { block?: Block }) {
	const editor = useDispatch(BLOCK_EDITOR_STORE) as unknown as { selectBlock: (clientId: string) => void }
	useEffect(() => {
		if (block !== undefined) {
			editor.selectBlock((block as Block & { clientId: string }).clientId)
		}
	}, [block, editor])
	return null
}

/**
 * Renders the block toolbar inside an editor holding the blocks.
 * @param blocks - The blocks the editor holds.
 * @param showPlaceholder - Whether the toolbar stands in a placeholder while no block is selected.
 * @param selected - The block to select, none to leave the selection empty.
 */
function renderToolbar(blocks: Block[], showPlaceholder?: boolean, selected?: Block) {
	render(
		<BlockEditorProvider value={blocks} settings={{ hasFixedToolbar: true }}>
			<Selection block={selected} />
			<BlockToolbar hideDragHandle showPlaceholder={showPlaceholder} />
		</BlockEditorProvider>,
	)
}

/**
 * Returns the name of the block toolbar on screen and the names of its buttons.
 * @returns The toolbar's name and its buttons' names, in order.
 */
function toolbarWords() {
	const toolbar = screen.getByRole('toolbar', { name: /./ })
	return {
		name: toolbar.getAttribute('aria-label'),
		buttons: within(toolbar).getAllByRole('button').map((button) => button.getAttribute('aria-label')),
	}
}

test('hands the editor the icons WordPress draws its settings, save and close buttons with', () => {
	expect(sdk.drawerRight).toBe(drawerRight)
	expect(sdk.drawerLeft).toBe(drawerLeft)
	expect(sdk.cloudUpload).toBe(cloudUpload)
	expect(sdk.closeSmall).toBe(closeSmall)
})

test('draws no toolbar while no block is selected, as the block editor does', () => {
	renderToolbar(paragraphs())

	expect(screen.queryByRole('toolbar', { name: 'Block tools' })).toBeNull()
})

test('stands a disabled placeholder in while no block is selected when asked to', () => {
	renderToolbar(paragraphs(), true)

	const toolbar = screen.getByRole('toolbar', { name: 'Block tools' })
	const names = within(toolbar).getAllByRole('button').map((button) => button.getAttribute('aria-label'))

	expect(toolbar).toHaveClass('block-editor-block-contextual-toolbar', 'is-placeholder', 'is-unstyled')
	expect(names).toEqual(['Paragraph', 'Move up', 'Move down', 'Options'])
	for (const button of within(toolbar).getAllByRole('button')) {
		expect(button).toHaveAttribute('aria-disabled', 'true')
	}
})

test('stands the generic block in when no default block is registered', () => {
	const held = getDefaultBlockName()
	onTestFinished(() => setDefaultBlockName(held as string))
	setDefaultBlockName('gophenberg/unregistered')

	renderToolbar(paragraphs(), true)

	expect(within(screen.getByRole('toolbar', { name: 'Block tools' })).getByRole('button', { name: 'Block' }))
		.toHaveAttribute('aria-disabled', 'true')
})

test('draws the block editor toolbar once a block is selected', async () => {
	const blocks = paragraphs()

	renderToolbar(blocks, true, blocks[1])

	await waitFor(() =>
		expect(screen.getByRole('toolbar', { name: 'Block tools' })).not.toHaveClass('is-placeholder'),
	)
	const toolbar = screen.getByRole('toolbar', { name: 'Block tools' })
	expect(within(toolbar).getByRole('button', { name: 'Move up' })).not.toHaveAttribute('aria-disabled', 'true')
	expect(within(toolbar).getByRole('button', { name: 'Options' })).not.toHaveAttribute('aria-disabled', 'true')
	expect(within(toolbar).queryByRole('button', { name: 'Drag' })).toBeNull()
})

test('names the empty row in the words of the toolbar it stands in for, whatever Gophenberg translates', async () => {
	setLocaleData(
		{ 'Block tools': ['Herramientas'], 'Move up': ['Subir'], 'Move down': ['Bajar'], Options: ['Opciones'] },
		'gophenberg',
	)
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	const blocks = paragraphs()
	renderToolbar(blocks, true)
	const empty = toolbarWords()
	cleanup()

	renderToolbar(blocks, true, blocks[1])
	await waitFor(() => expect(screen.getByRole('toolbar', { name: /./ })).not.toHaveClass('is-placeholder'))
	const live = toolbarWords()

	expect(empty.name).toBe(live.name)
	expect(live.buttons).toEqual(expect.arrayContaining(empty.buttons))
})

test('hands plugins the button the WordPress editor draws, red on an action that removes something', () => {
	render(
		<Button variant="secondary" isDestructive __next40pxDefaultSize>
			Move to trash
		</Button>,
	)

	expect(screen.getByRole('button', { name: 'Move to trash' })).toHaveClass(
		'components-button',
		'is-secondary',
		'is-destructive',
		'is-next-40px-default-size',
	)
})
