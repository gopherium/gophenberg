// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'

import { describe, expect, test } from 'vitest'

import * as config from '../config.ts'
import * as root from '../index.ts'

/** The manifest the package ships. */
const manifest = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8')) as {
	exports: Record<string, string>
}

describe('the entry a theme imports at runtime', () => {
	test('serves the theme contract and the content it renders', () => {
		expect(Object.keys(root)).toEqual(
			expect.arrayContaining(['defineTheme', 'GophenbergClient', 'parseBlocks', 'siteAssetUrls']),
		)
	})

	test('leaves the integration out, so a theme bundle carries no bundler', () => {
		expect(Object.keys(root)).not.toContain('gophenberg')
	})

	test('reaches no build-time module, which would drag vite into the artifact', () => {
		const source = readFileSync(new URL('../index.ts', import.meta.url), 'utf8')

		expect(source).not.toContain('./integration.ts')
		expect(source).not.toContain('./profile.ts')
	})
})

describe('the entry an astro config imports at build time', () => {
	test('serves the integration', () => {
		expect(config.gophenberg).toBeTypeOf('function')
	})

	test('is published under its own subpath', () => {
		expect(manifest.exports['./config']).toBe('./config.ts')
	})
})
