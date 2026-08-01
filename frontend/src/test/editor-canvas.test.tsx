// SPDX-License-Identifier: Apache-2.0

import { apiFetchAttempts } from '@gophenberg/frontend-sdk/editor'
import { screen } from '@testing-library/react'
import { beforeAll, expect, test } from 'vitest'

import { renderAt } from './render'

const EDITOR_PATH = '/posts/019fb000-0000-7000-8000-000000000001/edit'

beforeAll(async () => {
	await import('../posts/editorRoute.lazy')
}, 120000)

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

	expect(document.querySelector('.gophenberg-layout')).toBeNull()
	expect(document.querySelector('.gophenberg-layout__canvas')).toBeNull()
})
