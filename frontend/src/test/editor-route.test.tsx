// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor } from '@testing-library/react'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { renderAt, renderRoutedAt } from './render'
import { storedPost, storedPostWithId } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const OTHER_POST = { ...storedPostWithId('019fb000-0000-7000-8000-000000000002'), title: 'Another post' }

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)))
})

test('the editor route carries none of the admin chrome', async () => {
	renderAt(EDITOR_PATH)

	await screen.findByTitle('Editor canvas')

	expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
	expect(screen.queryByRole('link', { name: 'Gophenberg' })).not.toBeInTheDocument()
	expect(document.querySelector('.godmin-layout')).toBeNull()
	expect(document.querySelector('.godmin-layout__canvas')).toBeNull()
})

test('framed routes still render the admin chrome', async () => {
	renderAt('/')

	expect(await screen.findByRole('navigation')).toBeInTheDocument()
	expect(document.querySelector('.godmin-layout')).not.toBeNull()
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
