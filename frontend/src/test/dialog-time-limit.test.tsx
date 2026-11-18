// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { Toaster } from '@gopherium/godmin'
import type { ListRun } from '@gopherium/godmin'
import { renderAdmin } from '@gopherium/godmin/testing'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactNode } from 'react'
import { expect, test, vi } from 'vitest'

import { OpenPostDialog, usePostTools } from '../content/actions'
import { fetchPost } from '../content/api'
import type { Post } from '../content/api'
import { OpenTypeDialog, useTypeTools } from '../content/typeActions'
import { AddTypeDialog, MakeDefaultConfirm } from '../content/TypeModals'
import { listTypes } from '../content/types'
import type { ContentType } from '../content/types'
import { editInTurn } from '../content/typeWrites'
import { useMediaTools } from '../media/actions'
import { listMediaByIDs } from '../media/api'
import type { MediaItem } from '../media/api'
import { OpenMediaDialog } from '../media/MediaModals'
import { gate } from './gate'
import { CLIFF, HARBOR } from './mediaPicker'
import { storedPostWithId } from './postFixture'
import { GUIDE_TYPE, PAGE_TYPE, serveTypes } from './typeFixtures'

/** WordPress's words for a write the server never answered. */
const NO_ANSWER = 'Could not get a valid response from the server.'

/** The registry's words for a type write that got no answer. */
const UNREACHED = 'The registry could not be reached.'

/** A time limit far shorter than any test. */
const SHORT_LIMIT = 20

/**
 * Marks nothing as a run or a reload starts, and returns what ends that mark, which does nothing either.
 * @returns The end of the mark.
 */
function markNothing() {
	return () => {}
}

/** A list that does nothing around a run. */
const QUIET_LIST: ListRun = { onStart: markNothing, onRefresh: markNothing }

/** The posts a bulk delete picked. */
const PICKED_POSTS = ['019fb000-0000-7000-8000-000000000011', '019fb000-0000-7000-8000-000000000012']

/**
 * Renders the tree under the short time limit, with a toaster and the query client the writes run on.
 * @param ui - The tree to render.
 * @param client - The query client the writes run on.
 */
function renderLimited(ui: ReactNode, client = new QueryClient()) {
	renderAdmin(
		<QueryClientProvider client={client}>
			<Toaster>{ui}</Toaster>
		</QueryClientProvider>,
		{ writeTimeout: SHORT_LIMIT },
	)
}

/**
 * Records every request the route is asked for, answering none.
 * @param method - The method of the requests.
 * @param path - The route of the requests.
 * @returns The requests, filled as they arrive.
 */
function unanswered(method: 'post' | 'patch' | 'delete', path: string): Request[] {
	const sent: Request[] = []
	server.use(
		http[method](path, ({ request }) => {
			sent.push(request)
			return new Promise<never>(() => {})
		}),
	)
	return sent
}

/**
 * Returns whether each recorded request was stopped.
 * @param sent - The requests.
 * @returns One flag per request, in the order they arrived.
 */
function stopped(sent: Request[]): boolean[] {
	return sent.map((request) => request.signal.aborted)
}

/**
 * Waits for the dialog to say no answer came, then returns whether each recorded request was stopped.
 * @param sent - The requests.
 * @returns One flag per request, in the order they arrived.
 */
async function stoppedOnceTimedOut(sent: Request[]): Promise<boolean[]> {
	expect(await screen.findByText(NO_ANSWER)).toBeVisible()
	return stopped(sent)
}

/**
 * Renders the Make default confirm of a type with the types list's own run.
 * @param props - The type taking the root.
 * @returns The confirm.
 */
function MakeDefault({ type }: { type: ContentType }) {
	const tools = useTypeTools(QUIET_LIST)
	return <MakeDefaultConfirm type={type} run={tools.run} onClose={() => {}} />
}

/**
 * Renders the delete confirm of the picked types with the types list's own tools.
 * @param props - The picked types and the list the run marks.
 * @returns The confirm.
 */
function DeleteTypes({ items, list }: { items: ContentType[], list: ListRun }) {
	const tools = useTypeTools(list)
	const opened = { action: 'delete', items, trigger: 'bulk' } as const
	return <OpenTypeDialog opened={opened} tools={tools} onClose={() => {}} onBack={() => {}} />
}

/**
 * Renders the permanent delete confirm of the picked posts with the posts list's own tools.
 * @param props - The picked posts.
 * @returns The confirm.
 */
function DeletePosts({ items }: { items: Post[] }) {
	const tools = usePostTools(QUIET_LIST, () => {})
	const opened = { action: 'delete', items, trigger: 'bulk' } as const
	return <OpenPostDialog opened={opened} tools={tools} onClose={() => {}} onBack={() => {}} />
}

/**
 * Renders the permanent delete confirm of the picked files with the library's own tools.
 * @param props - The picked files.
 * @returns The confirm.
 */
function DeleteMedia({ items }: { items: MediaItem[] }) {
	const tools = useMediaTools(QUIET_LIST)
	const opened = { action: 'delete', items, trigger: 'bulk' } as const
	return <OpenMediaDialog opened={opened} tools={tools} onClose={() => {}} onBack={() => {}} />
}

/**
 * Reads the picked posts as the list holds them and returns their permanent delete.
 * @returns The confirm on the picked posts.
 */
async function postsDelete(): Promise<ReactNode> {
	server.use(http.get('/api/content/:id', ({ params }) => HttpResponse.json(storedPostWithId(String(params.id)))))
	return <DeletePosts items={await Promise.all(PICKED_POSTS.map((id) => fetchPost(id)))} />
}

/**
 * Reads the picked files as the library holds them and returns their permanent delete.
 * @returns The confirm on the picked files.
 */
async function mediaDelete(): Promise<ReactNode> {
	server.use(http.get('/api/media', () => HttpResponse.json({ items: [HARBOR, CLIFF], total: 2 })))
	return <DeleteMedia items={await listMediaByIDs([HARBOR.id, CLIFF.id])} />
}

/** Each permanent delete of picked items, by what it deletes, its route and its confirm. */
const PERMANENT_DELETES: [string, string, () => Promise<ReactNode>][] = [
	['posts', '/api/content/:id', postsDelete],
	['files', '/api/media/:id', mediaDelete],
]

test('says no answer came once a new type runs past the time limit, frees Cancel and stops the request', async () => {
	const sent = unanswered('post', '/api/types')
	const onClose = vi.fn()
	renderLimited(<AddTypeDialog onClose={onClose} list={QUIET_LIST} />)
	const user = userEvent.setup()
	await user.type(screen.getByRole('textbox', { name: 'Singular name' }), 'Recipe')
	await user.type(screen.getByRole('textbox', { name: 'Plural name' }), 'Recipes')

	await user.click(screen.getByRole('button', { name: 'Add' }))

	expect(await stoppedOnceTimedOut(sent)).toEqual([true])
	await user.click(screen.getByRole('button', { name: 'Cancel' }))
	await waitFor(() => expect(onClose).toHaveBeenCalledOnce())
})

test('stops handing the root over once Make default runs past the limit, closing on the failure toast', async () => {
	serveTypes(PAGE_TYPE)
	const [page] = await listTypes()
	const sent = unanswered('patch', '/api/types/:key')
	renderLimited(<MakeDefault type={page} />)
	const confirm = screen.getByRole('alertdialog')

	await userEvent.click(screen.getByRole('button', { name: 'Make default' }))

	expect(await screen.findByText(UNREACHED)).toBeVisible()
	await waitFor(() => expect(confirm).not.toBeInTheDocument())
	expect(stopped(sent)).toEqual([true])
})

test('closes a bulk delete of types on the failure toast at the limit, never sending a queued type', async () => {
	serveTypes(PAGE_TYPE, GUIDE_TYPE)
	const picked = await listTypes()
	const client = new QueryClient()
	const earlier = gate()
	server.use(
		http.patch('/api/types/guide', async () => {
			await earlier.held
			return HttpResponse.json(GUIDE_TYPE)
		}),
	)
	const switched = editInTurn(client, 'guide', { active: false })
	const sent: string[] = []
	server.use(
		http.delete('/api/types/:key', ({ request }) => {
			sent.push(new URL(request.url).pathname)
			return new HttpResponse(null, { status: 204 })
		}),
	)
	const endReload = vi.fn()
	renderLimited(<DeleteTypes items={picked} list={{ onStart: markNothing, onRefresh: () => endReload }} />, client)

	await userEvent.click(screen.getByRole('button', { name: 'Delete' }))

	expect(await screen.findByText('1 content type could not be permanently deleted.')).toBeVisible()
	expect(endReload).toHaveBeenCalledOnce()
	expect(screen.queryByText(NO_ANSWER)).not.toBeInTheDocument()
	earlier.release()
	await switched
	expect(sent).toEqual(['/api/types/page'])
})

test.each(PERMANENT_DELETES)('stops every permanent delete of picked %s at the time limit', async (_, path, ask) => {
	const sent = unanswered('delete', path)
	renderLimited(await ask())

	await userEvent.click(screen.getByRole('button', { name: 'Delete permanently' }))

	await waitFor(() => expect(stopped(sent)).toEqual([true, true]))
	expect(await screen.findByText('2 items could not be permanently deleted.')).toBeVisible()
})
