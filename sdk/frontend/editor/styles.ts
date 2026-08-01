// SPDX-License-Identifier: Apache-2.0

import blockLibraryStyle from '@wordpress/block-library/build-style/style.css?inline'
import blockLibraryTheme from '@wordpress/block-library/build-style/theme.css?inline'

const CANVAS_TYPOGRAPHY = `
	body {
		font-family: system-ui, sans-serif;
		font-size: 16px;
		margin: 0;
		padding: 32px 0;
	}
	p {
		font-size: inherit;
		line-height: inherit;
	}
	ul,
	ol {
		margin: 0;
		padding: 0 0 0 1.5em;
	}
	ul {
		list-style-type: disc;
	}
	ol {
		list-style-type: decimal;
	}
`

const CANVAS_COLUMN = `
	.wp-block {
		max-width: 700px;
		margin-left: auto;
		margin-right: auto;
	}
	.wp-block[data-align="wide"],
	.wp-block.alignwide {
		max-width: 900px;
	}
	.wp-block[data-align="full"],
	.wp-block.alignfull {
		max-width: none;
	}
`

export const CANVAS_STYLES = [
	{ css: blockLibraryStyle },
	{ css: blockLibraryTheme },
	{ css: CANVAS_TYPOGRAPHY },
	{ css: CANVAS_COLUMN },
]
