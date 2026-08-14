// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import '../content/editor.css'
import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)))
})

test('names the post and its type in the header', async () => {
	renderAt(EDITOR_PATH)

	const bar = await screen.findByTestId('document-bar')

	expect(bar).toHaveTextContent('Welcome to Gophenberg')
	await waitFor(() => expect(bar).toHaveTextContent('Post'))
})

test('calls an untitled post untitled in the bar', async () => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, title: '' }),
		),
	)
	renderAt(EDITOR_PATH)

	expect(await screen.findByTestId('document-bar')).toHaveTextContent('No title')
})

test('names a registered type by the label the registry carries', async () => {
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({
				items: [
					{
						key: 'guide',
						singular_label: 'Guide',
						plural_label: 'Guides',
						route_word: 'guides',
						hierarchical: false,
						revisions: true,
						revision_cap: 100,
						page_kind: 'single',
						default: false,
						active: true,
					},
				],
			}),
		),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, type: 'guide' }),
		),
	)
	renderAt(`/content/guide/${storedPost.id}/edit`)

	const bar = await screen.findByTestId('document-bar')

	await waitFor(() => expect(bar).toHaveTextContent('Guide'))
})

test('shows a type it holds no label for as the server named it', async () => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, type: 'briefing' }),
		),
	)
	renderAt(EDITOR_PATH)

	expect(await screen.findByTestId('document-bar')).toHaveTextContent('briefing')
})

test('centres the bar between the header controls', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByTestId('document-bar')

	const bar = document.querySelector('.gophenberg-editor__document') as HTMLElement
	const header = document.querySelector('.gophenberg-editor__header') as HTMLElement

	expect(getComputedStyle(header).position).toBe('relative')
	expect(getComputedStyle(bar).position).toBe('absolute')
	expect(getComputedStyle(bar).left).toBe('50%')
	expect(getComputedStyle(bar).transform).toBe('translateX(-50%)')
})

test('foots the editor with the trail to the selected block', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const foot = document.querySelector('.gophenberg-editor__foot')

	expect(foot).not.toBeNull()
	expect(document.querySelector('.gophenberg-editor')?.lastElementChild).toBe(foot)
})
