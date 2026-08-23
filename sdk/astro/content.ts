// SPDX-License-Identifier: Apache-2.0

/** A published post as a listing carries it, without its content. */
export interface PostSummary {
	id: string
	type: string
	path: string
	slug: string
	title: string
	excerpt: string
	published_at: string
	updated_at: string
}

/** A published post carrying the block markup the editor saved and the values it holds. */
export interface Post extends PostSummary {
	content: string
	fields: Record<string, unknown>
}

/** One item a relation field points at, as a reader sees it. */
export interface RelatedItem {
	id: string
	title: string
	path: string
}

/** One typed field a content type declares. */
export interface ContentTypeField {
	key: string
	label: string
	kind: string
	relates_to?: string
	many: boolean
	required: boolean
}

/** A content type as the handshake advertises it. */
export interface ContentType {
	key: string
	singular_label: string
	plural_label: string
	route_word: string
	hierarchical: boolean
	page_kind: string
	default: boolean
	fields: ContentTypeField[]
}

/** The shape an instance speaks and the types it serves. */
export interface Handshake {
	gophenberg: string
	api: number
	kit?: string[]
	types: ContentType[]
}

/** What a public address holds. */
export interface Resolved {
	kind: 'item' | 'archive' | 'term'
	type: ContentType
	item?: Post
	page?: Page<PostSummary>
}

/** One page of results with the total behind it. */
export interface Page<T> {
	items: T[]
	total: number
	page: number
	per_page: number
}
