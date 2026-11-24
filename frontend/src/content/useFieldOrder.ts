// SPDX-License-Identifier: Apache-2.0

import { useLayoutEffect, useRef, useState } from 'react'

import { RejectedWriteError } from './groups'
import type { ContentField } from './types'

/** What stores an order of field keys and what reports how the last store went. */
interface OrderStore {
	store: (keys: string[]) => Promise<unknown>
	onStored: () => Promise<void>
	/** Reports a store that failed, and whether the server answered it or it never arrived. */
	onFailed: (cause: unknown, answered: boolean) => Promise<void>
}

/** The fields in the order to show and the move that changes it. */
interface FieldOrder {
	rows: readonly ContentField[]
	move: (at: number, offset: number) => void
}

/**
 * Returns the fields in the order to show, with the move that shows a new order at once and stores one at a time.
 * @param fields - The fields in the order the server answered.
 * @param order - What stores an order and what reports the answer to the last one.
 * @returns The rows and the move.
 */
export function useFieldOrder(fields: readonly ContentField[], order: OrderStore): FieldOrder {
	const [local, setLocal] = useState<readonly string[] | null>(null)
	const [storing, setStoring] = useState(false)
	const inFlight = useRef(false)
	const [seen, setSeen] = useState(fields)
	if (seen !== fields) {
		setSeen(fields)
		if (!storing) {
			setLocal(null)
		}
	}
	const rows = arranged(fields, local)
	const shown = useRef<readonly string[]>([])
	useLayoutEffect(() => {
		shown.current = rows.map((row) => row.key)
	})

	/** Marks the last store as answered. */
	function settle() {
		inFlight.current = false
		setStoring(false)
	}

	/**
	 * Stores an order, then the order shown once it answers when the rows moved again meanwhile.
	 * @param keys - The field keys in the order to store.
	 * @param before - The order shown before this store, put back when it never arrives, null for the server's order.
	 */
	function send(keys: readonly string[], before: readonly string[] | null) {
		inFlight.current = true
		setStoring(true)
		void order.store([...keys]).then(
			() => {
				if (!sameOrder(shown.current, keys)) {
					send(shown.current, keys)
					return
				}
				settle()
				void order.onStored()
			},
			(cause: unknown) => {
				const answered = cause instanceof RejectedWriteError
				settle()
				setLocal(answered ? null : before)
				void order.onFailed(cause, answered)
			},
		)
	}

	/**
	 * Shows the rows with one field moved by an offset, and stores that order unless a store is on its way.
	 * @param at - The field's place in the order shown.
	 * @param offset - How far the field moves, negative for up.
	 */
	function move(at: number, offset: number) {
		const next = moved(rows, at, offset).map((row) => row.key)
		setLocal(next)
		if (!inFlight.current) {
			send(next, local)
		}
	}

	return { rows, move }
}

/**
 * Returns the fields in the order the reader set, keys no longer held left out and new fields last.
 * @param fields - The fields in the order the server answered.
 * @param local - The keys in the order the reader set, or null to follow the server.
 * @returns The rows to show.
 */
function arranged(fields: readonly ContentField[], local: readonly string[] | null): readonly ContentField[] {
	if (local === null) {
		return fields
	}
	const placed = (field: ContentField) => local.indexOf(field.key)
	const kept = fields.filter((field) => placed(field) >= 0).sort((one, other) => placed(one) - placed(other))
	return [...kept, ...fields.filter((field) => placed(field) < 0)]
}

/**
 * Reports whether two orders hold the same keys in the same places.
 * @param one - The first order.
 * @param other - The second order.
 * @returns Whether they match.
 */
function sameOrder(one: readonly string[], other: readonly string[]): boolean {
	return one.length === other.length && one.every((key, at) => other[at] === key)
}

/**
 * Returns a copy of the rows with one row moved by an offset.
 * @param rows - The rows in the order shown.
 * @param at - The place of the row to move.
 * @param offset - How many places the row moves, negative for up.
 * @returns The rows in their new order.
 */
function moved<T>(rows: readonly T[], at: number, offset: number): T[] {
	const next = [...rows]
	const [row] = next.splice(at, 1)
	next.splice(at + offset, 0, row)
	return next
}
