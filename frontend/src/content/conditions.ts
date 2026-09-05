// SPDX-License-Identifier: Apache-2.0

/** The operator matching a value exactly. */
const OPERATOR_IS = '=='

/** The operator excluding a value. */
const OPERATOR_IS_NOT = '!='

/** The operator matching a value below the rule value. */
const OPERATOR_LESS = '<'

/** The operator matching a value above the rule value. */
const OPERATOR_GREATER = '>'

/** The operator matching a value holding the rule value. */
const OPERATOR_CONTAINS = 'contains'

/** The operator matching a value that is not set. */
const OPERATOR_EMPTY = 'empty'

/** The operator matching a value that is set. */
const OPERATOR_NOT_EMPTY = 'not_empty'

/** One condition a field is shown under. */
export interface ConditionRule {
	source: string
	operator: string
	value: string
}

/** The field a condition reads and the settings it carries. */
export interface ConditionField {
	key: string
	kind: string
	settings: Record<string, unknown>
}

/**
 * Returns whether the operator compares against a rule value.
 * @param operator - The operator a rule carries.
 * @returns Whether a value is needed beside it.
 */
export function needsValue(operator: string): boolean {
	return operator !== OPERATOR_EMPTY && operator !== OPERATOR_NOT_EMPTY
}

/**
 * Returns the operators a field of the kind offers as a rule source.
 * @param kind - The kind the field holds.
 * @param multiple - Whether a choice field takes several values.
 * @returns The operators offered, none for a kind no rule reads.
 */
export function operatorsFor(kind: string, multiple: boolean): string[] {
	if (kind === 'text') {
		return [OPERATOR_IS, OPERATOR_IS_NOT, OPERATOR_CONTAINS, OPERATOR_EMPTY, OPERATOR_NOT_EMPTY]
	}
	if (kind === 'number' || kind === 'date') {
		return [OPERATOR_IS, OPERATOR_IS_NOT, OPERATOR_LESS, OPERATOR_GREATER, OPERATOR_EMPTY, OPERATOR_NOT_EMPTY]
	}
	if (kind === 'boolean') {
		return [OPERATOR_IS, OPERATOR_IS_NOT]
	}
	if (kind === 'choice') {
		return multiple
			? [OPERATOR_CONTAINS, OPERATOR_EMPTY, OPERATOR_NOT_EMPTY]
			: [OPERATOR_IS, OPERATOR_IS_NOT, OPERATOR_EMPTY, OPERATOR_NOT_EMPTY]
	}
	if (kind === 'media') {
		return [OPERATOR_EMPTY, OPERATOR_NOT_EMPTY]
	}
	return []
}

/**
 * Returns whether a value stands for a field nobody filled in.
 * @param held - The value the item holds.
 * @returns Whether it reads as empty.
 */
function empty(held: unknown): boolean {
	if (held === null || held === undefined) {
		return true
	}
	if (Array.isArray(held)) {
		return held.length === 0
	}
	if (typeof held === 'object') {
		return Object.keys(held).length === 0
	}
	return held === ''
}

/**
 * Returns whether a held string, number or boolean is the rule value.
 * @param held - The value the item holds.
 * @param value - The value the rule names.
 * @returns Whether the two are the same.
 */
function equals(held: unknown, value: string): boolean {
	if (typeof held === 'string') {
		return held === value
	}
	if (typeof held === 'boolean') {
		return String(held) === value
	}
	const asked = Number(value)
	return typeof held === 'number' && value.trim() !== '' && !Number.isNaN(asked) && held === asked
}

/**
 * Returns whether a held string or number sits on the given side of the rule value.
 * @param held - The value the item holds.
 * @param value - The value the rule names.
 * @param side - Below the value as minus one, above it as one.
 * @returns Whether it sits there.
 */
function ordered(held: unknown, value: string, side: number): boolean {
	if (typeof held === 'string') {
		return side > 0 ? held > value : held < value
	}
	const asked = Number(value)
	if (typeof held !== 'number' || value.trim() === '' || Number.isNaN(asked)) {
		return false
	}
	return (held - asked) * side > 0
}

/**
 * Returns whether a held string holds the rule value inside it, or a held list holds it as a member.
 * @param held - The value the item holds.
 * @param value - The value the rule names.
 * @returns Whether it is held.
 */
function contains(held: unknown, value: string): boolean {
	if (typeof held === 'string') {
		return held.includes(value)
	}
	if (Array.isArray(held)) {
		return held.some((member) => member === value)
	}
	return false
}

/**
 * Returns whether a held value satisfies the operator and the rule value, judged by the value's shape.
 * @param operator - The operator the rule carries.
 * @param held - The value the item holds.
 * @param value - The value the rule names.
 * @returns Whether the rule holds.
 */
export function compare(operator: string, held: unknown, value: string): boolean {
	if (operator === OPERATOR_IS) {
		return equals(held, value)
	}
	if (operator === OPERATOR_IS_NOT) {
		return !equals(held, value)
	}
	if (operator === OPERATOR_LESS) {
		return ordered(held, value, -1)
	}
	if (operator === OPERATOR_GREATER) {
		return ordered(held, value, 1)
	}
	if (operator === OPERATOR_CONTAINS) {
		return contains(held, value)
	}
	if (operator === OPERATOR_EMPTY) {
		return empty(held)
	}
	return operator === OPERATOR_NOT_EMPTY && !empty(held)
}

/**
 * Returns the rules a field is shown under, none when it carries none.
 * @param field - The field to read.
 * @returns The rules, as OR groups of AND rows.
 */
export function conditionsOf(field: ConditionField): ConditionRule[][] {
	const groups = field.settings.conditions
	if (!Array.isArray(groups)) {
		return []
	}
	return groups
		.map((group) => (Array.isArray(group) ? group.map(ruleOf).filter(named) : []))
		.filter((group) => group.length > 0)
}

/**
 * Returns whether a rule names anything at all.
 * @param rule - The rule to weigh.
 * @returns Whether any of its parts was written.
 */
function named(rule: ConditionRule): boolean {
	return rule.source !== '' || rule.operator !== '' || rule.value !== ''
}

/**
 * Returns one stored rule row as the evaluator reads it.
 * @param row - The row the settings hold.
 * @returns The rule, its parts empty where the row named none.
 */
function ruleOf(row: unknown): ConditionRule {
	const held = (row ?? {}) as Record<string, unknown>
	return {
		source: typeof held.source === 'string' ? held.source : '',
		operator: typeof held.operator === 'string' ? held.operator : '',
		value: typeof held.value === 'string' ? held.value : '',
	}
}

/**
 * Returns whether a choice field takes several values.
 * @param field - The field to read.
 * @returns Whether it takes several.
 */
export function multipleOf(field: ConditionField): boolean {
	return field.settings.multiple === true
}

/**
 * Returns the scope as the evaluator reads it, where an absent boolean reads as false.
 * @param fields - The fields of the scope.
 * @param scope - The values the scope holds.
 * @returns The screen the rules read.
 */
function screenOf(fields: ConditionField[], scope: Record<string, unknown>): Record<string, unknown> {
	const screen: Record<string, unknown> = { ...scope }
	for (const field of fields) {
		if (field.kind === 'boolean' && (screen[field.key] === undefined || screen[field.key] === null)) {
			screen[field.key] = false
		}
	}
	return screen
}

/**
 * Returns whether every declared source the field's conditions read is already ordered.
 * @param field - The field to weigh.
 * @param declared - The keys the scope declares.
 * @param placed - The keys already ordered.
 * @returns Whether the field may be ordered now.
 */
function sourcesPlaced(field: ConditionField, declared: Set<string>, placed: Set<string>): boolean {
	return conditionsOf(field).every((group) =>
		group.every((rule) => !declared.has(rule.source) || placed.has(rule.source)),
	)
}

/**
 * Returns the fields with every source before its dependents, and the keys a loop leaves unordered.
 * @param fields - The fields of one scope.
 * @returns The ordered fields and the looping keys.
 */
function dependencyOrder(fields: ConditionField[]): { ordered: ConditionField[]; looped: string[] } {
	const declared = new Set(fields.map((field) => field.key))
	const placed = new Set<string>()
	const ordered: ConditionField[] = []
	for (let moved = true; moved; ) {
		moved = false
		for (const field of fields) {
			if (placed.has(field.key) || !sourcesPlaced(field, declared, placed)) {
				continue
			}
			placed.add(field.key)
			ordered.push(field)
			moved = true
		}
	}
	return { ordered, looped: fields.filter((field) => !placed.has(field.key)).map((field) => field.key) }
}

/**
 * Returns whether one rule holds on the screen, a source no sibling offers failing it.
 * @param rule - The rule to weigh.
 * @param sources - The sibling fields a rule may read.
 * @param screen - The values the rules read.
 * @returns Whether the rule holds.
 */
function ruleHolds(rule: ConditionRule, sources: Set<string>, screen: Record<string, unknown>): boolean {
	return sources.has(rule.source) && compare(rule.operator, screen[rule.source], rule.value)
}

/**
 * Returns the values with every key the conditions hide taken away, at any depth.
 * @param fields - The fields of one scope, holding the fields their containers declare.
 * @param values - The values the scope holds.
 * @returns The values to send.
 */
export function shownValues(
	fields: ConditionField[],
	values: Record<string, unknown>,
): Record<string, unknown> {
	const hidden = hiddenKeys(fields, values)
	const shown: Record<string, unknown> = {}
	for (const [key, value] of Object.entries(values)) {
		if (!hidden.has(key)) {
			shown[key] = value
		}
	}
	return shown
}

/**
 * Returns the keys of the fields their conditions hide on the scope, a hidden source reading as absent.
 * @param fields - The fields of one scope.
 * @param scope - The values the scope holds.
 * @returns The hidden keys.
 */
export function hiddenKeys(fields: ConditionField[], scope: Record<string, unknown>): Set<string> {
	const sources = new Set(
		fields.filter((field) => operatorsFor(field.kind, multipleOf(field)).length > 0).map((f) => f.key),
	)
	const screen = screenOf(fields, scope)
	const hidden = new Set<string>()
	const { ordered, looped } = dependencyOrder(fields)
	for (const field of ordered) {
		const rules = conditionsOf(field).filter((group) => group.length > 0)
		if (rules.length === 0 || rules.some((group) => group.every((rule) => ruleHolds(rule, sources, screen)))) {
			continue
		}
		hidden.add(field.key)
		delete screen[field.key]
	}
	for (const key of looped) {
		hidden.add(key)
	}
	return hidden
}
