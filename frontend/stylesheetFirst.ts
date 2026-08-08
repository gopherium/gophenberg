// SPDX-License-Identifier: Apache-2.0

import type { Plugin } from 'vite'

const STYLESHEET = /[ \t]*<link[^>]+rel="stylesheet"[^>]*>\n?/g
const MODULE_SCRIPT = /[ \t]*<script[^>]+type="module"[^>]*><\/script>/

/**
 * Returns the page with its stylesheets moved above the first module script.
 * @param html - The built page source.
 * @returns The page source, stylesheets first.
 */
export function hoistStylesheet(html: string): string {
	const sheets = html.match(STYLESHEET)
	const script = html.match(MODULE_SCRIPT)
	if (sheets === null || script === null) {
		return html
	}
	if (html.indexOf(sheets[0]) < script.index!) {
		return html
	}
	const stripped = html.replace(STYLESHEET, '')
	return stripped.replace(MODULE_SCRIPT, sheets.join('') + script[0])
}

/**
 * Returns a bundler plugin serving stylesheets before the module script.
 * @returns The plugin to add to the bundler configuration.
 */
export function stylesheetFirst(): Plugin {
	return {
		name: 'stylesheet-first',
		transformIndexHtml: {
			order: 'post',
			handler: hoistStylesheet,
		},
	}
}
