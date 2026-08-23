// SPDX-License-Identifier: Apache-2.0

import { afterEach, describe, expect, test, vi } from 'vitest'

import { kitVersion } from '../kit.ts'
import { GET, prerender } from '../routes/health.ts'
import * as site from '../site.ts'

/** What a readiness answer carries. */
interface Readiness {
	gophenberg?: string
	ready: boolean
	kit?: string
	served?: string[]
	reason?: string
}

/**
 * Answers the next handshake with the given profile, or nothing when the site is unreachable.
 * @param profile - What the site answered.
 */
function siteAnswering(profile: { gophenberg: string; api: number; kit: string[] } | undefined): void {
	vi.spyOn(site, 'siteProfile').mockResolvedValue(profile)
}

afterEach(() => {
	vi.restoreAllMocks()
	site.forgetSite()
})

describe('the readiness probe when the site serves this kit', () => {
	test('answers that the theme process is serving', async () => {
		siteAnswering({ gophenberg: '0.8.0', api: 0, kit: [kitVersion] })

		const response = await GET({} as never)

		expect(response.status).toBe(200)
		expect((await response.json()) as Readiness).toEqual({ gophenberg: '0.8.0', ready: true })
	})

	test('reports the version the site runs, not the version the kit ships at', async () => {
		siteAnswering({ gophenberg: '1.2.3', api: 1, kit: [kitVersion] })

		const answered = (await (await GET({} as never)).json()) as Readiness

		expect(answered.gophenberg).toBe('1.2.3')
	})

	test('answers as JSON, since a supervisor reads it rather than a reader', async () => {
		siteAnswering({ gophenberg: '0.8.0', api: 0, kit: [kitVersion] })

		const response = await GET({} as never)

		expect(response.headers.get('Content-Type')).toContain('application/json')
	})
})

describe('the readiness probe when the site does not serve this kit', () => {
	test('refuses to report ready', async () => {
		siteAnswering({ gophenberg: '0.8.0', api: 0, kit: ['0.1.0'] })

		const response = await GET({} as never)

		expect(response.status).toBe(503)
		expect(((await response.json()) as Readiness).ready).toBe(false)
	})

	test('names the kit it was built with and the kits the site serves', async () => {
		siteAnswering({ gophenberg: '0.8.0', api: 0, kit: ['0.1.0'] })

		const answered = (await (await GET({} as never)).json()) as Readiness

		expect(answered.kit).toBe(kitVersion)
		expect(answered.served).toEqual(['0.1.0'])
		expect(answered.reason).toMatch(/kit/)
	})
})

describe('the readiness probe when the site cannot be reached', () => {
	test('refuses to report ready, naming why', async () => {
		siteAnswering(undefined)

		const response = await GET({} as never)

		expect(response.status).toBe(503)
		const answered = (await response.json()) as Readiness

		expect(answered.ready).toBe(false)
		expect(answered.reason).toMatch(/answer/)
	})
})

describe('how the probe is served', () => {
	test('is answered on demand rather than baked at build time', () => {
		expect(prerender).toBe(false)
	})
})
