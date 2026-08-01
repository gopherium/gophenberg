// SPDX-License-Identifier: Apache-2.0

export const EDITOR_SETTINGS = {
	hasFixedToolbar: true,
	bodyPlaceholder: 'Start writing.',
	maxWidth: 840,
	__experimentalBlockPatterns: [],
}

export const SPIKE_BLOCKS = [
	{
		name: 'core/heading',
		attributes: { content: 'Gophenberg writes with blocks', level: 2 },
		innerBlocks: [],
	},
	{
		name: 'core/paragraph',
		attributes: { content: 'The canvas is live. Part E fills in the rest.' },
		innerBlocks: [],
	},
]
