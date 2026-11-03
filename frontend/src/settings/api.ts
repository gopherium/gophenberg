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
	picker_page_size: z.number(),
	toast_milliseconds: z.number(),
	toast_action_milliseconds: z.number(),
	format_locale: z.string(),
	updates_progress_milliseconds: z.number(),
	updates_restart_wait_milliseconds: z.number(),
	show_avatars: z.boolean(),
	default_role: z.string(),
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

/** The settings every list, the Updates screen and Add User follow, as the server's environment names them. */
export type ListSettings = z.infer<typeof listSettingsSchema>

/** A setting the server can speak about on its own. */
export type SettingField = 'content_per_page' | 'jpeg_quality'

/** The setting each server code speaks about, for the codes about one setting. */
const fieldOfCode: Record<string, SettingField> = {
	per_page_invalid: 'content_per_page',
	jpeg_quality_invalid: 'jpeg_quality',
}

/** A failed request, naming the setting its message is about when it is about one. */
export class SettingError extends Error {
	/** The setting the message is about, if any. */
	readonly field: SettingField | undefined

	/**
	 * Holds the message and the setting it is about.
	 * @param message - The words for the reader.
	 * @param field - The setting the words are about, if any.
	 */
	constructor(message: string, field?: SettingField) {
		super(message)
		this.field = field
	}
}

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
async function errorFrom(response: Response): Promise<SettingError> {
	const parsed = errorSchema.safeParse(await response.json().catch(() => null))
	if (!parsed.success) {
		return new SettingError(errorText({ error: '' }))
	}
	return new SettingError(errorText(parsed.data), fieldOfCode[parsed.data.code ?? ''])
}
