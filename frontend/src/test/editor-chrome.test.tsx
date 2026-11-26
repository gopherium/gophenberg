// SPDX-License-Identifier: Apache-2.0

import { drawerLeft, drawerRight } from '@gophenberg/frontend-sdk'
import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import type { ReactElement } from 'react'
import { beforeAll, beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const patched: Record<string, unknown>[] = []

/**
 * Returns the outline an icon draws.
 * @param held - The element holding the icon, or the icon itself.
 * @returns The path data of its first path.
 */
function outlineOf(held: Element | ReactElement): string | null {
	const element = held instanceof Element ? held : render(held).container
	return element.querySelector('path')?.getAttribute('d') ?? null
}

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

/**
 * Serves the given post and records every write against it.
 * @param post - The post the server holds.
 */
function serve(post: Record<string, unknown>) {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(post)),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			patched.push(body)
			return HttpResponse.json({ ...post, ...body, published_at: '2026-08-01T10:00:00Z' })
		}),
	)
}

beforeEach(() => {
	patched.length = 0
	serve(storedPost)
})

test('offers a way back to the list', async () => {
	renderAt(EDITOR_PATH)

	const back = await screen.findByRole('link', { name: 'Back to posts' })

	expect(back).toHaveAttribute('href', '/admin/content/post')
})

test('offers undo and redo, both idle over an untouched post', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('button', { name: 'Undo' })).toHaveAttribute(
		'aria-disabled',
		'true',
	)
	expect(screen.getByRole('button', { name: 'Redo' })).toHaveAttribute('aria-disabled', 'true')
})

test('offers to publish a draft', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('button', { name: 'Publish' })).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Update' })).not.toBeInTheDocument()
})

test('publishes a draft carrying what was written with the transition', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Publish' }))

	await waitFor(() => expect(patched).toHaveLength(1))
	expect(patched[0]).toMatchObject({
		status: 'published',
		updated_at: storedPost.updated_at,
		title: storedPost.title,
	})
	expect(patched[0]).toHaveProperty('content')
})

test('swaps the control to update once the post is published', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Publish' }))

	expect(await screen.findByRole('button', { name: 'Update' })).toBeInTheDocument()
})

test('keeps saying update while a published post is being unpublished', async () => {
	serve({ ...storedPost, status: 'published' })
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('combobox', { name: 'Status' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Draft' }))

	expect(screen.getByRole('button', { name: 'Update' })).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Publish' })).not.toBeInTheDocument()
})

test('unpublishes a published post with the status the author chose', async () => {
	serve({ ...storedPost, status: 'published' })
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('combobox', { name: 'Status' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Draft' }))

	await userEvent.click(screen.getByRole('button', { name: 'Update' }))

	await waitFor(() => expect(patched).toHaveLength(1))
	expect(patched[0]).toMatchObject({ status: 'draft' })
})

test('offers no draft save on a post that is already published', async () => {
	serve({ ...storedPost, status: 'published' })
	renderAt(EDITOR_PATH)

	await screen.findByRole('button', { name: 'Update' })

	expect(screen.queryByRole('button', { name: 'Save draft' })).not.toBeInTheDocument()
})

test('announces the publication', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Publish' }))

	expect(await screen.findByText('Post published.')).toBeInTheDocument()
})

test('offers to update a post that is already published', async () => {
	serve({ ...storedPost, status: 'published', published_at: '2026-07-20T10:00:00Z' })
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('button', { name: 'Update' })).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Publish' })).not.toBeInTheDocument()
})

test('reports a publication the server refused', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(
		http.patch(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ error: 'post: invalid transition' }, { status: 422 }),
		),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Publish' }))

	expect(await screen.findByText(/invalid transition/i)).toBeInTheDocument()
})

test('offers the settings right before publish, pressed and expanded while they show', async () => {
	renderAt(EDITOR_PATH)

	const publish = await screen.findByRole('button', { name: 'Publish' })
	const settings = screen.getByRole('button', { name: 'Settings' })

	expect(settings.nextElementSibling).toBe(publish)
	expect(settings).toHaveAttribute('aria-pressed', 'true')
	expect(settings).toHaveAttribute('aria-expanded', 'true')
	expect(document.getElementById(settings.getAttribute('aria-controls') as string)).toBe(
		document.querySelector('.gophenberg-editor__sidebar'),
	)
})

test('hides the settings when asked and says so on the button', async () => {
	renderAt(EDITOR_PATH)
	const settings = await screen.findByRole('button', { name: 'Settings' })

	await userEvent.click(settings)

	expect(settings).toHaveAttribute('aria-pressed', 'false')
	expect(settings).toHaveAttribute('aria-expanded', 'false')
	expect(document.querySelector('.gophenberg-editor__sidebar')).toHaveAttribute('hidden')
	expect(screen.queryByRole('combobox', { name: 'Status' })).toBeNull()
})

test('draws the settings drawer on the right for a left to right reader', async () => {
	renderAt(EDITOR_PATH)

	const settings = await screen.findByRole('button', { name: 'Settings' })

	expect(outlineOf(settings)).toBe(outlineOf(drawerRight))
})

test('draws the settings drawer on the left for a right to left reader', async () => {
	setLocaleData({ 'text direction\u0004ltr': ['rtl'] })
	onTestFinished(() => resetLocaleData({}))
	renderAt(EDITOR_PATH)

	const settings = await screen.findByRole('button', { name: 'Settings' })

	expect(outlineOf(settings)).toBe(outlineOf(drawerLeft))
	expect(outlineOf(drawerLeft)).not.toBe(outlineOf(drawerRight))
})

test('leaves the title its focus ring, which the design system draws around it', async () => {
	renderAt(EDITOR_PATH)

	const title = await screen.findByRole('textbox', { name: 'Title' })

	expect(title.parentElement?.className).toMatch(/outset-ring--focus-within/)
})
