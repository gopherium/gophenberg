// SPDX-License-Identifier: Apache-2.0

import { defaultUser } from '@gopherium/react-auth/testing'
import { expect, test } from 'vitest'

import { mayChange } from '../capabilities'
import {
	ACCESS_ADMIN,
	CHANGE_OTHERS_WORK,
	MANAGE_SETTINGS,
	MANAGE_THEMES,
	MANAGE_TYPES,
	MANAGE_USERS,
	MEMBER,
	UPDATE_CORE,
	can,
	sessionMayChange,
} from '../index'

test('a session holds exactly the capabilities it lists, a plugin capability included', () => {
	const session = { ...defaultUser, role: 'designer', capabilities: [MANAGE_THEMES, 'events.manage'] }

	expect(can(session, MANAGE_THEMES)).toBe(true)
	expect(can(session, 'events.manage')).toBe(true)
	for (const capability of [MANAGE_USERS, MANAGE_TYPES, MANAGE_SETTINGS, CHANGE_OTHERS_WORK, UPDATE_CORE]) {
		expect(can(session, capability)).toBe(false)
	}
})

test('names the capability updating Gophenberg, which a session holds only when it lists it', () => {
	expect(UPDATE_CORE).toBe('update_core')
	expect(can({ ...defaultUser, role: 'admin', capabilities: [UPDATE_CORE] }, UPDATE_CORE)).toBe(true)
	expect(can({ ...defaultUser, role: 'editor', capabilities: [CHANGE_OTHERS_WORK] }, UPDATE_CORE)).toBe(false)
})

test('names the capability entering the admin, which a member never holds', () => {
	const member = { ...defaultUser, role: MEMBER, capabilities: [] }

	expect(ACCESS_ADMIN).toBe('access_admin')
	expect(MEMBER).toBe('member')
	expect(can(member, ACCESS_ADMIN)).toBe(false)
})

test('a role name alone holds nothing', () => {
	expect(can({ ...defaultUser, role: 'admin', capabilities: [] }, MANAGE_USERS)).toBe(false)
	expect(can({ ...defaultUser, role: 'admin', capabilities: undefined }, MANAGE_USERS)).toBe(false)
})

test('no session holds nothing', () => {
	expect(can(null, MANAGE_SETTINGS)).toBe(false)
	expect(can(undefined, MANAGE_THEMES)).toBe(false)
})

test('no session changes nothing, and a session follows mayChange', () => {
	expect(sessionMayChange(undefined, 'account-a')).toBe(false)
	expect(sessionMayChange({ id: 'account-a', capabilities: [] }, 'account-a')).toBe(true)
	expect(sessionMayChange({ id: 'account-a', capabilities: [CHANGE_OTHERS_WORK] }, 'account-b')).toBe(true)
	expect(sessionMayChange({ id: 'account-a', capabilities: [] }, 'account-b')).toBe(false)
})

test('mayChange follows the capability or the authorship', () => {
	expect(mayChange([CHANGE_OTHERS_WORK], 'account-a', 'account-b')).toBe(true)
	expect(mayChange([], 'account-a', 'account-a')).toBe(true)
	expect(mayChange([], 'account-a', 'account-b')).toBe(false)
	expect(mayChange(undefined, 'account-a', 'account-b')).toBe(false)
})
