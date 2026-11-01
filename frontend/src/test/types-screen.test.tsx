// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { badgeClasses, buttonClasses, setViewport, stubLayout } from '@gopherium/godmin/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import '../index.css'
import { typesQueryKey } from '../content/nav'
import { catalogFor } from '../i18n/catalog'
import { gate } from './gate'
import { holdReads, waitMarked } from './reloads'
import { leaveStale } from './reopen'
import { renderAt, renderRoutedAt } from './render'
import { siteSettings } from './siteSettings'
import {
	GUIDE_TYPE,
	PAGE_TYPE,
	POST_TYPE,
	SPANISH,
	findToast,
	rowOf,
	serveTypes,
	typesList,
	waitBusy,
	warmTypesScreen,
} from './typeFixtures'

warmTypesScreen()

beforeEach(() => {
	serveTypes(POST_TYPE, PAGE_TYPE, GUIDE_TYPE)
})

/**
 * Returns the names the list shows, in the order it shows them.
 * @returns The names of the listed types.
 */
async function listedNames(): Promise<string[]> {
	const region = await screen.findByRole('region', { name: 'Content Types' })
	await waitFor(() => expect(region.querySelector('.dataviews-title-field')).not.toBeNull())
	return [...region.querySelectorAll('.dataviews-title-field')].map((title) => title.textContent ?? '')
}

/**
 * Records every type the registry is asked to register, answering each with the type as it was sent.
 * @returns The bodies sent, filled as requests arrive.
 */
function recordRegistrations(): Record<string, unknown>[] {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.post('/api/types', async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			sent.push(body)
			return HttpResponse.json({ ...GUIDE_TYPE, ...body }, { status: 201 })
		}),
	)
	return sent
}

/**
 * Opens Add Content Type from the list header.
 * @returns The dialog.
 */
async function openAdd(): Promise<HTMLElement> {
	await userEvent.click(await screen.findByRole('button', { name: 'Add Content Type' }))
	return screen.findByRole('dialog', { name: 'Add Content Type' })
}

/**
 * Types the two names of a new type in the open Add Content Type dialog.
 * @param dialog - The dialog.
 * @param singular - The singular name.
 * @param plural - The plural name.
 */
async function nameNewType(dialog: HTMLElement, singular = 'Lesson', plural = 'Lessons') {
	await userEvent.type(within(dialog).getByLabelText('Singular name'), singular)
	await userEvent.type(within(dialog).getByLabelText('Plural name'), plural)
}

test('lists every registered type in a list region with the address it answers under', async () => {
	renderAt('/content-types')

	const pages = await rowOf('Pages')

	expect(pages.getByText('/pages')).toBeInTheDocument()
	expect((await rowOf('Posts')).getByText('/', { exact: true })).toBeInTheDocument()
	expect(screen.getByRole('region', { name: 'Content Types' })).toHaveAttribute('tabindex', '-1')
})

test('shows the types it holds at once when the list opens again, while it reads them anew', async () => {
	const { client, router } = renderRoutedAt('/content-types')
	const names = await listedNames()
	await leaveStale(router, client, typesQueryKey)
	const again = gate()
	let reading = false
	server.use(
		http.get('/api/types', async () => {
			reading = true
			await again.held
			return HttpResponse.json({ items: [POST_TYPE, PAGE_TYPE, { ...GUIDE_TYPE, plural_label: 'Handbooks' }] })
		}),
	)

	await act(async () => {
		await router.navigate({ to: '/content-types' })
	})

	expect(await listedNames()).toEqual(names)
	expect(screen.getByRole('table')).toHaveAttribute('aria-busy', 'false')
	await waitFor(() => expect(reading).toBe(true))
	again.release()
	expect(await (await typesList()).findByText('Handbooks', { exact: true })).toBeInTheDocument()
	expect(await listedNames()).not.toContain('Guides')
})

test('says under the title what the screen holds', async () => {
	renderAt('/content-types')

	expect(await screen.findByText('Every kind of content this site holds.')).toHaveClass('godmin-page__subtitle')
})

test('shows the key of each type under its name, as WordPress shows a description under a title', async () => {
	renderAt('/content-types')

	const pages = await rowOf('Pages')

	expect(pages.getByText('page', { exact: true })).toBeInTheDocument()
})

test('shows the columns Description, Address and Status beside the type, Created left in View options', async () => {
	renderAt('/content-types')
	const list = await typesList()
	await list.findByText('Pages', { exact: true })

	const headers = list.getAllByRole('columnheader').map((header) => header.textContent)

	expect(headers.filter(Boolean).slice(0, 4)).toEqual(['Type', 'Description', 'Address', 'Status'])
	expect(list.queryByRole('columnheader', { name: 'Created' })).not.toBeInTheDocument()
})

test('shows the description each type holds in its own column', async () => {
	renderAt('/content-types')

	expect((await rowOf('Pages')).getByText('Manage the pages on this site.')).toBeInTheDocument()
})

test('draws an em dash a screen reader reads as No description for a type that holds none', async () => {
	renderAt('/content-types')

	const guides = await rowOf('Guides')

	expect(guides.getByText('—')).toHaveAttribute('aria-hidden', 'true')
	expect(guides.getByText('No description')).toBeInTheDocument()
})

test('opens sorted by creation, the newest type first, as WordPress lists open on their newest item', async () => {
	renderAt('/content-types')

	expect(await listedNames()).toEqual(['Guides', 'Pages', 'Posts'])
})

test('sorts by name when the address asks for it', async () => {
	serveTypes(PAGE_TYPE, GUIDE_TYPE, POST_TYPE)
	renderAt('/content-types?sort=title&order=asc')

	expect(await listedNames()).toEqual(['Guides', 'Pages', 'Posts'])
})

test('sorts by address when the address asks for it', async () => {
	renderAt('/content-types?sort=address&order=desc')

	expect(await listedNames()).toEqual(['Pages', 'Guides', 'Posts'])
})

test('keeps the opening order for a sort the list does not offer, whatever the address holds', async () => {
	renderAt('/content-types?sort=status&order=asc')

	expect(await listedNames()).toEqual(['Guides', 'Pages', 'Posts'])
})

/**
 * Shows the Created column through View options.
 */
async function showCreated() {
	await userEvent.click(screen.getByRole('button', { name: 'View options' }))
	const properties = (await screen.findByText('Properties')).closest('.dataviews-field-control') as HTMLElement
	await userEvent.click(within(properties).getByRole('button', { name: 'Created' }))
	await screen.findByRole('columnheader', { name: 'Created' })
}

test('dates a type by its creation once a reader shows the Created column', async () => {
	renderAt('/content-types')
	await rowOf('Pages')

	await showCreated()

	expect((await rowOf('Pages')).getByText('Aug 2, 2026 10:00 am')).toBeInTheDocument()
})

test('dates nothing on a type that carries no creation time', async () => {
	const { created_at: _created, ...undated } = PAGE_TYPE
	serveTypes(POST_TYPE, undated)
	renderAt('/content-types')
	await rowOf('Pages')

	await showCreated()

	expect((await rowOf('Pages')).queryByText(/2026/)).not.toBeInTheDocument()
	expect((await rowOf('Posts')).getByText('Aug 1, 2026 10:00 am')).toBeInTheDocument()
})

test('searches the names of the types', async () => {
	renderAt('/content-types?search=guide')

	expect(await (await typesList()).findByText('Guides', { exact: true })).toBeInTheDocument()
	expect((await typesList()).queryByText('Pages', { exact: true })).not.toBeInTheDocument()
	expect((await typesList()).queryByText('Posts', { exact: true })).not.toBeInTheDocument()
})

test.each([
	{ match: 'its key', search: 'handbook' },
	{ match: 'its description', search: 'steps for new readers' },
])('finds a type by $match', async ({ search }) => {
	serveTypes(POST_TYPE, PAGE_TYPE, { ...GUIDE_TYPE, key: 'handbook', description: 'Steps for new readers.' })
	renderAt(`/content-types?search=${encodeURIComponent(search)}`)

	expect(await (await typesList()).findByText('Guides', { exact: true })).toBeInTheDocument()
	expect((await typesList()).queryByText('Pages', { exact: true })).not.toBeInTheDocument()
})

test('finds a type by its singular name, which the list searches without showing it anywhere', async () => {
	serveTypes(POST_TYPE, PAGE_TYPE, { ...GUIDE_TYPE, key: 'staff', singular_label: 'Person', plural_label: 'People' })
	renderAt('/content-types?search=person')
	const list = await typesList()

	expect(await list.findByText('People', { exact: true })).toBeInTheDocument()
	expect(list.queryByText('Pages', { exact: true })).not.toBeInTheDocument()
	expect(list.queryByText('Person')).not.toBeInTheDocument()
	await userEvent.click(screen.getByRole('button', { name: 'View options' }))
	const properties = (await screen.findByText('Properties')).closest('.dataviews-field-control') as HTMLElement
	expect(within(properties).queryByText('Singular name')).not.toBeInTheDocument()
})

test('names the search after the content types, as the posts list names its own', async () => {
	renderAt('/content-types')

	expect(await screen.findByRole('searchbox', { name: 'Search content types…' })).toBeInTheDocument()
})

test('writes the search a reader types into the address and narrows the list to it', async () => {
	const { router } = renderRoutedAt('/content-types')
	const list = await typesList()
	await rowOf('Pages')

	await userEvent.type(screen.getByRole('searchbox', { name: 'Search content types…' }), 'guide')

	await waitFor(() => expect(router.state.location.search).toMatchObject({ search: 'guide' }))
	await waitFor(() => expect(list.queryByText('Pages', { exact: true })).not.toBeInTheDocument())
})

test('says no results are found when a search matches no content type', async () => {
	renderAt('/content-types?search=nothing')

	expect(await screen.findByText('No results found')).toBeInTheDocument()
})

test('filters the list by status from the address', async () => {
	const nesting = encodeURIComponent('[{"field":"status","operator":"isAny","value":["nests"]}]')
	renderAt(`/content-types?filters=${nesting}`)

	expect(await (await typesList()).findByText('Pages', { exact: true })).toBeInTheDocument()
	expect((await typesList()).queryByText('Posts', { exact: true })).not.toBeInTheDocument()
	expect((await typesList()).queryByText('Guides', { exact: true })).not.toBeInTheDocument()
})

test.each([
	{ status: 'default', shows: 'Posts' },
	{ status: 'closed', shows: 'Guides' },
])('narrows the list to the $status types', async ({ status, shows }) => {
	serveTypes(POST_TYPE, PAGE_TYPE, { ...GUIDE_TYPE, active: false })
	const narrowing = `[{"field":"status","operator":"isAny","value":["${status}"]}]`
	renderAt(`/content-types?filters=${encodeURIComponent(narrowing)}`)

	expect(await (await typesList()).findByText(shows, { exact: true })).toBeInTheDocument()
	expect((await typesList()).getAllByRole('row')).toHaveLength(2)
})

test('marks which type answers at the root', async () => {
	renderAt('/content-types')

	expect((await rowOf('Posts')).getByText('Default')).toBeInTheDocument()
})

test('marks a type that nests', async () => {
	renderAt('/content-types')

	expect((await rowOf('Pages')).getByText('Nests')).toBeInTheDocument()
	expect((await rowOf('Guides')).queryByText('Nests')).not.toBeInTheDocument()
})

test('draws a closed type as an outline badge kept on one line, the WordPress badge of an inactive state', async () => {
	serveTypes(POST_TYPE, { ...PAGE_TYPE, active: false })
	renderAt('/content-types')

	const badge = (await rowOf('Pages')).getByText('Closed')

	expect([...badge.classList]).toEqual([...badgeClasses('none'), 'gophenberg-status'])
	expect(getComputedStyle(badge).whiteSpace).toBe('nowrap')
})

test('keeps the Default badge on one line', async () => {
	renderAt('/content-types')

	const badge = (await rowOf('Posts')).getByText('Default')

	expect([...badge.classList]).toEqual([...badgeClasses(undefined), 'gophenberg-status'])
})

test('names the plugin that declared a type', async () => {
	serveTypes(POST_TYPE, { ...PAGE_TYPE, origin: 'events' })
	renderAt('/content-types')

	expect((await rowOf('Pages')).getByText('From events')).toBeInTheDocument()
})

test('names no plugin on a type the site made', async () => {
	serveTypes(POST_TYPE, { ...PAGE_TYPE, origin: 'events' })
	renderAt('/content-types')

	expect((await rowOf('Posts')).queryByText(/^From/)).not.toBeInTheDocument()
})

test.each([
	{ operator: 'isAny', shown: ['Pages'] },
	{ operator: 'isNone', shown: ['Guides', 'Posts'] },
])('narrows the list by the plugin that declared a type with $operator', async ({ operator, shown }) => {
	serveTypes(POST_TYPE, { ...PAGE_TYPE, origin: 'events' }, GUIDE_TYPE)
	const narrowing = `[{"field":"status","operator":"${operator}","value":["plugin:events"]}]`
	renderAt(`/content-types?filters=${encodeURIComponent(narrowing)}`)

	await waitFor(async () => expect((await listedNames()).toSorted()).toEqual(shown))
})

test.each([
	{ served: 'a plugin type', origin: 'events', choices: ['Default', 'Nests', 'Closed', 'From events'] },
	{ served: 'no plugin type', origin: undefined, choices: ['Default', 'Nests', 'Closed'] },
])('offers the Status filter a choice for each mark the column draws with $served', async ({ origin, choices }) => {
	serveTypes(POST_TYPE, { ...PAGE_TYPE, origin }, GUIDE_TYPE)
	renderAt('/content-types')
	await rowOf('Pages')

	await userEvent.click(screen.getByRole('button', { name: 'Add filter' }))
	await userEvent.click(await screen.findByRole('menuitem', { name: 'Status' }))

	const listed = within(await screen.findByRole('listbox')).getAllByRole('option')
	expect(listed.map((option) => option.textContent)).toEqual(choices)
})

test('draws no action button on a row, leaving every action to its menu', async () => {
	renderAt('/content-types')

	const pages = await rowOf('Pages')

	expect(pages.getAllByRole('button').map((button) => button.getAttribute('aria-label') ?? button.textContent)).toEqual([
		'Actions',
	])
})

test('pages the list by the size the settings name', async () => {
	const paged = { ...siteSettings, list_page_size: 2, list_page_sizes: [2, 10] }
	server.use(http.get('/api/settings', () => HttpResponse.json(paged)))
	renderAt('/content-types')

	expect(await listedNames()).toEqual(['Guides', 'Pages'])
	await userEvent.click(screen.getByRole('button', { name: 'Next page' }))

	await waitFor(async () => expect(await listedNames()).toEqual(['Posts']))
})

test('lists every type on one page and offers no page sizes when the settings could not be read', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/settings', () => HttpResponse.json({}, { status: 500 })))
	renderAt('/content-types')
	expect(await listedNames()).toEqual(['Guides', 'Pages', 'Posts'])

	await userEvent.click(screen.getByRole('button', { name: 'View options' }))

	expect(await screen.findByRole('combobox', { name: 'Sort by' })).toBeInTheDocument()
	expect(screen.queryByRole('radiogroup', { name: 'Items per page' })).not.toBeInTheDocument()
})

test('shows the status beside the name on a phone', async () => {
	setViewport({ matches: true })
	renderAt('/content-types')

	const list = within(await screen.findByRole('grid'))

	expect(await list.findByText('Pages', { exact: true })).toBeInTheDocument()
	expect(list.getByText('Nests')).toBeInTheDocument()
	expect(list.queryByText('/pages')).not.toBeInTheDocument()
	expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()
})

test('announces a registry it could not read', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.get('/api/types', () => HttpResponse.json({ error: 'nope' }, { status: 500 })))
	renderAt('/content-types')

	expect(await screen.findByText('The content types could not be loaded.')).toBeInTheDocument()
	expect(screen.queryByRole('searchbox')).not.toBeInTheDocument()
})

test('names the built-in types in the words of WordPress es_ES for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/content-types')

	const list = await typesList(SPANISH)

	expect(await list.findByText('Entradas', { exact: true })).toBeInTheDocument()
	expect(list.getByText('Páginas', { exact: true })).toBeInTheDocument()
	expect(list.queryByText('Posts', { exact: true })).not.toBeInTheDocument()
	expect(screen.getByRole('searchbox', { name: 'Buscar tipos de contenido…' })).toBeInTheDocument()
})

test('names a type a plugin declared under a built-in key as the plugin named it to a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	serveTypes(POST_TYPE, { ...PAGE_TYPE, origin: 'shop' })
	renderAt('/content-types')

	const list = await typesList(SPANISH)

	expect(await list.findByText('Entradas', { exact: true })).toBeInTheDocument()
	expect(list.getByText('Pages', { exact: true })).toBeInTheDocument()
	expect(list.queryByText('Páginas', { exact: true })).not.toBeInTheDocument()
})

test('closes Add Content Type as its toast shows and marks the list until the registry answers', async () => {
	const created = { ...GUIDE_TYPE, key: 'lesson', plural_label: 'Lessons' }
	server.use(http.post('/api/types', () => HttpResponse.json(created, { status: 201 })))
	renderAt('/content-types')
	const dialog = await openAdd()
	await nameNewType(dialog)
	holdReads('/api/types')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Add' }))

	await screen.findByText('"Lessons" successfully created.', { selector: '.godmin-toast' })
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await waitMarked()
})

test('draws Add Content Type as the compact solid button every list header holds', async () => {
	renderAt('/content-types')

	const add = await screen.findByRole('button', { name: 'Add Content Type' })

	expect([...add.classList]).toEqual(buttonClasses('solid', 'compact'))
})

test('opens Add Content Type as a small dialog titled as WordPress titles Add Pattern, name focused', async () => {
	stubLayout()
	renderAt('/content-types')

	const dialog = await openAdd()

	expect(dialog).toHaveClass('has-size-small')
	expect(within(dialog).getByRole('heading', { name: 'Add Content Type' })).toBeInTheDocument()
	await waitFor(() => expect(within(dialog).getByLabelText('Singular name')).toHaveFocus())
})

test('spaces Add Content Type as WordPress spaces a create, Cancel as accent text before Add', async () => {
	renderAt('/content-types')

	const dialog = await openAdd()

	const form = within(dialog).getByLabelText('Singular name').closest('form') as HTMLElement
	expect(form).toHaveClass('godmin-modal-body', 'godmin-modal-body--create')
	const [cancel, submit, ...rest] = within(form).getAllByRole('button')
	expect([cancel.textContent, submit.textContent, rest]).toEqual(['Cancel', 'Add', []])
	expect([...cancel.classList]).toEqual(buttonClasses('minimal'))
	expect(submit).toHaveAttribute('type', 'submit')
})

test('keeps Add off until both names are typed, the description left optional', async () => {
	renderAt('/content-types')
	const dialog = await openAdd()
	const add = within(dialog).getByRole('button', { name: 'Add' })

	expect(add).toHaveAttribute('aria-disabled', 'true')
	await userEvent.type(within(dialog).getByLabelText('Singular name'), 'Lesson')
	expect(add).toHaveAttribute('aria-disabled', 'true')
	await userEvent.type(within(dialog).getByLabelText('Plural name'), '  ')
	expect(add).toHaveAttribute('aria-disabled', 'true')
	await userEvent.type(within(dialog).getByLabelText('Plural name'), 'Lessons')
	expect(add).not.toHaveAttribute('aria-disabled', 'true')
})

test('registers a type from the labels it was given', async () => {
	const sent = recordRegistrations()
	renderAt('/content-types')
	const dialog = await openAdd()

	await nameNewType(dialog, 'Guide', 'Guides')
	await userEvent.type(within(dialog).getByLabelText('Description'), 'Manage the guides on this site.')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add' }))

	await waitFor(() =>
		expect(sent).toEqual([
			{
				key: 'guide',
				singular_label: 'Guide',
				plural_label: 'Guides',
				description: 'Manage the guides on this site.',
				route_word: 'guides',
			},
		]),
	)
})

test('registers a type with no description from Enter in a field', async () => {
	const sent = recordRegistrations()
	renderAt('/content-types')
	const dialog = await openAdd()

	await nameNewType(dialog)
	await userEvent.keyboard('{Enter}')

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ key: 'lesson', plural_label: 'Lessons', description: '', route_word: 'lessons' })
})

test('names the new type in a toast, closes the dialog and lists the type', async () => {
	let added = false
	server.use(
		http.post('/api/types', () => {
			added = true
			const lesson = { ...GUIDE_TYPE, key: 'lesson', singular_label: 'Lesson', plural_label: 'Lessons' }
			return HttpResponse.json(lesson, { status: 201 })
		}),
		http.get('/api/types', () =>
			HttpResponse.json({
				items: [POST_TYPE, PAGE_TYPE, ...(added ? [{ ...GUIDE_TYPE, key: 'lesson', plural_label: 'Lessons' }] : [])],
			}),
		),
	)
	renderAt('/content-types')
	const dialog = await openAdd()

	await nameNewType(dialog)
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add' }))

	expect(await screen.findByText('"Lessons" successfully created.', { selector: '.godmin-toast' })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(await rowOf('Lessons')).toBeTruthy()
})

test('ties the address hint to the plural name of a new type', async () => {
	renderAt('/content-types')
	const dialog = await openAdd()

	const plural = within(dialog).getByLabelText('Plural name')
	expect(plural).toHaveAccessibleDescription('This type will answer under /address.')

	await userEvent.type(plural, 'Guides')

	expect(plural).toHaveAccessibleDescription('This type will answer under /guides.')
})

test('explains the description field of a new type', async () => {
	renderAt('/content-types')
	const dialog = await openAdd()

	expect(within(dialog).getByLabelText('Description')).toHaveAccessibleDescription(
		'A descriptive summary of the content type.',
	)
})

test('keeps Add Content Type open on the reason the registry refused a type, the typed names kept', async () => {
	server.use(
		http.post('/api/types', () =>
			HttpResponse.json({ error: 'content: the route word is taken', code: 'route_word_taken' }, { status: 422 }),
		),
	)
	renderAt('/content-types')
	const dialog = await openAdd()

	await nameNewType(dialog, 'Page', 'Pages')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add' }))

	expect(
		await screen.findByText('Another content type already answers under this route word. Pick a different word.', {
			selector: '.godmin-toast',
		}),
	).toBeInTheDocument()
	expect(screen.getByRole('dialog', { name: 'Add Content Type' })).toBe(dialog)
	expect(within(dialog).getByLabelText('Plural name')).toHaveValue('Pages')
	expect(within(dialog).getByRole('button', { name: 'Add' })).not.toHaveAttribute('aria-disabled', 'true')
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('says the registry could not be reached when the network drops a new type', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.post('/api/types', () => HttpResponse.error()))
	renderAt('/content-types')
	const dialog = await openAdd()

	await nameNewType(dialog)
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add' }))

	expect(await findToast('The registry could not be reached.')).toBeInTheDocument()
})

test('keeps Add busy with Cancel live while the type is registered', async () => {
	const register = gate()
	server.use(
		http.post('/api/types', async () => {
			await register.held
			return HttpResponse.json(GUIDE_TYPE, { status: 201 })
		}),
	)
	renderAt('/content-types')
	const dialog = await openAdd()

	await nameNewType(dialog)
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add' }))

	const add = within(dialog).getByRole('button', { name: 'Add' })
	await waitFor(() => expect(add.className).toMatch(/__is-loading\b/))
	expect(within(dialog).getByRole('button', { name: 'Cancel' })).not.toHaveAttribute('aria-disabled', 'true')
	register.release()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('closes Add Content Type on Cancel while the type is registered, naming it in a toast once done', async () => {
	const register = gate()
	server.use(
		http.post('/api/types', async () => {
			await register.held
			return HttpResponse.json({ ...GUIDE_TYPE, key: 'lesson', plural_label: 'Lessons' }, { status: 201 })
		}),
	)
	renderAt('/content-types')
	const dialog = await openAdd()
	await nameNewType(dialog)
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add' }))

	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	register.release()
	expect(await screen.findByText('"Lessons" successfully created.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test.each([
	{ outcome: 'is registered', status: 201 },
	{ outcome: 'is refused', status: 422 },
])('leaves a reopened Add Content Type open on its new names when an earlier type $outcome', async ({ status }) => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	const register = gate()
	let asked = 0
	server.use(
		http.post('/api/types', async () => {
			asked += 1
			await register.held
			return status === 201
				? HttpResponse.json(GUIDE_TYPE, { status })
				: HttpResponse.json({ error: 'content: the route word is taken' }, { status })
		}),
	)
	renderAt('/content-types')
	const first = await openAdd()
	await nameNewType(first)
	await userEvent.click(within(first).getByRole('button', { name: 'Add' }))
	await waitFor(() => expect(asked).toBe(1))
	await userEvent.click(within(first).getByRole('button', { name: 'Cancel' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

	const reopened = await openAdd()
	expect(within(reopened).getByLabelText('Singular name')).toHaveValue('')
	await nameNewType(reopened, 'Recipe', 'Recipes')
	register.release()
	await screen.findByText(/./, { selector: '.godmin-toast' })
	await act(() => new Promise((resolve) => setTimeout(resolve, 0)))

	expect(screen.getByRole('dialog', { name: 'Add Content Type' })).toBe(reopened)
	expect(within(reopened).getByLabelText('Singular name')).toHaveValue('Recipe')
	expect(within(reopened).getByLabelText('Plural name')).toHaveValue('Recipes')
	expect(within(reopened).getByRole('button', { name: 'Add' })).not.toHaveAttribute('aria-disabled', 'true')
})

test('names the type it registered when its plural name changes while it runs', async () => {
	const register = gate()
	server.use(
		http.post('/api/types', async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			await register.held
			return HttpResponse.json({ ...GUIDE_TYPE, ...body }, { status: 201 })
		}),
	)
	renderAt('/content-types')
	const dialog = await openAdd()
	await nameNewType(dialog, 'Guide', 'Guides')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Add' }))
	await waitBusy(dialog, 'Add')

	await userEvent.type(within(dialog).getByLabelText('Plural name'), ' and notes')
	register.release()

	expect(await screen.findByText('"Guides" successfully created.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('registers nothing when Add Content Type is cancelled, and opens empty again', async () => {
	const sent = recordRegistrations()
	renderAt('/content-types')
	const dialog = await openAdd()
	await nameNewType(dialog)

	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	const reopened = await openAdd()

	expect(within(reopened).getByLabelText('Singular name')).toHaveValue('')
	expect(within(reopened).getByLabelText('Plural name')).toHaveValue('')
	expect(within(reopened).getByLabelText('Description')).toHaveValue('')
	expect(sent).toEqual([])
})

test('names Add Content Type and its button in the words of WordPress es_ES', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/content-types')

	await userEvent.click(await screen.findByRole('button', { name: 'Añadir tipo de contenido' }))
	const dialog = await screen.findByRole('dialog', { name: 'Añadir tipo de contenido' })

	expect(within(dialog).getByRole('button', { name: 'Añadir' })).toBeInTheDocument()
	expect(within(dialog).getByRole('button', { name: 'Cancelar' })).toBeInTheDocument()
})

test.each([
	{
		locale: 'es-ES',
		add: 'Añadir tipo de contenido',
		singular: 'Nombre en singular',
		plural: 'Nombre en plural',
		send: 'Añadir',
		created: 'Se ha creado correctamente el tipo de contenido «Lessons».',
	},
	{
		locale: 'fr-FR',
		add: 'Ajouter un type de contenu',
		singular: 'Nom au singulier',
		plural: 'Nom au pluriel',
		send: 'Ajouter',
		created: 'Type de contenu « Lessons » créé avec succès.',
	},
])('names a new type by its kind for a $locale reader, so no word agrees with its name', async (reader) => {
	setLocaleData(await catalogFor(reader.locale), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	const lesson = { ...GUIDE_TYPE, key: 'lesson', singular_label: 'Lesson', plural_label: 'Lessons' }
	server.use(http.post('/api/types', () => HttpResponse.json(lesson, { status: 201 })))
	renderAt('/content-types')
	await userEvent.click(await screen.findByRole('button', { name: reader.add }))
	const dialog = await screen.findByRole('dialog', { name: reader.add })

	await userEvent.type(within(dialog).getByLabelText(reader.singular), 'Lesson')
	await userEvent.type(within(dialog).getByLabelText(reader.plural), 'Lessons')
	await userEvent.click(within(dialog).getByRole('button', { name: reader.send }))

	expect(await findToast(reader.created)).toBeInTheDocument()
})
