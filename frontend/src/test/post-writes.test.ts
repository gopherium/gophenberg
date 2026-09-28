// SPDX-License-Identifier: Apache-2.0

import { QueryClient } from '@tanstack/react-query'
import { expect, test } from 'vitest'

import { heldWrites, holdWrite, writesSettled } from '../content/postWrites'

/**
 * Returns a write that answers when the test says so.
 * @returns The write and the function answering it.
 */
function pendingWrite(): { write: Promise<void>, answer: () => void } {
	let answer = () => {}
	const write = new Promise<void>((resolve) => {
		answer = resolve
	})
	return { write, answer }
}

/**
 * Lets every promise already settled run its callbacks.
 */
async function settle() {
	await new Promise((resolve) => setTimeout(resolve, 0))
}

test('keeps waiting for a later write after an earlier one answers', async () => {
	const client = new QueryClient()
	const earlier = pendingWrite()
	const later = pendingWrite()
	holdWrite(client, 'post', earlier.write)
	holdWrite(client, 'post', later.write)
	earlier.answer()
	await settle()
	let waited = false

	void writesSettled(client, 'post').then(() => {
		waited = true
	})
	await settle()

	expect(waited).toBe(false)
	later.answer()
	await settle()
	expect(waited).toBe(true)
})

test('holds nothing for a post once every write to it answered', async () => {
	const client = new QueryClient()
	const only = pendingWrite()
	holdWrite(client, 'post', only.write)
	expect(heldWrites(client, 'post')).toBeDefined()

	only.answer()
	await settle()

	expect(heldWrites(client, 'post')).toBeUndefined()
})

test('holds nothing for a post once a write to it was refused', async () => {
	const client = new QueryClient()
	holdWrite(client, 'post', Promise.reject(new Error('refused'))).catch(() => {})

	await settle()

	expect(heldWrites(client, 'post')).toBeUndefined()
})

test('keeps the writes of each query client apart', () => {
	const held = new QueryClient()
	holdWrite(held, 'post', pendingWrite().write)

	expect(heldWrites(new QueryClient(), 'post')).toBeUndefined()
})
