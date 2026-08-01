// SPDX-License-Identifier: Apache-2.0

import blockLibraryStyle from '@wordpress/block-library/build-style/style.css?inline'
import blockLibraryTheme from '@wordpress/block-library/build-style/theme.css?inline'

const CANVAS_TYPOGRAPHY = `
	body {
		font-family: system-ui, sans-serif;
		margin: 0;
		padding: 32px;
	}
`

export const CANVAS_STYLES = [
	{ css: blockLibraryStyle },
	{ css: blockLibraryTheme },
	{ css: CANVAS_TYPOGRAPHY },
]
