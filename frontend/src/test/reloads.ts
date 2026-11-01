// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { waitFor } from '@testing-library/react'
import { expect } from 'vitest'

import { gate } from './gate'

/**
 * Holds every later read of a list, so what a write does before its reload ends can be read.
 * @param path - The address the list reads.
 * @param answer - What the read answers once it is let go, no rows unless a test names others.
 * @returns What lets the read go.
 */
export function holdReads(path: string, answer: Record<string, unknown> = { items: [], total: 0 }): () => void {
	const rows = gate()
	server.use(
		http.get(path, async () => {
			await rows.held
			return HttpResponse.json(answer)
		}),
	)
	return rows.release
}

/**
 * Returns the footer a list marks while it reloads.
 * @returns The footer.
 */
function footer(): Element | null {
	return document.querySelector('.dataviews-footer')
}

/**
 * Waits until the list on show is marked reloading.
 */
export async function waitMarked() {
	await waitFor(() => expect(footer()).toHaveAttribute('inert'))
}

/**
 * Expects the list on show to stand unmarked.
 */
export function expectUnmarked() {
	expect(footer()).not.toHaveAttribute('inert')
}

/**
 * Waits until the list on show stands unmarked.
 */
export async function waitUnmarked() {
	await waitFor(expectUnmarked)
}
