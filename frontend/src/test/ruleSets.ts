// SPDX-License-Identifier: Apache-2.0

import { buttonClasses } from '@gopherium/godmin/testing'
import { within } from '@testing-library/react'
import { expect } from 'vitest'

/**
 * Checks an add button keeps its own width, standing alone in a bare block.
 * @param add - The add button.
 */
function expectOwnWidth(add: HTMLElement) {
	const block = add.parentElement as HTMLElement
	expect(block.tagName).toBe('DIV')
	expect(block.attributes).toHaveLength(0)
	expect(block.children).toHaveLength(1)
}

/**
 * Checks a rule row holds its inputs and its trash in a rows line, the trash its only button.
 * @param rule - The group of the rule row.
 * @param name - The name the trash carries.
 */
function expectTrash(rule: HTMLElement, name: string) {
	const trash = within(rule).getByRole('button', { name })
	expect(within(rule).getAllByRole('button')).toEqual([trash])
	expect(trash).toHaveClass('godmin-rows__remove')
	expect(trash.textContent).toBe('')
	expect(trash.querySelector('svg')).not.toBeNull()
	expect(trash.parentElement?.firstElementChild).toBe(trash)
	const line = trash.closest('.godmin-rows__line') as HTMLElement
	expect(line.parentElement).toBe(rule)
	expect(rule).toHaveClass('godmin-rows__row')
	const source = within(rule).getByRole('combobox', { name: 'Source' })
	expect(source.closest('.godmin-rows__inputs')?.parentElement).toBe(line)
	expect(source.closest('.godmin-form__row')).not.toBeNull()
}

/**
 * Checks the shape every rule sets editor shares, each set headed by its name and each rule named by its place in it.
 * @param holder - The element holding the rule sets.
 * @param counts - How many rules each set holds, in order.
 */
export function expectRuleSets(holder: HTMLElement, counts: number[]) {
	expect(within(holder).getAllByRole('group', { name: /^Rule set \d+$/ })).toHaveLength(counts.length)
	counts.forEach((count, at) => {
		const name = `Rule set ${at + 1}`
		const set = within(holder).getByRole('group', { name })
		expect(within(set).getByText(name)).toBeInTheDocument()
		expect(set).not.toHaveStyle({ alignItems: 'start' })
		expect(set.parentElement).not.toHaveStyle({ alignItems: 'start' })
		expect(within(set).getAllByRole('group', { name: /^Rule \d+ of set \d+$/ })).toHaveLength(count)
		for (let spot = 1; spot <= count; spot++) {
			const rule = within(set).getByRole('group', { name: `Rule ${spot} of set ${at + 1}` })
			expect(within(rule).getByRole('combobox', { name: 'Source' })).toBeInTheDocument()
			expect(within(rule).getByRole('combobox', { name: 'Condition' })).toBeInTheDocument()
			expectTrash(rule, `Remove rule ${spot} of set ${at + 1}`)
		}
		const add = within(set).getByRole('button', { name: 'Add condition' })
		expect([...add.classList]).toEqual(buttonClasses('outline', 'compact'))
		expectOwnWidth(add)
	})
	const more = within(holder).getByRole('button', { name: 'Add rule set' })
	expect([...more.classList]).toEqual(buttonClasses('outline'))
	expectOwnWidth(more)
}
