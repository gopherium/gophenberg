// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod'

import { errorText } from './errors'
import { DEFAULT_LOCALE } from './catalog'

const localeSchema = z.object({
	locale: z.string(),
	supported: z.array(z.string()),
})

const errorSchema = z.object({
	error: z.string(),
	code: z.string().optional(),
	meta: z.record(z.string(), z.unknown()).optional(),
})

const settingsSchema = z.object({ locale_default: z.string() })

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
		throw await errorFrom(response)
	}
	return localeSchema.parse(await response.json())
}

/**
 * Returns the error a failed request carries.
 * @param response - The answer the server gave.
 * @returns The error to raise.
 */
async function errorFrom(response: Response): Promise<Error> {
	const parsed = errorSchema.safeParse(await response.json().catch(() => null))
	return new Error(parsed.success ? errorText(parsed.data) : errorText({ error: '' }))
}

/**
 * Returns the language the site chose for itself, empty when it chose none or
 * when the setting cannot be read.
 * @returns The site's own language.
 */
export async function fetchSiteLocale(): Promise<string> {
	try {
		const response = await fetch('/api/settings')
		if (!response.ok) {
			return ''
		}
		return settingsSchema.parse(await response.json()).locale_default
	} catch {
		return ''
	}
}

/**
 * Stores the language the site answers in when a reader chose none.
 * @param locale - The language to answer in, or empty to follow the browser.
 * @returns The language the server stored.
 */
export async function chooseSiteLocale(locale: string): Promise<string> {
	const response = await fetch('/api/settings', {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ locale_default: locale }),
	})
	if (!response.ok) {
		throw await errorFrom(response)
	}
	return settingsSchema.parse(await response.json()).locale_default
}
