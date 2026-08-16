// SPDX-License-Identifier: AGPL-3.0-or-later

import { z } from 'zod'

import { DEFAULT_LOCALE } from './catalog'

const localeSchema = z.object({
	locale: z.string(),
	supported: z.array(z.string()),
})

const refusalSchema = z.object({ error: z.string() })

/** The language the admin reads in, and the ones it may read in. */
export type Answered = z.infer<typeof localeSchema>

/**
 * Returns the language the server answers in, falling back when it cannot be asked.
 * @returns The resolved language and the ones on offer.
 */
export async function fetchLocale(): Promise<Answered> {
	try {
		const response = await fetch('/api/locale')
		if (!response.ok) {
			return { locale: DEFAULT_LOCALE, supported: [DEFAULT_LOCALE] }
		}
		return localeSchema.parse(await response.json())
	} catch {
		return { locale: DEFAULT_LOCALE, supported: [DEFAULT_LOCALE] }
	}
}

/**
 * Stores the language the reader chose for themselves.
 * @param locale - The language to read in.
 * @returns The language the server stored.
 */
export async function chooseLocale(locale: string): Promise<Answered> {
	const response = await fetch('/api/locale', {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ locale }),
	})
	if (!response.ok) {
		const parsed = refusalSchema.safeParse(await response.json().catch(() => null))
		throw new Error(parsed.success ? parsed.data.error : `the server answered ${response.status}`)
	}
	return localeSchema.parse(await response.json())
}
