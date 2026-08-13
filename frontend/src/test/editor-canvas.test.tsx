// SPDX-License-Identifier: Apache-2.0

import { apiFetchAttempts } from '@gophenberg/frontend-sdk/editor'
import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen } from '@testing-library/react'
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

test('mounts the block canvas in an iframe', async () => {
	renderAt(EDITOR_PATH)

	const canvas = await screen.findByTitle('Editor canvas')

	expect(canvas.tagName).toBe('IFRAME')
})

test('mounts the canvas the block editor drives, not a bare frame', async () => {
	renderAt(EDITOR_PATH)

	const canvas = await screen.findByTitle('Editor canvas')

	expect(canvas).toHaveAttribute('name', 'editor-canvas')
	expect(canvas.getAttribute('src')).toMatch(/^blob:/)
})

test('reaches for no WordPress REST endpoint while the canvas renders', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByTitle('Editor canvas')

	expect(apiFetchAttempts()).toEqual([])
})

test('carries none of the admin chrome', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByTitle('Editor canvas')

	expect(document.querySelector('.godmin-layout')).toBeNull()
	expect(document.querySelector('.godmin-layout__canvas')).toBeNull()
})
