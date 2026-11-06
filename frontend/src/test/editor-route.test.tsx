// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { adminUser, renderAt, renderRoutedAt } from './render'
import { storedPost, storedPostWithId } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const OTHER_POST = { ...storedPostWithId('019fb000-0000-7000-8000-000000000002'), title: 'Another post' }

const author = { ...adminUser, role: 'author', capabilities: [] }

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)))
})

test('the editor route carries none of the admin chrome', async () => {
	renderAt(EDITOR_PATH)

	await screen.findByTitle('Editor canvas')

	expect(screen.queryByRole('region', { name: 'Navigation' })).not.toBeInTheDocument()
	expect(screen.queryByRole('heading', { name: 'Gophenberg' })).not.toBeInTheDocument()
	expect(document.querySelector('.godmin-layout')).toBeNull()
	expect(document.querySelector('.godmin-layout__canvas')).toBeNull()
})

test('framed routes still render the admin chrome', async () => {
	renderAt('/')

	expect(await screen.findByRole('region', { name: 'Navigation' })).toBeInTheDocument()
	expect(document.querySelector('.godmin-layout')).not.toBeNull()
})

/**
 * Returns the menu row the admin sidebar draws for posts, once the registry answers.
 * @returns The Posts link in the Navigation region.
 */
async function postsMenuRow(): Promise<HTMLElement> {
	const menu = await screen.findByRole('region', { name: 'Navigation' })
	return within(menu).findByRole('link', { name: 'Posts' })
}

test('draws the reading view of a trashed post in the admin frame, beside the menu row of its type', async () => {
	server.use(http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json({ ...storedPost, status: 'trash' })))
	renderAt(EDITOR_PATH)

	const words = await screen.findByText(/This item is in the trash/)

	expect(await postsMenuRow()).toHaveAttribute('aria-current', 'page')
	expect(screen.getByRole('main')).toContainElement(words)
})

test('draws the reading view of a post another account wrote in the admin frame', async () => {
	renderAt(EDITOR_PATH, author)

	const words = await screen.findByText(/another account wrote this/i)

	expect(await postsMenuRow()).toHaveAttribute('aria-current', 'page')
	expect(screen.getByRole('main')).toContainElement(words)
})

test('names a reading view with its only second level heading, beside the sidebar', async () => {
	renderAt(EDITOR_PATH, author)

	const main = await screen.findByRole('main')

	expect(await within(main).findByRole('heading', { level: 2 })).toHaveTextContent(storedPost.title)
	expect(within(main).queryAllByRole('heading', { level: 1 })).toHaveLength(0)
})

test('reports a post that could not load in the admin frame', async () => {
	server.use(http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json({}, { status: 404 })))
	renderAt(EDITOR_PATH)

	const words = await screen.findByText('Could not load that post.')

	expect(await postsMenuRow()).toBeInTheDocument()
	expect(screen.getByRole('main')).toContainElement(words)
})

/**
 * Returns the Title field of the editor on screen.
 * @returns The Title textbox.
 */
function titleField(): HTMLElement {
	return screen.getByRole('textbox', { name: 'Title' })
}

test('going back to a post shows that post and not the one just left', async () => {
	server.use(
		http.get(`/api/content/${OTHER_POST.id}`, () => HttpResponse.json(OTHER_POST)),
		http.get('/api/content/:id/autosave', () => HttpResponse.json({}, { status: 404 })),
	)
	const { router } = renderRoutedAt(EDITOR_PATH)
	await screen.findByRole('textbox', { name: 'Title' })
	await waitFor(() => expect(titleField()).toHaveValue(storedPost.title))

	await act(async () => {
		await router.navigate({
			to: '/content/$typeKey/$postId/edit',
			params: { typeKey: 'post', postId: OTHER_POST.id },
		})
	})
	await waitFor(() => expect(titleField()).toHaveValue(OTHER_POST.title))

	await act(async () => {
		router.history.back()
	})
	await waitFor(() => expect(router.state.location.pathname).toContain(storedPost.id))

	await waitFor(() => expect(titleField()).toHaveValue(storedPost.title))
})
