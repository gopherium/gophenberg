// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen } from '@testing-library/react'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import '../posts/editor.css'
import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/posts/${storedPost.id}/edit`

beforeAll(async () => {
	await import('../posts/editorRoute.lazy')
}, 120000)

beforeEach(() => {
	server.use(http.get(`/api/posts/${storedPost.id}`, () => HttpResponse.json(storedPost)))
})

test('names the post and its type in the header', async () => {
	renderAt(EDITOR_PATH)

	const bar = await screen.findByTestId('document-bar')

	expect(bar).toHaveTextContent('Welcome to Gophenberg')
	expect(bar).toHaveTextContent('Post')
})

test('calls an untitled post untitled in the bar', async () => {
	server.use(
		http.get(`/api/posts/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, title: '' }),
		),
	)
	renderAt(EDITOR_PATH)

	expect(await screen.findByTestId('document-bar')).toHaveTextContent('No title')
})

test('shows a type it holds no label for as the server named it', async () => {
	server.use(
		http.get(`/api/posts/${storedPost.id}`, () =>
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
