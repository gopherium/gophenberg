// SPDX-License-Identifier: Apache-2.0

import { GophenbergClient } from './client.ts'

/** What the site answered when the theme first asked. */
export interface SiteProfile {
	gophenberg: string
	api: number
	kit: string[]
}

/** The answer the site gave, kept once it has given one. */
let known: SiteProfile | undefined

/**
 * Returns what the site answered, asking once and remembering only an answer.
 * @param client - The client to ask through.
 * @returns The profile, or nothing when the site did not answer.
 */
export async function siteProfile(client?: GophenbergClient): Promise<SiteProfile | undefined> {
	if (known) {
		return known
	}
	try {
		const answered = await (client ?? new GophenbergClient()).handshake()
		known = { gophenberg: answered.gophenberg, api: answered.api, kit: answered.kit ?? [] }
		return known
	} catch {
		return undefined
	}
}

/** Forgets the site answer, so the next ask reaches the site again. */
export function forgetSite(): void {
	known = undefined
}
