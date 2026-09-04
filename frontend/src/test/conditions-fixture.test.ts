// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { join } from 'node:path'

import { expect, test } from 'vitest'

import { repositoryRoot } from '../../scripts/config.ts'
import { compare, hiddenKeys, operatorsFor } from '../content/conditions'
import type { ConditionField } from '../content/conditions'

/** One row of the operators the fixture says each kind offers. */
interface OperatorRow {
	kind: string
	multiple?: boolean
	offers: string[]
}

/** One row of the comparisons the fixture pins. */
interface CompareRow {
	operator: string
	held: unknown
	value: string
	holds: boolean
}

/** One scope the fixture says hides a known set of keys. */
interface HiddenRow {
	name: string
	fields: { key: string; kind: string; multiple?: boolean; conditions?: unknown }[]
	scope: Record<string, unknown>
	hidden: string[]
}

/** The shared table both evaluators answer to. */
interface Fixture {
	operators: OperatorRow[]
	compare: CompareRow[]
	hidden: HiddenRow[]
}

const FIXTURE = JSON.parse(
	readFileSync(join(repositoryRoot(), 'internal', 'content', 'testdata', 'conditions.json'), 'utf8'),
) as Fixture

/**
 * Returns the fixture's field as the evaluator reads one.
 * @param declared - The field the fixture declares.
 * @returns The field to evaluate.
 */
function fieldOf(declared: HiddenRow['fields'][number]): ConditionField {
	const settings: Record<string, unknown> = {}
	if (declared.multiple === true) {
		settings.multiple = true
	}
	if (declared.conditions !== undefined) {
		settings.conditions = declared.conditions
	}
	return { key: declared.key, kind: declared.kind, settings }
}

test('offers the operators the server offers for every kind', () => {
	for (const row of FIXTURE.operators) {
		expect([row.kind, row.multiple === true, operatorsFor(row.kind, row.multiple === true)]).toEqual([
			row.kind,
			row.multiple === true,
			row.offers,
		])
	}
})

test('compares a value the way the server compares it', () => {
	for (const row of FIXTURE.compare) {
		expect([row.operator, row.held, row.value, compare(row.operator, row.held, row.value)]).toEqual([
			row.operator,
			row.held,
			row.value,
			row.holds,
		])
	}
})

test('hides the keys the server hides on every scope', () => {
	for (const row of FIXTURE.hidden) {
		const held = [...hiddenKeys(row.fields.map(fieldOf), row.scope)].sort()
		expect([row.name, held]).toEqual([row.name, [...row.hidden].sort()])
	}
})
