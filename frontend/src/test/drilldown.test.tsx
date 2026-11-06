// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { setViewport } from '@gopherium/godmin/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { gate } from './gate'
import { renderAt, renderRoutedAt } from './render'
import { warmPostsScreen } from './warm'

vi.mock('../plugins', async () => ({ plugins: [(await import('./sectionPlugin')).sectionPlugin] }))

warmPostsScreen()

/** The class of the wrapper a sidebar screen slides in. */
const WRAPPER = 'godmin-navigation__screen-wrapper'

/** The page type, which a test makes the default. */
const pageType = {
	key: 'page',
	singular_label: 'Page',
	plural_label: 'Pages',
	description: '',
	route_word: 'pages',
	hierarchical: true,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	fields: [],
}

beforeEach(() => {
	server.use(
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
	)
})

/**
 * Returns the sidebar region.
 * @returns The region named Navigation.
 */
function sidebar(): Promise<HTMLElement> {
	return screen.findByRole('region', { name: 'Navigation' })
}

/**
 * Returns the section row once the type rows above it have arrived, so the menu holds still.
 * @param where - Where the row sits, the sidebar when absent.
 * @returns The section row link.
 */
async function sectionRow(where?: HTMLElement): Promise<HTMLElement> {
	const holder = where ?? (await sidebar())
	await within(holder).findByRole('link', { name: 'Posts' })
	return within(holder).getByRole('link', { name: 'Section' })
}

/**
 * Opens the section sub menu from its row in the sidebar.
 * @param where - Where the row sits, the sidebar when absent.
 */
async function openSection(where?: HTMLElement) {
	await userEvent.click(await sectionRow(where))
}

test('swaps the sidebar for the section screen on a section route', async () => {
	renderAt('/section')

	const shown = await sidebar()
	expect(await within(shown).findByText('section sidebar')).toBeInTheDocument()
	expect(screen.getByText('section canvas')).toBeInTheDocument()
	expect(within(shown).queryByRole('link', { name: 'Users' })).toBeNull()
})

test('shows the main menu, not a section screen, at the list the root opens', async () => {
	renderAt('/content/post')

	expect(await sectionRow()).toBeInTheDocument()
	expect(within(await sidebar()).queryByText('section sidebar')).toBeNull()
})

test('slides the section in from its row and puts focus on Back', async () => {
	renderAt('/content/post')

	await openSection()

	const back = await within(await sidebar()).findByRole('link', { name: 'Back' })
	expect(within(await sidebar()).getByRole('heading', { level: 1, name: 'Section' })).toBeInTheDocument()
	await waitFor(() => expect(back).toHaveFocus())
	expect(back.closest(`.${WRAPPER}`)).toHaveClass(`${WRAPPER}--forward`)
})

test('slides the main menu back in with focus on the section row', async () => {
	const { router } = renderRoutedAt('/content/post')
	await openSection()

	await userEvent.click(await within(await sidebar()).findByRole('link', { name: 'Back' }))

	const row = await within(await sidebar()).findByRole('link', { name: 'Section' })
	await waitFor(() => expect(row).toHaveFocus())
	expect(row.closest(`.${WRAPPER}`)).toHaveClass(`${WRAPPER}--backward`)
	expect(router.state.location.pathname).toBe('/content/post')
})

test('sends Back to the list of the default type once the types answer', async () => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [pageType] })))
	const { router } = renderRoutedAt('/section')

	await userEvent.click(await within(await sidebar()).findByRole('link', { name: 'Back' }))

	await waitFor(() => expect(router.state.location.pathname).toBe('/content/page'))
	expect(await screen.findByRole('heading', { level: 2, name: 'Pages' })).toBeInTheDocument()
})

test('sends Back to the root while the types load, which opens the default list once they answer', async () => {
	const registry = gate()
	server.use(
		http.get('/api/types', async () => {
			await registry.held
			return HttpResponse.json({ items: [pageType] })
		}),
	)
	const { router } = renderRoutedAt('/section')

	await userEvent.click(await within(await sidebar()).findByRole('link', { name: 'Back' }))

	await waitFor(() => expect(router.state.location.pathname).toBe('/'))
	expect(await screen.findByRole('status')).toHaveTextContent('Loading the screen.')
	registry.release()
	await waitFor(() => expect(router.state.location.pathname).toBe('/content/page'))
})

test('sends Back to the root after the types fail, which reports the failure', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/types', () => HttpResponse.json({}, { status: 500 })))
	const { router } = renderRoutedAt('/section')

	await userEvent.click(await within(await sidebar()).findByRole('link', { name: 'Back' }))

	expect(await screen.findByRole('alert')).toHaveTextContent('The content types could not be loaded.')
	expect(router.state.location.pathname).toBe('/')
})

test('keeps the drawer open on a phone going into the section and coming back', async () => {
	setViewport({ matches: true })
	const { router } = renderRoutedAt('/content/post')
	await userEvent.click(await screen.findByRole('button', { name: 'Open navigation panel' }))
	const drawer = await screen.findByRole('dialog')

	await openSection(drawer)
	await within(drawer).findByRole('heading', { level: 1, name: 'Section' })
	await userEvent.click(within(drawer).getByRole('link', { name: 'Back' }))

	expect(await within(drawer).findByRole('link', { name: 'Section' })).toBeInTheDocument()
	await waitFor(() => expect(router.state.location.pathname).toBe('/content/post'))
	expect(screen.getByRole('dialog')).toBe(drawer)
})
