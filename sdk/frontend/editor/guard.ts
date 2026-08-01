// SPDX-License-Identifier: Apache-2.0

import apiFetch from '@wordpress/api-fetch'

const attempts: string[] = []

/**
 * Refuses every api-fetch call and records the path it asked for.
 */
export function installApiFetchGuard(): void {
	apiFetch.setFetchHandler((options: { path?: string, url?: string }) => {
		const path = options.path ?? options.url ?? '(no path)'
		attempts.push(path)
		return Promise.reject(new Error(`editor: refused an api-fetch call to ${path}`))
	})
}

/**
 * Returns the api-fetch paths refused so far.
 * @returns One entry per refused call.
 */
export function apiFetchAttempts(): string[] {
	return [...attempts]
}

/**
 * Forgets every api-fetch path refused so far.
 */
export function clearApiFetchAttempts(): void {
	attempts.length = 0
}
