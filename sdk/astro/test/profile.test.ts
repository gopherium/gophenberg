// SPDX-License-Identifier: Apache-2.0

import { describe, expect, test } from 'vitest'

import { buildProfile, profileComplaint } from '../profile.ts'

describe('buildProfile', () => {
	test('asks for a server that renders on demand', () => {
		expect(buildProfile().output).toBe('server')
	})

	test('asks for every dependency to be bundled into the artifact', () => {
		expect(buildProfile().vite.ssr.noExternal).toBe(true)
	})
})

describe('profileComplaint', () => {
	test('passes a config that keeps the profile', () => {
		expect(profileComplaint({ output: 'server', image: { service: { entrypoint: 'astro/assets/services/noop' } } }))
			.toBeUndefined()
	})

	test('names the setting when a theme renders statically', () => {
		const complaint = profileComplaint({ output: 'static' })

		expect(complaint).toContain('output')
		expect(complaint).toContain('server')
	})

	test('names the setting when a theme keeps an image service needing native code', () => {
		const complaint = profileComplaint({
			output: 'server',
			image: { service: { entrypoint: 'astro/assets/services/sharp' } },
		})

		expect(complaint).toContain('image')
		expect(complaint).toContain('sharp')
	})

	test('accepts a config that names no image service', () => {
		expect(profileComplaint({ output: 'server' })).toBeUndefined()
	})
})
