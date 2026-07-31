// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod'

const postSchema = z.object({
	id: z.string(),
	type: z.string(),
	slug: z.string(),
	title: z.string(),
	status: z.string(),
})

export type Post = z.infer<typeof postSchema>

/**
 * Creates a draft of the given type.
 * @param type - The post type to create.
 * @returns The stored draft.
 */
export async function createPost(type = 'post'): Promise<Post> {
	const response = await fetch('/api/posts', {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ type, title: '' }),
	})
	if (!response.ok) {
		throw new Error(`creating a post failed with status ${response.status}`)
	}
	return postSchema.parse(await response.json())
}
