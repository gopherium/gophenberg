// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod'

import { errorText } from '../i18n/errors'

/** The number of items a listing carries when the site chose none. */
const DEFAULT_PER_PAGE = 20

/** The quality a stored picture is written at when the site chose none. */
const DEFAULT_JPEG_QUALITY = 82

const settingsSchema = z.object({
	locale_default: z.string(),
	content_per_page: z.number(),
	jpeg_quality: z.number(),
})

const errorSchema = z.object({
	error: z.string(),
	code: z.string().optional(),
	meta: z.record(z.string(), z.unknown()).optional(),
})

/** The settings the site chose for itself. */
export type SiteSettings = z.infer<typeof settingsSchema>

/** The settings a request asks the site to store. */
export type SettingsAsked = Partial<SiteSettings>

/** What the site answers with while its settings cannot be read. */
const compiledDefaults: SiteSettings = {
	locale_default: '',
	content_per_page: DEFAULT_PER_PAGE,
	jpeg_quality: DEFAULT_JPEG_QUALITY,
}

/**
 * Returns the settings the site chose for itself, falling back to the compiled
 * defaults when they cannot be read.
 * @returns The site settings.
 */
export async function fetchSiteSettings(): Promise<SiteSettings> {
	try {
		const response = await fetch('/api/settings')
		if (!response.ok) {
			return compiledDefaults
		}
		return settingsSchema.parse(await response.json())
	} catch {
		return compiledDefaults
	}
}

/**
 * Stores the settings an administrator chose, sending only the named ones.
 * @param asked - The settings to store.
 * @returns The settings the server stored.
 */
export async function chooseSiteSettings(asked: SettingsAsked): Promise<SiteSettings> {
	const response = await fetch('/api/settings', {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(asked),
	})
	if (!response.ok) {
		throw await errorFrom(response)
	}
	return settingsSchema.parse(await response.json())
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
