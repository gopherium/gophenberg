// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import '../content/editor.css'
import { chosenPreview } from '../content/EditorHeader'
import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)))
})

test('keeps the block list out of the way until it is asked for', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	expect(screen.getByRole('button', { name: 'List view' })).toBeInTheDocument()
	expect(document.querySelector('.gophenberg-editor__outline')).toBeNull()
})

test('opens the block list beside the canvas', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	await userEvent.click(screen.getByRole('button', { name: 'List view' }))

	const outline = document.querySelector('.gophenberg-editor__outline') as HTMLElement
	expect(within(outline).getByRole('treegrid')).toBeInTheDocument()
	expect(within(outline).getByText('Paragraph')).toBeInTheDocument()
})

test('closes the block list when it is asked for again', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.click(screen.getByRole('button', { name: 'List view' }))

	await userEvent.click(screen.getByRole('button', { name: 'List view' }))

	expect(document.querySelector('.gophenberg-editor__outline')).toBeNull()
})

test('marks the list view control as pressed while it is open', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })
	const control = screen.getByRole('button', { name: 'List view' })

	await userEvent.click(control)

	expect(screen.getByRole('button', { name: 'List view' })).toHaveAttribute('aria-pressed', 'true')
})

test('opens the canvas at the width of a desktop', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	const canvas = document.querySelector('.gophenberg-editor__canvas') as HTMLElement

	expect(canvas).not.toHaveClass('gophenberg-editor__canvas--tablet')
	expect(canvas).not.toHaveClass('gophenberg-editor__canvas--mobile')
})

test('narrows the canvas to the width asked for', async () => {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })

	await userEvent.click(screen.getByRole('combobox', { name: 'Preview' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Mobile' }))

	const canvas = document.querySelector('.gophenberg-editor__canvas') as HTMLElement
	expect(canvas).toHaveClass('gophenberg-editor__canvas--mobile')
})

test('keeps the preview width when the select reports nothing', () => {
	expect(chosenPreview(null, 'tablet')).toBe('tablet')
	expect(chosenPreview({ value: null }, 'tablet')).toBe('tablet')
})

test('gives each preview width a size to render at', () => {
	const styles = Array.from(document.styleSheets)
		.flatMap((sheet) => Array.from(sheet.cssRules ?? []))
		.map((rule) => rule.cssText)
		.join('\n')

	expect(styles).toMatch(/canvas--tablet[^}]*max-width/)
	expect(styles).toMatch(/canvas--mobile[^}]*max-width/)
})
