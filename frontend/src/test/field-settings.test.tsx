// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { act, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { groupsQueryKey } from '../content/groups'
import { gate } from './gate'
import {
	DETAILS,
	EXTRAS,
	FEATURES,
	fieldRow,
	HERO,
	listing,
	openField,
	openFields,
	READING_TIME,
	saveOf,
	serveFieldGroups,
	showTab,
	SUBTITLE,
	toggleOf,
} from './groupFieldsFixtures'
import { renderAt, renderRoutedAt } from './render'
import { siteSettings } from './siteSettings'
import { expectReadOnly, findToast, waitBusy } from './typeFixtures'

const READ_STAMP = '2026-08-01T10:00:00Z'

const OTHER_SAVE_STAMP = '2026-08-02T09:00:00Z'

/** A time limit far shorter than any test. */
const SHORT_LIMIT = 20

serveFieldGroups()

/**
 * Returns how many milliseconds pass, so a request that went out has the time to arrive.
 * @returns A promise settling once the time has passed.
 */
function aMoment() {
	return act(() => new Promise((resolve) => setTimeout(resolve, 50)))
}

/**
 * Reports whether the header Save showed the spinner of a run while a step ran or just after.
 * @param main - The main work of the page.
 * @param step - The step to watch.
 * @returns Whether a run started.
 */
async function turnedBusy(main: HTMLElement, step: () => Promise<void>): Promise<boolean> {
	const save = saveOf(main)
	let busy = false
	const watch = new MutationObserver(() => {
		busy ||= /__is-loading\b/.test(save.className)
	})
	watch.observe(save, { attributes: true, attributeFilter: ['class'] })
	await step()
	await aMoment()
	watch.disconnect()
	return busy
}

/**
 * Returns what the header Save shows: off, on, or absent from a page with nothing to save.
 * @param main - The main work of the page.
 * @returns The state of the button.
 */
function saveState(main: HTMLElement): 'off' | 'on' | 'absent' {
	const save = within(main).queryByRole('button', { name: 'Save' })
	if (save === null) {
		return 'absent'
	}
	return save.getAttribute('aria-disabled') === 'true' ? 'off' : 'on'
}

/** One field edit the page sent: the dotted path it named and the body it carried. */
interface SentEdit {
	path: string
	body: unknown
}

/**
 * Records every field edit the page sends, answering each with the field as stored.
 * @param stored - The field each edit answers with.
 * @returns The edits sent so far.
 */
function recordingEdits(stored: object = SUBTITLE): SentEdit[] {
	const sent: SentEdit[] = []
	server.use(
		http.patch('/api/groups/3/fields/:path', async ({ request, params }) => {
			sent.push({ path: String(params.path), body: await request.json() })
			return HttpResponse.json(stored)
		}),
	)
	return sent
}

/**
 * Returns the Label cell of a field's own row.
 * @param row - The field's own row.
 * @returns The cell.
 */
function labelOf(row: HTMLElement) {
	return (row as HTMLTableRowElement).cells[0]
}

/**
 * Returns the body a field edit sends when only the parts named move.
 * @param field - The field as read.
 * @param field.label - The label the field holds.
 * @param field.required - Whether the field is required.
 * @param moved - The parts the edit changes.
 * @returns The body.
 */
function editOf(field: { label: string; required: boolean }, moved: Record<string, unknown>) {
	return { label: field.label, required: field.required, settings: {}, updated_at: READ_STAMP, ...moved }
}

test('sends the new label of a field from the header Save without moving what is stored under it', async () => {
	const sent = recordingEdits()
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')

	const box = within(part).getByRole('textbox', { name: 'Label' })
	await userEvent.clear(box)
	await userEvent.type(box, 'Standfirst')
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toEqual([{ path: 'subtitle', body: editOf(SUBTITLE, { label: 'Standfirst' }) }]))
})

test('requires a field that was optional from the Required box of its Validation tab', async () => {
	const sent = recordingEdits()
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')

	await showTab(part, 'Validation')
	await userEvent.click(within(part).getByRole('checkbox', { name: 'Required' }))
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toEqual([{ path: 'subtitle', body: editOf(SUBTITLE, { required: true }) }]))
})

test('makes a required field optional again', async () => {
	const sent = recordingEdits(READING_TIME)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Reading time')

	await showTab(part, 'Validation')
	const required = within(part).getByRole('checkbox', { name: 'Required' })
	expect(required).toBeChecked()
	await userEvent.click(required)
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(sent).toEqual([{ path: 'reading-time', body: editOf(READING_TIME, { required: false }) }]),
	)
})

test('sends one edit per changed field in the order the page shows them', async () => {
	const sent = recordingEdits()
	renderAt('/field-groups')
	const main = await openFields()
	const later = await openField(main, 'Reading time')
	const first = await openField(main, 'Subtitle')

	await showTab(later, 'Validation')
	await userEvent.type(within(later).getByRole('spinbutton', { name: 'Lowest' }), '1')
	await userEvent.type(within(first).getByRole('textbox', { name: 'Label' }), ' line')
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(sent).toEqual([
			{ path: 'subtitle', body: editOf(SUBTITLE, { label: 'Subtitle line' }) },
			{ path: 'reading-time', body: editOf(READING_TIME, { settings: { min: 1 } }) },
		]),
	)
})

test('sends back the settings no tab shows, so an email field stays one and keeps its rules', async () => {
	const rules = [[{ source: 'reading-time', operator: '==', value: '5' }]]
	const contact = { ...SUBTITLE, key: 'contact', label: 'Contact', settings: { variant: 'email', conditions: rules } }
	listing([{ ...DETAILS, fields: [contact, READING_TIME] }, EXTRAS])
	const sent = recordingEdits(contact)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Contact')

	await showTab(part, 'Presentation')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Instructions' }), 'Write to maria.perez@example.com')
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(sent).toEqual([
			{
				path: 'contact',
				body: editOf(contact, {
					settings: { variant: 'email', conditions: rules, instructions: 'Write to maria.perez@example.com' },
				}),
			},
		]),
	)
})

test('keeps Save off until a field changes, and off again once the change is undone', async () => {
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')
	expect(saveOf(main)).toHaveAttribute('aria-disabled', 'true')
	const box = within(part).getByRole('textbox', { name: 'Label' })

	await userEvent.type(box, 's')
	expect(saveOf(main)).not.toHaveAttribute('aria-disabled', 'true')

	await userEvent.type(box, '{Backspace}')
	expect(saveOf(main)).toHaveAttribute('aria-disabled', 'true')
})

test('asks before the page is left while only a field holds unsaved changes', async () => {
	const { router } = renderRoutedAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), ' line')

	const trail = within(main).getByRole('navigation', { name: 'Breadcrumbs' })
	await userEvent.click(within(trail).getByRole('link', { name: 'Field Groups' }))

	expect(await screen.findByRole('alertdialog', { name: 'Discard changes?' })).toBeInTheDocument()
	expect(router.state.location.pathname).toBe('/field-groups/3')
})

test('drops the draft and closes the field on Discard changes, the focus back on its label', async () => {
	const sent = recordingEdits()
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')
	expect(within(part).queryByRole('button', { name: 'Discard changes' })).toBeNull()
	const box = within(part).getByRole('textbox', { name: 'Label' })
	await userEvent.clear(box)
	await userEvent.type(box, 'Standfirst')

	await userEvent.click(within(part).getByRole('button', { name: 'Discard changes' }))

	const toggle = toggleOf(await fieldRow(main, 'Subtitle'), 'Subtitle')
	expect(toggle).toHaveAttribute('aria-expanded', 'false')
	expect(toggle).toHaveFocus()
	expect(saveOf(main)).toHaveAttribute('aria-disabled', 'true')
	await userEvent.click(toggle)
	expect(within(part).getByRole('textbox', { name: 'Label' })).toHaveValue('Subtitle')
	expect(sent).toEqual([])
})

test('keeps the draft of a field closed with Close field, shown again once it reopens', async () => {
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), ' line')

	await userEvent.click(within(part).getByRole('button', { name: 'Close field' }))
	await userEvent.click(toggleOf(await fieldRow(main, 'Subtitle'), 'Subtitle'))

	expect(within(part).getByRole('textbox', { name: 'Label' })).toHaveValue('Subtitle line')
	expect(saveOf(main)).not.toHaveAttribute('aria-disabled', 'true')
})

test('runs the header Save from Enter in a box of an open field', async () => {
	const sent = recordingEdits()
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')

	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), ' line{Enter}')

	await waitFor(() => expect(sent).toEqual([{ path: 'subtitle', body: editOf(SUBTITLE, { label: 'Subtitle line' }) }]))
	expect(await findToast('Article details updated.')).toBeInTheDocument()
})

test('closes a field once its save landed, the focus back on its label', async () => {
	recordingEdits()
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')

	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), ' line{Enter}')

	const toggle = toggleOf(await fieldRow(main, 'Subtitle'), 'Subtitle')
	await waitFor(() => expect(toggle).toHaveAttribute('aria-expanded', 'false'))
	expect(toggle).toHaveFocus()
	expect(within(labelOf(await fieldRow(main, 'Subtitle'))).queryByText('Unsaved changes')).toBeNull()
})

test('closes every open field once the header Save ends, the ones it left unchanged too', async () => {
	recordingEdits()
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')
	await openField(main, 'Reading time')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), ' line')

	await userEvent.click(saveOf(main))

	const subtitle = toggleOf(await fieldRow(main, 'Subtitle'), 'Subtitle')
	await waitFor(() => expect(subtitle).toHaveAttribute('aria-expanded', 'false'))
	expect(toggleOf(await fieldRow(main, 'Reading time'), 'Reading time')).toHaveAttribute('aria-expanded', 'false')
})

test('closes every open field once the header Save ends, though the group details were rejected', async () => {
	const said = 'content: another group already claims those items'
	server.use(
		http.patch('/api/groups/3', () => HttpResponse.json({ error: said, code: 'group_collides' }, { status: 422 })),
	)
	renderAt('/field-groups')
	const main = await openFields()
	await openField(main, 'Reading time')
	const details = within(main).getByRole('region', { name: 'Field group settings' })
	await userEvent.type(within(details).getByRole('textbox', { name: 'Title' }), ' here')

	await userEvent.click(saveOf(main))

	expect(await findToast(`Could not save the field group. ${said}`)).toBeInTheDocument()
	const toggle = toggleOf(await fieldRow(main, 'Reading time'), 'Reading time')
	await waitFor(() => expect(toggle).toHaveAttribute('aria-expanded', 'false'))
})

test('sends nothing from Enter in an open field while nothing changed', async () => {
	const sent = recordingEdits()
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Reading time')
	await showTab(part, 'Validation')

	const enter = () => userEvent.type(within(part).getByRole('spinbutton', { name: 'Lowest' }), '{Enter}')

	expect(await turnedBusy(main, enter)).toBe(false)
	expect(sent).toEqual([])
	expect(saveOf(main)).toHaveAttribute('aria-disabled', 'true')
})

test('stops a field save past the time limit the site serves, naming the field groups unreached', async () => {
	server.use(
		http.get('/api/settings', () =>
			HttpResponse.json({ ...siteSettings, dialog_write_timeout_milliseconds: SHORT_LIMIT }),
		),
		http.patch('/api/groups/3/fields/subtitle', () => new Promise<never>(() => {})),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), ' line')

	await userEvent.click(saveOf(main))

	expect(await findToast('Could not save Subtitle. The field groups could not be reached.')).toBeInTheDocument()
	expect(within(part).getByRole('textbox', { name: 'Label' })).toHaveValue('Subtitle line')
})

test('keeps every box of an open field read only and Discard changes off while the header Save runs', async () => {
	const answer = gate()
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async () => {
			await answer.held
			return HttpResponse.json(SUBTITLE)
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')
	await showTab(part, 'Presentation')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Instructions' }), 'Say who wrote it.')

	await userEvent.click(saveOf(main))
	await waitBusy(main, 'Save')

	expectReadOnly(part)
	expect(within(part).getByRole('button', { name: 'Discard changes' })).toHaveAttribute('aria-disabled', 'true')
	answer.release()
	const instructions = within(part).getByRole('textbox', { name: 'Instructions' })
	await waitFor(() => expect(instructions).not.toHaveAttribute('readonly'))
})

test('saves on the stamp the field was read at as the draft began, not one that landed after', async () => {
	const sent = recordingEdits()
	const client = renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), ' line')

	listing([{ ...DETAILS, fields: [{ ...SUBTITLE, updated_at: OTHER_SAVE_STAMP }, READING_TIME] }, EXTRAS])
	await act(async () => {
		await client.invalidateQueries({ queryKey: groupsQueryKey })
	})
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toEqual([{ path: 'subtitle', body: editOf(SUBTITLE, { label: 'Subtitle line' }) }]))
})

test('saves the same field twice in one sitting, the second time on the stamp the first save left', async () => {
	const sent: unknown[] = []
	const saved = { ...SUBTITLE, label: 'Subtitle line', updated_at: OTHER_SAVE_STAMP }
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent.push(await request.json())
			listing([{ ...DETAILS, fields: [saved, READING_TIME] }, EXTRAS])
			return HttpResponse.json(saved)
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')
	const box = within(part).getByRole('textbox', { name: 'Label' })
	await userEvent.type(box, ' line')
	await userEvent.click(saveOf(main))
	await waitFor(() => expect(box).toHaveValue('Subtitle line'))
	await waitFor(() => expect(saveOf(main)).toHaveAttribute('aria-disabled', 'true'))

	await openField(main, 'Subtitle line')
	await userEvent.type(box, ' again')
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toHaveLength(2))
	expect(sent[1]).toEqual({ label: 'Subtitle line again', required: false, settings: {}, updated_at: OTHER_SAVE_STAMP })
})

test('keeps a field open on what was typed when its save is rejected, naming why in the toast', async () => {
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json(
				{ error: 'content: settings disagree', code: 'setting_bounds', meta: { setting: 'min' } },
				{ status: 422 },
			),
		),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')
	await showTab(part, 'Presentation')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Instructions' }), 'Say who wrote it.')

	await userEvent.click(saveOf(main))

	const said = 'The setting min falls outside the others. Keep min below max and the default inside the bounds.'
	expect(await findToast(`Could not save Subtitle. ${said}`)).toBeInTheDocument()
	expect(toggleOf(await fieldRow(main, 'Subtitle'), 'Subtitle')).toHaveAttribute('aria-expanded', 'true')
	expect(within(part).getByRole('textbox', { name: 'Instructions' })).toHaveValue('Say who wrote it.')
	expect(within(part).getByRole('alert')).toHaveTextContent(said)
	expect(saveOf(main)).not.toHaveAttribute('aria-disabled', 'true')
})

test('keeps a failed field open with the words under the setting named, closing the one that saved', async () => {
	const said = 'The setting placeholder cannot hold the value it was given. Check what this setting takes and try again.'
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json(
				{ error: 'content: setting shape', code: 'setting_shape', meta: { setting: 'placeholder' } },
				{ status: 422 },
			),
		),
		http.patch('/api/groups/3/fields/reading-time', () => HttpResponse.json(READING_TIME)),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const failing = await openField(main, 'Subtitle')
	await showTab(failing, 'Presentation')
	await userEvent.type(within(failing).getByRole('textbox', { name: 'Placeholder' }), 'Maria Perez')
	await showTab(failing, 'General')
	const saving = await openField(main, 'Reading time')
	await userEvent.type(within(saving).getByRole('textbox', { name: 'Label' }), ' in minutes')

	await userEvent.click(saveOf(main))

	expect(await findToast(`Saved 1 of 2 changes. Could not save Subtitle. ${said}`)).toBeInTheDocument()
	expect(toggleOf(await fieldRow(main, 'Reading time'), 'Reading time')).toHaveAttribute('aria-expanded', 'false')
	expect(toggleOf(await fieldRow(main, 'Subtitle'), 'Subtitle')).toHaveAttribute('aria-expanded', 'true')
	expect(within(failing).getByRole('tab', { name: 'Presentation' })).toHaveAttribute('aria-selected', 'true')
	const placeholder = within(failing).getByRole('textbox', { name: 'Placeholder' })
	await waitFor(() => expect(placeholder).toHaveAccessibleDescription(said))
	expect(within(failing).queryByRole('alert')).toBeNull()
	await userEvent.type(placeholder, '!')
	expect(placeholder).toHaveAccessibleDescription('')
})

test('opens a closed field whose save was rejected, so the words show beside its draft', async () => {
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json({ error: 'content: the field group is locked' }, { status: 422 }),
		),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), ' line')
	await userEvent.click(within(part).getByRole('button', { name: 'Close field' }))

	await userEvent.click(saveOf(main))

	const toggle = toggleOf(await fieldRow(main, 'Subtitle'), 'Subtitle')
	await waitFor(() => expect(toggle).toHaveAttribute('aria-expanded', 'true'))
	expect(within(part).getByRole('alert')).toHaveTextContent('content: the field group is locked')
	expect(within(part).getByRole('textbox', { name: 'Label' })).toHaveValue('Subtitle line')
})

test('keeps the draft of a field someone else saved first, with Load saved version in its notice', async () => {
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 }),
		),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), ' line')

	await userEvent.click(saveOf(main))

	const said = 'Someone saved Subtitle after you opened it.'
	expect(await findToast(`Could not save Subtitle. ${said}`)).toBeInTheDocument()
	expect(within(part).getByRole('alert')).toHaveTextContent(said)
	expect(within(part).getByRole('button', { name: 'Load saved version' })).toBeInTheDocument()
	expect(within(part).getByRole('textbox', { name: 'Label' })).toHaveValue('Subtitle line')
	expect(within(labelOf(await fieldRow(main, 'Subtitle'))).getByText('Unsaved changes')).toBeInTheDocument()
})

test('moves a stale draft onto the saved version, keeping its own changes and marking the one both made', async () => {
	const theirs = {
		...SUBTITLE,
		settings: { instructions: 'Say who wrote it.', default: 'Their default', listed: false },
		updated_at: OTHER_SAVE_STAMP,
	}
	const sent: unknown[] = []
	server.use(
		http.patch('/api/groups/3/fields/subtitle', async ({ request }) => {
			sent.push(await request.json())
			if (sent.length === 1) {
				listing([{ ...DETAILS, fields: [theirs, READING_TIME] }, EXTRAS])
				return HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 })
			}
			return HttpResponse.json(theirs)
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')
	await showTab(part, 'Presentation')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Placeholder' }), 'Maria Perez')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Instructions' }), 'Write to maria.perez@example.com')
	await userEvent.click(within(part).getByRole('checkbox', { name: 'Show in the list' }))
	await userEvent.click(saveOf(main))
	await within(part).findByRole('button', { name: 'Load saved version' })

	await userEvent.click(within(part).getByRole('button', { name: 'Load saved version' }))

	const lost = 'Changed elsewhere too. Your edit was not kept.'
	await waitFor(() => expect(within(part).queryByRole('button', { name: 'Load saved version' })).toBeNull())
	expect(within(part).getByRole('textbox', { name: 'Placeholder' })).toHaveValue('Maria Perez')
	expect(within(part).getByRole('textbox', { name: 'Placeholder' })).toHaveAccessibleDescription('')
	const instructions = within(part).getByRole('textbox', { name: 'Instructions' })
	expect(instructions).toHaveValue('Say who wrote it.')
	expect(instructions).toHaveAccessibleDescription(lost)
	const listed = within(part).getByRole('checkbox', { name: 'Show in the list' })
	expect(listed).not.toBeChecked()
	expect(listed.closest('label')?.nextElementSibling).toHaveTextContent(lost)
	await showTab(part, 'General')
	const fallback = within(part).getByRole('textbox', { name: 'Default' })
	expect(fallback).toHaveValue('Their default')
	expect(fallback).toHaveAccessibleDescription('')
	await userEvent.click(saveOf(main))
	await waitFor(() => expect(sent).toHaveLength(2))
	expect(sent[1]).toEqual({
		label: 'Subtitle',
		required: false,
		settings: { instructions: 'Say who wrote it.', default: 'Their default', listed: false, placeholder: 'Maria Perez' },
		updated_at: OTHER_SAVE_STAMP,
	})
})

test('takes the notice of a rejected save away once the field drops its draft', async () => {
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json({ error: 'content: the field group is locked' }, { status: 422 }),
		),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), ' line')
	await userEvent.click(saveOf(main))
	expect(await within(part).findByRole('alert')).toHaveTextContent('content: the field group is locked')

	await userEvent.click(within(part).getByRole('button', { name: 'Discard changes' }))
	await openField(main, 'Subtitle')

	expect(within(part).queryByRole('alert')).toBeNull()
})

test.each([
	{ gone: 'the field', left: [{ ...DETAILS, fields: [READING_TIME] }, EXTRAS], shows: 'Reading time', save: 'off' },
	{ gone: 'its group', left: [EXTRAS], shows: 'This field group does not exist', save: 'absent' },
])('lets a stale draft go once Load saved version finds someone deleted $gone', async (asked) => {
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () => {
			listing(asked.left)
			return HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 })
		}),
	)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Label' }), ' line')
	await userEvent.click(saveOf(main))

	await userEvent.click(await within(part).findByRole('button', { name: 'Load saved version' }))

	expect(await within(main).findByText(new RegExp(asked.shows))).toBeInTheDocument()
	expect(within(main).queryByRole('button', { name: 'Subtitle' })).toBeNull()
	expect(saveState(main)).toBe(asked.save)
})

test('offers the settings a text field takes on their tabs and stores them', async () => {
	const sent = recordingEdits()
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')

	expect(within(part).getByRole('textbox', { name: 'Default' })).toBeInTheDocument()
	await showTab(part, 'Presentation')
	expect(within(part).getByRole('textbox', { name: 'Placeholder' })).toBeInTheDocument()
	expect(within(part).getByRole('textbox', { name: 'Instructions' }).tagName).toBe('TEXTAREA')
	await userEvent.type(within(part).getByRole('textbox', { name: 'Instructions' }), 'Say who wrote it.')
	await showTab(part, 'Validation')
	await userEvent.type(within(part).getByRole('spinbutton', { name: 'Longest' }), '80')
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(sent).toEqual([
			{ path: 'subtitle', body: editOf(SUBTITLE, { settings: { instructions: 'Say who wrote it.', maxlength: 80 } }) },
		]),
	)
})

test('offers a number field its own bounds', async () => {
	listing([{ ...DETAILS, fields: [READING_TIME] }, EXTRAS])
	const sent = recordingEdits(READING_TIME)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Reading time')

	expect(within(part).getByRole('spinbutton', { name: 'Default' })).toBeInTheDocument()
	await showTab(part, 'Validation')
	await userEvent.type(within(part).getByRole('spinbutton', { name: 'Lowest' }), '1')
	await userEvent.type(within(part).getByRole('spinbutton', { name: 'Highest' }), '10')
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(sent).toEqual([{ path: 'reading-time', body: editOf(READING_TIME, { settings: { min: 1, max: 10 } }) }]),
	)
})

test('offers a number field its steps and no placeholder, which its control never shows', async () => {
	listing([{ ...DETAILS, fields: [READING_TIME] }])
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Reading time')

	await showTab(part, 'Presentation')

	expect(within(part).getByRole('spinbutton', { name: 'Steps of' })).toBeInTheDocument()
	expect(within(part).queryByRole('textbox', { name: 'Placeholder' })).toBeNull()
})

test('offers a media field instructions and none of the bounds of a text', async () => {
	listing([{ ...DETAILS, fields: [{ ...SUBTITLE, key: 'cover', label: 'Cover', kind: 'media' }] }])
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Cover')

	expect(within(part).queryByRole('textbox', { name: 'Default' })).toBeNull()
	await showTab(part, 'Validation')
	expect(within(part).queryByRole('spinbutton', { name: 'Longest' })).toBeNull()
	expect(within(part).queryByRole('spinbutton', { name: 'Fewest files' })).toBeNull()
	await showTab(part, 'Presentation')
	expect(within(part).getByRole('textbox', { name: 'Instructions' })).toBeInTheDocument()
})

test('shows the settings a field already carries', async () => {
	listing([{ ...DETAILS, fields: [{ ...SUBTITLE, settings: { instructions: 'Held.', maxlength: 40 } }] }])
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')

	await showTab(part, 'Presentation')
	expect(within(part).getByRole('textbox', { name: 'Instructions' })).toHaveValue('Held.')
	await showTab(part, 'Validation')
	expect(within(part).getByRole('spinbutton', { name: 'Longest' })).toHaveValue(40)
})

test('drops a setting whose box is emptied', async () => {
	const held = { ...SUBTITLE, settings: { instructions: 'Held.' } }
	listing([{ ...DETAILS, fields: [held] }, EXTRAS])
	const sent = recordingEdits(held)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')

	await showTab(part, 'Presentation')
	await userEvent.clear(within(part).getByRole('textbox', { name: 'Instructions' }))
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toEqual([{ path: 'subtitle', body: editOf(SUBTITLE, { settings: {} }) }]))
})

test('offers a choice field its flags on their tabs', async () => {
	const style = { ...SUBTITLE, key: 'style', label: 'Style', kind: 'choice' }
	listing([{ ...DETAILS, fields: [style] }])
	const sent = recordingEdits(style)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Style')

	expect(within(part).getByRole('combobox', { name: 'Many values' })).toBeInTheDocument()
	await showTab(part, 'Validation')
	expect(within(part).getByRole('combobox', { name: 'Allow empty' })).toBeInTheDocument()
	await showTab(part, 'Presentation')
	await userEvent.click(within(part).getByRole('combobox', { name: 'Allow custom' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Yes' }))
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(sent).toEqual([{ path: 'style', body: editOf(style, { settings: { allow_custom: true } }) }]),
	)
})

test('offers a boolean field the value it starts on', async () => {
	const boxed = { ...SUBTITLE, key: 'boxed', label: 'Boxed', kind: 'boolean' }
	listing([{ ...DETAILS, fields: [boxed] }])
	const sent = recordingEdits(boxed)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Boxed')

	await userEvent.click(within(part).getByRole('combobox', { name: 'Default' }))
	await userEvent.click(await screen.findByRole('option', { name: 'Yes' }))
	await userEvent.click(saveOf(main))

	await waitFor(() => expect(sent).toEqual([{ path: 'boxed', body: editOf(boxed, { settings: { default: true } }) }]))
})

test('offers a repeater the count of rows it takes and stores it', async () => {
	const team = { ...SUBTITLE, key: 'team', label: 'Team', kind: 'repeater', fields: [] }
	listing([{ ...DETAILS, fields: [team] }])
	const sent = recordingEdits(team)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Team')

	await showTab(part, 'Validation')
	await userEvent.type(within(part).getByRole('spinbutton', { name: 'Fewest rows' }), '1')
	await userEvent.type(within(part).getByRole('spinbutton', { name: 'Most rows' }), '4')
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(sent).toEqual([{ path: 'team', body: editOf(team, { settings: { min: 1, max: 4 } }) }]),
	)
})

test('opens a layout in place inside its open flexible content field and stores its count of rows', async () => {
	listing([{ ...DETAILS, fields: [{ ...FEATURES, fields: [HERO] }] }])
	const sent = recordingEdits(HERO)
	renderAt('/field-groups')
	const main = await openFields()
	await openField(main, 'Features')
	const part = await openField(main, 'Hero')

	await showTab(part, 'Validation')
	await userEvent.type(within(part).getByRole('spinbutton', { name: 'Most rows' }), '2')
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(sent).toEqual([{ path: 'features.hero', body: editOf(HERO, { settings: { max: 2 } }) }]),
	)
})

test('offers a flexible content field the count of rows it takes', async () => {
	listing([{ ...DETAILS, fields: [FEATURES] }])
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Features')

	await showTab(part, 'Validation')

	expect(within(part).getByRole('spinbutton', { name: 'Fewest rows' })).toBeInTheDocument()
	expect(within(part).getByRole('spinbutton', { name: 'Most rows' })).toBeInTheDocument()
})

test('offers a text field no count of rows', async () => {
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Subtitle')

	await showTab(part, 'Validation')

	expect(within(part).queryByRole('spinbutton', { name: 'Most rows' })).toBeNull()
})

test('offers no Required box on a layout, and no Validation tab on a Linked from field', async () => {
	listing([
		{
			...DETAILS,
			fields: [
				{ ...FEATURES, fields: [HERO] },
				{ ...SUBTITLE, key: 'linked-from', label: 'Linked from', kind: 'backlinks' },
			],
		},
		EXTRAS,
	])
	renderAt('/field-groups')
	const main = await openFields()

	await openField(main, 'Features')
	const layout = await openField(main, 'Hero')
	const reading = await openField(main, 'Linked from')

	await showTab(layout, 'Validation')
	expect(within(layout).queryByRole('checkbox', { name: 'Required' })).toBeNull()
	expect(within(reading).getAllByRole('tab').map((tab) => tab.textContent)).toEqual(['General', 'Presentation'])
})

test('offers a gallery the count of files it takes and stores it', async () => {
	const gallery = { ...SUBTITLE, key: 'gallery', label: 'Gallery', kind: 'media', many: true }
	listing([{ ...DETAILS, fields: [gallery] }])
	const sent = recordingEdits(gallery)
	renderAt('/field-groups')
	const main = await openFields()
	const part = await openField(main, 'Gallery')

	await showTab(part, 'Validation')
	await userEvent.type(within(part).getByRole('spinbutton', { name: 'Fewest files' }), '2')
	await userEvent.type(within(part).getByRole('spinbutton', { name: 'Most files' }), '6')
	await userEvent.click(saveOf(main))

	await waitFor(() =>
		expect(sent).toEqual([{ path: 'gallery', body: editOf(gallery, { settings: { min: 2, max: 6 } }) }]),
	)
})
