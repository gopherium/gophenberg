// SPDX-License-Identifier: Apache-2.0

/**
 * Returns a gate a handler waits on, and the function opening it.
 * @returns The gate and its opener.
 */
export function gate(): { held: Promise<void>, release: () => void } {
	let release = () => {}
	const held = new Promise<void>((resolve) => {
		release = resolve
	})
	return { held, release }
}
