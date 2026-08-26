// SPDX-License-Identifier: Apache-2.0

import { act, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { useDebouncedValue } from '../hooks/useDebouncedValue'

beforeEach(() => vi.useFakeTimers())
afterEach(() => vi.useRealTimers())

/**
 * Renders a probe reporting the debounced value of the one it is given.
 * @returns A function handing the probe its next live value.
 */
function renderProbe(): (next: string) => void {
	function Probe({ live }: { live: string }) {
		const settled = useDebouncedValue(live, 300)
		return <output>{settled}</output>
	}
	const { rerender } = render(<Probe live="" />)
	return (next) => act(() => rerender(<Probe live={next} />))
}

test('reports the value it started with', () => {
	renderProbe()

	expect(screen.getByRole('status')).toHaveTextContent('')
})

test('withholds a change until the delay passes', () => {
	const type = renderProbe()

	type('gut')
	act(() => vi.advanceTimersByTime(200))

	expect(screen.getByRole('status')).toHaveTextContent('')
})

test('reports the change once the delay passes', () => {
	const type = renderProbe()

	type('gutenberg')
	act(() => vi.advanceTimersByTime(300))

	expect(screen.getByRole('status')).toHaveTextContent('gutenberg')
})

test('reports only the last of a burst of changes', () => {
	const type = renderProbe()

	type('g')
	act(() => vi.advanceTimersByTime(100))
	type('gu')
	act(() => vi.advanceTimersByTime(100))
	type('gut')
	act(() => vi.advanceTimersByTime(300))

	expect(screen.getByRole('status')).toHaveTextContent('gut')
})
