// SPDX-License-Identifier: Apache-2.0

import { http, server } from '@gophenberg/frontend-sdk/testing'
import { waitFor } from '@testing-library/react'
import { expect } from 'vitest'

/**
 * Holds every later read of a list, so what a write does before its reload ends can be read.
 * @param path - The address the list reads.
 */
export function holdReads(path: string) {
	server.use(http.get(path, () => new Promise(() => {})))
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
