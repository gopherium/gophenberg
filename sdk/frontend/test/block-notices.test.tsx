// SPDX-License-Identifier: Apache-2.0

import { RegistryProvider, createReduxStore, createRegistry, dispatch, select } from '@wordpress/data'
import { act, render } from '@testing-library/react'
import { StrictMode } from 'react'
import { beforeAll, expect, onTestFinished, test, vi } from 'vitest'

import { registerCuratedBlocks, useBlockNotices } from '../editor'

/** The store blocks leave their messages in. */
const NOTICES = 'core/notices'

/** What the hook hands a message to. */
type Show = Parameters<typeof useBlockNotices>[0]

/** What the store of messages takes from a block. */
interface NoticesWriter {
	createErrorNotice: (words: string, options: Record<string, unknown>) => Promise<unknown>
}

/** What empties the store of messages of one kind. */
interface AllRemover {
	removeAllNotices: (kind: string) => Promise<unknown>
}

/** One message held by a registry apart from the default one. */
interface Left {
	id: string
	content: string
	actions: unknown[]
}

/** What a store of messages apart from the default one is told. */
interface Change {
	type: string
	notice?: Left
	id?: string
}

/**
 * Returns the messages a registry apart from the default one holds after a change.
 * @param left - The messages before it.
 * @param change - A message left or one taken off.
 * @returns The messages after it.
 */
function keep(left: Left[] = [], change: Change): Left[] {
	if (change.notice !== undefined) {
		return [...left, change.notice]
	}
	return left.filter((notice) => notice.id !== change.id)
}

/** A store of messages under the name blocks leave them in, for a registry apart from the default one. */
const APART = createReduxStore(NOTICES, {
	reducer: keep,
	actions: {
		leave: (notice: Left) => ({ type: 'leave', notice }),
		removeNotice: (id: string) => ({ type: 'remove', id }),
	},
	selectors: {
		getNotices: (left: Left[]) => left,
	},
})

/**
 * Returns a registry apart from the default one, holding a store of messages of its own.
 * @returns The registry.
 */
function apartRegistry() {
	const registry = createRegistry()
	registry.register(APART)
	return registry
}

beforeAll(() => {
	registerCuratedBlocks()
}, 120000)

/**
 * Follows the messages blocks leave.
 * @param props - What takes each message.
 * @returns Nothing to draw.
 */
function Reader({ show }: { show: Show }) {
	useBlockNotices(show)
	return null
}

/**
 * Leaves a message as a block does.
 * @param words - The words of the message.
 * @param actions - What the message offers.
 */
async function leave(words: string, actions: unknown[] = []) {
	await act(async () => {
		await (dispatch(NOTICES) as unknown as NoticesWriter).createErrorNotice(words, { type: 'snackbar', actions })
	})
}

/**
 * Returns how many messages the store still holds.
 * @returns The count.
 */
function held(): number {
	return (select(NOTICES) as unknown as { getNotices: () => unknown[] }).getNotices().length
}

test('hands on a message a block leaves, once even under strict mode, and takes it off the store', async () => {
	const show = vi.fn<Show>()
	render(
		<StrictMode>
			<Reader show={show} />
		</StrictMode>,
	)

	await leave('harbor.zzz: the file type is not allowed')

	expect(show.mock.calls).toEqual([['harbor.zzz: the file type is not allowed', undefined]])
	expect(held()).toBe(0)
})

test('hands on a message left before the reader mounted', async () => {
	const show = vi.fn<Show>()
	await leave('A message left earlier')

	render(<Reader show={show} />)

	expect(show).toHaveBeenCalledWith('A message left earlier', undefined)
	expect(held()).toBe(0)
})

test('hands on the action a message offers', async () => {
	const show = vi.fn<Show>()
	const undo = vi.fn()
	render(<Reader show={show} />)

	await leave('Image cropped.', [{ label: 'Undo', onClick: undo }])

	const action = show.mock.calls[0][1]
	expect(action?.label).toBe('Undo')
	action?.onAct()
	expect(undo).toHaveBeenCalledOnce()
})

test('offers no action for a message whose action only links away', async () => {
	const show = vi.fn<Show>()
	render(<Reader show={show} />)

	await leave('Read more.', [{ label: 'Open', url: 'https://example.com/' }])

	expect(show).toHaveBeenCalledWith('Read more.', undefined)
})

test('follows the messages of the registry it is drawn under, leaving the default one alone', async () => {
	const show = vi.fn<Show>()
	const registry = apartRegistry()
	onTestFinished(() => {
		void (dispatch(NOTICES) as unknown as AllRemover).removeAllNotices('snackbar')
	})
	render(
		<RegistryProvider value={registry}>
			<Reader show={show} />
		</RegistryProvider>,
	)

	await act(async () => {
		await registry.dispatch(APART).leave({ id: 'apart', content: 'Left apart.', actions: [] })
	})
	await leave('Left in the default registry.')

	expect(show.mock.calls).toEqual([['Left apart.', undefined]])
	expect(registry.select(APART).getNotices()).toEqual([])
	expect(held()).toBe(1)
})
