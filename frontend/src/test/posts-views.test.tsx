// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import '../index.css'
import { renderAt } from './render'
import { warmPostsScreen } from './warm'

warmPostsScreen()

const PUBLISHED = {
	id: '019fb000-0000-7000-8000-000000000001',
	type: 'post',
	slug: 'welcome',
	title: 'Welcome to Gophenberg',
	excerpt: '',
	status: 'published',
	author_id: '019fb000-0000-7000-8000-0000000000ff',
	author_name: 'Maria Perez',
	published_at: '2026-07-20T10:00:00Z',
	created_at: '2026-07-19T10:00:00Z',
	updated_at: '2026-07-20T10:00:00Z',
}

const COUNTS = { draft: 2, pending: 1, private: 0, published: 3, trash: 1 }

const asked: URL[] = []

/**
 * Serves a page of posts of the given size and records what was asked for.
 * @param total - The number of matches the API reports.
 */
function serveList(total = 1) {
	server.use(
		http.get('/api/content', ({ request }) => {
			asked.push(new URL(request.url))
			return HttpResponse.json({ items: [PUBLISHED], total })
		}),
	)
}

/**
 * Returns the value a parameter carried on every list request so far.
 * @param name - The query parameter to read.
 * @returns One entry per request, empty when the parameter was absent.
 */
function askedFor(name: string): string[] {
	return asked.map((url) => url.searchParams.get(name) ?? '')
}

beforeEach(() => {
	asked.length = 0
	serveList()
	server.use(http.get('/api/content/counts', () => HttpResponse.json(COUNTS)))
})

test('holds the filter row space with chip ghosts until the counts arrive', async () => {
	serveList()
	server.use(http.get('/api/content/counts', () => new Promise(() => {})))
	renderAt('/content/post')
	await screen.findByRole('heading', { level: 1 })

	await waitFor(() =>
		expect(document.querySelectorAll('.gophenberg-status-ghost').length).toBe(5),
	)
	const chip = document.querySelector('.gophenberg-status-ghost') as Element
	expect(chip.closest('[aria-hidden="true"]')).not.toBeNull()
})

test('fades the filter row in when it replaces the chips', async () => {
	renderAt('/content/post')

	const row = await screen.findByRole('group', { name: 'Filter by status' })
	expect([...row.classList]).toContain('godmin-arrival')
})

test('leaves the chips their own pulse without a delay of the row', async () => {
	serveList()
	server.use(http.get('/api/content/counts', () => new Promise(() => {})))
	renderAt('/content/post')
	await screen.findByRole('heading', { level: 1 })

	await waitFor(() => expect(document.querySelector('.gophenberg-status-ghost')).not.toBeNull())
	const chip = document.querySelector('.gophenberg-status-ghost') as Element
	expect(getComputedStyle(chip).animation).toBe('')
	const row = chip.parentElement as Element
	expect(getComputedStyle(row).animation).toBe('')
	expect(getComputedStyle(row).opacity).not.toBe('0')
})

test('counts the posts each status view covers', async () => {
	renderAt('/content/post')

	expect(await screen.findByRole('button', { name: 'All (7)' })).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Published (3)' })).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Draft (2)' })).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Pending (1)' })).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Trash (1)' })).toBeInTheDocument()
})

test('hides the private view while no post is private', async () => {
	renderAt('/content/post')
	await screen.findByRole('button', { name: 'All (7)' })

	expect(screen.queryByRole('button', { name: /^Private/ })).not.toBeInTheDocument()
})

test('shows the private view once a post is private', async () => {
	server.use(
		http.get('/api/content/counts', () => HttpResponse.json({ ...COUNTS, private: 4 })),
	)
	renderAt('/content/post')

	expect(await screen.findByRole('button', { name: 'Private (4)' })).toBeInTheDocument()
})

test('opens on all posts', async () => {
	renderAt('/content/post')

	expect(await screen.findByRole('button', { name: 'All (7)' })).toHaveAttribute(
		'aria-pressed',
		'true',
	)
	expect(askedFor('status')).toEqual([''])
})

test('filters the list by the status view chosen', async () => {
	renderAt('/content/post')

	await userEvent.click(await screen.findByRole('button', { name: 'Draft (2)' }))

	await waitFor(() => expect(askedFor('status')).toContain('draft'))
})

test('marks the status view in force', async () => {
	renderAt('/content/post')

	await userEvent.click(await screen.findByRole('button', { name: 'Draft (2)' }))

	expect(screen.getByRole('button', { name: 'Draft (2)' })).toHaveAttribute('aria-pressed', 'true')
	expect(screen.getByRole('button', { name: 'All (7)' })).toHaveAttribute('aria-pressed', 'false')
})

test('returns to the first page when the status changes', async () => {
	serveList(45)
	renderAt('/content/post')
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
	await waitFor(() => expect(askedFor('page')).toContain('2'))
	await userEvent.click(screen.getByRole('button', { name: 'Draft (2)' }))

	await waitFor(() => expect(askedFor('status')).toContain('draft'))
	expect(asked.at(-1)?.searchParams.get('page')).toBeNull()
})

test('waits for typing to settle before searching', async () => {
	renderAt('/content/post')
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.type(screen.getByRole('searchbox'), 'notes')

	await waitFor(() => expect(askedFor('search')).toContain('notes'))
	expect(askedFor('search').filter((term) => term !== '')).toEqual(['notes'])
})

test('asks for the next page of a long listing', async () => {
	serveList(45)
	renderAt('/content/post')
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'Next page' }))

	await waitFor(() => expect(askedFor('page')).toContain('2'))
})

test('holds the page size fixed', async () => {
	serveList(45)
	renderAt('/content/post')
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	expect(await screen.findByText('Sort by')).toBeInTheDocument()
	expect(screen.queryByText('Items per page')).not.toBeInTheDocument()
	expect(askedFor('per_page')).toEqual(['20'])
})

test('reads a status the counts left out as none', async () => {
	server.use(http.get('/api/content/counts', () => HttpResponse.json({ draft: 2 })))
	renderAt('/content/post')

	expect(await screen.findByRole('button', { name: 'All (2)' })).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Published (0)' })).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: /^Private/ })).not.toBeInTheDocument()
})

test('lists posts even when the counts cannot be read', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/content/counts', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')

	expect(await screen.findByText('Welcome to Gophenberg')).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: /^All/ })).not.toBeInTheDocument()
})

test('drops the filter row when the counts cannot be read', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/content/counts', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content/post')
	await screen.findByText('Welcome to Gophenberg')

	await waitFor(() =>
		expect(document.querySelectorAll('.gophenberg-status-ghost')).toHaveLength(0),
	)
})
