// SPDX-License-Identifier: Apache-2.0

import { afterEach, describe, expect, test, vi } from 'vitest'

import { GophenbergClient } from '../client.ts'
import type { Handshake } from '../content.ts'
import { forgetSite, siteProfile } from '../site.ts'

/** What a current host answers a theme asking who it is. */
const answered: Handshake = { gophenberg: '0.8.0', api: 0, kit: ['0.9.0'], types: [] }

/**
 * Returns a client double answering the handshake, counting how often it was asked.
 * @param reply - What the handshake answers, or an error to throw.
 * @returns The double and its call count.
 */
function clientAnswering(reply: Handshake | Error) {
	const handshake = vi.fn(async () => {
		if (reply instanceof Error) {
			throw reply
		}
		return reply
	})
	return { client: { handshake } as unknown as GophenbergClient, handshake }
}

afterEach(() => {
	forgetSite()
})

describe('what the theme learns about the site it serves', () => {
	test('carries the versions the site answered', async () => {
		const { client } = clientAnswering(answered)

		const got = await siteProfile(client)

		expect(got).toEqual({ gophenberg: '0.8.0', api: 0, kit: ['0.9.0'] })
	})

	test('asks the site once and remembers the answer', async () => {
		const { client, handshake } = clientAnswering(answered)

		await siteProfile(client)
		await siteProfile(client)

		expect(handshake).toHaveBeenCalledTimes(1)
	})

	test('reads no kit list from a host too old to advertise one', async () => {
		const { client } = clientAnswering({ gophenberg: '0.7.0', api: 2, types: [] } as Handshake)

		const got = await siteProfile(client)

		expect(got?.kit).toEqual([])
	})
})

describe('when the site cannot be reached', () => {
	test('answers nothing rather than throwing', async () => {
		const { client } = clientAnswering(new Error('connect ECONNREFUSED'))

		await expect(siteProfile(client)).resolves.toBeUndefined()
	})

	test('asks again next time, since the host may not be listening yet', async () => {
		const { client, handshake } = clientAnswering(new Error('connect ECONNREFUSED'))

		await siteProfile(client)
		await siteProfile(client)

		expect(handshake).toHaveBeenCalledTimes(2)
	})

	test('remembers the first answer it does get', async () => {
		const failing = clientAnswering(new Error('connect ECONNREFUSED'))
		const serving = clientAnswering(answered)

		await siteProfile(failing.client)
		const got = await siteProfile(serving.client)

		expect(got?.gophenberg).toBe('0.8.0')
	})
})
