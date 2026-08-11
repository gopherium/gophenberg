// SPDX-License-Identifier: Apache-2.0

import { render, screen } from '@testing-library/react'
import { applyFilters } from '@wordpress/hooks'
import { expect, test, vi } from 'vitest'

import { BLOCK_EDITOR_STORE, registerMediaCategories, registerMediaLibrary } from '../editor'
import type { InserterMediaCategory, MediaLibraryProps } from '../editor'

/**
 * Renders the words a stand in library was handed.
 * @param props - The props the block editor passes a library.
 * @returns The stand in element.
 */
function StandIn({ render: renderTrigger }: MediaLibraryProps) {
	return <>{renderTrigger({ open: () => {} })}</>
}

test('hands the block editor the library component it was given', () => {
	registerMediaLibrary(StandIn)

	const Registered = applyFilters('editor.MediaUpload', () => null) as typeof StandIn
	render(<Registered onSelect={() => {}} render={({ open }) => <button onClick={open}>Pick</button>} />)

	expect(screen.getByRole('button', { name: 'Pick' })).toBeInTheDocument()
})

test('names the store the block editor registers itself under', () => {
	expect(BLOCK_EDITOR_STORE).toBe('core/block-editor')
})

test('registers a media category the block editor accepts whole', () => {
	const complained = vi.spyOn(console, 'error').mockImplementation(() => {})
	const category: InserterMediaCategory = {
		name: 'gophenberg-test-images',
		labels: { name: 'Test images' },
		mediaType: 'image',
		fetch: () => Promise.resolve([]),
	}

	registerMediaCategories([category])

	expect(complained).not.toHaveBeenCalled()
	complained.mockRestore()
})

test('the block editor refuses a category that is missing what it needs', () => {
	const complained = vi.spyOn(console, 'error').mockImplementation(() => {})

	registerMediaCategories([
		{ name: 'gophenberg-broken', mediaType: 'image' } as unknown as InserterMediaCategory,
	])

	expect(complained).toHaveBeenCalled()
	complained.mockRestore()
})
