// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { gate } from './gate'
import { renderAt, renderRoutedAt } from './render'
import { warmPostsScreen } from './warm'

warmPostsScreen()

const EVERY_STATUS_BUT_TRASH = 'draft,pending,private,scheduled,published'

const DRAFT = {
	id: '019fb000-0000-7000-8000-000000000002',
	type: 'post',
	slug: 'notes',
	title: 'Notes on the Next Release',
	excerpt: '',
	status: 'draft',
	author_id: '019fb000-0000-7000-8000-0000000000ff',
	author_name: 'Maria Perez',
	published_at: null,
	created_at: '2026-07-19T10:00:00Z',
	updated_at: '2026-07-28T09:00:00Z',
}

const TRASHED = { ...DRAFT, id: '019fb000-0000-7000-8000-000000000003', title: 'Old Notes', status: 'trash' }

const asked: URLSearchParams[] = []

/**
 * Returns the status each listing request asked for so far.
 * @returns One entry per request, empty when it named none.
 */
function askedStatuses(): string[] {
	return asked.map((search) => search.get('status') ?? '')
}

beforeEach(() => {
	asked.length = 0
	server.use(
		http.get('/api/content', ({ request }) => {
			const search = new URL(request.url).searchParams
			asked.push(search)
			const items = search.get('status') === 'trash' ? [TRASHED] : [DRAFT]
			return HttpResponse.json({ items, total: items.length, per_page: 20 })
		}),
	)
})

/**
 * Returns the tab of the status filter carrying the label.
 * @param label - The label of the tab.
 * @returns The tab link.
 */
function tab(label: string): HTMLElement {
	return within(screen.getByRole('navigation', { name: 'Filter by status' })).getByRole('link', { name: label })
}

test('shows the six status tabs with no counts, marking only the current one', async () => {
	renderAt('/content/post')

	const tabs = within(await screen.findByRole('navigation', { name: 'Filter by status' })).getAllByRole('link')

	expect(tabs.map((link) => link.textContent)).toEqual(['All', 'Published', 'Draft', 'Pending', 'Private', 'Trash'])
	expect(tabs.map((link) => link.getAttribute('aria-current'))).toEqual(['page', null, null, null, null, null])
})

test('names the six status tabs in the words of WordPress es_ES for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/content/post')

	const tabs = within(await screen.findByRole('navigation', { name: 'Filtrar por estado' })).getAllByRole('link')

	expect(tabs.map((link) => link.textContent)).toEqual([
		'Todo',
		'Publicada',
		'Borrador',
		'Pendiente',
		'Privada',
		'Papelera',
	])
})

test('marks the tab the address names as the current one', async () => {
	renderAt('/content/post?status=draft&page=2')

	await screen.findByRole('navigation', { name: 'Filter by status' })

	expect(tab('Draft')).toHaveAttribute('aria-current', 'page')
	expect(tab('All')).not.toHaveAttribute('aria-current')
})

test('drops the page and the search and keeps the sort, the filters and the page size on a tab', async () => {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [DRAFT], total: 145, per_page: 50 })))
	const filters = encodeURIComponent(JSON.stringify([{ field: 'field.on-sale', operator: 'is', value: 'true' }]))
	const kept = `sort=title&order=asc&perPage=50&filters=${filters}`
	const { router } = renderRoutedAt(`/content/post?page=2&search=notes&${kept}`)
	await screen.findByRole('navigation', { name: 'Filter by status' })

	await userEvent.click(tab('Draft'))

	await waitFor(() =>
		expect(router.state.location.search).toEqual({
			status: 'draft',
			sort: 'title',
			order: 'asc',
			perPage: 50,
			filters: [{ field: 'field.on-sale', operator: 'is', value: 'true' }],
		}),
	)
})

test('asks All for every status but the trash', async () => {
	renderAt('/content/post')

	await screen.findByText('Notes on the Next Release')

	expect(askedStatuses()).toEqual([EVERY_STATUS_BUT_TRASH])
})

test('asks a tab for its own status', async () => {
	renderAt('/content/post')
	await screen.findByText('Notes on the Next Release')

	await userEvent.click(tab('Trash'))

	expect(await screen.findByText('Old Notes')).toBeInTheDocument()
	expect(askedStatuses().at(-1)).toBe('trash')
})

test('asks the Pending and the Private tabs for their own status', async () => {
	renderAt('/content/post')
	await screen.findByText('Notes on the Next Release')

	await userEvent.click(tab('Pending'))
	await waitFor(() => expect(askedStatuses().at(-1)).toBe('pending'))
	await userEvent.click(tab('Private'))

	await waitFor(() => expect(askedStatuses().at(-1)).toBe('private'))
})

test('reads a status no tab names as All and asks the server for every status but the trash', async () => {
	renderAt('/content/post?status=archived')

	await screen.findByText('Notes on the Next Release')

	expect(tab('All')).toHaveAttribute('aria-current', 'page')
	expect(askedStatuses()).toEqual([EVERY_STATUS_BUT_TRASH])
})

test('clears the ticks once a pasted address shows another status', async () => {
	const { router } = renderRoutedAt('/content/post')
	await userEvent.click(await screen.findByRole('checkbox', { name: 'Notes on the Next Release' }))
	expect(screen.getByText('1 Item selected')).toBeInTheDocument()

	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'post' }, search: { status: 'trash' } })
	})
	await screen.findByText('Old Notes')
	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'post' } })
	})

	await screen.findByText('Notes on the Next Release')
	expect(screen.getByRole('checkbox', { name: 'Notes on the Next Release' })).not.toBeChecked()
})

test('clears the ticks once Back returns to another status', async () => {
	const { router } = renderRoutedAt('/content/post?status=trash')
	await screen.findByText('Old Notes')
	await userEvent.click(tab('All'))
	await userEvent.click(await screen.findByRole('checkbox', { name: 'Notes on the Next Release' }))

	await act(async () => {
		router.history.back()
	})
	await screen.findByText('Old Notes')
	await act(async () => {
		router.history.forward()
	})

	await screen.findByText('Notes on the Next Release')
	expect(screen.getByRole('checkbox', { name: 'Notes on the Next Release' })).not.toBeChecked()
})

/**
 * Trashes the listed draft through its row actions, the server refusing it.
 */
async function refuseTheTrash() {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.delete('/api/content/:id', () => HttpResponse.json({}, { status: 500 })))
	await userEvent.click(await screen.findByRole('button', { name: 'Actions' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Trash…' }))
	await userEvent.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Trash' }))
	await screen.findByText('The item could not be moved to the trash.')
}

test('forgets a failure once Back shows another status', async () => {
	const { router } = renderRoutedAt('/content/post?status=trash')
	await screen.findByText('Old Notes')
	await userEvent.click(tab('All'))
	await refuseTheTrash()

	await act(async () => {
		router.history.back()
	})

	await screen.findByText('Old Notes')
	expect(screen.queryByText('The item could not be moved to the trash.')).not.toBeInTheDocument()
})

test('forgets a failure once a pasted address shows another status', async () => {
	const { router } = renderRoutedAt('/content/post')
	await refuseTheTrash()

	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'post' }, search: { status: 'trash' } })
	})

	await screen.findByText('Old Notes')
	expect(screen.queryByText('The item could not be moved to the trash.')).not.toBeInTheDocument()
})

test('shows the status column on All and not on Draft, even after a sort', async () => {
	renderAt('/content/post')
	await screen.findByText('Notes on the Next Release')
	expect(screen.getByRole('columnheader', { name: 'Status' })).toBeInTheDocument()

	await userEvent.click(tab('Draft'))
	await waitFor(() => expect(tab('Draft')).toHaveAttribute('aria-current', 'page'))
	await screen.findByText('Notes on the Next Release')
	await userEvent.click(screen.getByRole('button', { name: 'View options' }))
	await userEvent.selectOptions(await screen.findByRole('combobox', { name: 'Sort by' }), 'title')

	await waitFor(() => expect(askedStatuses().at(-1)).toBe('draft'))
	await waitFor(() => expect(asked.at(-1)?.get('orderby')).toBe('title'))
	expect(screen.queryByRole('columnheader', { name: 'Status' })).not.toBeInTheDocument()
})

/**
 * Holds the next listing answer until the returned gate opens, answering it with the given rows.
 * @param items - The rows the held answer carries.
 * @param total - The number of matches the held answer reports.
 * @returns The gate holding the answer.
 */
function holdNextListing(items: unknown[], total: number) {
	const held = gate()
	server.use(
		http.get('/api/content', async () => {
			await held.held
			return HttpResponse.json({ items, total, per_page: 20 })
		}),
	)
	return held
}

test('offers no Empty Trash from the rows of All while an empty trash loads', async () => {
	renderAt('/content/post')
	await screen.findByText('Notes on the Next Release')
	const trash = holdNextListing([], 0)

	await userEvent.click(tab('Trash'))

	await waitFor(() => expect(tab('Trash')).toHaveAttribute('aria-current', 'page'))
	expect(screen.queryByRole('button', { name: 'Empty Trash' })).not.toBeInTheDocument()
	trash.release()
	expect(await screen.findByText('No items found.')).toBeInTheDocument()
})

test('keeps Empty Trash while the next page of the trash loads', async () => {
	server.use(http.get('/api/content', () => HttpResponse.json({ items: [TRASHED], total: 45, per_page: 20 })))
	renderAt('/content/post?status=trash')
	await screen.findByRole('button', { name: 'Empty Trash' })
	const older = { ...TRASHED, id: '019fb000-0000-7000-8000-000000000009', title: 'Older Notes' }
	const next = holdNextListing([older], 45)

	await userEvent.click(screen.getByRole('button', { name: 'Next page' }))

	expect(screen.getByRole('button', { name: 'Empty Trash' })).toBeInTheDocument()
	next.release()
	expect(await screen.findByText('Older Notes')).toBeInTheDocument()
})

test('offers no Empty Trash from the trash of another type while an empty trash loads', async () => {
	const page = { key: 'page', singular_label: 'Page', plural_label: 'Pages', description: '', route_word: 'pages' }
	const post = { key: 'post', singular_label: 'Post', plural_label: 'Posts', description: '', route_word: '' }
	const versions = { revisions: true, revision_cap: 100 }
	const shared = { ...versions, hierarchical: false, page_kind: 'single', active: true, fields: [] }
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [{ ...post, ...shared, default: true }, { ...page, ...shared, default: false }] }),
		),
	)
	const { router } = renderRoutedAt('/content/post?status=trash')
	await screen.findByRole('button', { name: 'Empty Trash' })
	const pages = holdNextListing([], 0)

	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'page' }, search: { status: 'trash' } })
	})

	expect(await screen.findByRole('heading', { level: 1, name: 'Pages' })).toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Empty Trash' })).not.toBeInTheDocument()
	pages.release()
	expect(await screen.findByText('No items found.')).toBeInTheDocument()
})

test('never shows the rows of All under Trash while the trash loads', async () => {
	renderAt('/content/post')
	await screen.findByText('Notes on the Next Release')
	const trash = gate()
	server.use(
		http.get('/api/content', async () => {
			await trash.held
			return HttpResponse.json({ items: [TRASHED], total: 1, per_page: 20 })
		}),
	)

	await userEvent.click(tab('Trash'))

	await waitFor(() => expect(tab('Trash')).toHaveAttribute('aria-current', 'page'))
	expect(screen.queryByText('Notes on the Next Release')).not.toBeInTheDocument()
	trash.release()
	expect(await screen.findByText('Old Notes')).toBeInTheDocument()
})
