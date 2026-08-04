// SPDX-License-Identifier: Apache-2.0

import { describe, expect, test } from 'vitest'

import { buildProfile, profileIssue } from '../profile.ts'
import type { ProfileConfig } from '../profile.ts'

/**
 * Returns a config holding every pinned setting.
 * @returns The config a compliant build resolves to.
 */
function heldConfig(): ProfileConfig {
	return {
		output: 'server',
		image: { service: { entrypoint: 'astro/assets/services/noop' } },
		vite: { ssr: { noExternal: true } },
	}
}

describe('buildProfile', () => {
	test('asks for a server that renders on demand', () => {
		expect(buildProfile().output).toBe('server')
	})

	test('asks for every dependency to be bundled into the artifact', () => {
		expect(buildProfile().vite.ssr.noExternal).toBe(true)
	})
})

describe('profileIssue', () => {
	test('passes a config that keeps the profile', () => {
		expect(profileIssue(heldConfig())).toBeUndefined()
	})

	test('passes the profile the integration itself applies', () => {
		expect(profileIssue(buildProfile())).toBeUndefined()
	})

	test('names the setting when a theme renders statically', () => {
		const issue = profileIssue({ ...heldConfig(), output: 'static' })

		expect(issue).toContain('output')
		expect(issue).toContain('server')
	})

	test('names the setting when a theme keeps an image service needing native code', () => {
		const issue = profileIssue({
			...heldConfig(),
			image: { service: { entrypoint: 'astro/assets/services/sharp' } },
		})

		expect(issue).toContain('image')
		expect(issue).toContain('sharp')
	})

	test('names the setting when a theme names no image service', () => {
		const issue = profileIssue({ ...heldConfig(), image: undefined })

		expect(issue).toContain('image')
		expect(issue).toContain('astro/assets/services/noop')
	})

	test('names the setting when a theme lost the bundling pin', () => {
		const issue = profileIssue({ ...heldConfig(), vite: { ssr: { noExternal: false } } })

		expect(issue).toContain('noExternal')
		expect(issue).toContain('true')
	})

	test('names the adapter when a theme swapped it', () => {
		const issue = profileIssue({ ...heldConfig(), adapter: { name: '@astrojs/vercel' } })

		expect(issue).toContain('adapter')
		expect(issue).toContain('@astrojs/node')
	})

	test('accepts the adapter the kit itself injects', () => {
		expect(profileIssue({ ...heldConfig(), adapter: { name: '@astrojs/node' } })).toBeUndefined()
	})
})
