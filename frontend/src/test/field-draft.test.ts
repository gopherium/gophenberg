// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import {
	atOrUnder,
	changedFieldParts,
	changedPaths,
	fieldDraftOf,
	fieldEditOf,
	holdsDraft,
	rebasedField,
	settingPart,
} from '../content/fieldDraft'
import type { FieldDraft, FieldDrafts } from '../content/fieldDraft'
import type { ContentField } from '../content/types'

const READ_STAMP = '2026-08-01T10:00:00Z'

const NEWER_STAMP = '2026-08-02T09:30:00Z'

const WITH_PHONE = [[{ source: 'phone', operator: '!=', value: '' }]]

const WITHOUT_PHONE = [[{ source: 'phone', operator: '==', value: '' }]]

const WITH_TEAM = [[{ source: 'team', operator: '!=', value: '' }]]

const EMAIL: ContentField = {
	key: 'email',
	label: 'Email',
	kind: 'text',
	relatesTo: '',
	many: false,
	required: false,
	settings: {
		variant: 'email',
		conditions: WITH_PHONE,
		instructions: 'Where replies go.',
		placeholder: 'maria.perez@example.com',
	},
	fields: [],
	updatedAt: READ_STAMP,
}

const PHONE: ContentField = { ...EMAIL, key: 'phone', label: 'Phone', settings: {} }

const ROLE: ContentField = { ...EMAIL, key: 'role', label: 'Role', settings: {} }

const TEAM: ContentField = { ...EMAIL, key: 'team', label: 'Team', kind: 'repeater', settings: {}, fields: [ROLE] }

/**
 * Returns the draft with a setting control holding new text.
 * @param draft - The draft held.
 * @param name - The key of the setting.
 * @param written - The text the control holds.
 * @returns The draft holding the text.
 */
function typing(draft: FieldDraft, name: string, written: string): FieldDraft {
	return { ...draft, typed: { ...draft.typed, [name]: written } }
}

/**
 * Returns the draft holding a setting no control types in.
 * @param draft - The draft held.
 * @param name - The key of the setting.
 * @param value - The value the setting holds.
 * @returns The draft holding the setting.
 */
function holding(draft: FieldDraft, name: string, value: unknown): FieldDraft {
	return { ...draft, settings: { ...draft.settings, [name]: value } }
}

/**
 * Returns the email field as saved elsewhere with some settings changed.
 * @param settings - The settings the save changed.
 * @returns The field as saved, with a newer stamp.
 */
function savedWith(settings: Record<string, unknown>): ContentField {
	return { ...EMAIL, settings: { ...EMAIL.settings, ...settings }, updatedAt: NEWER_STAMP }
}

test('sends the field back as it was read, the settings no control shows among them', () => {
	expect(fieldEditOf(fieldDraftOf(EMAIL))).toEqual({ label: 'Email', required: false, settings: EMAIL.settings })
})

test('sends the variant and the conditions unchanged beside an edited label, required flag and placeholder', () => {
	const edited = { ...fieldDraftOf(EMAIL), label: 'Work email', required: true }
	const draft = typing(edited, 'placeholder', 'team@example.com')

	expect(fieldEditOf(draft)).toEqual({
		label: 'Work email',
		required: true,
		settings: {
			variant: 'email',
			conditions: WITH_PHONE,
			instructions: 'Where replies go.',
			placeholder: 'team@example.com',
		},
	})
})

const EMPTIED = [
	{ held: 'nothing', written: '' },
	{ held: 'only spaces', written: '   ' },
]

test.each(EMPTIED)('leaves out the key of a setting whose control holds $held', (asked) => {
	const sent = fieldEditOf(typing(fieldDraftOf(EMAIL), 'placeholder', asked.written))

	expect(sent.settings).toEqual({ variant: 'email', conditions: WITH_PHONE, instructions: 'Where replies go.' })
})

test('sends a number typed in a control as a number', () => {
	expect(fieldEditOf(typing(fieldDraftOf(EMAIL), 'maxlength', '80')).settings.maxlength).toBe(80)
})

test('leaves out a choice pair missing its label', () => {
	const style = { ...EMAIL, key: 'style', label: 'Style', kind: 'choice', settings: {} }
	const pairs = [
		{ value: 'bold', label: 'Bold' },
		{ value: 'plain', label: '' },
	]

	expect(fieldEditOf(holding(fieldDraftOf(style), 'choices', pairs)).settings).toEqual({
		choices: [{ value: 'bold', label: 'Bold' }],
	})
})

test('names no part of a draft that holds the field as it was read', () => {
	expect(changedFieldParts(fieldDraftOf(EMAIL))).toEqual([])
})

test('names no part of a field read with a setting its control shows empty', () => {
	const unset = { ...EMAIL, settings: { ...EMAIL.settings, maxlength: null } }

	expect(changedFieldParts(fieldDraftOf(unset))).toEqual([])
})

test('names only the parts a draft changed', () => {
	const draft = fieldDraftOf(EMAIL)
	const { conditions: _dropped, ...unconditioned } = EMAIL.settings

	expect(changedFieldParts({ ...draft, label: 'Work email' })).toEqual(['label'])
	expect(changedFieldParts({ ...draft, required: true })).toEqual(['required'])
	expect(changedFieldParts(typing(draft, 'placeholder', 'team@example.com'))).toEqual([settingPart('placeholder')])
	expect(changedFieldParts(holding(draft, 'listed', true))).toEqual([settingPart('listed')])
	expect(changedFieldParts(holding(draft, 'conditions', WITHOUT_PHONE))).toEqual([settingPart('conditions')])
	expect(changedFieldParts({ ...draft, settings: unconditioned })).toEqual([settingPart('conditions')])
})

test('names nothing once a change is undone', () => {
	const changed = typing({ ...fieldDraftOf(EMAIL), label: 'Work email' }, 'placeholder', '')
	const undone = typing({ ...changed, label: 'Email' }, 'placeholder', 'maria.perez@example.com')

	expect(changedFieldParts(undone)).toEqual([])
})

test('keeps a setting only the draft changed and takes a setting only the saved version changed', () => {
	const draft = typing(fieldDraftOf(EMAIL), 'placeholder', 'team@example.com')
	const saved = savedWith({ instructions: 'Where the answers go.' })

	const moved = rebasedField(draft, saved)

	expect(moved.draft).toEqual(typing(fieldDraftOf(saved), 'placeholder', 'team@example.com'))
	expect(moved.lost).toEqual([])
	expect(changedFieldParts(moved.draft)).toEqual([settingPart('placeholder')])
})

test('takes the saved value of a setting both changed and names it', () => {
	const draft = typing(fieldDraftOf(EMAIL), 'placeholder', 'team@example.com')

	const moved = rebasedField(draft, savedWith({ placeholder: 'replies@example.com' }))

	expect(moved.draft.typed.placeholder).toBe('replies@example.com')
	expect(moved.lost).toEqual([settingPart('placeholder')])
})

test('names no setting both changed the same way', () => {
	const draft = typing(fieldDraftOf(EMAIL), 'placeholder', 'team@example.com')

	expect(rebasedField(draft, savedWith({ placeholder: 'team@example.com' })).lost).toEqual([])
})

test('keeps a required flag only the draft changed and takes the saved label of a label both changed', () => {
	const draft = { ...fieldDraftOf(EMAIL), label: 'Work email', required: true }

	const moved = rebasedField(draft, { ...EMAIL, label: 'Reply email', updatedAt: NEWER_STAMP })

	expect(moved.draft.label).toBe('Reply email')
	expect(moved.draft.required).toBe(true)
	expect(moved.lost).toEqual(['label'])
})

test('settles each setting no control types in on its own', () => {
	const draft = holding(fieldDraftOf(EMAIL), 'listed', true)
	const { conditions: _dropped, ...unconditioned } = EMAIL.settings
	const saved = { ...EMAIL, settings: unconditioned, updatedAt: NEWER_STAMP }

	const moved = rebasedField(draft, saved)

	expect(moved.draft.settings).toEqual({ ...unconditioned, listed: true })
	expect(moved.lost).toEqual([])
})

test('takes the saved conditions where both changed them and names them', () => {
	const draft = holding(fieldDraftOf(EMAIL), 'conditions', WITHOUT_PHONE)

	const moved = rebasedField(draft, savedWith({ conditions: WITH_TEAM }))

	expect(moved.draft.settings.conditions).toEqual(WITH_TEAM)
	expect(moved.lost).toEqual([settingPart('conditions')])
})

test('lists the paths of the changed drafts in page order, leaving out a path no field stands at', () => {
	const drafts: FieldDrafts = {
		email: { ...fieldDraftOf(EMAIL), label: 'Work email' },
		phone: fieldDraftOf(PHONE),
		'team.role': { ...fieldDraftOf(ROLE), required: true },
		'team.lead': { ...fieldDraftOf(ROLE), label: 'Lead' },
	}

	expect(changedPaths([TEAM, PHONE, EMAIL], drafts)).toEqual(['team.role', 'email'])
})

test('reads no draft for a field keyed like a member every object inherits', () => {
	const constructorField: ContentField = { ...EMAIL, key: 'constructor', label: 'Constructor', settings: {} }
	const renamedDrafts: FieldDrafts = { constructor: { ...fieldDraftOf(constructorField), label: 'Builder' } }

	expect(changedPaths([constructorField, EMAIL], {})).toEqual([])
	expect(changedPaths([constructorField], renamedDrafts)).toEqual(['constructor'])
})

test('tells whether a field or a field inside it holds a changed draft', () => {
	const changed = ['team.role', 'email']

	expect(holdsDraft(changed, 'team')).toBe(true)
	expect(holdsDraft(changed, 'team.role')).toBe(true)
	expect(holdsDraft(changed, 'email')).toBe(true)
	expect(holdsDraft(changed, 'phone')).toBe(false)
	expect(holdsDraft(['teammate'], 'team')).toBe(false)
})

test('places a path at or under a field, never under a field whose key only starts the same', () => {
	expect(atOrUnder('team', 'team')).toBe(true)
	expect(atOrUnder('team.role', 'team')).toBe(true)
	expect(atOrUnder('teammate', 'team')).toBe(false)
	expect(atOrUnder('team', 'team.role')).toBe(false)
})
