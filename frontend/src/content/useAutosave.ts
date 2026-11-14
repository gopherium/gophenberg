// SPDX-License-Identifier: Apache-2.0

import { useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useRef, useState } from 'react'
import type { Dispatch, SetStateAction } from 'react'

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
			if (!(trashed.current || parking.current)) {
				leave(client, postId, latest.current, false)
			}
		}
	}, [client, postId])
	return useCallback(async () => {
		parking.current = true
		const next = () => nextCommit(closed.current, committed.current, rerender).then(() => latest.current)
		const flush = () => parkAtUnload(client, postId, latest.current, closed.current)
		window.addEventListener('beforeunload', flush)
		try {
			await writesSettled(client, postId)
			await parkUntilStill(client, postId, next)
		} catch (failure) {
			if (closed.current) {
				leave(client, postId, latest.current, shownAgain(client, postId))
			}
			throw failure
		} finally {
			parking.current = false
			window.removeEventListener('beforeunload', flush)
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
 * Waits for the editor's next commit while it is open, and not at all once it has closed.
 * @param closed - Whether the editor has closed.
 * @param waiting - The waiters the next commit lets go of.
 * @param rerender - Asks the editor for another render.
 * @returns A promise settling once the commit lands.
 */
function nextCommit(
	closed: boolean,
	waiting: (() => void)[],
	rerender: Dispatch<SetStateAction<number>>,
): Promise<void> {
	if (closed) {
		return Promise.resolve()
	}
	return new Promise<void>((resume) => {
		waiting.push(resume)
		rerender((renders) => renders + 1)
	})
}

/**
 * Parks the unsaved words of the buffer, again each time they changed while the last park ran.
 * @param client - The query client caching the post.
 * @param postId - The post the words belong to.
 * @param next - Waits for the editor's next commit and returns the buffer it holds.
 * @param parked - The words the last park sent, nothing before the first.
 */
async function parkUntilStill(
	client: QueryClient,
	postId: string,
	next: () => Promise<Parkable>,
	parked?: AutosaveBuffer,
): Promise<void> {
	const held = await next()
	const words = wordsOf(held)
	if (!held.dirty || sentAlready(parked, words)) {
		return
	}
	const write = autosavePost(postId, words, held.version)
	await holdWrite(client, postId, write.then((outcome) => land(client, postId, held, outcome)))
	await parkUntilStill(client, postId, next, words)
}

/**
 * Tells whether the last park sent the words held, the shown fields compared by reference.
 * @param parked - The words the last park sent, nothing before the first.
 * @param words - The words held.
 * @returns Whether the last park sent them.
 */
function sentAlready(parked: AutosaveBuffer | undefined, words: AutosaveBuffer): boolean {
	return parked !== undefined &&
		parked.title === words.title &&
		parked.content === words.content &&
		parked.excerpt === words.excerpt &&
		parked.fields === words.fields
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
 * Parks the unsaved words a closed editor left as the page unloads, and nothing while the editor is open.
 * @param client - The query client caching the post.
 * @param postId - The post the words belong to.
 * @param held - The buffer as the editor last committed it.
 * @param closed - Whether the editor has closed.
 */
function parkAtUnload(client: QueryClient, postId: string, held: Parkable, closed: boolean): void {
	if (closed && held.dirty) {
		send(client, postId, held, autosavePost(postId, wordsOf(held), held.version, true))
	}
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
 * Tells whether an editor shows the post again from its cached copy.
 * @param client - The query client caching the post.
 * @param postId - The post.
 * @returns Whether an editor shows it.
 */
function shownAgain(client: QueryClient, postId: string): boolean {
	return client.getQueriesData({ queryKey: ['post', postId], exact: true, type: 'active' }).length > 0
}

/**
 * Parks unsaved words as the editor closes and drops the cached post they supersede, unless an editor shows it again.
 * @param client - The query client caching the post.
 * @param postId - The post being left.
 * @param held - The buffer as the editor closes.
 * @param shown - Whether an editor shows the post again, which keeps the cached post it uses.
 */
function leave(client: QueryClient, postId: string, held: Parkable, shown: boolean): void {
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
	if (!shown) {
		void client.resetQueries({ queryKey: ['post', postId], exact: true })
	}
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
