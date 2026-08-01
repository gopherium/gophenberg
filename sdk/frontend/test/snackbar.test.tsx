// SPDX-License-Identifier: Apache-2.0

import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { SnackbarProvider, useSnackbar } from '../snackbar'

beforeEach(() => {
	vi.useFakeTimers({ shouldAdvanceTime: true })
})

afterEach(() => {
	vi.useRealTimers()
})

/**
 * Renders a button that announces the given message when pressed.
 * @param props - The message to announce and its optional action.
 * @returns The announcing button.
 */
function Announcer({ message, action }: { message: string, action?: { label: string, onAct: () => void } }) {
	const snackbar = useSnackbar()
	return (
		<button type="button" onClick={() => snackbar.show(message, action)}>
			announce
		</button>
	)
}

/**
 * Renders the given announcer inside a snackbar region.
 * @param ui - The announcer to render.
 */
function renderWithSnackbars(ui: React.ReactNode) {
	render(<SnackbarProvider>{ui}</SnackbarProvider>)
}

test('refuses to announce without a region above it', () => {
	vi.spyOn(console, 'error').mockImplementation(() => {})

	expect(() => render(<Announcer message="Nowhere to go." />)).toThrow(/SnackbarProvider/)
})

test('shows nothing until something is announced', () => {
	renderWithSnackbars(<Announcer message="Post published." />)

	expect(screen.queryByText('Post published.')).not.toBeInTheDocument()
})

test('shows an announced message', async () => {
	renderWithSnackbars(<Announcer message="Post published." />)

	await userEvent.click(screen.getByRole('button', { name: 'announce' }))

	expect(await screen.findByText('Post published.')).toBeInTheDocument()
})

test('clears the message once its time is up', async () => {
	renderWithSnackbars(<Announcer message="Post published." />)
	await userEvent.click(screen.getByRole('button', { name: 'announce' }))
	await screen.findByText('Post published.')

	await vi.advanceTimersByTimeAsync(10000)

	await waitFor(() => expect(screen.queryByText('Post published.')).not.toBeInTheDocument())
})

test('holds the message right up to the moment its time is up', async () => {
	renderWithSnackbars(<Announcer message="Post published." />)
	await userEvent.click(screen.getByRole('button', { name: 'announce' }))
	await screen.findByText('Post published.')

	await act(async () => {
		await vi.advanceTimersByTimeAsync(9000)
	})

	expect(screen.getByText('Post published.')).toBeInTheDocument()
	await act(async () => {
		await vi.advanceTimersByTimeAsync(2000)
	})
	expect(screen.queryByText('Post published.')).not.toBeInTheDocument()
})

test('offers an action and runs it', async () => {
	const acted = vi.fn()
	renderWithSnackbars(
		<Announcer message="Moved to the trash." action={{ label: 'Undo', onAct: acted }} />,
	)
	await userEvent.click(screen.getByRole('button', { name: 'announce' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Undo' }))

	expect(acted).toHaveBeenCalledOnce()
})

test('clears the message once its action runs', async () => {
	renderWithSnackbars(
		<Announcer message="Moved to the trash." action={{ label: 'Undo', onAct: () => {} }} />,
	)
	await userEvent.click(screen.getByRole('button', { name: 'announce' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Undo' }))

	await waitFor(() => expect(screen.queryByText('Moved to the trash.')).not.toBeInTheDocument())
})

test('lets the reader dismiss a message before its time is up', async () => {
	renderWithSnackbars(<Announcer message="Post published." />)
	await userEvent.click(screen.getByRole('button', { name: 'announce' }))

	await userEvent.click(await screen.findByRole('button', { name: 'Dismiss' }))

	await waitFor(() => expect(screen.queryByText('Post published.')).not.toBeInTheDocument())
})

test('stacks messages announced together', async () => {
	function Two() {
		const snackbar = useSnackbar()
		return (
			<button
				type="button"
				onClick={() => {
					snackbar.show('First message.')
					snackbar.show('Second message.')
				}}
			>
				announce
			</button>
		)
	}
	renderWithSnackbars(<Two />)

	await userEvent.click(screen.getByRole('button', { name: 'announce' }))

	expect(await screen.findByText('First message.')).toBeInTheDocument()
	expect(screen.getByText('Second message.')).toBeInTheDocument()
})
