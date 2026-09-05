// SPDX-License-Identifier: Apache-2.0

import { env } from 'node:process'

import { contentApiPath } from './kit.ts'
import type { Handshake, Page, PostSummary, Resolved } from './content.ts'

/** What a caller may narrow a listing by. */
export interface ListQuery {
	type?: string
	page?: number
	perPage?: number
	fields?: Record<string, string | number | boolean>
}

/** What a caller may ask an archive for. */
export interface ResolveOptions {
	perPage?: number
}

/** How a client reaches one Gophenberg instance. */
export interface ClientOptions {
	baseUrl?: string
	fetch?: typeof globalThis.fetch
}

/** Reads the public content API of one Gophenberg instance. */
export class GophenbergClient {
	private readonly baseUrl: string | undefined
	private readonly fetch: typeof globalThis.fetch

	/**
	 * Builds a client reading through the given address.
	 * @param options - The address to read through and the fetch to read with.
	 */
	constructor(options: ClientOptions = {}) {
		this.baseUrl = options.baseUrl
		this.fetch = options.fetch ?? globalThis.fetch
	}

	/**
	 * Returns one page of published summaries.
	 * @param query - The type and paging the caller wants.
	 * @returns The page the instance served.
	 */
	async listPosts(query: ListQuery = {}): Promise<Page<PostSummary>> {
		const search = new URLSearchParams({ page: String(query.page ?? 1) })
		if (query.perPage !== undefined) {
			search.set('per_page', String(query.perPage))
		}
		if (query.type !== undefined) {
			search.set('type', query.type)
		}
		for (const [key, value] of Object.entries(query.fields ?? {})) {
			search.set(`field[${key}]`, String(value))
		}
		const response = await this.read(`/items?${search}`)
		return (await response.json()) as Page<PostSummary>
	}

	/**
	 * Returns the shape the instance speaks and the types it serves.
	 * @returns The handshake the instance answered.
	 */
	async handshake(): Promise<Handshake> {
		const response = await this.read('')
		return (await response.json()) as Handshake
	}

	/**
	 * Returns what a public address holds, or nothing when it holds nothing.
	 * @param path - The public address to resolve.
	 * @param options - The page size an archive answers with.
	 * @returns What the address holds, or undefined when the instance serves nothing there.
	 */
	async resolve(path: string, options: ResolveOptions = {}): Promise<Resolved | undefined> {
		const search = new URLSearchParams({ path })
		if (options.perPage !== undefined) {
			search.set('per_page', String(options.perPage))
		}
		const response = await this.read(`/resolve?${search}`, [404])
		if (response.status === 404) {
			return undefined
		}
		return (await response.json()) as Resolved
	}

	/**
	 * Returns the response the instance served, refusing a status the caller does not expect.
	 * @param address - The address under the content API.
	 * @param allowed - The non-success statuses the caller handles itself.
	 * @returns The response.
	 */
	private async read(address: string, allowed: number[] = []): Promise<Response> {
		const response = await this.fetch(`${this.origin()}${contentApiPath}${address}`)
		if (!response.ok && !allowed.includes(response.status)) {
			throw new Error(`gophenberg: reading ${address} answered ${response.status}`)
		}
		return response
	}

	/**
	 * Returns the address to read through, resolved when the request is made.
	 * @returns The origin, without a trailing slash.
	 */
	private origin(): string {
		const configured = this.baseUrl ?? env.GOPHENBERG_API_URL
		if (!configured) {
			throw new Error('gophenberg: no baseUrl was given and GOPHENBERG_API_URL is unset')
		}
		return configured.replace(/\/+$/, '')
	}
}
