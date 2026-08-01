// SPDX-License-Identifier: Apache-2.0

import { SnackbarProvider } from '@gophenberg/frontend-sdk'
import { parse, registerCuratedBlocks } from '@gophenberg/frontend-sdk/editor'
import type { Block } from '@gophenberg/frontend-sdk/editor'
import { QueryClientProvider, QueryClient } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import { beforeAll, expect, test } from 'vitest'

import { useEditorBuffer } from '../posts/useEditorBuffer'
import { storedPost } from './postFixture'

beforeAll(() => {
	registerCuratedBlocks()
}, 120000)

/**
 * Returns a buffer held over the stored fixture.
 * @returns The rendered hook result.
 */
function renderBuffer() {
	const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
	return renderHook(() => useEditorBuffer(storedPost.id, { ...storedPost, publishedAt: null } as never), {
		wrapper: ({ children }) => (
			<QueryClientProvider client={client}>
				<SnackbarProvider>{children}</SnackbarProvider>
			</QueryClientProvider>
		),
	})
}

/**
 * Returns blocks carrying the given paragraph text.
 * @param text - The words the paragraph holds.
 * @returns The parsed blocks.
 */
function paragraph(text: string): Block[] {
	return parse(`<!-- wp:paragraph -->\n<p>${text}</p>\n<!-- /wp:paragraph -->`)
}

test('opens with no history to walk', () => {
	const { result } = renderBuffer()

	expect(result.current.hasUndo).toBe(false)
	expect(result.current.hasRedo).toBe(false)
})

test('records a committed change as a step it can walk back', () => {
	const { result } = renderBuffer()

	act(() => result.current.onChange(paragraph('Committed words.')))

	expect(result.current.hasUndo).toBe(true)
	expect(result.current.dirty).toBe(true)
})

test('walks a committed change back and forward again', () => {
	const { result } = renderBuffer()
	act(() => result.current.onChange(paragraph('Committed words.')))

	act(() => result.current.undo())

	expect(result.current.dirty).toBe(false)
	expect(result.current.hasRedo).toBe(true)
	act(() => result.current.redo())
	expect(result.current.dirty).toBe(true)
})

test('keeps typing out of the history until it is committed', () => {
	const { result } = renderBuffer()

	act(() => result.current.onInput(paragraph('Typed words.')))

	expect(result.current.dirty).toBe(true)
	expect(result.current.hasUndo).toBe(false)
})
