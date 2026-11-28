// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { afterEach, expect, test, vi } from 'vitest'

import { fieldDraftOf } from '../content/fieldDraft'
import type { FieldDraft, FieldDrafts } from '../content/fieldDraft'
import { draftOf, withPick } from '../content/groupDraft'
import type { GroupDraft } from '../content/groupDraft'
import { RejectedWriteError, StaleWriteError } from '../content/groups'
import type { FieldGroup } from '../content/groups'
import { runSave, saveNotes } from '../content/groupSave'
import type { SaveAsked } from '../content/groupSave'
import type { ContentField } from '../content/types'
import { gate } from './gate'

const READ_STAMP = '2026-08-01T10:00:00Z'

const SAVED_STAMP = '2026-08-02T09:30:00Z'

const OTHER_SAVE_STAMP = '2026-08-02T08:00:00Z'

const NO_LIMIT = Number.POSITIVE_INFINITY

const SHORT_LIMIT = 20

/** The time limit each request of the run gets, shorter than the run of two requests. */
const PER_REQUEST_LIMIT = 300

/** How long each request takes to answer, under the limit on its own. */
const ANSWER_TIME = 200

const UNREACHED = 'The field groups could not be reached.'

afterEach(() => {
	vi.useRealTimers()
})

const EMAIL: ContentField = {
	key: 'email',
	label: 'Email',
	kind: 'text',
	relatesTo: '',
	many: false,
	required: false,
	settings: { variant: 'email', placeholder: 'maria.perez@example.com' },
	fields: [],
	updatedAt: READ_STAMP,
}

const PHONE: ContentField = { ...EMAIL, key: 'phone', label: 'Phone', settings: {} }

const ROLE: ContentField = { ...EMAIL, key: 'role', label: 'Role', settings: {} }

const TEAM: ContentField = { ...EMAIL, key: 'team', label: 'Team', kind: 'repeater', settings: {}, fields: [ROLE] }

const LINKED_FROM: ContentField = {
	...EMAIL,
	key: 'linked-from',
	label: 'Linked from',
	kind: 'backlinks',
	settings: { source_group: 'cars', source_field: ['maker'] },
}

const MAKER: ContentField = {
	...EMAIL,
	key: 'maker',
	label: 'Maker',
	kind: 'relation',
	relatesTo: 'post',
	settings: {},
}

const KNOWN_FIELDS = [EMAIL, PHONE, ROLE, TEAM, LINKED_FROM]

const CONTACT: FieldGroup = {
	id: 3,
	key: 'contact-details',
	title: 'Contact details',
	location: [[{ source: 'content_type', operator: '==', value: 'post' }]],
	position: 1,
	active: true,
	fields: [EMAIL, PHONE, TEAM],
	createdAt: READ_STAMP,
	updatedAt: READ_STAMP,
}

const CARS: FieldGroup = {
	...CONTACT,
	id: 6,
	key: 'cars',
	title: 'Cars',
	fields: [MAKER, { ...MAKER, key: 'rival', label: 'Rival' }],
}

/**
 * Returns the row the server answers for a field.
 * @param field - The field as the admin reads it.
 * @param updatedAt - The stamp the row carries.
 * @returns The row.
 */
function rowOf(field: ContentField, updatedAt = SAVED_STAMP): Record<string, unknown> {
	return {
		key: field.key,
		label: field.label,
		kind: field.kind,
		relates_to: field.relatesTo,
		many: field.many,
		required: field.required,
		settings: field.settings,
		fields: field.fields.map((held) => rowOf(held, held.updatedAt)),
		updated_at: updatedAt,
	}
}

/**
 * Returns the row the server answers for a group.
 * @param group - The group as the admin reads it.
 * @returns The row.
 */
function groupRowOf(group: FieldGroup): Record<string, unknown> {
	return {
		id: group.id,
		key: group.key,
		title: group.title,
		location: group.location,
		position: group.position,
		active: group.active,
		fields: group.fields.map((field) => rowOf(field, field.updatedAt)),
		created_at: group.createdAt,
		updated_at: SAVED_STAMP,
	}
}

/**
 * Returns what the header Save of the contact details page sends.
 * @param drafts - The drafts of its fields.
 * @param draft - The draft of its details, the group as read when none is given.
 * @returns What the run is asked to send.
 */
function asking(drafts: FieldDrafts, draft: GroupDraft = draftOf(CONTACT)): SaveAsked {
	return { held: CONTACT, groups: [CONTACT, CARS], base: CONTACT, draft, drafts }
}

/**
 * Returns the draft of a field with its label changed.
 * @param field - The field as read.
 * @param label - The label the draft holds.
 * @returns The draft.
 */
function relabelled(field: ContentField, label: string): FieldDraft {
	return { ...fieldDraftOf(field), label }
}

/**
 * Answers every field edit and group edit as the server stores them, and records each request as it arrives.
 * @param sent - The requests so far, each as its method, its path and its body.
 */
function storing(sent: { path: string; body: Record<string, unknown> }[]) {
	server.use(
		http.patch('/api/groups/3/fields/:path', async ({ request, params }) => {
			const body = (await request.json()) as Record<string, unknown>
			const path = String(params.path)
			sent.push({ path: `fields/${path}`, body })
			const key = path.split('.').at(-1)
			const held = KNOWN_FIELDS.find((field) => field.key === key) ?? EMAIL
			return HttpResponse.json({ ...rowOf(held), ...body, updated_at: SAVED_STAMP })
		}),
		http.patch('/api/groups/3', async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			sent.push({ path: 'group', body })
			return HttpResponse.json({ ...groupRowOf(CONTACT), ...body, updated_at: SAVED_STAMP })
		}),
	)
}

/**
 * Answers the field edit at a path with a rejection naming a setting.
 * @param path - The dotted path of the field.
 * @param setting - The setting the rejection names.
 */
function rejecting(path: string, setting: string) {
	server.use(
		http.patch(`/api/groups/3/fields/${path}`, () =>
			HttpResponse.json(
				{ error: `content: settings disagree: ${setting}`, code: 'setting_bounds', meta: { setting } },
				{ status: 422 },
			),
		),
	)
}

/**
 * Waits for a span of time.
 * @param milliseconds - How long to wait.
 * @returns A promise settling once the time has passed.
 */
function pause(milliseconds: number): Promise<unknown> {
	return new Promise((resolve) => setTimeout(resolve, milliseconds))
}

/**
 * Waits long enough for a request that does not wait its turn to arrive.
 * @returns A promise settling once the time has passed.
 */
function aMoment(): Promise<unknown> {
	return pause(50)
}

test('sends each changed field in page order, one after another, and the group last', async () => {
	const sent: { path: string; body: Record<string, unknown> }[] = []
	storing(sent)
	const first = gate()
	server.use(
		http.patch('/api/groups/3/fields/email', async ({ request }) => {
			sent.push({ path: 'fields/email', body: (await request.json()) as Record<string, unknown> })
			await first.held
			return HttpResponse.json(rowOf(EMAIL))
		}),
	)
	const drafts: FieldDrafts = {
		'team.role': { ...fieldDraftOf(ROLE), required: true },
		phone: relabelled(PHONE, 'Mobile'),
		email: relabelled(EMAIL, 'Reply to'),
	}

	const running = runSave(asking(drafts, { ...draftOf(CONTACT), title: 'Customer details' }), NO_LIMIT)
	await vi.waitFor(() => expect(sent).toHaveLength(1))
	await aMoment()
	expect(sent.map((one) => one.path)).toEqual(['fields/email'])
	first.release()
	await running

	expect(sent.map((one) => one.path)).toEqual(['fields/email', 'fields/phone', 'fields/team.role', 'group'])
})

test('sends a field with its label, its required flag, its whole settings and the stamp it was read at', async () => {
	const sent: { path: string; body: Record<string, unknown> }[] = []
	storing(sent)
	const typed = fieldDraftOf(EMAIL)

	await runSave(asking({ email: { ...typed, typed: { ...typed.typed, placeholder: 'replies@example.com' } } }), NO_LIMIT)

	expect(sent).toEqual([
		{
			path: 'fields/email',
			body: {
				label: 'Email',
				required: false,
				settings: { variant: 'email', placeholder: 'replies@example.com' },
				updated_at: READ_STAMP,
			},
		},
	])
})

test('sends nothing for an unchanged draft, a draft no field stands at, or details left as read', async () => {
	const sent: { path: string; body: Record<string, unknown> }[] = []
	storing(sent)
	const fax = { ...PHONE, key: 'fax', label: 'Fax' }

	const ran = await runSave(asking({ phone: fieldDraftOf(PHONE), fax: relabelled(fax, 'Facsimile') }), NO_LIMIT)

	expect(sent).toEqual([])
	expect(ran.outcome).toEqual({ asked: 0, done: 0, failures: [] })
	expect(saveNotes(ran)).toEqual({})
})

test('sends only the parts of the group its details changed, after the fields', async () => {
	const sent: { path: string; body: Record<string, unknown> }[] = []
	storing(sent)

	const switchedOff = { ...draftOf(CONTACT), active: false }

	const ran = await runSave(asking({ phone: relabelled(PHONE, 'Mobile') }, switchedOff), NO_LIMIT)

	expect(sent.at(-1)).toEqual({ path: 'group', body: { active: false, updated_at: READ_STAMP } })
	expect(ran.stored).toMatchObject({ id: 3, active: false })
	expect(ran.saved.phone).toMatchObject({ label: 'Mobile', updatedAt: SAVED_STAMP })
})

test('points a Linked from field with the stamp its own save in the same run answered', async () => {
	const sent: { path: string; body: Record<string, unknown> }[] = []
	storing(sent)
	const held = { ...CONTACT, fields: [LINKED_FROM] }
	const picked = withPick(draftOf(held), LINKED_FROM, { group: 'cars', field: 'rival' })
	const drafts = { 'linked-from': relabelled(LINKED_FROM, 'Rivals') }

	await runSave({ held, groups: [held, CARS], base: held, draft: picked, drafts }, NO_LIMIT)

	expect(sent.map((one) => one.path)).toEqual(['fields/linked-from', 'group'])
	expect(sent[1].body).toEqual({
		backlinks: { 'linked-from': { source_group: 'cars', source_field: ['rival'], updated_at: SAVED_STAMP } },
		updated_at: READ_STAMP,
	})
})

test('keeps the stamp it read for a Linked from pick whose field was not in the run', async () => {
	const sent: { path: string; body: Record<string, unknown> }[] = []
	storing(sent)
	const held = { ...CONTACT, fields: [LINKED_FROM] }
	const picked = withPick(draftOf(held), LINKED_FROM, { group: 'cars', field: 'rival' })

	await runSave({ held, groups: [held, CARS], base: held, draft: picked, drafts: {} }, NO_LIMIT)

	expect(sent).toEqual([
		{
			path: 'group',
			body: {
				backlinks: { 'linked-from': { source_group: 'cars', source_field: ['rival'], updated_at: READ_STAMP } },
				updated_at: READ_STAMP,
			},
		},
	])
})

test('sends the newest stamp of a Linked from field keyed constructor while the run saved no field', async () => {
	const sent: { path: string; body: Record<string, unknown> }[] = []
	storing(sent)
	const group = { ...CONTACT, fields: [{ ...LINKED_FROM, key: 'constructor', updatedAt: OTHER_SAVE_STAMP }] }
	const previousSource = { group: 'cars', field: 'maker' }
	const draft: GroupDraft = {
		...draftOf(group),
		links: { constructor: { group: 'cars', field: 'rival', was: previousSource, updatedAt: READ_STAMP } },
	}

	await runSave({ held: group, groups: [group, CARS], base: group, draft, drafts: {} }, NO_LIMIT)

	expect(sent).toEqual([
		{
			path: 'group',
			body: {
				backlinks: { constructor: { source_group: 'cars', source_field: ['rival'], updated_at: OTHER_SAVE_STAMP } },
				updated_at: READ_STAMP,
			},
		},
	])
})

test('goes on past a rejected field, saving the others and the group, and names the field it failed', async () => {
	const sent: { path: string; body: Record<string, unknown> }[] = []
	storing(sent)
	rejecting('email', 'default')
	const drafts = { email: relabelled(EMAIL, 'Reply to'), phone: relabelled(PHONE, 'Mobile') }

	const ran = await runSave(asking(drafts, { ...draftOf(CONTACT), title: 'Customer details' }), NO_LIMIT)

	expect(sent.map((one) => one.path)).toEqual(['fields/phone', 'group'])
	expect(ran.outcome.asked).toBe(3)
	expect(ran.outcome.done).toBe(2)
	expect(ran.outcome.failures).toHaveLength(1)
	expect(ran.outcome.failures[0].item).toEqual({ path: 'email', label: 'Email' })
	expect(ran.outcome.failures[0].error).toBeInstanceOf(RejectedWriteError)
	expect(ran.outcome.failures[0].error).toMatchObject({ meta: { setting: 'default' } })
	expect(Object.keys(ran.saved)).toEqual(['phone'])
	expect(ran.stored).toMatchObject({ title: 'Customer details' })
})

test('says how many changes saved, names the field that did not, and gives the server its words', async () => {
	storing([])
	rejecting('email', 'default')
	const drafts = { email: relabelled(EMAIL, 'Reply to'), phone: relabelled(PHONE, 'Mobile') }

	const ran = await runSave(asking(drafts, { ...draftOf(CONTACT), title: 'Customer details' }), NO_LIMIT)

	expect(saveNotes(ran).notice).toBe(
		'Saved 2 of 3 changes. Could not save Email. ' +
			'The setting default falls outside the others. Keep min below max and the default inside the bounds.',
	)
})

test('names every field it could not save and leaves their reasons to each field', async () => {
	storing([])
	rejecting('email', 'default')
	rejecting('phone', 'default')
	const drafts = { email: relabelled(EMAIL, 'Reply to'), phone: relabelled(PHONE, 'Mobile') }

	const ran = await runSave(asking(drafts, { ...draftOf(CONTACT), title: 'Customer details' }), NO_LIMIT)

	expect(saveNotes(ran).notice).toBe('Saved 1 of 3 changes. Could not save Email, Phone.')
})

test('names a lone field it could not save with the server words and counts nothing', async () => {
	rejecting('email', 'default')

	const ran = await runSave(asking({ email: relabelled(EMAIL, 'Reply to') }), NO_LIMIT)

	expect(saveNotes(ran)).toEqual({
		notice:
			'Could not save Email. ' +
			'The setting default falls outside the others. Keep min below max and the default inside the bounds.',
	})
})

test('says the group was updated under the title it now carries once every change saved', async () => {
	storing([])

	const ran = await runSave(
		asking({ phone: relabelled(PHONE, 'Mobile') }, { ...draftOf(CONTACT), title: 'Customer details' }),
		NO_LIMIT,
	)

	expect(saveNotes(ran)).toEqual({ toast: 'Customer details updated.' })
})

test('says the group was updated under the title it read once only its fields saved', async () => {
	storing([])

	const ran = await runSave(asking({ phone: relabelled(PHONE, 'Mobile') }), NO_LIMIT)

	expect(ran.stored).toBeUndefined()
	expect(saveNotes(ran)).toEqual({ toast: 'Contact details updated.' })
})

test('names the group someone saved first when the group was all the run sent', async () => {
	server.use(
		http.patch('/api/groups/3', () =>
			HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 }),
		),
	)

	const ran = await runSave(asking({}, { ...draftOf(CONTACT), title: 'Customer details' }), NO_LIMIT)

	expect(ran.outcome.failures[0].error).toBeInstanceOf(StaleWriteError)
	expect(saveNotes(ran)).toEqual({
		notice: 'Could not save the field group. Someone saved Contact details after you opened it.',
	})
})

test('stops a field request past the time limit, names the groups unreached, and sends the next', async () => {
	const sent: { path: string; body: Record<string, unknown> }[] = []
	storing(sent)
	const unanswered: Request[] = []
	server.use(
		http.patch('/api/groups/3/fields/email', ({ request }) => {
			unanswered.push(request)
			return new Promise<never>(() => {})
		}),
	)

	const ran = await runSave(
		asking({ email: relabelled(EMAIL, 'Reply to'), phone: relabelled(PHONE, 'Mobile') }),
		SHORT_LIMIT,
	)

	expect(unanswered.map((request) => request.signal.aborted)).toEqual([true])
	expect(sent.map((one) => one.path)).toEqual(['fields/phone'])
	expect(ran.outcome.failures).toHaveLength(1)
	expect(ran.outcome.failures[0].item.path).toBe('email')
	expect((ran.outcome.failures[0].error as Error).message).toBe(UNREACHED)
	expect(saveNotes(ran).notice).toBe(`Saved 1 of 2 changes. Could not save Email. ${UNREACHED}`)
})

test('gives each request the whole time limit, so a run longer than the limit still saves every change', async () => {
	vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
	const arrived: string[] = []
	const reached = [gate(), gate()]
	const answers = [gate(), gate()]
	server.use(
		http.patch('/api/groups/3/fields/:path', async ({ request, params }) => {
			const body = (await request.json()) as Record<string, unknown>
			const at = arrived.push(String(params.path)) - 1
			reached[at].release()
			await answers[at].held
			return HttpResponse.json({ ...rowOf(PHONE), ...body })
		}),
	)

	const running = runSave(
		asking({ email: relabelled(EMAIL, 'Reply to'), phone: relabelled(PHONE, 'Mobile') }),
		PER_REQUEST_LIMIT,
	)
	await reached[0].held
	await vi.advanceTimersByTimeAsync(ANSWER_TIME)
	answers[0].release()
	await reached[1].held
	await vi.advanceTimersByTimeAsync(ANSWER_TIME)
	answers[1].release()
	const ran = await running

	expect(arrived).toEqual(['email', 'phone'])
	expect(ran.outcome).toEqual({ asked: 2, done: 2, failures: [] })
})
