// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { atOrUnder, fieldDraftOf, settingPart } from '../content/fieldDraft'
import { fieldAt, notesRan, notesStarted, openedRan, withoutPaths } from '../content/fieldNotes'
import { groupsUnreached, RejectedWriteError, StaleWriteError } from '../content/groups'
import type { ContentField } from '../content/types'

const READ_STAMP = '2026-08-01T10:00:00Z'

const PHONE: ContentField = {
	key: 'phone',
	label: 'Phone',
	kind: 'text',
	relatesTo: '',
	many: false,
	required: false,
	settings: { default: '+34 600 000 000' },
	fields: [],
	updatedAt: READ_STAMP,
}

const ROLE: ContentField = { ...PHONE, key: 'role', label: 'Role', settings: {} }

const TEAM: ContentField = { ...PHONE, key: 'team', label: 'Team', kind: 'repeater', settings: {}, fields: [ROLE] }

const BOUNDS_REJECTED = 'The setting default falls outside the others.'

test('names the setting a rejected save named and the text its box held as the save went', () => {
	const rejected = new RejectedWriteError(BOUNDS_REJECTED, { code: 'setting_bounds', meta: { setting: 'default' } })

	const notes = notesRan({}, [], [{ path: 'phone', error: rejected }], { phone: fieldDraftOf(PHONE) })

	expect(notes).toEqual({
		phone: { stale: false, lost: [], rejected: { words: BOUNDS_REJECTED, setting: 'default', sent: '+34 600 000 000' } },
	})
})

test('leaves the unreached words and no setting in the note of a save past the time limit', () => {
	const notes = notesRan({}, [], [{ path: 'phone', error: new Error(groupsUnreached()) }], {})

	expect(notes.phone.rejected).toEqual({ words: 'The field groups could not be reached.', setting: '', sent: '' })
})

test('marks a stale save and keeps the parts the field lost before it', () => {
	const lost = [settingPart('default')]
	const stale = new StaleWriteError('stale', { code: 'content_stale_update' })

	const notes = notesRan({ phone: { stale: false, lost } }, [], [{ path: 'phone', error: stale }], {})

	expect(notes).toEqual({ phone: { stale: true, lost } })
})

test('drops the note of a field the run saved and keeps the notes of the others', () => {
	const kept = { stale: true, lost: [] }

	const notes = notesRan({ phone: { stale: false, lost: ['label'] }, 'team.role': kept }, ['phone'], [], {})

	expect(notes).toEqual({ 'team.role': kept })
})

test('clears what the last save said as a save starts, keeping the parts each field lost', () => {
	const rejected = { words: BOUNDS_REJECTED, setting: 'default', sent: '' }

	expect(notesStarted({ phone: { stale: true, rejected, lost: ['label'] } })).toEqual({
		phone: { stale: false, lost: ['label'] },
	})
})

test('closes every open field once the run ends and opens each field it could not save', () => {
	const opened = openedRan([{ path: 'phone', error: new Error('rejected') }])

	expect([...opened]).toEqual(['phone'])
})

test('opens every container above a field the run could not save', () => {
	const opened = openedRan([
		{ path: 'team.role', error: new Error('rejected') },
		{ path: 'features.hero.title', error: new Error('rejected') },
	])

	expect([...opened]).toEqual(['team', 'team.role', 'features', 'features.hero', 'features.hero.title'])
})

test('finds a field by its dotted path, however deep, and none at a path no field stands at', () => {
	expect(fieldAt([PHONE, TEAM], 'team.role')).toBe(ROLE)
	expect(fieldAt([PHONE, TEAM], 'phone')).toBe(PHONE)
	expect(fieldAt([PHONE, TEAM], 'team.gone')).toBeUndefined()
	expect(fieldAt([PHONE, TEAM], 'gone.role')).toBeUndefined()
})

test('drops what is held at a field and under it, keeping a field whose key only starts the same', () => {
	expect(withoutPaths({ team: 1, 'team.role': 2, teammate: 3 }, (path) => atOrUnder(path, 'team'))).toEqual({
		teammate: 3,
	})
})
