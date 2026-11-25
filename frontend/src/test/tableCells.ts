// SPDX-License-Identifier: Apache-2.0

/**
 * Returns the box each cell of a table row holds first, none for a cell whose first element is no cell box.
 * @param row - The table row.
 * @returns The box of each cell, in the order of the cells.
 */
export function cellBoxes(row: HTMLElement): (HTMLElement | null)[] {
	return [...(row as HTMLTableRowElement).cells].map((cell) => {
		const first = cell.firstElementChild
		return first instanceof HTMLElement && first.classList.contains('godmin-table__cell') ? first : null
	})
}
