// SPDX-License-Identifier: Apache-2.0

import { env } from 'node:process'

import { contentApiPath, defaultPostType } from './kit.ts'
import type { Page, PostSummary, Resolved } from './content.ts'

/** The default page size, matching what the content API reports back. */
const defaultPerPage = 20

/** What a caller may narrow a listing by. */
export interface ListQuery {
	type?: string
	page?: number
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
		const search = new URLSearchParams({
			type: query.type ?? defaultPostType,
			page: String(query.page ?? 1),
			per_page: String(query.perPage ?? defaultPerPage),
		})
		const response = await this.read(`/items?${search}`)
		return (await response.json()) as Page<PostSummary>
	}

	/**
	 * Returns what a public address holds, or nothing when it holds nothing.
	 * @param path - The public address to resolve.
	 * @returns What the address holds, or undefined when the instance serves nothing there.
	 */
	async resolve(path: string): Promise<Resolved | undefined> {
		const search = new URLSearchParams({ path })
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
