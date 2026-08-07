// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import '../index.css'
import { renderAt } from './render'

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
		http.get('/api/posts', ({ request }) => {
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
	server.use(http.get('/api/posts/counts', () => HttpResponse.json(COUNTS)))
})

test('holds the filter row space with chip ghosts until the counts arrive', async () => {
	serveList()
	server.use(http.get('/api/posts/counts', () => new Promise(() => {})))
	renderAt('/posts')
	await screen.findByRole('heading', { level: 1 })

	await waitFor(() =>
		expect(document.querySelectorAll('.gophenberg-status-ghost').length).toBe(5),
	)
	const chip = document.querySelector('.gophenberg-status-ghost') as Element
	expect(chip.closest('[aria-hidden="true"]')).not.toBeNull()
})

test('reserves the filter row without painting the chips right away', async () => {
	serveList()
	server.use(http.get('/api/posts/counts', () => new Promise(() => {})))
	renderAt('/posts')
	await screen.findByRole('heading', { level: 1 })

	await waitFor(() => expect(document.querySelector('.gophenberg-status-row')).not.toBeNull())
	const row = document.querySelector('.gophenberg-status-row') as Element
	const style = getComputedStyle(row)
	expect(style.opacity).toBe('0')
	expect(style.animation).toContain('gophenberg-status-appear')
	expect(style.animation).toContain('200ms')
	expect(style.animation).toContain('forwards')
})

test('leaves the chips their own pulse and delays the row instead', async () => {
	serveList()
	server.use(http.get('/api/posts/counts', () => new Promise(() => {})))
	renderAt('/posts')
	await screen.findByRole('heading', { level: 1 })

	await waitFor(() => expect(document.querySelector('.gophenberg-status-ghost')).not.toBeNull())
	const chip = document.querySelector('.gophenberg-status-ghost') as Element
	expect(getComputedStyle(chip).animation).toBe('')
	const row = chip.parentElement as Element
	expect(getComputedStyle(row).opacity).toBe('0')
	expect(getComputedStyle(row).animation).toContain('gophenberg-status-appear')
})

test('lands the chip animation at full opacity', () => {
	const keyframes: string[] = []
	for (const sheet of document.styleSheets) {
		for (const rule of sheet.cssRules) {
			if (/@keyframes\s+gophenberg-status-appear\s*\{/.test(rule.cssText)) {
				keyframes.push(rule.cssText)
			}
		}
	}

	expect(keyframes).toHaveLength(1)
	expect(keyframes[0]).toMatch(/(?:100%|to)\s*\{\s*opacity:\s*1\s*;?\s*\}/)
})

test('counts the posts each status view covers', async () => {
	renderAt('/posts')

	expect(await screen.findByRole('button', { name: 'All (7)' })).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Published (3)' })).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Draft (2)' })).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Pending (1)' })).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Trash (1)' })).toBeInTheDocument()
})

test('hides the private view while no post is private', async () => {
	renderAt('/posts')
	await screen.findByRole('button', { name: 'All (7)' })

	expect(screen.queryByRole('button', { name: /^Private/ })).not.toBeInTheDocument()
})

test('shows the private view once a post is private', async () => {
	server.use(
		http.get('/api/posts/counts', () => HttpResponse.json({ ...COUNTS, private: 4 })),
	)
	renderAt('/posts')

	expect(await screen.findByRole('button', { name: 'Private (4)' })).toBeInTheDocument()
})

test('opens on all posts', async () => {
	renderAt('/posts')

	expect(await screen.findByRole('button', { name: 'All (7)' })).toHaveAttribute(
		'aria-pressed',
		'true',
	)
	expect(askedFor('status')).toEqual([''])
})

test('filters the list by the status view chosen', async () => {
	renderAt('/posts')

	await userEvent.click(await screen.findByRole('button', { name: 'Draft (2)' }))

	await waitFor(() => expect(askedFor('status')).toContain('draft'))
})

test('marks the status view in force', async () => {
	renderAt('/posts')

	await userEvent.click(await screen.findByRole('button', { name: 'Draft (2)' }))

	expect(screen.getByRole('button', { name: 'Draft (2)' })).toHaveAttribute('aria-pressed', 'true')
	expect(screen.getByRole('button', { name: 'All (7)' })).toHaveAttribute('aria-pressed', 'false')
})

test('returns to the first page when the status changes', async () => {
	serveList(45)
	renderAt('/posts')
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'Next page' }))
	await waitFor(() => expect(askedFor('page')).toContain('2'))
	await userEvent.click(screen.getByRole('button', { name: 'Draft (2)' }))

	await waitFor(() => expect(askedFor('status')).toContain('draft'))
	expect(asked.at(-1)?.searchParams.get('page')).toBeNull()
})

test('waits for typing to settle before searching', async () => {
	renderAt('/posts')
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.type(screen.getByRole('searchbox'), 'notes')

	await waitFor(() => expect(askedFor('search')).toContain('notes'))
	expect(askedFor('search').filter((term) => term !== '')).toEqual(['notes'])
})

test('asks for the next page of a long listing', async () => {
	serveList(45)
	renderAt('/posts')
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'Next page' }))

	await waitFor(() => expect(askedFor('page')).toContain('2'))
})

test('holds the page size fixed', async () => {
	serveList(45)
	renderAt('/posts')
	await screen.findByText('Welcome to Gophenberg')

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	expect(await screen.findByText('Sort by')).toBeInTheDocument()
	expect(screen.queryByText('Items per page')).not.toBeInTheDocument()
	expect(askedFor('per_page')).toEqual(['20'])
})

test('reads a status the counts left out as none', async () => {
	server.use(http.get('/api/posts/counts', () => HttpResponse.json({ draft: 2 })))
	renderAt('/posts')

	expect(await screen.findByRole('button', { name: 'All (2)' })).toBeInTheDocument()
	expect(screen.getByRole('button', { name: 'Published (0)' })).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: /^Private/ })).not.toBeInTheDocument()
})

test('lists posts even when the counts cannot be read', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/posts/counts', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/posts')

	expect(await screen.findByText('Welcome to Gophenberg')).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: /^All/ })).not.toBeInTheDocument()
})

test('drops the filter row when the counts cannot be read', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/posts/counts', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/posts')
	await screen.findByText('Welcome to Gophenberg')

	await waitFor(() =>
		expect(document.querySelectorAll('.gophenberg-status-ghost')).toHaveLength(0),
	)
})
