// SPDX-License-Identifier: Apache-2.0

import { describe, expect, test } from 'vitest'

import { kitFeatureVersion } from '../kit.ts'
import { GET, prerender } from '../routes/health.ts'

describe('the readiness probe', () => {
	test('answers that the theme process is serving', async () => {
		const response = await GET({} as never)

		expect(response.status).toBe(200)
		const answered = (await response.json()) as { gophenberg: string; ready: boolean }

		expect(answered).toEqual({ gophenberg: kitFeatureVersion, ready: true })
		expect(answered.gophenberg).toMatch(/^\d+\.\d+$/)
	})

	test('answers as JSON, since a supervisor reads it rather than a reader', async () => {
		const response = await GET({} as never)

		expect(response.headers.get('Content-Type')).toContain('application/json')
	})

	test('is answered on demand rather than baked at build time', () => {
		expect(prerender).toBe(false)
	})
})
