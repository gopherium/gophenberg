// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const NESTING_TYPE = {
	key: 'page',
	singular_label: 'Page',
	plural_label: 'Pages',
	route_word: 'pages',
	hierarchical: true,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: false,
	active: true,
}

const FLAT_TYPE = { ...NESTING_TYPE, key: 'post', plural_label: 'Posts', hierarchical: false }

const ABOUT = {
	...storedPost,
	id: '019fb000-0000-7000-8000-0000000000aa',
	type: 'page',
	slug: 'about',
	path: 'pages/about',
	title: 'About',
}

const EDITED = { ...storedPost, type: 'page', slug: 'team', path: 'pages/team', title: 'Team' }

/**
 * Serves the editor over a hierarchical type holding one other page.
 * @param registered - The types the registry answers with.
 * @returns The bodies the editor patched.
 */
function serveNesting(registered: unknown[]): unknown[] {
	const patched: unknown[] = []
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: registered })),
		http.get(`/api/content/${EDITED.id}`, () => HttpResponse.json(EDITED)),
		http.get(`/api/content/${EDITED.id}/autosave`, () => HttpResponse.json({}, { status: 404 })),
		http.get('/api/content', () => HttpResponse.json({ items: [ABOUT, EDITED], total: 2 })),
		http.patch(`/api/content/${EDITED.id}`, async ({ request }) => {
			patched.push(await request.json())
			return HttpResponse.json(EDITED)
		}),
	)
	return patched
}

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	serveNesting([NESTING_TYPE, FLAT_TYPE])
})

test('offers a parent for a type that nests', async () => {
	renderAt(`/content/page/${EDITED.id}/edit`)

	expect(await screen.findByLabelText('Parent')).toBeInTheDocument()
})

test('leaves the parent out for a type that does not nest', async () => {
	serveNesting([FLAT_TYPE])
	renderAt(`/content/post/${EDITED.id}/edit`)

	await screen.findByRole('textbox', { name: 'Slug' })

	expect(screen.queryByLabelText('Parent')).not.toBeInTheDocument()
})

test('offers every other item of the type as a parent', async () => {
	renderAt(`/content/page/${EDITED.id}/edit`)

	const picker = await screen.findByLabelText('Parent')
	await userEvent.click(picker)

	expect(await screen.findByRole('option', { name: 'About' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Team' })).not.toBeInTheDocument()
})

test('files the item under the parent it was given', async () => {
	const patched = serveNesting([NESTING_TYPE, FLAT_TYPE])
	renderAt(`/content/page/${EDITED.id}/edit`)

	const picker = await screen.findByLabelText('Parent')
	await userEvent.click(picker)
	await userEvent.click(await screen.findByRole('option', { name: 'About' }))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	expect(patched.at(-1)).toMatchObject({ parent_id: ABOUT.id })
})
