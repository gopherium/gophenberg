// SPDX-License-Identifier: Apache-2.0

import type { QueryClient } from '@tanstack/react-query'

/** The writes each query client still waits on, by the post they write to. */
const inFlight = new WeakMap<QueryClient, Map<string, Promise<unknown>>>()

/**
 * Returns the writes a query client still waits on.
 * @param client - The query client the writes belong to.
 * @returns The writes by post.
 */
function writesOf(client: QueryClient): Map<string, Promise<unknown>> {
	const known = inFlight.get(client)
	if (known !== undefined) {
		return known
	}
	const fresh = new Map<string, Promise<unknown>>()
	inFlight.set(client, fresh)
	return fresh
}

/**
 * Holds a write to a post until it answers, so a later read of that post waits for it.
 * @param client - The query client the write belongs to.
 * @param postId - The post the write goes to.
 * @param write - The write in flight.
 * @returns The same write.
 */
export function holdWrite<T>(client: QueryClient, postId: string, write: Promise<T>): Promise<T> {
	const writes = writesOf(client)
	const entry = Promise.allSettled([writes.get(postId), write])
	writes.set(postId, entry)
	void entry.then(() => {
		if (writes.get(postId) === entry) {
			writes.delete(postId)
		}
	})
	return write
}

/**
 * Returns a promise settling once every write held for a post has answered.
 * @param client - The query client the writes belong to.
 * @param postId - The post to wait for.
 * @returns The promise.
 */
export function writesSettled(client: QueryClient, postId: string): Promise<unknown> {
	return writesOf(client).get(postId) ?? Promise.resolve()
}
