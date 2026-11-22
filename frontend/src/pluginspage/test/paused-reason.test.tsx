// SPDX-License-Identifier: Apache-2.0

import { render } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { PausedReason } from '../PausedReason'
import type { PausedRecord } from '../PausedReason'

/** The error a paused record holds in these cases, as a plugin wrote it. */
const ERROR = 'the item count must be a whole number, got "banana"'

/** The sentence the app gives the step a settings rejection pauses a plugin at. */
const SET_UP = 'It failed while the site set it up. Error message: %s'

/** The sentence the app gives a step it does not know, or none. */
const ANY_STEP = 'It failed. Error message: %s'

/**
 * Returns the sentence the app gives each step, as the app's words do.
 * @param step - The step the record names.
 * @returns The sentence for the set up step, the general one for any other step.
 */
function sentenceOf(step: string): string {
	return step === 'register' ? SET_UP : ANY_STEP
}

/**
 * Returns a record paused at a step with an error.
 * @param step - The step the record names.
 * @param reason - The plugin's own error.
 * @returns The record.
 */
function pausedAt(step: string, reason = ERROR): PausedRecord {
	return { status: 'paused', paused_step: step, paused_reason: reason }
}

/**
 * Renders the reason of a record and returns the element it drew.
 * @param record - The record.
 * @param sentence - Returns the sentence for a step.
 * @returns The first element the reason drew, none when it drew nothing.
 */
function drawn(record: PausedRecord, sentence: (step: string) => string = sentenceOf): Element | null {
	return render(<PausedReason plugin={record} sentenceOf={sentence} />).container.firstElementChild
}

test('draws the sentence the app gives the step a plugin paused at, its own error after it', () => {
	expect(drawn(pausedAt('register'))?.textContent).toBe(`It failed while the site set it up. Error message: ${ERROR}`)
})

test.each(['register', 'roles', 'types', 'hooks', 'routes', 'migrate', 'start', 'crash', ''])(
	'asks the app for the sentence of the step %j as the record names it',
	(step) => {
		const sentence = vi.fn(sentenceOf)

		const reason = drawn(pausedAt(step), sentence)

		expect(sentence).toHaveBeenCalledWith(step)
		expect(reason?.textContent).toBe(sentenceOf(step).replace('%s', ERROR))
	},
)

test('holds the whole sentence in one span with no class, the error whole in one code element', () => {
	const reason = drawn(pausedAt('register')) as HTMLElement

	expect(reason.tagName).toBe('SPAN')
	expect(reason.className).toBe('')
	expect(reason.parentElement?.children).toHaveLength(1)
	expect([...reason.childNodes].map((node) => node.nodeName)).toEqual(['#text', 'CODE'])
	expect(reason.querySelector('code')?.textContent).toBe(ERROR)
})

test('keeps the words a translation puts after the error', () => {
	const reason = drawn(pausedAt('register'), () => 'Erreur : %s. Corrigez-la puis redémarrez.') as HTMLElement

	expect(reason.textContent).toBe(`Erreur : ${ERROR}. Corrigez-la puis redémarrez.`)
	expect(reason.querySelector('code')?.textContent).toBe(ERROR)
})

test('still shows the error after a sentence that holds no place for it', () => {
	const reason = drawn(pausedAt('register'), () => 'It failed.') as HTMLElement

	expect(reason.textContent).toBe(`It failed.${ERROR}`)
	expect(reason.querySelector('code')?.textContent).toBe(ERROR)
})

test('keeps an error that holds markup as text', () => {
	const markup = '<b>title</b> & <script>alert(1)</script> is too long'

	const reason = drawn(pausedAt('register', markup)) as HTMLElement

	expect(reason.querySelector('code')?.textContent).toBe(markup)
	expect(reason.querySelector('b')).toBeNull()
	expect(reason.querySelector('script')).toBeNull()
})

test.each(['active', 'inactive'])('draws nothing for a plugin that is %s', (status) => {
	const sentence = vi.fn(sentenceOf)

	expect(drawn({ status, paused_step: '', paused_reason: '' }, sentence)).toBeNull()
	expect(sentence).not.toHaveBeenCalled()
})
