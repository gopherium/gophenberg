// SPDX-License-Identifier: Apache-2.0

import type { QueryClient } from '@tanstack/react-query'

import { typesQueryKey } from './nav'
import { holdWrite, writesSettled } from './postWrites'
import { deleteType, updateType } from './types'
import type { ContentType, TypeEdit } from './types'

/**
 * Reads the registry again, for the list, the menu and every screen holding it.
 * @param client - The query client holding the registry.
 * @returns A promise settling once the registry was read again.
 */
export function refreshTypes(client: QueryClient): Promise<void> {
	return client.invalidateQueries({ queryKey: typesQueryKey })
}

/**
 * Returns the name the writes to a type are held under, apart from the writes to any item.
 * @param key - The key of the type.
 * @returns The name.
 */
function slotOf(key: string): string {
	return `content type ${key}`
}

/**
 * Returns a promise settling once every write held under the name has answered, or failing once the signal fires.
 * @param client - The query client the writes belong to.
 * @param slot - The name the writes are held under.
 * @param signal - The signal that stops the wait, none for a wait lasting until the writes answer.
 * @returns The promise.
 */
function turnOf(client: QueryClient, slot: string, signal?: AbortSignal): Promise<unknown> {
	const earlier = writesSettled(client, slot)
	if (signal === undefined) {
		return earlier
	}
	return new Promise<void>((settle, fail) => {
		const stop = () => fail(signal.reason)
		signal.throwIfAborted()
		signal.addEventListener('abort', stop, { once: true })
		void earlier.then(() => {
			signal.removeEventListener('abort', stop)
			settle()
		})
	})
}

/**
 * Sends a write to a type once every earlier write to it has answered, holding it until it answers in turn.
 * @param client - The query client the writes belong to.
 * @param key - The key of the type.
 * @param write - Sends the write.
 * @param signal - The signal that stops the wait for its turn, none for a write that waits for every earlier one.
 * @returns The write.
 */
function inTurn<T>(client: QueryClient, key: string, write: () => Promise<T>, signal?: AbortSignal): Promise<T> {
	const slot = slotOf(key)
	return holdWrite(client, slot, turnOf(client, slot, signal).then(write))
}

/**
 * Edits a type once every earlier write to it has answered, so the registry never takes two writes of one type at once.
 * @param client - The query client the writes belong to.
 * @param key - The key of the type.
 * @param edit - The fields to move.
 * @param signal - The signal that stops its wait and its request, none for an edit that waits for its answer.
 * @returns The stored type.
 */
export function editInTurn(
	client: QueryClient,
	key: string,
	edit: TypeEdit,
	signal?: AbortSignal,
): Promise<ContentType> {
	return inTurn(client, key, () => updateType(key, edit, signal), signal)
}

/**
 * Removes a type once every earlier write to it has answered.
 * @param client - The query client the writes belong to.
 * @param key - The key of the type.
 * @param signal - The signal that stops its wait and its request, none for a removal that waits for its answer.
 * @returns The removal.
 */
export function deleteInTurn(client: QueryClient, key: string, signal?: AbortSignal): Promise<void> {
	return inTurn(client, key, () => deleteType(key, signal), signal)
}
