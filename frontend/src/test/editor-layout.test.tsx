// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen } from '@testing-library/react'
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

/**
 * Returns the editor element carrying the given chrome class.
 * @param part - The chrome part to look for.
 * @returns The element.
 */
function chrome(part: string): HTMLElement {
	return document.querySelector(`.gophenberg-editor__${part}`) as HTMLElement
}

test('stands the editor up as a column filling the screen', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const editor = document.querySelector('.gophenberg-editor') as HTMLElement

	expect(getComputedStyle(editor).flexDirection).toBe('column')
	expect(getComputedStyle(editor).height).toBe('100vh')
})

test('holds the header to one row under a rule', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const header = chrome('header')

	expect(getComputedStyle(header).height).toBe('46px')
	expect(getComputedStyle(header).display).toBe('flex')
	expect(getComputedStyle(header).borderBottomWidth).toBe('1px')
})

test('gives the writing area the room the sidebar does not take', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	expect(getComputedStyle(chrome('content')).flexGrow).toBe('1')
	expect(getComputedStyle(chrome('content')).minWidth).toBe('0px')
})

test('holds the sidebar to its own column beside the writing area', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const sidebar = chrome('sidebar')

	expect(getComputedStyle(sidebar).flexBasis).toBe('280px')
	expect(getComputedStyle(sidebar).flexGrow).toBe('0')
	expect(getComputedStyle(sidebar).borderLeftWidth).toBe('1px')
})

test('lets the main area fill what the header leaves', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const main = chrome('main')

	expect(getComputedStyle(main).display).toBe('flex')
	expect(getComputedStyle(main).flexGrow).toBe('1')
	expect(getComputedStyle(main).minHeight).toBe('0px')
})

test('writes the title over the column the blocks sit in', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const title = chrome('title')

	expect(getComputedStyle(title).maxWidth).toBe('700px')
	expect(getComputedStyle(title).marginLeft).toBe('auto')
})

test('leaves the block toolbar to float by its block', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	expect(chrome('toolbar')).toBeNull()
	expect(chrome('header').querySelector('.block-editor-block-toolbar')).toBeNull()
})
