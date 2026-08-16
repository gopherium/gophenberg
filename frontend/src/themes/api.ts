// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod'

import { refusalText } from '../i18n/refusal'

const themeSchema = z.object({
	name: z.string(),
	version: z.string().optional(),
	broken: z.string().optional(),
	active: z.boolean(),
	serving: z.boolean(),
})

const listSchema = z.object({
	themes: z.array(themeSchema),
	rollback: z.object({ theme: z.string() }).optional(),
})

const installedSchema = z.object({ name: z.string() })

const activeSchema = z.object({ active: z.string() })

const errorSchema = z.object({
	error: z.string(),
	code: z.string().optional(),
	meta: z.record(z.string(), z.unknown()).optional(),
})

export interface Theme {
	name: string
	version: string
	broken: string
	active: boolean
	serving: boolean
}

export interface ThemeList {
	themes: Theme[]
	rollback: string | null
}

export type ThemeOutcome =
	| { kind: 'done'; name: string }
	| { kind: 'refused'; reason: string }

export const themesQueryKey = ['themes']

/**
 * Returns the theme a listed row describes.
 * @param row - The row the server listed.
 * @returns The theme the screen shows.
 */
function toTheme(row: z.infer<typeof themeSchema>): Theme {
	return {
		name: row.name,
		version: row.version ?? '',
		broken: row.broken ?? '',
		active: row.active,
		serving: row.serving,
	}
}

/**
 * Returns the message an error response carried.
 * @param response - The response the API refused with.
 * @returns The message, or a stand in when the body carries none.
 */
async function messageFrom(response: Response): Promise<string> {
	const parsed = errorSchema.safeParse(await response.json().catch(() => null))
	if (!parsed.success) {
		return refusalText({ error: '' })
	}
	return refusalText(parsed.data)
}

/**
 * Returns the installed themes and what a rollback would return to.
 * @param signal - Optional signal aborting the request.
 * @returns The installed themes, with the rollback target or null when none is offered.
 */
export async function listThemes(signal?: AbortSignal): Promise<ThemeList> {
	const response = await fetch('/api/themes', { signal })
	if (!response.ok) {
		throw new Error(`listing themes failed with status ${response.status}`)
	}
	const listed = listSchema.parse(await response.json())
	return {
		themes: listed.themes.map(toTheme),
		rollback: listed.rollback ? listed.rollback.theme : null,
	}
}

/**
 * Installs a packaged theme archive.
 * @param archive - The archive the administrator chose.
 * @returns The name it installed under, or the reason it was refused.
 */
export async function uploadTheme(archive: File): Promise<ThemeOutcome> {
	const body = new FormData()
	body.append('theme', archive, archive.name)
	const response = await fetch('/api/themes', { method: 'POST', body })
	if (!response.ok) {
		return { kind: 'refused', reason: await messageFrom(response) }
	}
	return { kind: 'done', name: installedSchema.parse(await response.json()).name }
}

/**
 * Serves the public site through the named theme.
 * @param name - The theme to serve through.
 * @returns The theme now serving, or the reason it was refused.
 */
export async function activateTheme(name: string): Promise<ThemeOutcome> {
	const response = await fetch('/api/themes/active', {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ name }),
	})
	return activeOutcome(response)
}

/**
 * Returns the public site to the built-in renderer.
 * @returns The theme now serving, or the reason it was refused.
 */
export async function deactivateTheme(): Promise<ThemeOutcome> {
	return activeOutcome(await fetch('/api/themes/active', { method: 'DELETE' }))
}

/**
 * Returns the public site to the choice before the current one.
 * @returns The theme now serving, or the reason it was refused.
 */
export async function rollbackTheme(): Promise<ThemeOutcome> {
	return activeOutcome(await fetch('/api/themes/rollback', { method: 'POST' }))
}

/**
 * Returns the outcome of a request that changes which theme is serving.
 * @param response - The response the API answered with.
 * @returns The theme now serving, or the reason it was refused.
 */
async function activeOutcome(response: Response): Promise<ThemeOutcome> {
	if (!response.ok) {
		return { kind: 'refused', reason: await messageFrom(response) }
	}
	return { kind: 'done', name: activeSchema.parse(await response.json()).active }
}
