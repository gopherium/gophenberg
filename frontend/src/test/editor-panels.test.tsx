// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/posts/${storedPost.id}/edit`

const patched: Record<string, unknown>[] = []

beforeAll(async () => {
	await import('../posts/editorRoute.lazy')
}, 120000)

beforeEach(() => {
	patched.length = 0
	server.use(
		http.get(`/api/posts/${storedPost.id}`, () => HttpResponse.json(storedPost)),
		http.patch(`/api/posts/${storedPost.id}`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			patched.push(body)
			return HttpResponse.json({ ...storedPost, ...body })
		}),
	)
})

test('offers the slug and the excerpt in the document tab', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('textbox', { name: 'Slug' })).toHaveValue('welcome')
	expect(screen.getByRole('textbox', { name: 'Excerpt' })).toBeInTheDocument()
})

test('offers the statuses a post can be authored into', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('combobox', { name: 'Status' }))

	expect(await screen.findByRole('option', { name: 'Draft' })).toBeInTheDocument()
	expect(screen.getByRole('option', { name: 'Pending' })).toBeInTheDocument()
	expect(screen.getByRole('option', { name: 'Private' })).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'Published' })).not.toBeInTheDocument()
})

test('keeps published in the select once the post holds it', async () => {
	server.use(
		http.get(`/api/posts/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, status: 'published' }),
		),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('combobox', { name: 'Status' }))

	expect(await screen.findByRole('option', { name: 'Published' })).toBeInTheDocument()
})

test('sends the status the author chose with the save', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('combobox', { name: 'Status' }))

	await userEvent.click(await screen.findByRole('option', { name: 'Pending' }))
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(patched).toHaveLength(1))
	expect(patched[0]).toMatchObject({ status: 'pending' })
})

test('holds a panel edit back until the post is saved', async () => {
	renderAt(EDITOR_PATH)
	const slug = await screen.findByRole('textbox', { name: 'Slug' })

	await userEvent.type(slug, '-again')

	expect(patched).toEqual([])
	expect(screen.getByRole('button', { name: 'Save draft' })).toHaveAttribute(
		'aria-disabled',
		'false',
	)
})

test('sends the edited slug and excerpt with the save', async () => {
	renderAt(EDITOR_PATH)
	const slug = await screen.findByRole('textbox', { name: 'Slug' })
	await userEvent.type(slug, '-again')
	await userEvent.type(screen.getByRole('textbox', { name: 'Excerpt' }), 'A short summary.')

	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(patched).toHaveLength(1))
	expect(patched[0]).toMatchObject({ slug: 'welcome-again', excerpt: 'A short summary.' })
})

test('shows the slug the server settled on', async () => {
	server.use(
		http.patch(`/api/posts/${storedPost.id}`, async ({ request }) => {
			patched.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, slug: 'welcome-again-2' })
		}),
	)
	renderAt(EDITOR_PATH)
	const slug = await screen.findByRole('textbox', { name: 'Slug' })
	await userEvent.type(slug, '-again')

	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(screen.getByRole('textbox', { name: 'Slug' })).toHaveValue('welcome-again-2'))
})
