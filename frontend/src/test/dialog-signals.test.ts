// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { QueryClient } from '@tanstack/react-query'
import { expect, test } from 'vitest'

import { createPost, deletePost, fetchPost, renamePost } from '../content/api'
import {
	createFieldInGroup,
	createGroup,
	createSubField,
	deleteFieldInGroup,
	deleteGroup,
	deleteSubField,
	moveField,
	renameFieldInGroup,
	setFieldSettingsInGroup,
} from '../content/groups'
import type { Location } from '../content/groups'
import { createType, deleteType, updateType } from '../content/types'
import type { NewField, NewType } from '../content/types'
import { deleteInTurn, editInTurn } from '../content/typeWrites'
import { deleteMedia, editMedia } from '../media/api'

/** A field a test declares. */
const NEW_FIELD: NewField = { key: 'subtitle', label: 'Subtitle', kind: 'text' }

/** A type a test registers. */
const NEW_TYPE: NewType = {
	key: 'recipe',
	singularLabel: 'Recipe',
	pluralLabel: 'Recipes',
	description: '',
	routeWord: 'recipes',
}

/** The rules placing a group on posts. */
const ON_POSTS: Location = [[{ source: 'content_type', operator: '==', value: 'post' }]]

/** A field a test edits, as the editor read it. */
const READ_FIELD = { group: 3, key: 'subtitle', updatedAt: '2026-11-16T10:00:00Z' }

/**
 * Returns the signal a dialog fired once its write ran past the time limit.
 * @returns The fired signal.
 */
function firedSignal(): AbortSignal {
	return AbortSignal.abort(new DOMException('Could not get a valid response from the server.', 'TimeoutError'))
}

/** Each write a dialog waits on, sent with a signal that already fired. */
const writes: [string, (signal: AbortSignal) => Promise<unknown>][] = [
	['createPost', (signal) => createPost('post', { title: 'Draft' }, 'Not created.', signal)],
	['fetchPost', (signal) => fetchPost('01a1e5a4', 'Not read.', signal)],
	['renamePost', (signal) => renamePost('01a1e5a4', 'Renamed', signal)],
	['deletePost', (signal) => deletePost('01a1e5a4', signal)],
	['createType', (signal) => createType(NEW_TYPE, signal)],
	['updateType', (signal) => updateType('recipe', { description: 'Dishes' }, signal)],
	['deleteType', (signal) => deleteType('recipe', signal)],
	['editInTurn', (signal) => editInTurn(new QueryClient(), 'recipe', { description: 'Dishes' }, signal)],
	['deleteInTurn', (signal) => deleteInTurn(new QueryClient(), 'recipe', signal)],
	['editMedia', (signal) => editMedia(7, { title: 'Harbour' }, 'Not saved.', signal)],
	['deleteMedia', (signal) => deleteMedia(7, signal)],
	['createGroup', (signal) => createGroup('Details', ON_POSTS, signal)],
	['deleteGroup', (signal) => deleteGroup(3, signal)],
	['renameFieldInGroup', (signal) => renameFieldInGroup(READ_FIELD, 'Strapline', signal)],
	['setFieldSettingsInGroup', (signal) => setFieldSettingsInGroup(READ_FIELD, {}, signal)],
	['createFieldInGroup', (signal) => createFieldInGroup(3, NEW_FIELD, signal)],
	['createSubField', (signal) => createSubField(3, 'team', NEW_FIELD, signal)],
	['deleteFieldInGroup', (signal) => deleteFieldInGroup(3, 'subtitle', signal)],
	['deleteSubField', (signal) => deleteSubField(3, 'team.subtitle', signal)],
	['moveField', (signal) => moveField(3, 'subtitle', { group: 4 }, signal)],
]

test.each(writes)('%s sends nothing once the dialog signal fired', async (_name, write) => {
	const sent: string[] = []
	server.use(
		http.all('*', ({ request }) => {
			sent.push(`${request.method} ${new URL(request.url).pathname}`)
			return HttpResponse.json({ error: 'not served' }, { status: 500 })
		}),
	)

	await expect(write(firedSignal())).rejects.toBeDefined()

	expect(sent).toEqual([])
})
