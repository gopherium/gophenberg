// SPDX-License-Identifier: Apache-2.0

import { useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useRef, useState } from 'react'

import { autosavePost } from './api'
import type { Autosave, AutosaveBuffer, AutosaveOutcome, PostDetail } from './api'
import { heldWrites, holdWrite, writesSettled } from './postWrites'

const AUTOSAVE_INTERVAL = 60000

const AUTOSAVE_TARGET_POST = 'post'

/** The editing buffer autosave parks, with what it needs to follow the server. */
type Parkable = AutosaveBuffer & {
	shown: Record<string, unknown>
	dirty: boolean
	saving: boolean
	version: string
	adoptParked: (parked: Autosave) => void
}

/**
 * Parks the editing buffer on the server while it holds unsaved words, and as the editor closes.
 * @param postId - The post being edited.
 * @param buffer - The words held, and whether they are unsaved.
 * @returns What parks the unsaved words at once, failing when rejected, answering with what marks the post trashed.
 */
export function useAutosave(postId: string, buffer: Parkable): () => Promise<() => void> {
	const client = useQueryClient()
	const latest = useRef(buffer)
	const trashed = useRef(false)
	const parking = useRef(false)
	const closed = useRef(false)
	const committed = useRef<(() => void)[]>([])
	const [, rerender] = useState(0)
	useEffect(() => {
		latest.current = buffer
		release(committed.current)
	})
	useEffect(() => {
		closed.current = false
		const waiting = committed.current
		/**
		 * Parks the buffer when it holds unsaved words and no write is in flight.
		 * @param keepalive - Whether the request should outlive the page.
		 */
		function park(keepalive: boolean) {
			const held = latest.current
			if (held.dirty && !held.saving) {
				send(client, postId, held, autosavePost(postId, wordsOf(held), held.version, keepalive))
			}
		}
		const timer = setInterval(() => park(false), AUTOSAVE_INTERVAL)
		const flush = () => park(true)
		window.addEventListener('beforeunload', flush)
		return () => {
			closed.current = true
			clearInterval(timer)
			window.removeEventListener('beforeunload', flush)
			release(waiting)
			leave(client, postId, latest.current, trashed.current || parking.current)
		}
	}, [client, postId])
	return useCallback(async () => {
		parking.current = true
		try {
			await writesSettled(client, postId)
			if (!closed.current) {
				await new Promise<void>((resume) => {
					committed.current.push(resume)
					rerender((renders) => renders + 1)
				})
			}
			const held = latest.current
			if (held.dirty) {
				const write = autosavePost(postId, wordsOf(held), held.version)
				await holdWrite(client, postId, write.then((outcome) => land(client, postId, held, outcome)))
			}
		} finally {
			parking.current = false
		}
		return () => {
			trashed.current = true
		}
	}, [client, postId])
}

/**
 * Lets go of everything waiting for the editor's next commit.
 * @param waiting - The waiters, emptied as each is called.
 */
function release(waiting: (() => void)[]): void {
	for (const resume of waiting.splice(0)) {
		resume()
	}
}

/**
 * Returns the words a buffer parks.
 * @param held - The buffer.
 * @returns The words.
 */
function wordsOf(held: Parkable): AutosaveBuffer {
	return { title: held.title, content: held.content, excerpt: held.excerpt, fields: held.shown }
}

/**
 * Holds an autosave until it answers and lands its answer in the cache.
 * @param client - The query client caching the post.
 * @param postId - The post the words belong to.
 * @param held - The buffer the words came from.
 * @param write - The autosave in flight.
 */
function send(client: QueryClient, postId: string, held: Parkable, write: Promise<AutosaveOutcome>): void {
	void holdWrite(client, postId, write.then((outcome) => land(client, postId, held, outcome))).catch(() => {})
}

/**
 * Parks unsaved words as the editor closes and drops the cached post they supersede.
 * @param client - The query client caching the post.
 * @param postId - The post being left.
 * @param held - The buffer as the editor closes.
 * @param trashed - Whether the post went to the trash or Move to trash parks the words, either taking no autosave.
 */
function leave(client: QueryClient, postId: string, held: Parkable, trashed: boolean): void {
	if (!held.dirty || trashed) {
		return
	}
	const words = wordsOf(held)
	const version = held.version
	/**
	 * Sends the words the editor closed with.
	 * @param keepalive - Whether the request should outlive the page.
	 * @returns Where the words landed.
	 */
	const park = (keepalive: boolean) => autosavePost(postId, words, version, keepalive)
	const earlier = heldWrites(client, postId)
	send(client, postId, held, earlier === undefined ? park(false) : afterEarlier(earlier, park))
	void client.resetQueries({ queryKey: ['post', postId], exact: true })
}

/**
 * Parks words once the writes before them answer, or at once if the page unloads first.
 * @param earlier - The writes to wait for.
 * @param park - Sends the words, asking the request to outlive the page or not.
 * @returns The autosave, sent once.
 */
function afterEarlier(
	earlier: Promise<unknown>,
	park: (keepalive: boolean) => Promise<AutosaveOutcome>,
): Promise<AutosaveOutcome> {
	let unloading: Promise<AutosaveOutcome> | undefined
	const flush = () => {
		if (unloading !== undefined) {
			return
		}
		const attempt = park(true)
		unloading = attempt
		attempt.catch(() => {
			unloading = undefined
		})
	}
	window.addEventListener('beforeunload', flush)
	return earlier.then(() => {
		window.removeEventListener('beforeunload', flush)
		return unloading === undefined ? park(false) : unloading.catch(() => park(false))
	})
}

/**
 * Brings the buffer and the cached post in line with an autosave the server wrote into the post itself.
 * @param client - The query client caching the post.
 * @param postId - The post the words belong to.
 * @param held - The buffer the words came from.
 * @param outcome - Where the words landed and what the server holds there.
 */
function land(client: QueryClient, postId: string, held: Parkable, outcome: AutosaveOutcome): void {
	if (outcome.target !== AUTOSAVE_TARGET_POST) {
		return
	}
	held.adoptParked(outcome)
	client.setQueryData<PostDetail>(['post', postId], (cached) =>
		cached && {
			...cached,
			title: outcome.title,
			content: outcome.content,
			excerpt: outcome.excerpt,
			fields: { ...cached.fields, ...outcome.fields },
			updatedAt: outcome.savedAt,
		},
	)
	void client.invalidateQueries({ queryKey: ['posts'] })
}
