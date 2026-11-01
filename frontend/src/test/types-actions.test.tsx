// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { buttonClasses, stubLayout } from '@gopherium/godmin/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { typesQueryKey } from '../content/nav'
import { catalogFor } from '../i18n/catalog'
import { gate } from './gate'
import { expectUnmarked, holdReads, waitMarked } from './reloads'
import { renderAt } from './render'
import {
	GUIDE_TYPE,
	PAGE_TYPE,
	POST_TYPE,
	SPANISH,
	clickDimmedPage,
	findToast,
	menuItems,
	openRowDialog,
	openRowMenu,
	recordEdits,
	refuseEdits,
	rowOf,
	serveTypes,
	waitBusy,
	warmTypesScreen,
} from './typeFixtures'

warmTypesScreen()

beforeEach(() => {
	serveTypes(POST_TYPE, PAGE_TYPE)
})

/** How a Spanish and a French reader see the Make default confirm of the Pages row. */
const READERS = [
	{
		locale: 'es-ES',
		region: 'Tipos de contenido',
		pages: 'Páginas',
		makeDefault: 'Establecer como predeterminado…',
		heading: 'Establecer como predeterminado',
		question: [
			'El tipo Páginas responderá en la raíz.',
			'El tipo Entradas pasa a /posts. Cambian todas las direcciones de ambos tipos.',
		].join(' '),
		nesting: 'Dejar de anidar',
		updated: 'Tipo de contenido actualizado.',
	},
	{
		locale: 'fr-FR',
		region: 'Types de contenu',
		pages: 'Pages',
		makeDefault: 'Rendre par défaut…',
		heading: 'Rendre par défaut',
		question:
			'Le type Pages répondra à la racine. Le type Articles passe à /posts. Toutes les adresses des deux types changent.',
		nesting: "Arrêter l'imbrication",
		updated: 'Type de contenu mis à jour.',
	},
]

/**
 * Returns how many milliseconds pass, so a write that does not wait its turn has the time to go out.
 * @returns A promise settling once the time has passed.
 */
function aMoment() {
	return act(() => new Promise((resolve) => setTimeout(resolve, 50)))
}

/**
 * Types new text into the one field of an open row dialog, in place of what it held.
 * @param dialog - The dialog.
 * @param label - The label of the field.
 * @param text - The new text.
 */
async function retype(dialog: HTMLElement, label: string, text: string) {
	const field = within(dialog).getByLabelText(label)
	await userEvent.clear(field)
	await userEvent.type(field, text)
}

/**
 * Writes a new description for a listed type through Describe and saves it.
 * @param name - The name the row shows.
 * @param text - The new description.
 */
async function describeAs(name: string, text: string) {
	const dialog = await openRowDialog(name, 'Describe…')
	await retype(dialog, 'Description', text)
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
}

test('offers Describe…, Change address…, Make default…, the nesting switch, Deactivate and Delete… last', async () => {
	renderAt('/content-types')

	await openRowMenu('Pages')

	expect(menuItems()).toEqual(['Describe…', 'Change address…', 'Make default…', 'Stop nesting', 'Deactivate', 'Delete…'])
})

test('offers the default type no Make default, Deactivate or Delete', async () => {
	renderAt('/content-types')

	await openRowMenu('Posts')

	expect(menuItems()).toEqual(['Describe…', 'Change address…', 'Let items nest'])
})

test('offers Activate in place of Deactivate on a closed type', async () => {
	serveTypes(POST_TYPE, { ...PAGE_TYPE, active: false })
	renderAt('/content-types')

	await openRowMenu('Pages')

	expect(menuItems()).toEqual(['Describe…', 'Change address…', 'Make default…', 'Stop nesting', 'Activate', 'Delete…'])
})

test('keeps the shape of a type a plugin declared out of reach, offering only to switch it off', async () => {
	serveTypes(POST_TYPE, { ...PAGE_TYPE, origin: 'events' })
	renderAt('/content-types')

	await openRowMenu('Pages')

	expect(menuItems()).toEqual(['Deactivate'])
})

test('describes in a medium dialog headed Describe, its field holding the stored description', async () => {
	stubLayout()
	renderAt('/content-types')

	const dialog = await openRowDialog('Pages', 'Describe…')

	expect(dialog).toHaveClass('has-size-medium')
	expect(within(dialog).getByRole('heading', { name: 'Describe' })).toBeInTheDocument()
	const field = within(dialog).getByLabelText('Description')
	expect(field).toHaveValue('Manage the pages on this site.')
	expect(field).toHaveAccessibleDescription('A descriptive summary of the content type.')
	expect(field).toHaveAttribute('autocomplete', 'off')
	await waitFor(() => expect(field).toHaveFocus())
})

test('spaces Describe as WordPress spaces a row edit, Cancel as accent text before Save', async () => {
	renderAt('/content-types')

	const dialog = await openRowDialog('Pages', 'Describe…')

	const form = within(dialog).getByLabelText('Description').closest('form') as HTMLElement
	expect([...form.classList]).toEqual(['godmin-modal-body'])
	const [cancel, submit, ...rest] = within(form).getAllByRole('button')
	expect([cancel.textContent, submit.textContent, rest]).toEqual(['Cancel', 'Save', []])
	expect([...cancel.classList]).toEqual(buttonClasses('minimal'))
	expect(submit).toHaveAttribute('type', 'submit')
})

test('closes Describe as Save is pressed, before the write answers, then says the description changed', async () => {
	const answer = gate()
	const sent: unknown[] = []
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			sent.push(await request.json())
			await answer.held
			return HttpResponse.json({ ...PAGE_TYPE, description: 'Every page this site keeps.' })
		}),
	)
	renderAt('/content-types')

	await describeAs('Pages', 'Every page this site keeps.')

	await waitFor(() => expect(sent).toEqual([{ description: 'Every page this site keeps.' }]))
	expect(screen.queryByText('Description updated.')).not.toBeInTheDocument()
	answer.release()
	expect(await screen.findByText('Description updated.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('sends a description from Enter in its field', async () => {
	const sent = recordEdits(PAGE_TYPE)
	renderAt('/content-types')
	const dialog = await openRowDialog('Pages', 'Describe…')

	await retype(dialog, 'Description', 'Every page this site keeps.')
	await userEvent.keyboard('{Enter}')

	await waitFor(() => expect(sent).toEqual([{ description: 'Every page this site keeps.' }]))
})

test('keeps Save on while the description is empty, so a description can be cleared', async () => {
	const sent = recordEdits(PAGE_TYPE)
	renderAt('/content-types')
	const dialog = await openRowDialog('Pages', 'Describe…')

	await userEvent.clear(within(dialog).getByLabelText('Description'))
	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))

	await waitFor(() => expect(sent).toEqual([{ description: '' }]))
})

test('shows the stored description as the list reads it again', async () => {
	let stored = PAGE_TYPE.description
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			stored = ((await request.json()) as { description: string }).description
			return HttpResponse.json({ ...PAGE_TYPE, description: stored })
		}),
		http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, { ...PAGE_TYPE, description: stored }] })),
	)
	renderAt('/content-types')

	await describeAs('Pages', 'Every page this site keeps.')

	expect(await (await rowOf('Pages')).findByText('Every page this site keeps.')).toBeInTheDocument()
	const again = await openRowDialog('Pages', 'Describe…')
	expect(within(again).getByLabelText('Description')).toHaveValue('Every page this site keeps.')
})

test('shows the reason a refused description gave in a toast, never on the page', async () => {
	refuseEdits(PAGE_TYPE, 'content: the registry is busy')
	renderAt('/content-types')

	await describeAs('Pages', 'Every page this site keeps.')

	expect(await screen.findByText('content: the registry is busy', { selector: '.godmin-toast' })).toBeInTheDocument()
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('offers the description a refused save left unsaved when Describe opens again', async () => {
	refuseEdits(PAGE_TYPE)
	renderAt('/content-types')
	await describeAs('Pages', 'Every page this site keeps.')
	await screen.findByText('content: the type still holds content', { selector: '.godmin-toast' })

	const again = await openRowDialog('Pages', 'Describe…')

	expect(within(again).getByLabelText('Description')).toHaveValue('Every page this site keeps.')
})

test('keeps the description a refused save left unsaved through a Cancel', async () => {
	refuseEdits(PAGE_TYPE)
	renderAt('/content-types')
	await describeAs('Pages', 'Every page this site keeps.')
	await screen.findByText('content: the type still holds content', { selector: '.godmin-toast' })
	const kept = await openRowDialog('Pages', 'Describe…')
	await userEvent.click(within(kept).getByRole('button', { name: 'Cancel' }))
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())

	const again = await openRowDialog('Pages', 'Describe…')

	expect(within(again).getByLabelText('Description')).toHaveValue('Every page this site keeps.')
})

test('offers the description a refused save left unsaved only on the type it was typed for', async () => {
	refuseEdits(PAGE_TYPE)
	serveTypes(POST_TYPE, PAGE_TYPE, GUIDE_TYPE)
	renderAt('/content-types')
	await describeAs('Pages', 'Every page this site keeps.')
	await screen.findByText('content: the type still holds content', { selector: '.godmin-toast' })

	const other = await openRowDialog('Posts', 'Describe…')

	expect(within(other).getByLabelText('Description')).toHaveValue('Manage the posts on this site.')
})

test('opens Describe on the stored description again once a later save lands', async () => {
	let refuse = true
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			if (refuse) {
				return HttpResponse.json({ error: 'content: down' }, { status: 500 })
			}
			return HttpResponse.json({ ...PAGE_TYPE, ...((await request.json()) as object) })
		}),
	)
	renderAt('/content-types')
	await describeAs('Pages', 'Every page this site keeps.')
	await screen.findByText('content: down', { selector: '.godmin-toast' })
	refuse = false
	const kept = await openRowDialog('Pages', 'Describe…')
	await retype(kept, 'Description', 'Manage the pages on this site.')
	await userEvent.click(within(kept).getByRole('button', { name: 'Save' }))
	await screen.findByText('Description updated.', { selector: '.godmin-toast' })

	const again = await openRowDialog('Pages', 'Describe…')

	expect(within(again).getByLabelText('Description')).toHaveValue('Manage the pages on this site.')
})

test('opens Describe on the saved description when an earlier save fails after a later one was sent', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	const first = gate()
	let stored = PAGE_TYPE.description
	let writes = 0
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, { ...PAGE_TYPE, description: stored }] })),
		http.patch('/api/types/page', async ({ request }) => {
			writes += 1
			const asked = ((await request.json()) as { description: string }).description
			if (writes === 1) {
				await first.held
				return HttpResponse.json({ error: 'content: down' }, { status: 500 })
			}
			stored = asked
			return HttpResponse.json({ ...PAGE_TYPE, description: stored })
		}),
	)
	renderAt('/content-types')
	await describeAs('Pages', 'A failed description.')
	await waitFor(() => expect(writes).toBe(1))
	await describeAs('Pages', 'A saved description.')

	first.release()

	await screen.findByText('content: down', { selector: '.godmin-toast' })
	await screen.findByText('Description updated.', { selector: '.godmin-toast' })
	const again = await openRowDialog('Pages', 'Describe…')
	expect(within(again).getByLabelText('Description')).toHaveValue('A saved description.')
})

test('leaves a Describe opened again while a save runs on the text typed there once that save fails', async () => {
	const answer = gate()
	server.use(
		http.patch('/api/types/page', async () => {
			await answer.held
			return HttpResponse.json({ error: 'content: down' }, { status: 500 })
		}),
	)
	renderAt('/content-types')
	await describeAs('Pages', 'A failed description.')
	const reopened = await openRowDialog('Pages', 'Describe…')
	await retype(reopened, 'Description', 'Pages the team keeps.')

	answer.release()
	await screen.findByText('content: down', { selector: '.godmin-toast' })
	await act(() => new Promise((resolve) => setTimeout(resolve, 0)))

	expect(screen.getByRole('dialog', { name: 'Describe' })).toBe(reopened)
	expect(within(reopened).getByLabelText('Description')).toHaveValue('Pages the team keeps.')
})

test.each([
	{
		way: 'Cancel',
		close: (dialog: HTMLElement) => userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' })),
	},
	{ way: 'Escape', close: () => userEvent.keyboard('{Escape}') },
])('changes no description when Describe is closed with $way', async ({ close }) => {
	const sent = recordEdits(PAGE_TYPE)
	renderAt('/content-types')
	const dialog = await openRowDialog('Pages', 'Describe…')
	await userEvent.type(within(dialog).getByLabelText('Description'), ' Kept by the team.')

	await close(dialog)
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	const reopened = await openRowDialog('Pages', 'Describe…')

	expect(within(reopened).getByLabelText('Description')).toHaveValue('Manage the pages on this site.')
	expect(sent).toEqual([])
})

test('names Describe in Spanish and leaves the stored description of a built-in type to edit', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/content-types')

	const dialog = await openRowDialog('Páginas', 'Describir…', SPANISH)

	expect(within(dialog).getByRole('heading', { name: 'Describir' })).toBeInTheDocument()
	expect(within(dialog).getByLabelText('Descripción')).toHaveValue('Manage the pages on this site.')
	expect(within(dialog).getByRole('button', { name: 'Guardar' })).toBeInTheDocument()
	expect(within(dialog).getByRole('button', { name: 'Cancelar' })).toBeInTheDocument()
})

test('changes an address in a medium dialog headed Change address, the warning before the field', async () => {
	stubLayout()
	renderAt('/content-types')

	const dialog = await openRowDialog('Pages', 'Change address…')

	expect(dialog).toHaveClass('has-size-medium')
	expect(within(dialog).getByRole('heading', { name: 'Change address' })).toBeInTheDocument()
	const warning = within(dialog).getByText(
		'Every address of this type moves. Links kept elsewhere to the old addresses stop working.',
	)
	const field = within(dialog).getByLabelText('Route word')
	expect(warning.compareDocumentPosition(field) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
	expect(field).toHaveValue('pages')
	expect(field).toHaveAttribute('autocomplete', 'off')
	const [cancel, submit] = within(dialog).getAllByRole('button').filter((button) => button.closest('form') !== null)
	expect([cancel.textContent, submit.textContent]).toEqual(['Cancel', 'Move every address'])
	await waitFor(() => expect(field).toHaveFocus())
})

test('keeps Move every address off while the route word is empty', async () => {
	renderAt('/content-types')
	const dialog = await openRowDialog('Pages', 'Change address…')
	const submit = within(dialog).getByRole('button', { name: 'Move every address' })

	await userEvent.clear(within(dialog).getByLabelText('Route word'))
	expect(submit).toHaveAttribute('aria-disabled', 'true')
	await userEvent.type(within(dialog).getByLabelText('Route word'), '  ')
	expect(submit).toHaveAttribute('aria-disabled', 'true')
	await userEvent.type(within(dialog).getByLabelText('Route word'), 'sections')

	expect(submit).not.toHaveAttribute('aria-disabled', 'true')
})

test('moves every address once Move every address is pressed, closing at once and saying so', async () => {
	const sent = recordEdits(PAGE_TYPE)
	renderAt('/content-types')
	const dialog = await openRowDialog('Pages', 'Change address…')

	await retype(dialog, 'Route word', 'sections')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move every address' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await waitFor(() => expect(sent).toEqual([{ route_word: 'sections' }]))
	expect(await screen.findByText('Address updated.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('sends the route word without the spaces typed around it', async () => {
	const sent = recordEdits(PAGE_TYPE)
	renderAt('/content-types')
	const dialog = await openRowDialog('Pages', 'Change address…')

	await retype(dialog, 'Route word', '  sections  ')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move every address' }))

	await waitFor(() => expect(sent).toEqual([{ route_word: 'sections' }]))
})

test('offers the route word a refused move left unsaved when Change address opens again', async () => {
	server.use(
		http.patch('/api/types/page', () =>
			HttpResponse.json({ error: 'content: route word taken', code: 'route_word_taken' }, { status: 422 }),
		),
	)
	renderAt('/content-types')
	const dialog = await openRowDialog('Pages', 'Change address…')
	await retype(dialog, 'Route word', 'sections')
	await userEvent.click(within(dialog).getByRole('button', { name: 'Move every address' }))
	await screen.findByText('Another content type already answers under this route word. Pick a different word.', {
		selector: '.godmin-toast',
	})

	const again = await openRowDialog('Pages', 'Change address…')

	expect(within(again).getByLabelText('Route word')).toHaveValue('sections')
})

test('asks before handing the root over in a medium dialog headed Make default, naming both types', async () => {
	stubLayout()
	const sent = recordEdits(PAGE_TYPE)
	renderAt('/content-types')

	const dialog = await openRowDialog('Pages', 'Make default…')

	expect(dialog).toHaveClass('has-size-medium')
	expect(within(dialog).getByRole('heading', { name: 'Make default' })).toBeInTheDocument()
	expect(
		within(dialog).getByText(
			'Pages will answer at the root. Posts moves to /posts. Every address of both types changes.',
		),
	).toBeInTheDocument()
	await waitFor(() => expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveFocus())
	expect(within(dialog).getByRole('button', { name: 'Make default' })).toBeInTheDocument()
	expect(sent).toEqual([])
})

test.each([
	{ key: 'resume', plural: 'Résumés', word: 'resumes' },
	{ key: 'model', plural: '3D Models', word: 'model' },
])('names the word the server derives for $plural as where the root holder moves', async ({ key, plural, word }) => {
	const holder = { ...GUIDE_TYPE, key, plural_label: plural, route_word: '', derived_route_word: word, default: true }
	serveTypes({ ...POST_TYPE, default: false }, PAGE_TYPE, holder)
	renderAt('/content-types')

	const dialog = await openRowDialog('Pages', 'Make default…')

	const question = `Pages will answer at the root. ${plural} moves to /${word}. Every address of both types changes.`
	expect(within(dialog).getByText(question)).toBeInTheDocument()
})

test('asks only where the type will answer when no type holds the root', async () => {
	serveTypes({ ...POST_TYPE, default: false }, PAGE_TYPE)
	renderAt('/content-types')

	const dialog = await openRowDialog('Pages', 'Make default…')

	expect(within(dialog).getByText('Pages will answer at the root.')).toBeInTheDocument()
})

test.each(READERS)('names the built-in types of Make default by kind for a $locale reader', async (reader) => {
	setLocaleData(await catalogFor(reader.locale), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/content-types')

	const dialog = await openRowDialog(reader.pages, reader.makeDefault, reader)

	expect(within(dialog).getByRole('heading', { name: reader.heading })).toBeInTheDocument()
	expect(within(dialog).getByText(reader.question)).toBeInTheDocument()
})

test('hands the root over once confirmed, busy with Cancel greyed until the write ends, then says so', async () => {
	const answer = gate()
	const sent: unknown[] = []
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			sent.push(await request.json())
			await answer.held
			return HttpResponse.json({ ...PAGE_TYPE, default: true, route_word: '' })
		}),
	)
	renderAt('/content-types')
	const dialog = await openRowDialog('Pages', 'Make default…')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Make default' }))

	await waitFor(() => expect(sent).toEqual([{ default: true }]))
	const confirm = within(dialog).getByRole('button', { name: 'Make default' })
	await waitFor(() => expect(confirm.className).toMatch(/__is-loading\b/))
	expect(within(dialog).getByRole('button', { name: 'Cancel' })).toHaveAttribute('aria-disabled', 'true')
	answer.release()
	expect(await screen.findByText('Default content type updated.', { selector: '.godmin-toast' })).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
})

test('shows the reason a refused hand over gave in a toast, the confirm closed', async () => {
	refuseEdits(PAGE_TYPE, 'content: the route word is reserved')
	renderAt('/content-types')
	const dialog = await openRowDialog('Pages', 'Make default…')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Make default' }))

	expect(await findToast('content: the route word is reserved')).toBeInTheDocument()
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('hands nothing over when the confirm is cancelled', async () => {
	const sent = recordEdits(PAGE_TYPE)
	renderAt('/content-types')
	const dialog = await openRowDialog('Pages', 'Make default…')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	expect(sent).toEqual([])
})

test('opens Make default without reading the registry again, naming the holder from the list on show', async () => {
	let reads = 0
	server.use(
		http.get('/api/types', () => {
			reads += 1
			return HttpResponse.json({ items: [POST_TYPE, PAGE_TYPE] })
		}),
	)
	const client = renderAt('/content-types')
	client.setQueryDefaults(typesQueryKey, { staleTime: 0 })
	await openRowMenu('Pages')
	const before = reads

	await userEvent.click(screen.getByRole('menuitem', { name: 'Make default…' }))
	const dialog = await screen.findByRole('dialog')
	await aMoment()

	const question = 'Pages will answer at the root. Posts moves to /posts. Every address of both types changes.'
	expect(within(dialog).getByText(question)).toBeInTheDocument()
	expect(reads).toBe(before)
})

/**
 * Returns the address of the types list narrowed by one Status filter.
 * @param operator - Whether the list keeps the types carrying the mark or the ones that do not.
 * @param mark - The Status mark.
 * @returns The address.
 */
function narrowedTo(operator: 'isAny' | 'isNone', mark: string): string {
	const filters = JSON.stringify([{ field: 'status', operator, value: [mark] }])
	return `/content-types?filters=${encodeURIComponent(filters)}`
}

/**
 * Serves the Pages type as stored until its first edit, and as given after it.
 * @param stored - The Pages type before the edit, as the registry answers it.
 * @param after - The Pages type after the edit.
 */
function servePagesUntilEdited(stored: Record<string, unknown>, after: Record<string, unknown>) {
	let edited = false
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE, edited ? after : stored] })),
		http.patch('/api/types/page', () => {
			edited = true
			return HttpResponse.json(after)
		}),
	)
}

/**
 * Waits until the list region holds the focus.
 */
async function waitListFocused() {
	await waitFor(() => expect(screen.getByRole('region', { name: 'Content Types' })).toHaveFocus())
}

test.each([
	{
		item: 'Activate',
		narrowing: narrowedTo('isAny', 'closed'),
		stored: { ...PAGE_TYPE, active: false },
		after: PAGE_TYPE,
	},
	{
		item: 'Stop nesting',
		narrowing: narrowedTo('isAny', 'nests'),
		stored: PAGE_TYPE,
		after: { ...PAGE_TYPE, hierarchical: false },
	},
])('lands the focus on the list once $item takes the type out of the filtered list', async (switched) => {
	servePagesUntilEdited(switched.stored, switched.after)
	renderAt(switched.narrowing)
	await openRowMenu('Pages')

	await userEvent.click(screen.getByRole('menuitem', { name: switched.item }))

	await waitListFocused()
})

test('lands the focus on the list once Make default takes the type out of the filtered list', async () => {
	servePagesUntilEdited(PAGE_TYPE, { ...PAGE_TYPE, default: true, route_word: '' })
	renderAt(narrowedTo('isNone', 'default'))
	const dialog = await openRowDialog('Pages', 'Make default…')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Make default' }))

	await findToast('Default content type updated.')
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await waitListFocused()
})

test('shows the toast of a saved description, then marks the list until the registry answers', async () => {
	recordEdits(PAGE_TYPE)
	renderAt('/content-types')
	const dialog = await openRowDialog('Pages', 'Describe…')
	await retype(dialog, 'Description', 'Every section this site keeps.')
	holdReads('/api/types')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Save' }))

	await findToast('Description updated.')
	await waitMarked()
})

test('marks nothing when a description could not be saved', async () => {
	refuseEdits(PAGE_TYPE)
	renderAt('/content-types')
	await describeAs('Pages', 'Every section this site keeps.')

	await findToast(/content/)

	expectUnmarked()
})

test('shows the toast of a nesting switch, then marks the list until the registry answers', async () => {
	recordEdits(PAGE_TYPE)
	renderAt('/content-types')
	await openRowMenu('Pages')
	holdReads('/api/types')

	await userEvent.click(screen.getByRole('menuitem', { name: 'Stop nesting' }))

	await findToast('Content type updated.')
	await waitMarked()
})

test('closes Make default as its toast shows and marks the list until the registry answers', async () => {
	recordEdits(PAGE_TYPE)
	renderAt('/content-types')
	const dialog = await openRowDialog('Pages', 'Make default…')
	holdReads('/api/types')

	await userEvent.click(within(dialog).getByRole('button', { name: 'Make default' }))

	await findToast('Default content type updated.')
	await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
	await waitMarked()
})

test('puts the focus back on the row menu once a saved description leaves the type on the page', async () => {
	recordEdits(PAGE_TYPE)
	renderAt('/content-types')

	await describeAs('Pages', 'Every section this site keeps.')

	await findToast('Description updated.')
	const row = await rowOf('Pages')
	await waitFor(() => expect(row.getByRole('button', { name: 'Actions' })).toHaveFocus())
})

test('puts the focus back on the row menu once a nesting switch leaves the type on the page', async () => {
	recordEdits(PAGE_TYPE)
	renderAt('/content-types')
	await openRowMenu('Pages')

	await userEvent.click(screen.getByRole('menuitem', { name: 'Stop nesting' }))

	await findToast('Content type updated.')
	const row = await rowOf('Pages')
	await waitFor(() => expect(row.getByRole('button', { name: 'Actions' })).toHaveFocus())
})

test('lands the focus on the list once a saved description takes the type out of the search', async () => {
	const rewritten = { ...PAGE_TYPE, description: 'Every section this site keeps.' }
	servePagesUntilEdited(PAGE_TYPE, rewritten)
	renderAt(`/content-types?search=${encodeURIComponent('pages on')}`)

	await describeAs('Pages', rewritten.description)

	await findToast('Description updated.')
	await waitListFocused()
})

test('keeps the focus in a Describe opened while an Activate that empties the filtered list runs', async () => {
	stubLayout()
	const answer = gate()
	let opened = false
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [POST_TYPE, { ...PAGE_TYPE, active: opened }, { ...GUIDE_TYPE, active: false }] }),
		),
		http.patch('/api/types/page', async () => {
			await answer.held
			opened = true
			return HttpResponse.json(PAGE_TYPE)
		}),
	)
	renderAt(narrowedTo('isAny', 'closed'))
	await openRowMenu('Pages')
	await userEvent.click(screen.getByRole('menuitem', { name: 'Activate' }))
	const describe = await openRowDialog('Guides', 'Describe…')
	const field = within(describe).getByLabelText('Description')
	await waitFor(() => expect(field).toHaveFocus())

	answer.release()
	await findToast('"Pages" activated.')
	await act(() => new Promise((resolve) => setTimeout(resolve, 0)))

	expect(field).toHaveFocus()
})

test('leaves a Describe opened after a hand over dismissed by the dimmed page open once it ends', async () => {
	const answer = gate()
	server.use(
		http.patch('/api/types/page', async () => {
			await answer.held
			return HttpResponse.json({ error: 'content: down' }, { status: 500 })
		}),
	)
	renderAt('/content-types')
	const confirm = await openRowDialog('Pages', 'Make default…')
	await userEvent.click(within(confirm).getByRole('button', { name: 'Make default' }))
	await waitBusy(confirm, 'Make default')
	await clickDimmedPage()
	const describe = await openRowDialog('Pages', 'Describe…')

	answer.release()
	await screen.findByText('content: down', { selector: '.godmin-toast' })
	await act(() => new Promise((resolve) => setTimeout(resolve, 0)))

	expect(screen.getByRole('dialog', { name: 'Describe' })).toBe(describe)
})

test('stops a type nesting from its menu, saying the type was updated', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...PAGE_TYPE, hierarchical: false })
		}),
		http.get('/api/types', () =>
			HttpResponse.json({ items: [POST_TYPE, { ...PAGE_TYPE, hierarchical: sent.length === 0 }] }),
		),
	)
	renderAt('/content-types')
	await openRowMenu('Pages')

	await userEvent.click(screen.getByRole('menuitem', { name: 'Stop nesting' }))

	await waitFor(() => expect(sent).toEqual([{ hierarchical: false }]))
	expect(await screen.findByText('Content type updated.', { selector: '.godmin-toast' })).toBeInTheDocument()
	await waitFor(async () => expect((await rowOf('Pages')).queryByText('Nests')).not.toBeInTheDocument())
	await openRowMenu('Pages')
	expect(menuItems()).toContain('Let items nest')
})

test('lets a flat type nest from its menu', async () => {
	const sent = recordEdits(POST_TYPE)
	renderAt('/content-types')
	await openRowMenu('Posts')

	await userEvent.click(screen.getByRole('menuitem', { name: 'Let items nest' }))

	await waitFor(() => expect(sent).toEqual([{ hierarchical: true }]))
	expect(await screen.findByText('Content type updated.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('says in a toast how many items keep a type nesting', async () => {
	server.use(
		http.patch('/api/types/page', () =>
			HttpResponse.json(
				{
					error: 'content: items of the type still nest: 2 in page',
					code: 'type_nesting_in_use',
					meta: { type: 'page', items: 2 },
				},
				{ status: 422 },
			),
		),
	)
	renderAt('/content-types')
	await openRowMenu('Pages')

	await userEvent.click(screen.getByRole('menuitem', { name: 'Stop nesting' }))

	const toast = await screen.findByText(/2 of its items sit inside another/, { selector: '.godmin-toast' })
	expect(toast).toBeInTheDocument()
})

test.each(READERS)('names no type in the toast of a $locale edit, so no word has to agree with it', async (reader) => {
	setLocaleData(await catalogFor(reader.locale), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	recordEdits(PAGE_TYPE)
	renderAt('/content-types')
	await openRowMenu(reader.pages, reader)

	await userEvent.click(screen.getByRole('menuitem', { name: reader.nesting }))

	expect(await screen.findByText(reader.updated, { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('closes a type from its menu without removing it, naming it in a toast', async () => {
	const sent = recordEdits(PAGE_TYPE)
	renderAt('/content-types')
	await openRowMenu('Pages')

	await userEvent.click(screen.getByRole('menuitem', { name: 'Deactivate' }))

	await waitFor(() => expect(sent).toEqual([{ active: false }]))
	expect(await screen.findByText('"Pages" deactivated.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('reopens a closed type from its menu, naming it in a toast', async () => {
	serveTypes(POST_TYPE, { ...PAGE_TYPE, active: false })
	const sent = recordEdits(PAGE_TYPE)
	renderAt('/content-types')
	await openRowMenu('Pages')

	await userEvent.click(screen.getByRole('menuitem', { name: 'Activate' }))

	await waitFor(() => expect(sent).toEqual([{ active: true }]))
	expect(await screen.findByText('"Pages" activated.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('names a reopened type in Spanish with no word agreeing with it', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	serveTypes(POST_TYPE, { ...PAGE_TYPE, active: false })
	recordEdits(PAGE_TYPE)
	renderAt('/content-types')
	await openRowMenu('Páginas', SPANISH)

	await userEvent.click(screen.getByRole('menuitem', { name: 'Activar' }))

	expect(await screen.findByText('Se ha activado «Páginas».', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('names a closed type in French by its kind, so no word agrees with its name', async () => {
	setLocaleData(await catalogFor('fr-FR'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	recordEdits(PAGE_TYPE)
	renderAt('/content-types')
	await openRowMenu('Pages', { region: 'Types de contenu' })

	await userEvent.click(screen.getByRole('menuitem', { name: 'Désactiver' }))

	expect(await findToast('Type de contenu « Pages » désactivé.')).toBeInTheDocument()
})

test('shows the reason a refused switch gave in a toast', async () => {
	refuseEdits(PAGE_TYPE)
	renderAt('/content-types')
	await openRowMenu('Pages')

	await userEvent.click(screen.getByRole('menuitem', { name: 'Deactivate' }))

	expect(await findToast('content: the type still holds content')).toBeInTheDocument()
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('says the registry could not be reached when the network drops an edit', async () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	server.use(http.patch('/api/types/page', () => HttpResponse.error()))
	renderAt('/content-types')
	await openRowMenu('Pages')

	await userEvent.click(screen.getByRole('menuitem', { name: 'Stop nesting' }))

	expect(await findToast('The registry could not be reached.')).toBeInTheDocument()
})

/**
 * Holds the first edit of the Pages type until the returned gate opens, and records the order every edit went in.
 * @returns The gate the first edit waits on, and what the registry heard.
 */
function holdFirstPagesEdit() {
	const first = gate()
	const heard: string[] = []
	server.use(
		http.patch('/api/types/page', async ({ request }) => {
			const asked = JSON.stringify(await request.json())
			heard.push(`sent ${asked}`)
			if (heard.length === 1) {
				await first.held
			}
			heard.push(`answered ${asked}`)
			return HttpResponse.json(PAGE_TYPE)
		}),
	)
	return { first, heard }
}

test('sends a second Describe save only once the first save to the type has answered', async () => {
	const { first, heard } = holdFirstPagesEdit()
	renderAt('/content-types')
	await describeAs('Pages', 'Every page this site keeps.')
	await waitFor(() => expect(heard).toHaveLength(1))

	await describeAs('Pages', 'Pages the team keeps.')
	await aMoment()
	first.release()

	await waitFor(() => expect(heard).toHaveLength(4))
	expect(heard).toEqual([
		'sent {"description":"Every page this site keeps."}',
		'answered {"description":"Every page this site keeps."}',
		'sent {"description":"Pages the team keeps."}',
		'answered {"description":"Pages the team keeps."}',
	])
})

test('sends a one-click edit only once a Describe save to the same type has answered', async () => {
	const { first, heard } = holdFirstPagesEdit()
	renderAt('/content-types')
	await describeAs('Pages', 'Every page this site keeps.')
	await waitFor(() => expect(heard).toHaveLength(1))

	await openRowMenu('Pages')
	await userEvent.click(screen.getByRole('menuitem', { name: 'Stop nesting' }))
	await aMoment()
	first.release()

	await waitFor(() => expect(heard).toHaveLength(4))
	expect(heard.slice(1, 3)).toEqual([
		'answered {"description":"Every page this site keeps."}',
		'sent {"hierarchical":false}',
	])
})

test('sends an edit to one type while a save to another type runs', async () => {
	const save = gate()
	let nested = false
	server.use(
		http.patch('/api/types/page', async () => {
			await save.held
			return HttpResponse.json(PAGE_TYPE)
		}),
		http.patch('/api/types/post', () => {
			nested = true
			return HttpResponse.json({ ...POST_TYPE, hierarchical: true })
		}),
	)
	renderAt('/content-types')
	await describeAs('Pages', 'Every page this site keeps.')

	await openRowMenu('Posts')
	await userEvent.click(screen.getByRole('menuitem', { name: 'Let items nest' }))

	await waitFor(() => expect(nested).toBe(true))
	save.release()
	await screen.findByText('Description updated.', { selector: '.godmin-toast' })
})
