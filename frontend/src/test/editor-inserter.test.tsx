// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
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

test('offers a way to add a block from the header', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const add = screen.getByRole('button', { name: 'Add block' })

	expect(document.querySelector('.gophenberg-editor__header')?.contains(add)).toBe(true)
})

test('opens the block library on it', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	await userEvent.click(screen.getByRole('button', { name: 'Add block' }))

	expect(await screen.findByRole('searchbox', { name: /search/i })).toBeInTheDocument()
})

test('offers the curated blocks in the library', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	await userEvent.click(screen.getByRole('button', { name: 'Add block' }))
	await screen.findByRole('searchbox', { name: /search/i })

	expect(await screen.findByRole('option', { name: 'Heading' })).toBeInTheDocument()
})

test('writes the title as a headline rather than a labelled field', async () => {
	renderAt(EDITOR_PATH)

	const title = await screen.findByRole('textbox', { name: 'Title' })

	expect(getComputedStyle(title).fontSize).toBe('32px')
})


test('names the title field without printing a label over it', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const printed = document.querySelector('.gophenberg-editor__title label')

	expect(printed === null || getComputedStyle(printed).position === 'absolute').toBe(true)
})
