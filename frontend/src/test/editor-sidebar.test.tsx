// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)))
})

test('offers a document tab and a block tab', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('tab', { name: 'Document' })).toBeInTheDocument()
	expect(screen.getByRole('tab', { name: 'Block' })).toBeInTheDocument()
})

test('opens on the document tab', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('tab', { name: 'Document' })).toHaveAttribute(
		'aria-selected',
		'true',
	)
})

test('tells the document tab what the post is', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('tab', { name: 'Document' })

	expect(screen.getByRole('combobox', { name: 'Status' })).toHaveTextContent('Draft')
})

test('turns to the block tab when asked', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('tab', { name: 'Block' }))

	expect(screen.getByRole('tab', { name: 'Block' })).toHaveAttribute('aria-selected', 'true')
	expect(screen.getByRole('tab', { name: 'Document' })).toHaveAttribute('aria-selected', 'false')
})

test('says which block is selected when none is', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('tab', { name: 'Block' }))

	expect(await screen.findByText(/no block selected/i)).toBeInTheDocument()
})

test('shows a status it holds no label for as the server named it', async () => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, status: 'scheduled' }),
		),
	)
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('combobox', { name: 'Status' })).toHaveTextContent('scheduled')
})

test('shows a published post as published in the document tab', async () => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({
				...storedPost,
				status: 'published',
				published_at: '2026-07-20T10:00:00Z',
			}),
		),
	)
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('combobox', { name: 'Status' })).toHaveTextContent('Published')
})
