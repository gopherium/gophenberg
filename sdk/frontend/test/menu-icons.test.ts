// SPDX-License-Identifier: Apache-2.0

import { brush, category, customPostType, language, list, media, page, post, settings, update } from '@wordpress/icons'
import { expect, test } from 'vitest'

import {
	brushIcon,
	categoryIcon,
	customPostTypeIcon,
	languageIcon,
	listIcon,
	mediaIcon,
	pageIcon,
	postIcon,
	settingsIcon,
	updateIcon,
} from '../index'

test('hands the sidebar rows the glyphs WordPress draws them with', () => {
	expect(postIcon).toBe(post)
	expect(pageIcon).toBe(page)
	expect(categoryIcon).toBe(category)
	expect(mediaIcon).toBe(media)
	expect(brushIcon).toBe(brush)
	expect(settingsIcon).toBe(settings)
	expect(languageIcon).toBe(language)
	expect(customPostTypeIcon).toBe(customPostType)
	expect(listIcon).toBe(list)
	expect(updateIcon).toBe(update)
})
