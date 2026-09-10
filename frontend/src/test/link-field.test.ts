// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { linkHeld, linkToStore } from '../content/LinkField'

test('reads the three parts a stored link holds', () => {
	expect(linkHeld({ url: '/about', title: 'About', new_tab: true })).toEqual({
		url: '/about',
		title: 'About',
		new_tab: true,
	})
})

test('reads a blank link from a value that holds none', () => {
	const blank = { url: '', title: '', new_tab: false }

	expect(linkHeld(undefined)).toEqual(blank)
	expect(linkHeld('https://example.com')).toEqual(blank)
	expect(linkHeld({ url: 7, title: null, new_tab: 'yes' })).toEqual(blank)
})

test('stores a link once its address or its title is written', () => {
	expect(linkToStore({ url: '/about', title: '', new_tab: false })).toEqual({
		url: '/about',
		title: '',
		new_tab: false,
	})
	expect(linkToStore({ url: '', title: 'About', new_tab: false })).not.toBeNull()
})

test('stores nothing for a link with neither address nor title', () => {
	expect(linkToStore({ url: '', title: '', new_tab: true })).toBeNull()
})
