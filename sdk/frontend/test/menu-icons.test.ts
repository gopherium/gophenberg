// SPDX-License-Identifier: Apache-2.0

import {
	audio,
	brush,
	category,
	customPostType,
	file,
	image,
	language,
	list,
	media,
	page,
	plugins,
	post,
	settings,
	update,
	video,
} from '@wordpress/icons'
import { Tooltip as UiTooltip } from '@wordpress/ui'
import { expect, test } from 'vitest'

import {
	audioIcon,
	brushIcon,
	categoryIcon,
	customPostTypeIcon,
	fileIcon,
	imageIcon,
	languageIcon,
	listIcon,
	mediaIcon,
	pageIcon,
	pluginsIcon,
	postIcon,
	settingsIcon,
	Tooltip,
	updateIcon,
	videoIcon,
} from '../index'

test('hands the sidebar rows the glyphs WordPress draws them with', () => {
	expect(postIcon).toBe(post)
	expect(pageIcon).toBe(page)
	expect(categoryIcon).toBe(category)
	expect(mediaIcon).toBe(media)
	expect(brushIcon).toBe(brush)
	expect(pluginsIcon).toBe(plugins)
	expect(settingsIcon).toBe(settings)
	expect(languageIcon).toBe(language)
	expect(customPostTypeIcon).toBe(customPostType)
	expect(listIcon).toBe(list)
	expect(updateIcon).toBe(update)
})

test('hands a media card the glyph WordPress draws for each kind of file', () => {
	expect(imageIcon).toBe(image)
	expect(videoIcon).toBe(video)
	expect(audioIcon).toBe(audio)
	expect(fileIcon).toBe(file)
})

test('hands out the tooltip WordPress shows a cut file name in', () => {
	expect(Tooltip).toBe(UiTooltip)
})
