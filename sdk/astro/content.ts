// SPDX-License-Identifier: Apache-2.0

/** A published post as a listing carries it, without its content. */
export interface PostSummary {
	id: string
	type: string
	slug: string
	title: string
	excerpt: string
	published_at: string
	updated_at: string
}

/** A published post carrying the block markup the editor saved. */
export interface Post extends PostSummary {
	content: string
}

/** One page of results with the total behind it. */
export interface Page<T> {
	items: T[]
	total: number
	page: number
	per_page: number
}
