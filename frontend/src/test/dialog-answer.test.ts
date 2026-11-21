// SPDX-License-Identifier: Apache-2.0

import { expect, test } from 'vitest'

import { unlessTimedOut } from '../dialogAnswer'

/** What a write failed with. */
const FAILURE = new Error('The registry could not be reached.')

test('answers a failed write with the words given while the time limit has not run out', () => {
	const answer = unlessTimedOut(new AbortController().signal, FAILURE, () => ({ error: 'Not saved.' }))

	expect(answer).toEqual({ error: 'Not saved.' })
})

test('hands a failed write back to the dialog once the time limit ran out', async () => {
	const signal = AbortSignal.abort(new DOMException('Could not get a valid response from the server.', 'TimeoutError'))

	await expect(unlessTimedOut(signal, FAILURE, () => ({ error: 'Not saved.' }))).rejects.toBe(FAILURE)
})
