// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'

import { storedPost } from './postFixture'

/** An autosave the editor sent, with whether it asked to outlive the page. */
export interface Park {
	body: Record<string, unknown>
	keepalive: boolean
}

/**
 * Answers the autosaves of a post and returns the ones this test sent.
 * @param answer - What the server answers each autosave with, over a kept copy of the stored post.
 * @param id - The post whose autosaves are answered.
 * @returns The autosaves in the order they arrived, held by this test alone.
 */
export function collectParks(answer: Record<string, unknown> = {}, id: string = storedPost.id): Park[] {
	const parks: Park[] = []
	server.use(
		http.post(`/api/content/${id}/autosave`, async ({ request }) => {
			parks.push({ body: (await request.json()) as Record<string, unknown>, keepalive: request.keepalive })
			return HttpResponse.json({
				target: 'autosave',
				content_id: id,
				title: storedPost.title,
				content: storedPost.content,
				excerpt: '',
				saved_at: '2026-08-01T12:00:00Z',
				...answer,
			})
		}),
	)
	return parks
}
