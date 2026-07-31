// SPDX-License-Identifier: Apache-2.0

import { useEffect, useState } from 'react'

/**
 * Returns the given value once it has stopped changing for the delay.
 * @param value - The value to settle.
 * @param delay - The quiet period in milliseconds.
 * @returns The settled value.
 */
export function useDebouncedValue<T>(value: T, delay: number): T {
	const [settled, setSettled] = useState(value)
	useEffect(() => {
		const timer = setTimeout(() => setSettled(value), delay)
		return () => clearTimeout(timer)
	}, [value, delay])
	return settled
}
