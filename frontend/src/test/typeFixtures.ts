// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { fireEvent, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, expect } from 'vitest'

/** The built-in type a new site opens on, as the registry answers it. */
export const POST_TYPE = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	description: 'Manage the posts on this site.',
	route_word: '',
	derived_route_word: 'posts',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
	fields: [],
}

/** The built-in type that nests, registered a day after the posts. */
export const PAGE_TYPE = {
	...POST_TYPE,
	key: 'page',
	singular_label: 'Page',
	plural_label: 'Pages',
	description: 'Manage the pages on this site.',
	route_word: 'pages',
	derived_route_word: 'pages',
	hierarchical: true,
	default: false,
	created_at: '2026-08-02T10:00:00Z',
	updated_at: '2026-08-02T10:00:00Z',
}

/** A type the site registered last, holding no description. */
export const GUIDE_TYPE = {
	...POST_TYPE,
	key: 'guide',
	singular_label: 'Guide',
	plural_label: 'Guides',
	description: '',
	route_word: 'guides',
	derived_route_word: 'guides',
	default: false,
	created_at: '2026-09-01T10:00:00Z',
	updated_at: '2026-09-01T10:00:00Z',
}

/** The cell a row shows the name of its type in, or anything inside it. */
const TITLE_CELL = '.dataviews-title-field, .dataviews-title-field *'

/** Loads the types screen before a file renders its lazily mounted route. */
export function warmTypesScreen() {
	beforeAll(async () => {
		await import('../content/TypesScreen')
	})
}

/**
 * Serves a registry holding the given types.
 * @param rows - The types as the registry answers them.
 */
export function serveTypes(...rows: Record<string, unknown>[]) {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: rows })))
}

/** What a reader of one language finds the list under. */
export interface Reader {
	/** The name the list region is known by. */
	region: string
}

/** How an English reader finds the list. */
const ENGLISH: Reader = { region: 'Content Types' }

/** How a Spanish reader finds the list. */
export const SPANISH: Reader = { region: 'Tipos de contenido' }

/**
 * Returns the queries of the list region once it shows.
 * @param reader - How the reader finds the list, an English one unless a test names another.
 * @returns The queries bound to the region.
 */
export async function typesList(reader = ENGLISH) {
	return within(await screen.findByRole('region', { name: reader.region }))
}

/**
 * Returns the queries of the row a type is listed in, found by the name it shows.
 * @param name - The name the row shows, as the reader reads it.
 * @param reader - How the reader finds the list, an English one unless a test names another.
 * @returns The queries bound to the row.
 */
export async function rowOf(name: string, reader = ENGLISH) {
	const list = await typesList(reader)
	const title = await list.findByText(name, { exact: true, selector: TITLE_CELL })
	return within(title.closest('tr') as HTMLElement)
}

/**
 * Returns the toast showing the given words, once it shows.
 * @param words - The words of the toast.
 * @returns The toast.
 */
export function findToast(words: string | RegExp): Promise<HTMLElement> {
	return screen.findByText(words, { selector: '.godmin-toast' })
}

/**
 * Returns the toast showing the given words beside its action, such as Undo, once it shows.
 * @param words - The words of the toast.
 * @returns The toast, its action inside.
 */
export async function findActionToast(words: string | RegExp): Promise<HTMLElement> {
	const message = await screen.findByText(words, { selector: '.godmin-toast__message' })
	return message.closest('.godmin-toast') as HTMLElement
}

/**
 * Waits until a button of a dialog shows the spinner of a write it runs.
 * @param dialog - The dialog.
 * @param name - The words of the button.
 */
export async function waitBusy(dialog: HTMLElement, name: string) {
	await waitFor(() => expect(within(dialog).getByRole('button', { name }).className).toMatch(/__is-loading\b/))
}

/**
 * Opens the actions menu of the row a type is listed in.
 * @param name - The name the row shows.
 * @param reader - How the reader finds the list, an English one unless a test names another.
 */
export async function openRowMenu(name: string, reader = ENGLISH) {
	const row = await rowOf(name, reader)
	await userEvent.click(row.getByRole('button', { name: 'Actions' }))
	await screen.findByRole('menu')
}

/**
 * Returns the words of every item the open actions menu holds, in order.
 * @returns The labels.
 */
export function menuItems(): (string | null)[] {
	return screen.getAllByRole('menuitem').map((item) => item.textContent)
}

/**
 * Picks an item of the actions menu of a row and returns the dialog it opens.
 * @param name - The name the row shows.
 * @param item - The words of the menu item.
 * @param reader - How the reader finds the list, an English one unless a test names another.
 * @returns The dialog.
 */
export async function openRowDialog(name: string, item: string, reader = ENGLISH): Promise<HTMLElement> {
	await openRowMenu(name, reader)
	await userEvent.click(await screen.findByRole('menuitem', { name: item }))
	return screen.findByRole('dialog')
}

/**
 * Closes the open dialog by a press on the dimmed page around it, the way out that works while its buttons are busy.
 */
export async function clickDimmedPage() {
	const overlay = document.querySelector('.components-modal__screen-overlay') as HTMLElement
	fireEvent.pointerDown(overlay, { button: 0 })
	fireEvent.pointerUp(overlay, { button: 0 })
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
}

/**
 * Records the body of every edit the registry is asked for on a type, answering each with the type as it was asked.
 * @param stored - The type as the registry answers it.
 * @returns The bodies, filled as requests arrive.
 */
export function recordEdits(stored: Record<string, unknown>): Record<string, unknown>[] {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.patch(`/api/types/${String(stored.key)}`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			sent.push(body)
			return HttpResponse.json({ ...stored, ...body })
		}),
	)
	return sent
}

/**
 * Refuses every edit of a type with the given reason.
 * @param stored - The type whose edits are refused, as the registry answers it.
 * @param error - The reason the registry gives.
 */
export function refuseEdits(stored: Record<string, unknown>, error = 'content: the type still holds content') {
	server.use(http.patch(`/api/types/${String(stored.key)}`, () => HttpResponse.json({ error }, { status: 422 })))
}
