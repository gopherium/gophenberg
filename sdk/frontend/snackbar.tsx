// SPDX-License-Identifier: Apache-2.0

import { Notice, Stack } from '@wordpress/ui'
import { createContext, useCallback, useContext, useMemo, useState } from 'react'
import type { ReactNode } from 'react'

const DISMISS_AFTER = 10000

export interface SnackbarAction {
	label: string
	onAct: () => void
}

interface Snack {
	id: number
	message: string
	action?: SnackbarAction
}

export interface Snackbar {
	show: (message: string, action?: SnackbarAction) => void
}

const SnackbarContext = createContext<Snackbar | null>(null)

let nextId = 0

/**
 * Returns the handle announcing messages in the snackbar region.
 * @returns The snackbar handle.
 */
export function useSnackbar(): Snackbar {
	const handle = useContext(SnackbarContext)
	if (handle === null) {
		throw new Error('useSnackbar needs a SnackbarProvider above it')
	}
	return handle
}

/**
 * Renders one announced message.
 * @param props - The message and the handler clearing it.
 * @returns The message element.
 */
function Snack({ snack, onClear }: { snack: Snack, onClear: (id: number) => void }) {
	return (
		<Notice.Root intent="neutral" spokenMessage={snack.message}>
			<Notice.Description>{snack.message}</Notice.Description>
			<Notice.Actions>
				{snack.action !== undefined && (
					<Notice.ActionButton
						onClick={() => {
							snack.action?.onAct()
							onClear(snack.id)
						}}
					>
						{snack.action.label}
					</Notice.ActionButton>
				)}
				<Notice.ActionButton onClick={() => onClear(snack.id)}>Dismiss</Notice.ActionButton>
			</Notice.Actions>
		</Notice.Root>
	)
}

/**
 * Renders the region holding announced messages around the given tree.
 * @param props - The tree the region wraps.
 * @returns The wrapped tree with its region.
 */
export function SnackbarProvider({ children }: { children: ReactNode }) {
	const [snacks, setSnacks] = useState<Snack[]>([])
	const clear = useCallback((id: number) => {
		setSnacks((held) => held.filter((snack) => snack.id !== id))
	}, [])
	const show = useCallback(
		(message: string, action?: SnackbarAction) => {
			nextId += 1
			const id = nextId
			setSnacks((held) => [...held, { id, message, action }])
			setTimeout(() => clear(id), DISMISS_AFTER)
		},
		[clear],
	)
	const handle = useMemo(() => ({ show }), [show])
	return (
		<SnackbarContext.Provider value={handle}>
			{children}
			<Stack direction="column" gap="xs" className="gophenberg-snackbars">
				{snacks.map((snack) => (
					<Snack key={snack.id} snack={snack} onClear={clear} />
				))}
			</Stack>
		</SnackbarContext.Provider>
	)
}
