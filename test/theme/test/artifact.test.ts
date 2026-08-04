// SPDX-License-Identifier: Apache-2.0

import { existsSync, readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'

import { describe, expect, test } from 'vitest'

/** Where the build leaves the artifact a theme directory ships. */
const artifact = new URL('../dist/', import.meta.url).pathname

/** How an import names a module in the built bundle. */
const staticImport = /(?:^|[\s;}])(?:import(?:\s[^'";]*?\bfrom)?|export\s[^'";]*?\bfrom)\s*['"]([^'"]+)['"]/gm

/** How a bundled module names one it loads on demand. */
const dynamicImport = /\bimport\s*\(\s*['"]([^'"]+)['"]/gm

/**
 * Returns every JavaScript file under a directory.
 * @param directory - Where to look.
 * @returns The paths found.
 */
function bundledFiles(directory: string): string[] {
	return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
		const path = join(directory, entry.name)
		if (entry.isDirectory()) {
			return bundledFiles(path)
		}
		return path.endsWith('.mjs') || path.endsWith('.js') ? [path] : []
	})
}

/**
 * Returns every module the bundle names.
 * @param files - The bundled files to read.
 * @returns The specifiers found.
 */
function namedModules(files: string[]): string[] {
	const found = new Set<string>()
	for (const file of files) {
		const source = readFileSync(file, 'utf8')
		for (const pattern of [staticImport, dynamicImport]) {
			pattern.lastIndex = 0
			let match = pattern.exec(source)
			while (match) {
				found.add(match[1] as string)
				match = pattern.exec(source)
			}
		}
	}
	return [...found]
}

const built = existsSync(join(artifact, 'server', 'entry.mjs'))

describe('what a theme directory declares about itself', () => {
	test('names itself, its version, and the kit it was built against', () => {
		const manifest = JSON.parse(readFileSync(new URL('../theme.json', import.meta.url), 'utf8')) as Record<
			string,
			string
		>

		expect(manifest).toEqual({ name: 'starter', version: '0.1.0', kit: '^0.1.0' })
	})
})

describe.skipIf(!built)('the artifact a theme directory ships', () => {
	test('holds the server entry the supervisor spawns', () => {
		expect(existsSync(join(artifact, 'server', 'entry.mjs'))).toBe(true)
	})

	test('holds the client assets the server serves', () => {
		expect(existsSync(join(artifact, 'client'))).toBe(true)
	})

	test('names no module outside itself, so it runs without node_modules', () => {
		const foreign = namedModules(bundledFiles(join(artifact, 'server'))).filter(
			(name) => !name.startsWith('./') && !name.startsWith('../') && !name.startsWith('node:'),
		)

		expect(foreign).toEqual([])
	})

	test('reads the instance address at request time rather than baking it in', () => {
		const entry = readFileSync(join(artifact, 'server', 'entry.mjs'), 'utf8')
		const chunks = bundledFiles(join(artifact, 'server')).map((file) => readFileSync(file, 'utf8'))

		expect([entry, ...chunks].some((source) => source.includes('GOPHENBERG_API_URL'))).toBe(true)
	})
})

test.skipIf(built)('the artifact tests need a build, run pnpm --filter @gophenberg/theme-starter build', () => {
	expect(built).toBe(false)
})
