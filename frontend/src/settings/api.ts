// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod'

import { errorText } from '../i18n/errors'

const settingsSchema = z.object({
	locale_default: z.string(),
	content_per_page: z.number(),
	jpeg_quality: z.number(),
})

const listSettingsSchema = z.object({
	list_page_sizes: z.array(z.number()),
	list_page_size: z.number(),
	toast_milliseconds: z.number(),
	toast_name_length: z.number(),
	format_locale: z.string(),
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

/** The settings every list follows, as the server's environment names them. */
export type ListSettings = z.infer<typeof listSettingsSchema>

/**
 * Returns the settings the site chose for itself, raising when they cannot be read.
 * @returns The site settings.
 */
export async function fetchSiteSettings(): Promise<SiteSettings> {
	return settingsSchema.parse(await servedSettings())
}

/**
 * Returns the settings every list follows, raising when they cannot be read.
 * @returns The list settings.
 */
export async function fetchListSettings(): Promise<ListSettings> {
	return listSettingsSchema.parse(await servedSettings())
}

/**
 * Returns the settings the server answers with, raising when they cannot be read.
 * @returns The settings as the server sent them.
 */
async function servedSettings(): Promise<unknown> {
	const response = await fetch('/api/settings')
	if (!response.ok) {
		throw await errorFrom(response)
	}
	return response.json()
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
