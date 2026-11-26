// SPDX-License-Identifier: Apache-2.0

import * as icons from '@wordpress/icons'
import { Tooltip as UiTooltip } from '@wordpress/ui'
import { expect, test } from 'vitest'

import * as sdk from '../index'

test('hands the sidebar rows the glyphs WordPress draws them with', () => {
	expect(sdk.post).toBe(icons.post)
	expect(sdk.page).toBe(icons.page)
	expect(sdk.category).toBe(icons.category)
	expect(sdk.media).toBe(icons.media)
	expect(sdk.brush).toBe(icons.brush)
	expect(sdk.plugins).toBe(icons.plugins)
	expect(sdk.settings).toBe(icons.settings)
	expect(sdk.language).toBe(icons.language)
	expect(sdk.customPostType).toBe(icons.customPostType)
	expect(sdk.list).toBe(icons.list)
	expect(sdk.update).toBe(icons.update)
})

test('hands a media card the glyph WordPress draws for each kind of file', () => {
	expect(sdk.image).toBe(icons.image)
	expect(sdk.video).toBe(icons.video)
	expect(sdk.audio).toBe(icons.audio)
	expect(sdk.file).toBe(icons.file)
})

test('hands out every icon under the name @wordpress/icons gives it', () => {
	const names = [
		'audio',
		'backup',
		'brush',
		'category',
		'check',
		'chevronDown',
		'chevronLeft',
		'chevronUp',
		'closeSmall',
		'cloudUpload',
		'commentAuthorAvatar',
		'customPostType',
		'drawerLeft',
		'drawerRight',
		'file',
		'image',
		'language',
		'list',
		'listView',
		'media',
		'notAllowed',
		'page',
		'plugins',
		'post',
		'redo',
		'settings',
		'trash',
		'undo',
		'update',
		'upload',
		'video',
	] as const
	for (const name of names) {
		expect(sdk[name]).toBe(icons[name])
	}
	expect(Object.keys(sdk).filter((name) => /^[a-z]\w*Icon$/.test(name))).toEqual([])
})

test('hands out the tooltip WordPress shows a cut file name in', () => {
	expect(sdk.Tooltip).toBe(UiTooltip)
})
