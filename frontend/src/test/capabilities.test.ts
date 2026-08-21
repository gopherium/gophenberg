// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import {
	CHANGE_OTHERS_WORK,
	MANAGE_SETTINGS,
	MANAGE_THEMES,
	MANAGE_TYPES,
	MANAGE_USERS,
	can,
	mayChange,
	sessionMayChange,
} from '../capabilities'

test('an admin holds every capability the screens ask for', () => {
	for (const capability of [MANAGE_USERS, MANAGE_THEMES, MANAGE_TYPES, MANAGE_SETTINGS]) {
		expect(can('admin', capability)).toBe(true)
	}
})

test('an editor and an author hold none of the administration capabilities', () => {
	for (const rank of ['editor', 'author']) {
		for (const capability of [MANAGE_USERS, MANAGE_THEMES, MANAGE_TYPES, MANAGE_SETTINGS]) {
			expect(can(rank, capability)).toBe(false)
		}
	}
})

test('a rank the table does not know holds nothing', () => {
	expect(can('archivist', MANAGE_USERS)).toBe(false)
	expect(can('', MANAGE_SETTINGS)).toBe(false)
	expect(can(undefined, MANAGE_THEMES)).toBe(false)
})

test('an admin and an editor change work another account wrote, an author does not', () => {
	expect(can('admin', CHANGE_OTHERS_WORK)).toBe(true)
	expect(can('editor', CHANGE_OTHERS_WORK)).toBe(true)
	expect(can('author', CHANGE_OTHERS_WORK)).toBe(false)
})

test('no session changes nothing, and a session follows mayChange', () => {
	expect(sessionMayChange(undefined, 'account-a')).toBe(false)
	expect(sessionMayChange({ rank: 'author', id: 'account-a' }, 'account-a')).toBe(true)
	expect(sessionMayChange({ rank: 'editor', id: 'account-a' }, 'account-b')).toBe(true)
})

test('mayChange follows the capability or the authorship', () => {
	expect(mayChange('editor', 'account-a', 'account-b')).toBe(true)
	expect(mayChange('author', 'account-a', 'account-a')).toBe(true)
	expect(mayChange('author', 'account-a', 'account-b')).toBe(false)
	expect(mayChange('archivist', 'account-a', 'account-a')).toBe(true)
	expect(mayChange(undefined, 'account-a', 'account-b')).toBe(false)
})
