// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import {
	MANAGE_SETTINGS,
	MANAGE_THEMES,
	MANAGE_TYPES,
	MANAGE_USERS,
	can,
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
