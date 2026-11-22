// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod'

import type { InstalledPlugin } from '../pluginspage'

/** One installed plugin as the server lists it, any key it does not name left out. */
const pluginSchema = z.object({
	id: z.string(),
	name: z.string(),
	description: z.string(),
	version: z.string(),
	status: z.enum(['active', 'inactive', 'paused']),
	paused_step: z.string(),
	paused_reason: z.string(),
})

/** The answer listing every installed plugin. */
const listSchema = z.object({ plugins: z.array(pluginSchema) })

/** The key the installed plugins are cached under. */
export const pluginsQueryKey = ['plugins']

/**
 * Returns every plugin the site holds, with its state and why it paused, raising when they cannot be read.
 * @param signal - The signal that stops the request.
 * @returns The plugins in the order the server lists them, by name then id.
 */
export async function listPlugins(signal?: AbortSignal): Promise<InstalledPlugin[]> {
	const response = await fetch('/api/plugin-admin', { signal })
	if (!response.ok) {
		throw new Error(`reading the plugins failed with status ${response.status}`)
	}
	return listSchema.parse(await response.json()).plugins
}
