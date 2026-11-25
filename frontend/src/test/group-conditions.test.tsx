// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeEach, expect, onTestFinished, test } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { gate } from './gate'
import { fieldRow, openField, openFields, saveOf, showTab, toggleOf } from './groupFieldsFixtures'
import { renderAt, renderRoutedAt } from './render'
import { expectRuleSets } from './ruleSets'
import { siteSettings } from './siteSettings'
import { expectReadOnly, findToast, waitBusy } from './typeFixtures'

const READ_STAMP = '2026-08-01T10:00:00Z'

const OTHER_SAVE_STAMP = '2026-08-02T09:00:00Z'

/** A time limit far shorter than any test. */
const SHORT_LIMIT = 20

/** The name every Conditional logic tab starts with, whether or not it carries its dot. */
const RULES_TAB = /^Conditional logic/

/** The name of the trash on the first rule of the first rule set. */
const FIRST_TRASH = 'Remove rule 1 of set 1'

const POST_TYPE = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	description: '',
	route_word: '',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	fields: [],
}

const ON_SALE = {
	key: 'on-sale',
	label: 'On sale',
	kind: 'boolean',
	many: false,
	required: false,
	created_at: READ_STAMP,
	updated_at: READ_STAMP,
}

const SALE_NOTE = { ...ON_SALE, key: 'sale-note', label: 'Sale note', kind: 'text' }

const PHOTO = { ...ON_SALE, key: 'photo', label: 'Photo', kind: 'media' }

const DETAILS = {
	id: 3,
	key: 'article-details',
	title: 'Article details',
	location: [[{ source: 'content_type', operator: '==', value: 'post' }]],
	position: 1,
	active: true,
	fields: [ON_SALE, SALE_NOTE, PHOTO],
	created_at: READ_STAMP,
	updated_at: READ_STAMP,
}

/** The rule showing a field while On sale is ticked. */
const WHILE_ON_SALE = { source: 'on-sale', operator: '==', value: 'true' }

/** The rule showing a field while On sale is not ticked. */
const WHILE_NOT_ON_SALE = { source: 'on-sale', operator: '!=', value: 'true' }

const CYCLE = 'The rules on sale-note lead back to sale-note itself, so nothing can decide whether to show it. '
	+ 'Point one of them at another field.'

/**
 * Serves the article details group holding the given fields.
 * @param fields - The fields the group holds.
 */
function serve(fields: unknown[]) {
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: [{ ...DETAILS, fields }] })))
}

/**
 * Shows the Conditional logic tab of an open field.
 * @param part - The row holding the open part.
 * @returns The panel holding the field's rules.
 */
async function showRules(part: HTMLElement) {
	await userEvent.click(within(part).getByRole('tab', { name: RULES_TAB }))
	return within(part).getByRole('tabpanel', { name: RULES_TAB })
}

/**
 * Opens the page of the group, the named field in place, then its Conditional logic tab.
 * @param named - The field whose rules to show.
 * @returns The main work of the page, the row holding the open part and the panel holding the rules.
 */
async function openRules(named: string) {
	const main = await openFields()
	const part = await openField(main, named)
	return { main, part, panel: await showRules(part) }
}

/**
 * Deletes the On sale field from its row, the groups served without it once it went.
 * @param main - The main work of the page.
 */
async function deleteOnSale(main: HTMLElement) {
	server.use(
		http.delete('/api/groups/3/fields/on-sale', () => {
			serve([SALE_NOTE])
			return new HttpResponse(null, { status: 204 })
		}),
	)
	await userEvent.click(within(main).getByRole('button', { name: 'Delete On sale' }))
	const warning = await screen.findByRole('alertdialog', { name: 'Delete "On sale"?' })
	await userEvent.click(within(warning).getByRole('button', { name: 'Delete the field' }))
}

/**
 * Records the body of every field edit the page sends, answering each with the sale note.
 * @returns The bodies sent so far.
 */
function recordingEdits(): unknown[] {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/groups/3/fields/:key', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json(SALE_NOTE)
		}),
	)
	return sent
}

/**
 * Returns the body the header Save sends for a field whose settings alone moved.
 * @param settings - The settings the field holds.
 * @param label - The label the field holds.
 * @returns The body.
 */
function editOf(settings: Record<string, unknown>, label = 'Sale note') {
	return { label, required: false, settings, updated_at: READ_STAMP }
}

beforeEach(() => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [POST_TYPE] })))
	serve(DETAILS.fields)
	server.use(
		http.get('/api/definitions/drift', () => HttpResponse.json({ orphans: [], collisions: [], nesting_kept: [] })),
	)
})

test('offers a Conditional logic tab last on a field whose sibling a rule can read, its rules in a form', async () => {
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Sale note')

	const tabs = within(part).getAllByRole('tab')
	expect(tabs.map((tab) => tab.textContent)).toEqual(['General', 'Validation', 'Presentation', 'Conditional logic'])
	const panel = await showRules(part)
	expect(panel).toHaveClass('godmin-form')
	expect(within(panel).getByText('Without a rule this field always shows.')).toBeInTheDocument()
})

test('offers no Conditional logic tab while no sibling can be read, but one on the sibling reading it', async () => {
	const author = { ...ON_SALE, key: 'author', label: 'Author', kind: 'relation', relates_to: 'post' }
	serve([PHOTO, author])
	renderAt('/field-groups')
	const main = await openFields()

	const part = await openField(main, 'Photo')
	const reading = await openField(main, 'Author')

	expect(within(part).queryByRole('tab', { name: RULES_TAB })).toBeNull()
	expect(within(part).getByRole('button', { name: 'Close field' })).toBeInTheDocument()
	expect(within(reading).getByRole('tab', { name: RULES_TAB })).toBeInTheDocument()
})

test('reads the siblings of a sub field only, never the fields beside its container', async () => {
	const fee = { ...SALE_NOTE, key: 'fee', label: 'Fee' }
	const crew = { ...ON_SALE, key: 'crew', label: 'Crew', kind: 'section', fields: [fee] }
	serve([ON_SALE, crew])
	renderAt('/field-groups')
	const main = await openFields()
	const container = await openField(main, 'Crew')

	const part = await openField(main, 'Fee')

	expect(within(part).queryByRole('tab', { name: RULES_TAB })).toBeNull()
	expect(within(container).getAllByRole('tab', { name: RULES_TAB })).toHaveLength(1)
})

test('shows General once the last sibling a rule reads is deleted while Conditional logic is shown', async () => {
	serve([ON_SALE, SALE_NOTE])
	renderAt('/field-groups')
	const { main, part } = await openRules('Sale note')

	await deleteOnSale(main)

	await waitFor(() => expect(within(part).queryByRole('tab', { name: RULES_TAB })).toBeNull())
	expect(within(part).getByRole('tab', { name: 'General' })).toHaveAttribute('aria-selected', 'true')
	expect(within(part).getByRole('textbox', { name: 'Label' })).toHaveValue('Sale note')
})

test('keeps General shown once a sibling a rule reads is added after the last one went', async () => {
	const summary = { ...SALE_NOTE, key: 'summary', label: 'Summary' }
	serve([ON_SALE, SALE_NOTE])
	server.use(
		http.post('/api/groups/3/fields', () => {
			serve([SALE_NOTE, summary])
			return HttpResponse.json(summary, { status: 201 })
		}),
	)
	renderAt('/field-groups')
	const { main, part } = await openRules('Sale note')
	await deleteOnSale(main)
	await waitFor(() => expect(within(part).queryByRole('tab', { name: RULES_TAB })).toBeNull())

	const add = within(main).getByRole('button', { name: 'Add field' })
	await userEvent.type(within(add.closest('form') as HTMLFormElement).getByRole('textbox', { name: 'Label' }), 'Summary')
	await userEvent.click(add)

	expect(await within(part).findByRole('tab', { name: RULES_TAB })).toHaveAttribute('aria-selected', 'false')
	expect(within(part).getByRole('tab', { name: 'General' })).toHaveAttribute('aria-selected', 'true')
})

test('offers no Rules button and no rules dialog on an open field', async () => {
	renderAt('/field-groups')
	const main = await openFields()

	const part = await openField(main, 'Sale note')

	expect(within(part).queryByRole('button', { name: /Rules/ })).toBeNull()
	expect(screen.queryByRole('dialog')).toBeNull()
})

test('names the Conditional logic tab Active while the field holds stored rules, and no other tab', async () => {
	serve([ON_SALE, { ...SALE_NOTE, settings: { conditions: [[WHILE_ON_SALE]] } }])
	renderAt('/field-groups')
	const main = await openFields()
	const ruled = await openField(main, 'Sale note')
	const plain = await openField(main, 'On sale')

	const tab = within(ruled).getByRole('tab', { name: RULES_TAB })
	expect([...within(tab).getByText('Active').classList]).toEqual(['gophenberg-conditions__active'])
	expect(within(ruled).getAllByText('Active')).toHaveLength(1)
	expect(within(plain).getByRole('tab', { name: RULES_TAB })).toHaveTextContent(/^Conditional logic$/)
})

test('puts Active on the tab once the draft holds a rule, and takes it away once the last rule goes', async () => {
	renderAt('/field-groups')
	const { part, panel } = await openRules('Sale note')
	const tab = within(part).getByRole('tab', { name: RULES_TAB })
	expect(within(tab).queryByText('Active')).toBeNull()

	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	expect(within(tab).getByText('Active')).toBeInTheDocument()

	await userEvent.click(within(panel).getByRole('button', { name: FIRST_TRASH }))
	expect(within(tab).queryByText('Active')).toBeNull()
})

test('sends a drafted rule in the field\'s one request from the header Save, beside its other edits', async () => {
	const sent = recordingEdits()
	renderAt('/field-groups')
	const { main, part, panel } = await openRules('Sale note')

	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	await showTab(part, 'General')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), ' line')
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toEqual([editOf({ conditions: [[WHILE_ON_SALE]] }, 'Sale note line')]))
})

test('keeps the settings the tab does not touch', async () => {
	serve([ON_SALE, { ...SALE_NOTE, settings: { instructions: 'Fill me in' } }])
	const sent = recordingEdits()
	renderAt('/field-groups')
	const { main, panel } = await openRules('Sale note')

	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(sent).toEqual([editOf({ instructions: 'Fill me in', conditions: [[WHILE_ON_SALE]] })]),
	)
})

test('marks the field changed once a rule is drafted, and unchanged once the rule is taken away again', async () => {
	renderAt('/field-groups')
	const { main, panel } = await openRules('Sale note')
	expect(saveOf(main)).toHaveAttribute('aria-disabled', 'true')

	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	expect(saveOf(main)).not.toHaveAttribute('aria-disabled', 'true')
	const label = (await fieldRow(main, 'Sale note') as HTMLTableRowElement).cells[0]
	expect(within(label).getByText('Unsaved changes')).toBeInTheDocument()

	await userEvent.click(within(panel).getByRole('button', { name: FIRST_TRASH }))
	expect(saveOf(main)).toHaveAttribute('aria-disabled', 'true')
	expect(within(label).queryByText('Unsaved changes')).toBeNull()
})

test('asks before the page is left while only a drafted rule is held', async () => {
	const { router } = renderRoutedAt('/field-groups')
	const { main, panel } = await openRules('Sale note')
	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))

	const trail = within(main).getByRole('navigation', { name: 'Breadcrumbs' })
	await userEvent.click(within(trail).getByRole('link', { name: 'Field Groups' }))

	expect(await screen.findByRole('alertdialog', { name: 'Discard changes?' })).toBeInTheDocument()
	expect(router.state.location.pathname).toBe('/field-groups/3')
})

test('takes a drafted rule back on Discard changes, the stored rules shown once the field opens again', async () => {
	const sent = recordingEdits()
	renderAt('/field-groups')
	const { main, part, panel } = await openRules('Sale note')
	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))

	await userEvent.click(within(part).getByRole('button', { name: 'Discard changes' }))
	await userEvent.click(toggleOf(await fieldRow(main, 'Sale note'), 'Sale note'))

	const reopened = await showRules(part)
	expect(within(reopened).getByText('Without a rule this field always shows.')).toBeInTheDocument()
	expect(within(part).getByRole('tab', { name: RULES_TAB })).toHaveTextContent(/^Conditional logic$/)
	expect(saveOf(main)).toHaveAttribute('aria-disabled', 'true')
	expect(sent).toEqual([])
})

test('takes a stored rule away, sending the settings without it', async () => {
	serve([ON_SALE, { ...SALE_NOTE, settings: { conditions: [[WHILE_ON_SALE]] } }])
	const sent = recordingEdits()
	renderAt('/field-groups')
	const { main, panel } = await openRules('Sale note')

	await userEvent.click(within(panel).getByRole('button', { name: FIRST_TRASH }))
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toEqual([editOf({})]))
})

test('takes away the rule whose trash is pressed, the focus on the rule taking its place', async () => {
	const sent = recordingEdits()
	renderAt('/field-groups')
	const { main, panel } = await openRules('Sale note')
	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(panel).getByRole('button', { name: 'Add condition' }))
	const second = within(panel).getByRole('group', { name: 'Rule 2 of set 1' })
	await userEvent.click(within(second).getByRole('combobox', { name: 'Condition' }))
	await userEvent.click(await screen.findByRole('option', { name: 'is not' }))

	await userEvent.click(within(panel).getByRole('button', { name: FIRST_TRASH }))

	const taking = within(panel).getByRole('group', { name: 'Rule 1 of set 1' })
	expect(taking).toHaveFocus()
	expect(within(taking).getByRole('combobox', { name: 'Condition' })).toHaveTextContent('is not')
	expect(within(panel).queryByRole('group', { name: 'Rule 2 of set 1' })).not.toBeInTheDocument()
	await userEvent.click(saveOf(main))
	await waitFor(() => expect(sent).toEqual([editOf({ conditions: [[WHILE_NOT_ON_SALE]] })]))
})

test('moves the focus to the rule before, the set before, then Add rule set as trashes empty them', async () => {
	renderAt('/field-groups')
	const { panel } = await openRules('Sale note')
	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(panel).getByRole('button', { name: 'Add condition' }))
	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))

	await userEvent.click(within(panel).getByRole('button', { name: 'Remove rule 2 of set 1' }))
	expect(within(panel).getByRole('group', { name: 'Rule 1 of set 1' })).toHaveFocus()
	await userEvent.click(within(panel).getByRole('button', { name: 'Remove rule 1 of set 2' }))
	expect(within(panel).getByRole('group', { name: 'Rule set 1' })).toHaveFocus()
	await userEvent.click(within(panel).getByRole('button', { name: FIRST_TRASH }))

	expect(within(panel).getByRole('button', { name: 'Add rule set' })).toHaveFocus()
	expect(within(panel).getByText('Without a rule this field always shows.')).toBeInTheDocument()
})

test('moves the focus into the rule set and the rule its add buttons make', async () => {
	renderAt('/field-groups')
	const { panel } = await openRules('Sale note')

	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	expect(within(panel).getByRole('group', { name: 'Rule set 1' })).toHaveFocus()
	await userEvent.click(within(panel).getByRole('button', { name: 'Add condition' }))

	expect(within(panel).getByRole('group', { name: 'Rule 2 of set 1' })).toHaveFocus()
})

test('keeps a drafted rule after a rejected save, naming why above the tabs, and sends it again', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch('/api/groups/3/fields/:key', async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ error: 'loop', code: 'rule_cycle', meta: { field: 'sale-note' } }, { status: 422 })
		}),
	)
	renderAt('/field-groups')
	const { main, part, panel } = await openRules('Sale note')
	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(panel).getByRole('combobox', { name: 'Value' }))
	await userEvent.click(await screen.findByRole('option', { name: 'No' }))

	await userEvent.click(saveOf(main))

	expect(await findToast(`Could not save Sale note. ${CYCLE}`)).toBeInTheDocument()
	expect(within(part).getByRole('alert')).toHaveTextContent(CYCLE)
	expect(toggleOf(await fieldRow(main, 'Sale note'), 'Sale note')).toHaveAttribute('aria-expanded', 'true')
	expect(within(panel).getByRole('combobox', { name: 'Value' })).toHaveTextContent('No')
	await userEvent.click(saveOf(main))
	await waitFor(() => expect(sent).toHaveLength(2))
	expect(sent[1]).toEqual(sent[0])
	expect(sent[1]).toEqual(editOf({ conditions: [[{ ...WHILE_ON_SALE, value: 'false' }]] }))
})

test('keeps a drafted rule after a stale save, and marks the rules the saved version won', async () => {
	const theirs = { ...SALE_NOTE, settings: { conditions: [[WHILE_NOT_ON_SALE]] }, updated_at: OTHER_SAVE_STAMP }
	server.use(
		http.patch('/api/groups/3/fields/:key', () => {
			serve([ON_SALE, theirs, PHOTO])
			return HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 })
		}),
	)
	renderAt('/field-groups')
	const { main, part, panel } = await openRules('Sale note')
	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(saveOf(main))
	await within(part).findByRole('button', { name: 'Load saved version' })
	expect(within(panel).getByRole('combobox', { name: 'Condition' })).toHaveTextContent(/^is$/)

	await userEvent.click(within(part).getByRole('button', { name: 'Load saved version' }))

	await waitFor(() => expect(within(part).queryByRole('button', { name: 'Load saved version' })).toBeNull())
	expect(within(panel).getByRole('combobox', { name: 'Condition' })).toHaveTextContent('is not')
	expect(within(panel).getByText('Changed elsewhere too. Your edit was not kept.')).toBeInTheDocument()
})

test('stops a rule save past the time limit the site serves, keeping the drafted rule', async () => {
	server.use(
		http.get('/api/settings', () =>
			HttpResponse.json({ ...siteSettings, dialog_write_timeout_milliseconds: SHORT_LIMIT }),
		),
		http.patch('/api/groups/3/fields/sale-note', () => new Promise<never>(() => {})),
	)
	renderAt('/field-groups')
	const { main, panel } = await openRules('Sale note')
	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))

	await userEvent.click(saveOf(main))

	expect(await findToast('Could not save Sale note. The field groups could not be reached.')).toBeInTheDocument()
	expect(within(panel).getByRole('group', { name: 'Rule 1 of set 1' })).toBeInTheDocument()
})

test('keeps every rule of a field read only while the header Save runs', async () => {
	const answer = gate()
	server.use(
		http.patch('/api/groups/3/fields/:key', async () => {
			await answer.held
			return HttpResponse.json(SALE_NOTE)
		}),
	)
	renderAt('/field-groups')
	const { main, panel } = await openRules('Sale note')
	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))

	await userEvent.click(saveOf(main))
	await waitBusy(main, 'Save')

	expectReadOnly(panel)
	for (const name of [FIRST_TRASH, 'Add condition', 'Add rule set']) {
		expect(within(panel).getByRole('button', { name })).toHaveAttribute('aria-disabled', 'true')
	}
	answer.release()
	await waitFor(() => expect(saveOf(main)).not.toHaveClass(/__is-loading\b/))
})

test('offers the siblings a rule may read, leaving the field itself out', async () => {
	renderAt('/field-groups')
	const { panel } = await openRules('Sale note')

	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))

	const sources = within(panel).getByRole('combobox', { name: 'Source' })
	expect(within(sources).getByText('On sale')).toBeInTheDocument()
})

test('carries a rule onto another source, starting its condition afresh', async () => {
	const price = { ...ON_SALE, key: 'price', label: 'Price', kind: 'number' }
	serve([ON_SALE, price, SALE_NOTE])
	const sent = recordingEdits()
	renderAt('/field-groups')
	const { main, panel } = await openRules('Sale note')

	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(panel).getByRole('combobox', { name: 'Source' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Price' }))
	await userEvent.type(within(panel).getByRole('spinbutton', { name: 'Value' }), '10')
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(sent).toEqual([editOf({ conditions: [[{ source: 'price', operator: '==', value: '10' }]] })]),
	)
})

test('carries a rule onto another condition and another value', async () => {
	const sent = recordingEdits()
	renderAt('/field-groups')
	const { main, panel } = await openRules('Sale note')

	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(panel).getByRole('combobox', { name: 'Condition' }))
	await userEvent.click(await screen.findByRole('option', { name: 'is not' }))
	await userEvent.click(within(panel).getByRole('combobox', { name: 'Value' }))
	await userEvent.click(await screen.findByRole('option', { name: 'No' }))
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toEqual([editOf({ conditions: [[{ ...WHILE_NOT_ON_SALE, value: 'false' }]] })]))
})

test('holds a second condition in the same rule set', async () => {
	const sent = recordingEdits()
	renderAt('/field-groups')
	const { main, panel } = await openRules('Sale note')

	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(panel).getByRole('button', { name: 'Add condition' }))
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toEqual([editOf({ conditions: [[WHILE_ON_SALE, WHILE_ON_SALE]] })]))
})

test('offers the choices a choice source holds', async () => {
	const kind = { ...ON_SALE, key: 'kind', label: 'Kind', kind: 'choice', settings: { choices: [
		{ value: 'book', label: 'Book' },
	] } }
	serve([kind, SALE_NOTE])
	renderAt('/field-groups')
	const { panel } = await openRules('Sale note')

	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))

	const values = within(panel).getByRole('combobox', { name: 'Value' })
	expect(within(values).getByText('Book')).toBeInTheDocument()
})

test('takes a typed value for a source that offers no choices, Enter in it running the header Save', async () => {
	serve([{ ...ON_SALE, kind: 'text' }, SALE_NOTE])
	const sent = recordingEdits()
	renderAt('/field-groups')
	const { panel } = await openRules('Sale note')

	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	await userEvent.type(within(panel).getByRole('textbox', { name: 'Value' }), 'yes{Enter}')

	await waitFor(() => expect(sent).toEqual([editOf({ conditions: [[{ ...WHILE_ON_SALE, value: 'yes' }]] })]))
})

test('asks for no value beside an operator that needs none', async () => {
	serve([{ ...ON_SALE, kind: 'text' }, SALE_NOTE])
	const sent = recordingEdits()
	renderAt('/field-groups')
	const { main, panel } = await openRules('Sale note')

	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(panel).getByRole('combobox', { name: 'Condition' }))
	await userEvent.click(await screen.findByRole('option', { name: 'is empty' }))

	expect(within(panel).queryByRole('textbox', { name: 'Value' })).not.toBeInTheDocument()
	await userEvent.click(saveOf(main))
	await waitFor(() =>
		expect(sent).toEqual([editOf({ conditions: [[{ source: 'on-sale', operator: 'empty', value: '' }]] })]),
	)
})

test('edits one rule set without disturbing the other', async () => {
	const sent = recordingEdits()
	renderAt('/field-groups')
	const { main, panel } = await openRules('Sale note')

	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	const second = within(panel).getByRole('group', { name: 'Rule set 2' })
	await userEvent.click(within(second).getByRole('button', { name: 'Add condition' }))
	const rows = within(second).getAllByRole('combobox', { name: 'Condition' })
	await userEvent.click(rows[1] as HTMLElement)
	await userEvent.click(await screen.findByRole('option', { name: 'is not' }))
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(sent).toEqual([editOf({ conditions: [[WHILE_ON_SALE], [WHILE_ON_SALE, WHILE_NOT_ON_SALE]] })]),
	)
})

test('takes one rule set away without disturbing the other', async () => {
	const sent = recordingEdits()
	renderAt('/field-groups')
	const { main, panel } = await openRules('Sale note')

	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	const second = within(panel).getByRole('group', { name: 'Rule set 2' })
	await userEvent.click(within(second).getByRole('combobox', { name: 'Condition' }))
	await userEvent.click(await screen.findByRole('option', { name: 'is not' }))
	const first = within(panel).getByRole('group', { name: 'Rule set 1' })
	await userEvent.click(within(first).getByRole('button', { name: FIRST_TRASH }))
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toEqual([editOf({ conditions: [[WHILE_NOT_ON_SALE]] })]))
})

test('shows a stored rule naming a sibling it cannot read as stored, with no condition to choose from', async () => {
	serve([ON_SALE, { ...SALE_NOTE, settings: { conditions: [[{ source: 'vanished', operator: '~', value: 'x' }]] } }])
	renderAt('/field-groups')

	const { panel } = await openRules('Sale note')

	const sources = within(panel).getByRole('combobox', { name: 'Source' })
	expect(within(sources).getByText('vanished')).toBeInTheDocument()
	const operators = within(panel).getByRole('combobox', { name: 'Condition' })
	expect(within(operators).getByText('~')).toBeInTheDocument()
	expect(within(panel).getByRole('combobox', { name: 'Value' })).toHaveTextContent('x')
	await userEvent.click(operators)
	expect(await screen.findByRole('listbox')).toBeInTheDocument()
	expect(screen.queryByRole('option', { name: 'is' })).not.toBeInTheDocument()
})

test('names each rule set above its rules and each rule by its place, each add button at its own width', async () => {
	renderAt('/field-groups')
	const { panel } = await openRules('Sale note')

	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	const second = within(panel).getByRole('group', { name: 'Rule set 2' })
	await userEvent.click(within(second).getByRole('button', { name: 'Add condition' }))

	expectRuleSets(panel, [1, 2])
})

test('names the conditions and the values of a rule in the language the admin reads as the rules open', async () => {
	renderAt('/field-groups')
	const part = await openField(await openFields(), 'Sale note')
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))

	await userEvent.click(within(part).getByRole('tab', { name: RULES_TAB }))
	const panel = within(part).getByRole('tabpanel')
	await userEvent.click(within(panel).getByRole('button', { name: 'Añadir conjunto de reglas' }))
	await userEvent.click(within(panel).getByRole('combobox', { name: 'Condición' }))
	await userEvent.click(await screen.findByRole('option', { name: 'no es' }))

	expect(within(panel).getByRole('combobox', { name: 'Condición' })).toHaveTextContent('no es')
	expect(within(panel).getByRole('combobox', { name: 'Valor' })).toHaveTextContent('Sí')
})

test('stores the rules of a field standing inside a container', async () => {
	const paid = { ...ON_SALE, key: 'paid', label: 'Paid' }
	const fee = { ...SALE_NOTE, key: 'fee', label: 'Fee' }
	const crew = { ...ON_SALE, key: 'crew', label: 'Crew', kind: 'section', fields: [paid, fee] }
	serve([crew])
	const sent = recordingEdits()
	renderAt('/field-groups')
	const main = await openFields()
	await openField(main, 'Crew')

	const panel = await showRules(await openField(main, 'Fee'))
	await userEvent.click(within(panel).getByRole('button', { name: 'Add rule set' }))
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(sent).toEqual([editOf({ conditions: [[{ ...WHILE_ON_SALE, source: 'paid' }]] }, 'Fee')]),
	)
})

test('puts a field in the admin list from the Show in the list box of its Presentation tab', async () => {
	const sent = recordingEdits()
	renderAt('/field-groups')
	const dialog = await openFields()
	const part = await openField(dialog, 'Sale note')

	await showTab(part, 'Presentation')
	await userEvent.click(within(part).getByRole('checkbox', { name: 'Show in the list' }))
	await userEvent.click(saveOf(dialog))

	await waitFor(() =>
		expect(sent).toEqual([
			{ label: 'Sale note', required: false, settings: { listed: true }, updated_at: '2026-08-01T10:00:00Z' },
		]),
	)
})

test('takes a field out of the admin list again', async () => {
	const listed = { ...SALE_NOTE, settings: { listed: true } }
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: [{ ...DETAILS, fields: [listed] }] })))
	const sent = recordingEdits()
	renderAt('/field-groups')
	const dialog = await openFields()
	const part = await openField(dialog, 'Sale note')

	await showTab(part, 'Presentation')
	const box = within(part).getByRole('checkbox', { name: 'Show in the list' })
	expect(box).toBeChecked()
	await userEvent.click(box)
	await userEvent.click(saveOf(dialog))

	await waitFor(() =>
		expect(sent).toEqual([
			{ label: 'Sale note', required: false, settings: { listed: false }, updated_at: '2026-08-01T10:00:00Z' },
		]),
	)
})

test('counts no change once the Show in the list box is ticked and cleared again', async () => {
	renderAt('/field-groups')
	const dialog = await openFields()
	const part = await openField(dialog, 'Sale note')
	await showTab(part, 'Presentation')
	const box = within(part).getByRole('checkbox', { name: 'Show in the list' })

	await userEvent.click(box)
	expect(saveOf(dialog)).not.toHaveAttribute('aria-disabled', 'true')
	await userEvent.click(box)

	expect(saveOf(dialog)).toHaveAttribute('aria-disabled', 'true')
})

test('offers no Show in the list box inside a container, where the list never reaches', async () => {
	const paid = { ...ON_SALE, key: 'paid', label: 'Paid' }
	const crew = { ...ON_SALE, key: 'crew', label: 'Crew', kind: 'section', fields: [paid] }
	server.use(http.get('/api/groups', () => HttpResponse.json({ items: [{ ...DETAILS, fields: [crew] }] })))
	renderAt('/field-groups')
	const dialog = await openFields()
	await openField(dialog, 'Crew')

	const part = await openField(dialog, 'Paid')

	await showTab(part, 'Presentation')
	expect(within(part).queryByRole('checkbox', { name: 'Show in the list' })).not.toBeInTheDocument()
})

test('offers no Show in the list box on a kind the list cannot show', async () => {
	renderAt('/field-groups')
	const dialog = await openFields()

	const part = await openField(dialog, 'Photo')

	await showTab(part, 'Presentation')
	expect(within(part).queryByRole('checkbox', { name: 'Show in the list' })).not.toBeInTheDocument()
})
