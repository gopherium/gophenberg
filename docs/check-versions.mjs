// SPDX-License-Identifier: Apache-2.0

import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

import { pinsALiteral } from './version.mjs'

const pages = fileURLToPath(new URL('src/content/docs', import.meta.url))

/**
 * Returns every page under a directory.
 * @param dir - The directory to walk.
 * @returns The pages found, as absolute paths.
 */
function pagesUnder(dir) {
	const held = []
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		const path = join(dir, entry.name)
		if (entry.isDirectory()) {
			held.push(...pagesUnder(path))
		} else if (path.endsWith('.md') || path.endsWith('.mdx')) {
			held.push(path)
		}
	}
	return held
}

const pinned = pagesUnder(pages).filter((page) => pinsALiteral(readFileSync(page, 'utf8')))

if (pinned.length > 0) {
	for (const page of pinned) {
		console.error(`${page}: pins a version literal, use %VERSION%, %FEATURE_VERSION% or %KIT_VERSION%`)
	}
	process.exit(1)
}
