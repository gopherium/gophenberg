// SPDX-License-Identifier: Apache-2.0

import { Notice } from '@gophenberg/frontend-sdk'
import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, configure, renderHook, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { beforeAll, beforeEach, expect, onTestFinished, test, vi } from 'vitest'

import { writesSettled } from '../content/postWrites'
import { useAutosave } from '../content/useAutosave'
import { catalogFor } from '../i18n/catalog'
import { listenAt } from './announcements'
import { comeBack } from './anotherPost'
import { classesOf } from './classes'
import { gate } from './gate'
import { watchKeepalive } from './parks'
import { adminUser, renderAt, renderRoutedAt } from './render'
import { storedPost, storedPostWithId } from './postFixture'
import { findActionToast, findToast } from './typeFixtures'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const TRASHED_TOAST = '"Welcome to Gophenberg" moved to the trash.'

const RESTORED_AT = '2026-07-29T09:00:00Z'

const SAVED_AT = '2026-07-28T12:00:00Z'

const EDITED_TRASHED_TOAST = '"Welcome to Gophenberg!" moved to the trash.'

const UNSAVED_TOAST = '"Welcome to Gophenberg!" was not moved to the trash, because its changes could not be saved.'

const PARENT_TRASHED = 'The item you picked as parent is in the trash. Restore it, or pick another parent.'

const PARENT_TRASHED_ANSWER = { error: 'content: parent is in the trash', code: 'parent_trashed' }

const OTHER_ID = '019fb000-0000-7000-8000-000000000002'

const DECLARED_AT = '2026-08-01T10:00:00Z'

const TYPE_WITH_FIELD = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	description: '',
	route_word: '',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	created_at: DECLARED_AT,
	updated_at: DECLARED_AT,
	fields: [
		{ key: 'color', label: 'Color', kind: 'text', many: false, required: false, updated_at: DECLARED_AT },
	],
}

const trashed: string[] = []
const restored: string[] = []

/**
 * Serves the post in the trash until a restore returns it to draft at a newer version.
 * @returns The bodies of the saves made.
 */
function serveRestorable(): { patched: Record<string, unknown>[] } {
	let held: Record<string, unknown> = { ...storedPost, status: 'trash' }
	const patched: Record<string, unknown>[] = []
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(held)),
		http.post(`/api/content/${storedPost.id}/restore`, () => {
			restored.push(storedPost.id)
			held = { ...storedPost, updated_at: RESTORED_AT }
			return HttpResponse.json(held)
		}),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			patched.push(body)
			return HttpResponse.json({ ...held, ...body })
		}),
	)
	return { patched }
}

/**
 * Serves the post in the trash and refuses its restore with the given answer once the gate opens.
 * @param body - The body the refusal carries.
 * @param status - The status the refusal carries.
 * @param held - The gate the refusal waits on.
 */
function refuseRestore(body: Record<string, unknown>, status: number, held: Promise<void> = Promise.resolve()) {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json({ ...storedPost, status: 'trash' })),
		http.post(`/api/content/${storedPost.id}/restore`, async () => {
			await held
			return HttpResponse.json(body, { status })
		}),
	)
}

/** Serves the post in the trash until a restore finds it already out of the trash. */
function serveRestoredElsewhere() {
	let held: Record<string, unknown> = { ...storedPost, status: 'trash' }
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(held)),
		http.post(`/api/content/${storedPost.id}/restore`, () => {
			held = { ...storedPost, updated_at: RESTORED_AT }
			return HttpResponse.json(
				{ error: 'content: only a trashed item can be restored', code: 'restore_not_trashed' },
				{ status: 422 },
			)
		}),
	)
}

/** Presses Restore in the reading view, silencing the error a failed restore logs. */
async function pressRestore() {
	vi.spyOn(console, 'error').mockImplementation(() => {})
	await userEvent.click(await screen.findByRole('button', { name: 'Restore' }))
}

/**
 * Moves the admin to the editor of a post.
 * @param router - The router the admin runs on.
 * @param postId - The post to open.
 */
async function openEditor(router: ReturnType<typeof renderRoutedAt>['router'], postId: string) {
	await act(async () => {
		await router.navigate({ to: '/content/$typeKey/$postId/edit', params: { typeKey: 'post', postId } })
	})
}

beforeAll(async () => {
	await Promise.all([
		import('../content/EditorScreen'),
		import('../content/PostsScreen'),
	])
}, 120000)

beforeEach(() => {
	trashed.length = 0
	restored.length = 0
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(storedPost)),
		http.get('/api/content', () => HttpResponse.json({ items: [], total: 0 })),
		http.delete(`/api/content/${storedPost.id}`, ({ params }) => {
			trashed.push(String(params.id ?? storedPost.id))
			return HttpResponse.json({ ...storedPost, status: 'trash' })
		}),
		http.post(`/api/content/${storedPost.id}/restore`, () => {
			restored.push(storedPost.id)
			return HttpResponse.json({ ...storedPost, status: 'draft' })
		}),
	)
})

test('offers to trash the post from the document tab', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('button', { name: 'Move to trash' })).toBeInTheDocument()
})

test('draws Move to trash as the red secondary button of the default size, as WordPress draws it', async () => {
	renderAt(EDITOR_PATH)

	const trigger = await screen.findByRole('button', { name: 'Move to trash' })

	expect(trigger).toHaveClass(
		'components-button',
		'is-secondary',
		'is-destructive',
		'is-next-40px-default-size',
		'gophenberg-editor__trash',
	)
	expect(trigger).not.toHaveClass('is-busy')
	expect(trigger).not.toHaveAttribute('aria-disabled', 'true')
})

test('places Move to trash 16 pixels under the document panels, as the WordPress sidebar does', async () => {
	renderAt(EDITOR_PATH)

	const trigger = await screen.findByRole('button', { name: 'Move to trash' })

	const row = trigger.parentElement as HTMLElement
	expect(row.style.flexDirection).toBe('column')
	expect(row.style.gap).toMatch(/^var\(--wpds-dimension-gap-lg[,)]/)
	expect(row.lastElementChild).toBe(trigger)
	expect(trigger.previousElementSibling).toContainElement(screen.getByRole('textbox', { name: 'Slug' }))
})

test('places Move to trash above the fields the type declares, as the WordPress sidebar does', async () => {
	server.use(http.get('/api/types', () => HttpResponse.json({ items: [TYPE_WITH_FIELD] })))
	renderAt(EDITOR_PATH)

	const field = await screen.findByLabelText('Color')
	const trigger = screen.getByRole('button', { name: 'Move to trash' })

	expect(trigger.compareDocumentPosition(field) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
	expect(trigger.previousElementSibling).toContainElement(screen.getByRole('textbox', { name: 'Excerpt' }))
	expect(trigger.nextElementSibling).toContainElement(field)
})

/**
 * Holds the answer to the trash of the post until the returned gate opens.
 * @returns The gate the answer waits on.
 */
function holdTrash(): ReturnType<typeof gate> {
	const answer = gate()
	server.use(
		http.delete(`/api/content/${storedPost.id}`, async () => {
			await answer.held
			trashed.push(storedPost.id)
			return HttpResponse.json({ ...storedPost, status: 'trash' })
		}),
	)
	return answer
}

test('shows the trash running on Move to trash at once, with no dialog', async () => {
	const answer = holdTrash()
	renderAt(EDITOR_PATH)
	const trigger = await screen.findByRole('button', { name: 'Move to trash' })

	await userEvent.click(trigger)

	expect(trigger).toHaveClass('is-busy')
	expect(trigger).toHaveAttribute('aria-disabled', 'true')
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
	expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
	expect(trashed).toEqual([])
	answer.release()
	expect(await screen.findByRole('heading', { name: 'Posts', level: 2 })).toBeInTheDocument()
})

test('sends one trash when Move to trash is pressed again while the trash runs', async () => {
	const answer = holdTrash()
	renderAt(EDITOR_PATH)
	const trigger = await screen.findByRole('button', { name: 'Move to trash' })
	await userEvent.click(trigger)

	await userEvent.click(trigger)

	answer.release()
	expect(await screen.findByRole('heading', { name: 'Posts', level: 2 })).toBeInTheDocument()
	expect(trashed).toEqual([storedPost.id])
})

test('keeps the focus on Move to trash while the trash runs', async () => {
	const answer = holdTrash()
	renderAt(EDITOR_PATH)
	const trigger = await screen.findByRole('button', { name: 'Move to trash' })

	await userEvent.click(trigger)

	expect(trigger).toHaveFocus()
	answer.release()
	expect(await screen.findByRole('heading', { name: 'Posts', level: 2 })).toBeInTheDocument()
})

test('keeps the reader on a post opened while the trash ran, naming the trashed post in a toast', async () => {
	const answer = holdTrash()
	server.use(
		http.get(`/api/content/${OTHER_ID}`, () =>
			HttpResponse.json({ ...storedPostWithId(OTHER_ID), title: 'A second post' }),
		),
	)
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))
	await openEditor(router, OTHER_ID)
	await waitFor(() => expect(screen.getByRole('textbox', { name: 'Title' })).toHaveValue('A second post'))

	answer.release()

	expect(await findActionToast(TRASHED_TOAST)).toBeInTheDocument()
	expect(trashed).toEqual([storedPost.id])
	expect(screen.getByRole('textbox', { name: 'Title' })).toHaveValue('A second post')
	expect(screen.queryByRole('heading', { name: 'Posts', level: 2 })).not.toBeInTheDocument()
})

test('trashes the post at once', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await waitFor(() => expect(trashed).toHaveLength(1))
})

test('leaves the editor for the list once the post is trashed', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await waitFor(() => expect(screen.queryByTitle('Editor canvas')).not.toBeInTheDocument())
})

test('lands on the list of the type it left', async () => {
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	expect(await screen.findByRole('heading', { name: 'Posts', level: 2 })).toBeInTheDocument()
})

test('names the post it trashed in a toast carried to the list, whose Undo restores it', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))
	await waitFor(() => expect(screen.queryByTitle('Editor canvas')).not.toBeInTheDocument())
	const toast = await findActionToast(TRASHED_TOAST)
	expect(restored).toEqual([])

	await userEvent.click(within(toast).getByRole('button', { name: 'Undo' }))

	expect(await findToast('"Welcome to Gophenberg" has been restored.')).toBeInTheDocument()
	expect(restored).toEqual([storedPost.id])
})

test('puts the focus on the list once the Undo of the editor trash lands, as an Undo of a list run does', async () => {
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))
	const toast = await findActionToast(TRASHED_TOAST)

	await userEvent.click(within(toast).getByRole('button', { name: 'Undo' }))

	await findToast('"Welcome to Gophenberg" has been restored.')
	await waitFor(() => expect(screen.getByRole('region', { name: 'Posts' })).toHaveFocus())
})

test('reports an Undo the server refused in a toast', async () => {
	server.use(http.post(`/api/content/${storedPost.id}/restore`, () => HttpResponse.json({}, { status: 500 })))
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))
	const toast = await findActionToast(TRASHED_TOAST)

	await userEvent.click(within(toast).getByRole('button', { name: 'Undo' }))

	expect(await findToast('The item could not be restored.')).toBeInTheDocument()
	expect(screen.queryByText('"Welcome to Gophenberg" has been restored.')).not.toBeInTheDocument()
})

test('names a post that has no title yet in the toast', async () => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, title: '' }),
		),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	expect(await findActionToast('"(no title)" moved to the trash.')).toBeInTheDocument()
})

test('opens a trashed post as a reading view', async () => {
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, status: 'trash' }),
		),
	)
	renderAt(EDITOR_PATH)

	expect(await screen.findByText(/This item is in the trash/)).toBeInTheDocument()
	expect(screen.queryByRole('textbox', { name: 'Title' })).not.toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Move to trash' })).not.toBeInTheDocument()
	expect(screen.queryByRole('button', { name: /save draft|publish/i })).not.toBeInTheDocument()
})

test('opens a trashed post without speaking its notice, as WordPress leaves a static notice silent', async () => {
	const polite = listenAt('polite')
	const assertive = listenAt('assertive')
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, status: 'trash' }),
		),
	)
	renderAt(EDITOR_PATH)

	await screen.findByText(/This item is in the trash/)

	expect(polite()).toEqual([])
	expect(assertive()).toEqual([])
})

test('restores a trashed post and opens it in the editor', async () => {
	serveRestorable()
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Restore' }))

	expect(await screen.findByRole('textbox', { name: 'Title' })).toHaveValue(storedPost.title)
	expect(restored).toEqual([storedPost.id])
	expect(await screen.findByText('"Welcome to Gophenberg" has been restored.', { selector: '.godmin-toast' }))
		.toBeInTheDocument()
})

test('names an untitled post it restored from the reading view in the toast', async () => {
	let held: Record<string, unknown> = { ...storedPost, title: '', status: 'trash' }
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(held)),
		http.post(`/api/content/${storedPost.id}/restore`, () => {
			held = { ...storedPost, title: '' }
			return HttpResponse.json(held)
		}),
	)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Restore' }))

	expect(await screen.findByText('"(no title)" has been restored.', { selector: '.godmin-toast' })).toBeInTheDocument()
})

test('saves a restored post against the version the restore stamped', async () => {
	const { patched } = serveRestorable()
	renderAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Restore' }))
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')

	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(patched).toHaveLength(1))
	expect(patched[0].updated_at).toBe(RESTORED_AT)
})

test('opens the editor when the restore finds the post already out of the trash', async () => {
	serveRestoredElsewhere()
	renderAt(EDITOR_PATH)

	await pressRestore()

	expect(await screen.findByRole('textbox', { name: 'Title' })).toBeInTheDocument()
})

test('speaks nothing when the restore finds the post already out of the trash', async () => {
	const polite = listenAt('polite')
	const assertive = listenAt('assertive')
	serveRestoredElsewhere()
	const client = renderAt(EDITOR_PATH)

	await pressRestore()

	await screen.findByRole('textbox', { name: 'Title' })
	await waitFor(() => expect(client.isMutating()).toBe(0))
	expect(assertive()).toEqual([])
	expect(polite()).toEqual([])
})

test('reports a restore the reading view could not make in a toast', async () => {
	refuseRestore({}, 500)
	renderAt(EDITOR_PATH)

	await pressRestore()

	expect(await findToast('The item could not be restored.')).toBeInTheDocument()
})

test('reports the reason a refused restore gave in a toast', async () => {
	refuseRestore(PARENT_TRASHED_ANSWER, 422)
	renderAt(EDITOR_PATH)

	await pressRestore()

	expect(await findToast(PARENT_TRASHED)).toBeInTheDocument()
})

test('keeps the trash words and the warning colour in the notice when a restore fails', async () => {
	refuseRestore(PARENT_TRASHED_ANSWER, 422)
	renderAt(EDITOR_PATH)

	await pressRestore()

	await findToast(PARENT_TRASHED)
	const words = screen.getByText(/This item is in the trash/)
	expect(words.parentElement?.className).toBe(classesOf(<Notice.Root intent="warning">Words</Notice.Root>))
})

test('speaks the reason a restore failed once, as the toast speaks it', async () => {
	const polite = listenAt('polite')
	const assertive = listenAt('assertive')
	refuseRestore(PARENT_TRASHED_ANSWER, 422)
	renderAt(EDITOR_PATH)

	await pressRestore()

	await findToast(PARENT_TRASHED)
	expect(polite()).toEqual([PARENT_TRASHED])
	expect(assertive()).toEqual([])
})

test('shows and speaks nothing when the reader left before the restore failed', async () => {
	const polite = listenAt('polite')
	const assertive = listenAt('assertive')
	const answer = gate()
	refuseRestore(PARENT_TRASHED_ANSWER, 422, answer.held)
	const { router, client } = renderRoutedAt(EDITOR_PATH)
	await pressRestore()
	await act(async () => {
		await router.navigate({ to: '/content/$typeKey', params: { typeKey: 'post' } })
	})
	await screen.findByRole('heading', { name: 'Posts', level: 2 })

	answer.release()

	await waitFor(() => expect(client.isMutating()).toBe(0))
	expect(screen.queryByText(PARENT_TRASHED)).not.toBeInTheDocument()
	expect(polite()).toEqual([])
	expect(assertive()).toEqual([])
})

test('keeps a restore the reader left off the next trashed post they open', async () => {
	const polite = listenAt('polite')
	const assertive = listenAt('assertive')
	const answer = gate()
	refuseRestore(PARENT_TRASHED_ANSWER, 422, answer.held)
	server.use(
		http.get(`/api/content/${OTHER_ID}`, () =>
			HttpResponse.json({ ...storedPostWithId(OTHER_ID), title: 'A second post', status: 'trash' }),
		),
	)
	const { router, client } = renderRoutedAt(`/content/post/${OTHER_ID}/edit`)
	await screen.findByRole('heading', { name: 'A second post' })
	await openEditor(router, storedPost.id)
	await screen.findByRole('heading', { name: storedPost.title })
	await pressRestore()
	await openEditor(router, OTHER_ID)

	answer.release()

	await waitFor(() => expect(client.isMutating()).toBe(0))
	expect(screen.getByText(/This item is in the trash/)).toBeInTheDocument()
	expect(screen.queryByText(PARENT_TRASHED)).not.toBeInTheDocument()
	expect(polite()).toEqual([])
	expect(assertive()).toEqual([])
})

test('opens a post trashed from the editor as a reading view', async () => {
	let held: Record<string, unknown> = storedPost
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(held)),
		http.delete(`/api/content/${storedPost.id}`, () => {
			held = { ...storedPost, status: 'trash' }
			return HttpResponse.json(held)
		}),
	)
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))
	await screen.findByRole('heading', { name: 'Posts', level: 2 })

	await openEditor(router, storedPost.id)

	expect(await screen.findByText(/This item is in the trash/)).toBeInTheDocument()
	expect(screen.queryByRole('textbox', { name: 'Title' })).not.toBeInTheDocument()
})

/**
 * Trashes the post in the editor, the server answering with the given refusal.
 * @param body - The body the refusal carries.
 * @param status - The status the refusal carries.
 * @returns The Move to trash button.
 */
async function trashRefused(body: Record<string, unknown>, status: number): Promise<HTMLElement> {
	server.use(http.delete(`/api/content/${storedPost.id}`, () => HttpResponse.json(body, { status })))
	renderAt(EDITOR_PATH)
	const trigger = await screen.findByRole('button', { name: 'Move to trash' })
	await userEvent.click(trigger)
	return trigger
}

test('reports a trash the server refused in a toast, leaving the editor open and Move to trash ready', async () => {
	const trigger = await trashRefused({}, 500)

	expect(await screen.findByText('The item could not be moved to the trash.', { selector: '.godmin-toast' }))
		.toBeInTheDocument()
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
	expect(screen.getByRole('textbox', { name: 'Title' })).toHaveValue(storedPost.title)
	await waitFor(() => expect(trigger).not.toHaveClass('is-busy'))
	expect(trigger).not.toHaveAttribute('aria-disabled', 'true')
})

test('reports the reason a refused trash gave in a toast', async () => {
	await trashRefused({ error: 'content: item holds children', code: 'content_holds_children' }, 422)

	expect(
		await screen.findByText('This item still holds items nested inside it. Move or delete those first.', {
			selector: '.godmin-toast',
		}),
	).toBeInTheDocument()
})

test('trashes again from Move to trash once a refused trash settles', async () => {
	const trigger = await trashRefused({}, 500)
	await screen.findByText('The item could not be moved to the trash.', { selector: '.godmin-toast' })
	await waitFor(() => expect(trigger).not.toHaveClass('is-busy'))
	server.use(
		http.delete(`/api/content/${storedPost.id}`, () => {
			trashed.push(storedPost.id)
			return HttpResponse.json({ ...storedPost, status: 'trash' })
		}),
	)

	await userEvent.click(trigger)

	await waitFor(() => expect(trashed).toEqual([storedPost.id]))
	expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
})

/** A post as the fake server holds it, and what the server took for it in order. */
interface Served {
	post: Record<string, unknown>
	order: string[]
	parks: Record<string, unknown>[]
}

/** How the fake answers one autosave in turn, once its gate opens, failing when asked. */
interface Turn {
	held?: Promise<void>
	fails?: boolean
}

/**
 * Serves the post as the server does, writing an own draft's autosave into it, and records what it took in order.
 * @param start - The post as it starts.
 * @param turns - How each autosave in turn is answered, recording the answer, at once and accepted when left out.
 * @returns The state the fake holds.
 */
function serveTrash(start: Record<string, unknown>, turns: Turn[] = []): Served {
	const served: Served = { post: start, order: [], parks: [] }
	server.use(
		http.get(`/api/content/${storedPost.id}`, () => HttpResponse.json(served.post)),
		http.post(`/api/content/${storedPost.id}/autosave`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			served.order.push('autosave')
			served.parks.push(body)
			const turn = turns[served.parks.length - 1]
			if (turn !== undefined) {
				await turn.held
				served.order.push('answered')
			}
			if (turn?.fails) {
				return HttpResponse.json({ error: 'internal error' }, { status: 500 })
			}
			const words = { title: body.title, content: body.content, excerpt: body.excerpt }
			const post = served.post
			const inPlace =
				post.status === 'draft' && post.author_id === adminUser.id && body.updated_at === post.updated_at
			if (inPlace) {
				served.post = { ...post, ...words, updated_at: SAVED_AT }
			}
			return HttpResponse.json({
				target: inPlace ? 'post' : 'autosave',
				content_id: storedPost.id,
				...words,
				fields: body.fields,
				saved_at: SAVED_AT,
			})
		}),
		http.delete(`/api/content/${storedPost.id}`, () => {
			served.order.push('trash')
			trashed.push(storedPost.id)
			served.post = { ...served.post, status: 'trash' }
			return HttpResponse.json(served.post)
		}),
		http.post(`/api/content/${storedPost.id}/restore`, () => {
			restored.push(storedPost.id)
			served.post = { ...served.post, status: 'draft', updated_at: RESTORED_AT }
			return HttpResponse.json(served.post)
		}),
	)
	return served
}

/**
 * Counts the autosaves of the post the page sends, at the moment each leaves.
 * @returns A reader of how many left so far.
 */
function countAutosavesSent(): () => number {
	const sends = vi.spyOn(globalThis, 'fetch')
	onTestFinished(() => sends.mockRestore())
	return () =>
		sends.mock.calls.filter(
			([url, init]) => url === `/api/content/${storedPost.id}/autosave` && init?.method === 'POST',
		).length
}

test('saves the unsaved words as an autosave before it moves the post to the trash', async () => {
	const served = serveTrash(storedPost)
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')

	await userEvent.click(screen.getByRole('button', { name: 'Move to trash' }))

	await waitFor(() => expect(served.order).toEqual(['autosave', 'trash']))
	expect(served.parks[0]).toMatchObject({ title: 'Welcome to Gophenberg!', updated_at: storedPost.updated_at })
})

test('moves an untouched post to the trash without an autosave', async () => {
	const served = serveTrash(storedPost)
	renderAt(EDITOR_PATH)

	await userEvent.click(await screen.findByRole('button', { name: 'Move to trash' }))

	await findActionToast(TRASHED_TOAST)
	expect(served.order).toEqual(['trash'])
})

test('waits for a save still in flight before it autosaves the words typed after it', async () => {
	const served = serveTrash(storedPost)
	const answer = gate()
	server.use(
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			served.order.push('save')
			await answer.held
			served.order.push('saved')
			served.post = { ...served.post, ...body, updated_at: SAVED_AT }
			return HttpResponse.json(served.post)
		}),
	)
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))
	await userEvent.type(title, '?')
	await userEvent.click(screen.getByRole('button', { name: 'Move to trash' }))
	await waitFor(() => expect(served.order).toEqual(['save']))

	answer.release()

	await waitFor(() => expect(served.order).toEqual(['save', 'saved', 'autosave', 'trash']))
	expect(served.parks[0]).toMatchObject({ title: 'Welcome to Gophenberg!?' })
})

test('keeps the unsaved words of an own draft through the trash and its Undo', async () => {
	const served = serveTrash({ ...storedPost, author_id: adminUser.id })
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await userEvent.click(screen.getByRole('button', { name: 'Move to trash' }))
	const toast = await findActionToast(EDITED_TRASHED_TOAST)
	await userEvent.click(within(toast).getByRole('button', { name: 'Undo' }))
	await findToast('"Welcome to Gophenberg!" has been restored.')

	await openEditor(router, storedPost.id)

	await waitFor(() => expect(screen.getByRole('textbox', { name: 'Title' })).toHaveValue('Welcome to Gophenberg!'))
	expect(served.order).toEqual(['autosave', 'trash'])
})

test.each([
	{ render: 'a plain render', strict: false },
	{ render: 'StrictMode, which mounts the editor twice', strict: true },
])('keeps the words typed during a save of an own draft through the trash and its Undo in $render', async (said) => {
	configure({ reactStrictMode: said.strict })
	onTestFinished(() => configure({ reactStrictMode: false }))
	const served = serveTrash({ ...storedPost, author_id: adminUser.id })
	const answer = gate()
	server.use(
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			await answer.held
			served.post = { ...served.post, ...body, updated_at: SAVED_AT }
			return HttpResponse.json(served.post)
		}),
	)
	const { router } = renderRoutedAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))
	await userEvent.type(title, '?')
	await userEvent.click(screen.getByRole('button', { name: 'Move to trash' }))
	answer.release()
	const toast = await findActionToast('"Welcome to Gophenberg!?" moved to the trash.')
	await userEvent.click(within(toast).getByRole('button', { name: 'Undo' }))
	await findToast('"Welcome to Gophenberg!?" has been restored.')

	await openEditor(router, storedPost.id)

	await waitFor(() => expect(screen.getByRole('textbox', { name: 'Title' })).toHaveValue('Welcome to Gophenberg!?'))
	expect(served.parks).toMatchObject([{ title: 'Welcome to Gophenberg!?', updated_at: SAVED_AT }])
})

test('keeps a post whose autosave failed out of the trash with its words, naming why in a toast', async () => {
	const served = serveTrash(storedPost)
	server.use(
		http.post(`/api/content/${storedPost.id}/autosave`, () => {
			served.order.push('autosave')
			return HttpResponse.json({ error: 'internal error' }, { status: 500 })
		}),
	)
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	const trigger = screen.getByRole('button', { name: 'Move to trash' })

	await userEvent.click(trigger)

	expect(await findToast(UNSAVED_TOAST)).toBeInTheDocument()
	expect(served.order).toEqual(['autosave'])
	expect(screen.getByRole('textbox', { name: 'Title' })).toHaveValue('Welcome to Gophenberg!')
	await waitFor(() => expect(trigger).not.toHaveClass('is-busy'))
	expect(trigger).not.toHaveAttribute('aria-disabled', 'true')
})

test.each([
	{
		locale: 'es-ES',
		title: 'Título',
		trash: 'Mover a la papelera',
		words:
			'«Welcome to Gophenberg!» no se ha trasladado a la papelera, porque sus cambios no se han podido guardar.',
	},
	{
		locale: 'fr-FR',
		title: 'Titre',
		trash: 'Déplacer dans la corbeille',
		words:
			"« Welcome to Gophenberg! » n'a pas été déplacé dans la corbeille, car ses modifications " +
			"n'ont pas pu être enregistrées.",
	},
])('names why a post whose autosave failed stays out of the trash in $locale', async (said) => {
	setLocaleData(await catalogFor(said.locale), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	server.use(
		http.post(`/api/content/${storedPost.id}/autosave`, () =>
			HttpResponse.json({ error: 'internal error' }, { status: 500 }),
		),
	)
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: said.title }), '!')

	await userEvent.click(screen.getByRole('button', { name: said.trash }))

	expect(await findToast(said.words)).toBeInTheDocument()
})

test('sends no second autosave as the editor closes after the trash', async () => {
	const served = serveTrash(storedPost)
	const sent = countAutosavesSent()
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')

	await userEvent.click(screen.getByRole('button', { name: 'Move to trash' }))

	await findActionToast(EDITED_TRASHED_TOAST)
	expect(sent()).toBe(1)
	expect(served.order).toEqual(['autosave', 'trash'])
})

test('moves the post to the trash when the editor closes while Move to trash waits on a save', async () => {
	const served = serveTrash(storedPost)
	const answer = gate()
	server.use(
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			served.order.push('save')
			await answer.held
			served.post = { ...served.post, ...body, updated_at: SAVED_AT }
			return HttpResponse.json(served.post)
		}),
		http.get(`/api/content/${OTHER_ID}`, () =>
			HttpResponse.json({ ...storedPostWithId(OTHER_ID), title: 'A second post' }),
		),
	)
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))
	await waitFor(() => expect(served.order).toEqual(['save']))
	await userEvent.click(screen.getByRole('button', { name: 'Move to trash' }))
	await openEditor(router, OTHER_ID)
	await waitFor(() => expect(screen.getByRole('textbox', { name: 'Title' })).toHaveValue('A second post'))

	answer.release()

	expect(await findActionToast(EDITED_TRASHED_TOAST)).toBeInTheDocument()
	expect(served.order).toEqual(['save', 'autosave', 'trash'])
	expect(served.parks).toMatchObject([{ title: 'Welcome to Gophenberg!' }])
})

test('lets Move to trash go on when the editor closes before the render its park asked for', async () => {
	const client = new QueryClient()
	const clean = {
		title: storedPost.title,
		content: storedPost.content,
		excerpt: storedPost.excerpt,
		fields: {},
		shown: {},
		dirty: false,
		saving: false,
		version: storedPost.updated_at,
		adoptParked: () => {},
	}
	const { result, unmount } = renderHook(() => useAutosave(storedPost.id, clean), {
		wrapper: ({ children }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>,
	})
	let settled = false
	void result.current().then(() => {
		settled = true
	})
	await Promise.resolve()

	unmount()

	await waitFor(() => expect(settled).toBe(true))
})

test('sends no autosave as the editor closes on a trashed post after a later Move to trash ends', async () => {
	const second = gate()
	const served = serveTrash({ ...storedPost, status: 'published' }, [{}, { held: second.held }])
	const client = new QueryClient()
	const edited = {
		title: 'Welcome to Gophenberg!',
		content: storedPost.content,
		excerpt: storedPost.excerpt,
		fields: {},
		shown: {},
		dirty: true,
		saving: false,
		version: storedPost.updated_at,
		adoptParked: () => {},
	}
	const { result, unmount } = renderHook(() => useAutosave(storedPost.id, edited), {
		wrapper: ({ children }) => <QueryClientProvider client={client}>{children}</QueryClientProvider>,
	})
	const trashed = await result.current()
	const again = result.current()
	await waitFor(() => expect(served.parks).toHaveLength(2))
	trashed()
	second.release()
	await again

	unmount()

	await writesSettled(client, storedPost.id)
	expect(served.parks).toHaveLength(2)
})

test('reads the kept words in a reopened editor only once the autosave sent before the trash answers', async () => {
	const served = serveTrash(storedPost)
	const answer = gate()
	server.use(
		http.post(`/api/content/${storedPost.id}/autosave`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			served.order.push('autosave')
			await answer.held
			served.order.push('answered')
			return HttpResponse.json({ target: 'autosave', content_id: storedPost.id, ...body, saved_at: SAVED_AT })
		}),
		http.get(`/api/content/${storedPost.id}/autosave`, () => {
			served.order.push('read')
			return HttpResponse.json({}, { status: 404 })
		}),
	)
	const { router } = renderRoutedAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await userEvent.click(screen.getByRole('button', { name: 'Move to trash' }))
	await waitFor(() => expect(served.order).toEqual(['read', 'autosave']))
	await userEvent.click(screen.getByRole('link', { name: 'Back to posts' }))
	await comeBack(router)
	await screen.findByRole('textbox', { name: 'Title' })

	answer.release()

	expect(await findActionToast(EDITED_TRASHED_TOAST)).toBeInTheDocument()
	await waitFor(() => expect(served.order.filter((entry) => entry === 'read')).toHaveLength(2))
	expect(served.order.lastIndexOf('read')).toBeGreaterThan(served.order.indexOf('answered'))
})

test('parks the words typed after a rejected trash as the editor closes', async () => {
	const served = serveTrash(storedPost)
	server.use(http.delete(`/api/content/${storedPost.id}`, () => HttpResponse.json({}, { status: 500 })))
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')
	await userEvent.click(screen.getByRole('button', { name: 'Move to trash' }))
	await findToast('The item could not be moved to the trash.')
	await userEvent.type(title, '?')

	await userEvent.click(screen.getByRole('link', { name: 'Back to posts' }))

	await waitFor(() => expect(served.parks).toHaveLength(2))
	expect(served.parks[1]).toMatchObject({ title: 'Welcome to Gophenberg!?' })
})

test('saves an own draft whose trash was rejected against the version its autosave wrote', async () => {
	const served = serveTrash({ ...storedPost, author_id: adminUser.id })
	const patches: Record<string, unknown>[] = []
	server.use(
		http.delete(`/api/content/${storedPost.id}`, () => HttpResponse.json({}, { status: 500 })),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			patches.push(body)
			if (body.updated_at !== served.post.updated_at) {
				return HttpResponse.json(
					{ error: 'content: conflicting update', code: 'content_stale_update' },
					{ status: 409 },
				)
			}
			served.post = { ...served.post, ...body }
			return HttpResponse.json(served.post)
		}),
	)
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')
	await userEvent.click(screen.getByRole('button', { name: 'Move to trash' }))
	await findToast('The item could not be moved to the trash.')
	await userEvent.type(title, '?')

	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	expect(await findToast('Draft saved.')).toBeInTheDocument()
	expect(patches).toMatchObject([{ title: 'Welcome to Gophenberg!?', updated_at: SAVED_AT }])
})

test('parks the words parked before a rejected trash again as the editor closes after a later save', async () => {
	const served = serveTrash(storedPost)
	server.use(
		http.delete(`/api/content/${storedPost.id}`, () => HttpResponse.json({}, { status: 500 })),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			const body = (await request.json()) as Record<string, unknown>
			served.post = { ...served.post, ...body, updated_at: SAVED_AT }
			return HttpResponse.json(served.post)
		}),
	)
	renderAt(EDITOR_PATH)
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')
	await userEvent.click(screen.getByRole('button', { name: 'Move to trash' }))
	await findToast('The item could not be moved to the trash.')
	await userEvent.type(title, '?')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))
	expect(await screen.findByText('Saved')).toBeInTheDocument()
	await userEvent.type(title, '{Backspace}')

	await userEvent.click(screen.getByRole('link', { name: 'Back to posts' }))

	await waitFor(() => expect(served.parks).toHaveLength(2))
	expect(served.parks[1]).toMatchObject({ title: 'Welcome to Gophenberg!' })
})

test('sends the words of an autosave that failed before the trash again as the editor closes', async () => {
	const parks: Record<string, unknown>[] = []
	serveTrash(storedPost)
	server.use(
		http.post(`/api/content/${storedPost.id}/autosave`, async ({ request }) => {
			parks.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ error: 'internal error' }, { status: 500 })
		}),
	)
	renderAt(EDITOR_PATH)
	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), '!')
	await userEvent.click(screen.getByRole('button', { name: 'Move to trash' }))
	await findToast(UNSAVED_TOAST)

	await userEvent.click(screen.getByRole('link', { name: 'Back to posts' }))

	await waitFor(() => expect(parks).toHaveLength(2))
	expect(parks[1]).toMatchObject({ title: 'Welcome to Gophenberg!' })
})

/**
 * Types into the title, presses Move to trash and waits until its autosave reaches the server.
 * @param served - The fake serving the post.
 * @returns The title box, still open for typing.
 */
async function startTrash(served: Served): Promise<HTMLElement> {
	const title = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(title, '!')
	await userEvent.click(screen.getByRole('button', { name: 'Move to trash' }))
	await waitFor(() => expect(served.order).toEqual(['autosave']))
	return title
}

test('autosaves the words typed while Move to trash autosaves before it trashes the post', async () => {
	const first = gate()
	const second = gate()
	const served = serveTrash({ ...storedPost, status: 'published' }, [{ held: first.held }, { held: second.held }])
	renderAt(EDITOR_PATH)
	const title = await startTrash(served)
	await userEvent.type(title, '?')
	first.release()
	await waitFor(() => expect(served.order).toEqual(['autosave', 'answered', 'autosave']))

	second.release()

	expect(await findActionToast(EDITED_TRASHED_TOAST)).toBeInTheDocument()
	expect(served.order).toEqual(['autosave', 'answered', 'autosave', 'answered', 'trash'])
	expect(served.parks).toMatchObject([
		{ title: 'Welcome to Gophenberg!', updated_at: storedPost.updated_at },
		{ title: 'Welcome to Gophenberg!?', updated_at: storedPost.updated_at },
	])
})

test('autosaves an own draft again against the version its first autosave answered with', async () => {
	const first = gate()
	const served = serveTrash({ ...storedPost, author_id: adminUser.id }, [{ held: first.held }])
	renderAt(EDITOR_PATH)
	const title = await startTrash(served)
	await userEvent.type(title, '?')

	first.release()

	expect(await findActionToast(EDITED_TRASHED_TOAST)).toBeInTheDocument()
	expect(served.order).toEqual(['autosave', 'answered', 'autosave', 'trash'])
	expect(served.parks[1]).toMatchObject({ title: 'Welcome to Gophenberg!?', updated_at: SAVED_AT })
	expect(served.post).toMatchObject({ title: 'Welcome to Gophenberg!?', status: 'trash' })
})

test('sends one autosave before the trash when the words typed during it end as the words it sent', async () => {
	const first = gate()
	const served = serveTrash({ ...storedPost, status: 'published' }, [{ held: first.held }])
	const sent = countAutosavesSent()
	renderAt(EDITOR_PATH)
	const title = await startTrash(served)
	await userEvent.type(title, '?{Backspace}')

	first.release()

	expect(await findActionToast(EDITED_TRASHED_TOAST)).toBeInTheDocument()
	expect(served.order).toEqual(['autosave', 'answered', 'trash'])
	expect(sent()).toBe(1)
})

test('keeps the post out of the trash when the autosave of the words typed during the first fails', async () => {
	const first = gate()
	const served = serveTrash({ ...storedPost, status: 'published' }, [{ held: first.held }, { fails: true }])
	renderAt(EDITOR_PATH)
	const title = await startTrash(served)
	await userEvent.type(title, '?')

	first.release()

	expect(await findToast(UNSAVED_TOAST)).toBeInTheDocument()
	expect(served.order).toEqual(['autosave', 'answered', 'autosave', 'answered'])
	expect(trashed).toEqual([])
	expect(title).toHaveValue('Welcome to Gophenberg!?')
})

test('autosaves the words typed before the editor closed during the autosave, then trashes the post', async () => {
	const first = gate()
	const served = serveTrash({ ...storedPost, status: 'published' }, [{ held: first.held }])
	server.use(
		http.get(`/api/content/${OTHER_ID}`, () =>
			HttpResponse.json({ ...storedPostWithId(OTHER_ID), title: 'A second post' }),
		),
	)
	const { router } = renderRoutedAt(EDITOR_PATH)
	const title = await startTrash(served)
	await userEvent.type(title, '?')
	await openEditor(router, OTHER_ID)
	await waitFor(() => expect(screen.getByRole('textbox', { name: 'Title' })).toHaveValue('A second post'))

	first.release()

	expect(await findActionToast(EDITED_TRASHED_TOAST)).toBeInTheDocument()
	expect(served.order).toEqual(['autosave', 'answered', 'autosave', 'trash'])
	expect(served.parks[1]).toMatchObject({ title: 'Welcome to Gophenberg!?', updated_at: storedPost.updated_at })
})

test('sends the latest words once when the editor closed before the autosave for the trash failed', async () => {
	const first = gate()
	const served = serveTrash({ ...storedPost, status: 'published' }, [{ held: first.held, fails: true }])
	server.use(
		http.get(`/api/content/${OTHER_ID}`, () =>
			HttpResponse.json({ ...storedPostWithId(OTHER_ID), title: 'A second post' }),
		),
	)
	const sent = countAutosavesSent()
	const { router } = renderRoutedAt(EDITOR_PATH)
	const title = await startTrash(served)
	await userEvent.type(title, '?')
	await openEditor(router, OTHER_ID)
	await waitFor(() => expect(screen.getByRole('textbox', { name: 'Title' })).toHaveValue('A second post'))

	first.release()

	expect(await findToast(UNSAVED_TOAST)).toBeInTheDocument()
	await waitFor(() => expect(served.parks).toHaveLength(2))
	expect(served.parks[1]).toMatchObject({ title: 'Welcome to Gophenberg!?', updated_at: storedPost.updated_at })
	expect(served.order).toEqual(['autosave', 'answered', 'autosave'])
	expect(trashed).toEqual([])
	expect(sent()).toBe(2)
})

test('keeps the editor open again on the post when the autosave for the trash fails after it closed', async () => {
	const first = gate()
	const served = serveTrash({ ...storedPost, status: 'published' }, [{ held: first.held, fails: true }])
	const { router } = renderRoutedAt(EDITOR_PATH)
	const title = await startTrash(served)
	await userEvent.type(title, '?')
	await userEvent.click(screen.getByRole('link', { name: 'Back to posts' }))
	await screen.findByRole('heading', { name: 'Posts', level: 2 })
	await comeBack(router)
	const reopened = await screen.findByRole('textbox', { name: 'Title' })
	await userEvent.type(reopened, 'x')

	first.release()

	expect(await findToast(UNSAVED_TOAST)).toBeInTheDocument()
	await waitFor(() => expect(served.parks).toHaveLength(2))
	await userEvent.type(reopened, 'y')
	expect(screen.getByRole('textbox', { name: 'Title' })).toBe(reopened)
	expect(reopened).toHaveValue('Welcome to Gophenbergxy')
	expect(served.parks).toMatchObject([{ title: 'Welcome to Gophenberg!' }, { title: 'Welcome to Gophenberg!?' }])
})

/** Fires the event a page sends as it unloads. */
async function unloadPage() {
	await act(async () => {
		window.dispatchEvent(new Event('beforeunload'))
		await Promise.resolve()
	})
}

test('sends the latest words as the page unloads once the editor closed during the autosave for a trash', async () => {
	const first = gate()
	const served = serveTrash({ ...storedPost, status: 'published' }, [{ held: first.held }])
	server.use(
		http.get(`/api/content/${OTHER_ID}`, () =>
			HttpResponse.json({ ...storedPostWithId(OTHER_ID), title: 'A second post' }),
		),
	)
	const keepaliveAt = watchKeepalive(storedPost.id)
	const { router } = renderRoutedAt(EDITOR_PATH)
	const title = await startTrash(served)
	await userEvent.type(title, '?')
	await openEditor(router, OTHER_ID)
	await waitFor(() => expect(screen.getByRole('textbox', { name: 'Title' })).toHaveValue('A second post'))

	await unloadPage()

	await waitFor(() => expect(served.parks).toHaveLength(2))
	expect(served.parks[1]).toMatchObject({ title: 'Welcome to Gophenberg!?', updated_at: storedPost.updated_at })
	expect(keepaliveAt(1)).toBe(true)
	first.release()
	expect(await findActionToast(EDITED_TRASHED_TOAST)).toBeInTheDocument()
})

test('sends nothing as the page unloads once the editor closed on the saved words during a trash', async () => {
	const first = gate()
	const served = serveTrash({ ...storedPost, status: 'published' }, [{ held: first.held }])
	server.use(
		http.get(`/api/content/${OTHER_ID}`, () =>
			HttpResponse.json({ ...storedPostWithId(OTHER_ID), title: 'A second post' }),
		),
	)
	const sent = countAutosavesSent()
	const { router } = renderRoutedAt(EDITOR_PATH)
	const title = await startTrash(served)
	await userEvent.type(title, '{Backspace}')
	await openEditor(router, OTHER_ID)
	await waitFor(() => expect(screen.getByRole('textbox', { name: 'Title' })).toHaveValue('A second post'))

	await unloadPage()

	expect(sent()).toBe(1)
	first.release()
	expect(await findActionToast(EDITED_TRASHED_TOAST)).toBeInTheDocument()
	expect(served.order).toEqual(['autosave', 'answered', 'trash'])
})

test('sends the latest words once as the page unloads during the autosave for the trash', async () => {
	const first = gate()
	const served = serveTrash({ ...storedPost, status: 'published' }, [{ held: first.held }])
	const sent = countAutosavesSent()
	renderAt(EDITOR_PATH)
	const title = await startTrash(served)
	await userEvent.type(title, '?')

	await unloadPage()

	expect(sent()).toBe(2)
	await waitFor(() => expect(served.parks).toHaveLength(2))
	expect(served.parks[1]).toMatchObject({ title: 'Welcome to Gophenberg!?' })
	first.release()
	expect(await findActionToast(EDITED_TRASHED_TOAST)).toBeInTheDocument()
})
