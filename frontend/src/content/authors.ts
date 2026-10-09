// SPDX-License-Identifier: Apache-2.0

import { z } from 'zod'

const authorsSchema = z.object({ items: z.array(z.object({ id: z.string(), name: z.string() })) })

/**
 * Returns every account that may write, as the choices of the author filter.
 * @returns Each account as the id the filter sends and the name it shows.
 */
export async function authorElements(): Promise<{ value: string, label: string }[]> {
	const response = await fetch('/api/authors')
	if (!response.ok) {
		throw new Error(`listing authors failed with status ${response.status}`)
	}
	return authorsSchema.parse(await response.json()).items.map((author) => ({ value: author.id, label: author.name }))
}
