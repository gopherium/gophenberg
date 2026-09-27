// SPDX-License-Identifier: Apache-2.0

import { useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'

import { autosavePost } from './api'
import type { Autosave, AutosaveBuffer, AutosaveOutcome, PostDetail } from './api'
import { heldWrites, holdWrite } from './postWrites'

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
 */
export function useAutosave(postId: string, buffer: Parkable): void {
	const client = useQueryClient()
	const latest = useRef(buffer)
	useEffect(() => {
		latest.current = buffer
	})
	useEffect(() => {
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
			clearInterval(timer)
			window.removeEventListener('beforeunload', flush)
			leave(client, postId, latest.current)
		}
	}, [client, postId])
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
 */
function leave(client: QueryClient, postId: string, held: Parkable): void {
	if (!held.dirty) {
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
	void client.resetQueries({ queryKey: ['post-autosave', postId], exact: true })
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
		unloading ??= park(true)
	}
	window.addEventListener('beforeunload', flush)
	return earlier.then(() => {
		window.removeEventListener('beforeunload', flush)
		return unloading ?? park(false)
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
