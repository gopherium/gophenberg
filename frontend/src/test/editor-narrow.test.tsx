// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { setViewport } from '@gopherium/godmin/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import '../content/editor.css'
import { renderAt } from './render'
import { storedPost } from './postFixture'
import { findActionToast } from './typeFixtures'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	setViewport({ matches: true })
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

/**
 * Renders the editor and waits for its title.
 * @returns Nothing once the editor stands.
 */
async function openEditor() {
	renderAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })
}

/**
 * Holds every save the editor sends until released, then answers it as stored.
 * @returns The function releasing the held saves.
 */
function holdSaves(): () => void {
	let release: () => void = () => {}
	const held = new Promise<void>((resolve) => {
		release = resolve
	})
	server.use(
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			const written = (await request.json()) as Record<string, unknown>
			await held
			return HttpResponse.json({ ...storedPost, ...written, updated_at: '2026-07-28T10:00:00Z' })
		}),
	)
	return release
}

test('fixes the editor to the screen so the page never grows past it', async () => {
	await openEditor()

	const editor = document.querySelector('.gophenberg-editor') as HTMLElement

	expect(editor).toHaveClass('gophenberg-editor--narrow')
	expect(getComputedStyle(editor).position).toBe('fixed')
	expect(getComputedStyle(editor).overflow).toBe('hidden')
})

test('keeps the settings closed until they are asked for', async () => {
	await openEditor()

	expect(screen.getByRole('button', { name: 'Settings' })).toHaveAttribute('aria-expanded', 'false')
	expect(screen.queryByRole('combobox', { name: 'Status' })).toBeNull()
})

test('opens the settings over the writing area at the full width of the screen', async () => {
	await openEditor()

	await userEvent.click(screen.getByRole('button', { name: 'Settings' }))

	const sidebar = chrome('sidebar')
	expect(screen.getByRole('combobox', { name: 'Status' })).toBeInTheDocument()
	expect(getComputedStyle(chrome('main')).position).toBe('relative')
	expect(getComputedStyle(sidebar).position).toBe('absolute')
	expect(getComputedStyle(sidebar).width).toBe('100%')
	expect(getComputedStyle(sidebar).zIndex).toBe('100000')
	expect(getComputedStyle(sidebar).borderTopWidth).toBe('1px')
})

test('opens the list view over the writing area at the full width of the screen', async () => {
	await openEditor()

	await userEvent.click(screen.getByRole('button', { name: 'List view' }))

	const outline = chrome('outline')
	expect(getComputedStyle(outline).position).toBe('absolute')
	expect(getComputedStyle(outline).width).toBe('100%')
	expect(getComputedStyle(outline).zIndex).toBe('100000')
	expect(getComputedStyle(outline).borderTopWidth).toBe('1px')
})

test('keeps the way back, the inserter, the list view, the save, the settings and publish in the header', async () => {
	await openEditor()

	const header = within(chrome('header'))

	expect(await header.findByRole('link', { name: 'Back to posts' })).toBeInTheDocument()
	expect(header.getByRole('button', { name: 'Add block' })).toBeInTheDocument()
	expect(header.getByRole('button', { name: 'List view' })).toBeInTheDocument()
	expect(header.getByRole('button', { name: 'Saved' })).toBeInTheDocument()
	expect(header.getByRole('button', { name: 'Settings' })).toBeInTheDocument()
	expect(header.getByRole('button', { name: 'Publish' })).toBeInTheDocument()
})

test('leaves undo, redo, the preview select and the state word out of the header', async () => {
	await openEditor()

	const header = within(chrome('header'))

	expect(header.queryByRole('button', { name: 'Undo' })).toBeNull()
	expect(header.queryByRole('button', { name: 'Redo' })).toBeNull()
	expect(header.queryByRole('combobox', { name: 'Preview' })).toBeNull()
	expect(header.queryByText('Saved')).toBeNull()
	expect(header.queryByText('Unsaved changes')).toBeNull()
})

test('saves through an icon named after the state it shows', async () => {
	const release = holdSaves()
	await openEditor()
	const saved = screen.getByRole('button', { name: 'Saved' })
	expect(saved).toHaveAttribute('aria-disabled', 'true')
	expect(saved).toHaveTextContent('')
	expect(saved).not.toHaveClass('components-animate__loading')

	await userEvent.type(screen.getByRole('textbox', { name: 'Title' }), '!')
	const save = screen.getByRole('button', { name: 'Save draft' })
	expect(save).not.toHaveAttribute('aria-disabled', 'true')
	await userEvent.click(save)

	const saving = await screen.findByRole('button', { name: 'Saving' })
	expect(saving).toHaveAttribute('aria-disabled', 'true')
	expect(saving).toHaveClass('components-animate__loading')
	release()
	expect(await screen.findByRole('button', { name: 'Saved' })).not.toHaveClass('components-animate__loading')
})

test('names the save button after its state from 600 to 781, pulsing while it saves as WordPress does', async () => {
	setViewport({ width: 700 })
	const release = holdSaves()
	await openEditor()
	const saved = screen.getByRole('button', { name: 'Saved' })
	expect(saved).toHaveTextContent('Saved')
	expect(saved).toHaveAttribute('aria-disabled', 'true')

	await userEvent.type(screen.getByRole('textbox', { name: 'Title' }), '!')
	const save = screen.getByRole('button', { name: 'Save draft' })
	expect(save).toHaveTextContent('Save draft')
	await userEvent.click(save)

	const saving = await screen.findByRole('button', { name: 'Saving' })
	expect(saving).toHaveTextContent('Saving')
	expect(saving).toHaveClass('components-animate__loading')
	expect(saving.className).not.toMatch(/is-loading/)
	release()
	expect(await screen.findByRole('button', { name: 'Saved' })).toHaveTextContent('Saved')
})

test('offers no save on a post that is already published', async () => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json({ ...storedPost, status: 'published' })),
	)
	await openEditor()

	expect(screen.getByRole('button', { name: 'Update' })).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Saved' })).toBeNull()
	expect(screen.queryByRole('button', { name: 'Save draft' })).toBeNull()
})

test('leaves the document name out and gives the actions the room', async () => {
	await openEditor()

	expect(screen.queryByTestId('document-bar')).toBeNull()
	expect(getComputedStyle(chrome('header')).gridTemplateColumns).toBe('max-content minmax(min-content, 1fr)')
})

test('squeezes the document name between the header groups while it shows', async () => {
	setViewport({ width: 500 })
	await openEditor()

	expect(screen.getByTestId('document-bar')).toBeInTheDocument()
	expect(getComputedStyle(chrome('header')).gridTemplateColumns).toBe('min-content minmax(0, 1fr) min-content')
})

test('keeps the words of every header control on one line', async () => {
	await openEditor()

	const publish = screen.getByRole('button', { name: 'Publish' })

	expect(getComputedStyle(publish).whiteSpace).toBe('nowrap')
	expect(getComputedStyle(publish).flexShrink).toBe('0')
})

test('leaves the block trail out', async () => {
	await openEditor()

	expect(chrome('foot')).toBeNull()
})

test('holds a disabled block toolbar above the title before any block is selected', async () => {
	await openEditor()

	const toolbar = within(chrome('content')).getByRole('toolbar', { name: 'Block tools' })
	const title = screen.getByRole('textbox', { name: 'Title' })

	expect(toolbar.compareDocumentPosition(title) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
	for (const name of ['Paragraph', 'Move up', 'Move down', 'Options']) {
		expect(within(toolbar).getByRole('button', { name })).toHaveAttribute('aria-disabled', 'true')
	}
})

test('fills the toolbar row with the block tools once a paragraph is selected', async () => {
	await openEditor()
	await userEvent.click(screen.getByRole('button', { name: 'List view' }))

	await userEvent.click(within(chrome('outline')).getByRole('link', { name: /Paragraph/ }))

	await waitFor(() =>
		expect(within(chrome('content')).getByRole('toolbar', { name: 'Block tools' })).not.toHaveClass('is-placeholder'),
	)
	const toolbar = within(chrome('content')).getByRole('toolbar', { name: 'Block tools' })
	expect(within(toolbar).getByRole('button', { name: 'Options' })).not.toHaveAttribute('aria-disabled', 'true')
	expect(within(toolbar).queryByRole('button', { name: 'Drag' })).toBeNull()
})

test('leaves the drag handle out of the toolbar row, as WordPress does on a phone', async () => {
	const second = '<!-- wp:paragraph -->\n<p>More stored words.</p>\n<!-- /wp:paragraph -->'
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, content: `${storedPost.content}\n\n${second}` }),
		),
	)
	await openEditor()
	await userEvent.click(screen.getByRole('button', { name: 'List view' }))

	await userEvent.click(within(chrome('outline')).getAllByRole('link', { name: /Paragraph/ })[1])

	await waitFor(() =>
		expect(within(chrome('content')).getByRole('toolbar', { name: 'Block tools' })).not.toHaveClass('is-placeholder'),
	)
	const toolbar = within(chrome('content')).getByRole('toolbar', { name: 'Block tools' })
	expect(within(toolbar).getByRole('button', { name: 'Move up' })).not.toHaveAttribute('aria-disabled', 'true')
	expect(within(toolbar).queryByRole('button', { name: 'Drag' })).toBeNull()
})

test('moves the post to the trash through the settings', async () => {
	server.use(
		http.delete(`/api/content/${storedPost.id}`, () => HttpResponse.json({ ...storedPost, status: 'trash' })),
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
	)
	await openEditor()
	await userEvent.click(screen.getByRole('button', { name: 'Settings' }))

	await userEvent.click(screen.getByRole('button', { name: 'Move to trash' }))

	expect(await findActionToast('"Welcome to Gophenberg" moved to the trash.')).toBeInTheDocument()
})
