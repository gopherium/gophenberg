// SPDX-License-Identifier: Apache-2.0

import { Button, InputControl, SelectControl, Stack, Text } from '@gophenberg/frontend-sdk'
import { RowControls } from '@gopherium/godmin'
import { formatNumber } from '@gopherium/gottext'
import { __, sprintf } from '@wordpress/i18n'
import { useLayoutEffect, useRef, useState } from 'react'

import { needsValue } from './conditions'
import { chosenOf } from './select'
import type { Choice } from './select'

/** One rule a set holds: the source it reads, its condition, and the value it compares against. */
interface Rule {
	source: string
	operator: string
	value: string
}

/** One source a rule may read, under its name, with the conditions it offers and the values it takes. */
export interface SourceChoices extends Choice {
	/** The conditions the source offers, as the server names them. */
	operators: string[]
	/** The values offered to pick. */
	values: Choice[]
	/** The input type a value is written in while no value is offered to pick, none for a source always picked from. */
	typed?: string
}

/** What the rule sets editor shows and edits. */
interface RuleSetsProps {
	draft: Rule[][]
	sources: SourceChoices[]
	/** The words shown while no rule set stands. */
	empty: string
	readOnly: boolean
	onDraft: (next: Rule[][]) => void
}

/** Where a rule sits: its set and its place in that set. */
interface Spot {
	set: number
	rule: number
}

/** What each rule set and each rule draws from, beside the editor's own props. */
interface SpotProps extends RuleSetsProps {
	/** Returns the ref registering an element under its key. */
	hold: (key: string) => (element: HTMLElement | null) => void
	/** Names the key to focus once the next draft shows, while the element given holds the focus. */
	after: (key: string, from: Element | undefined) => void
	/** Takes away the rule at the spot given. */
	onRemove: (at: Spot) => void
}

/** The choices a rule reads while its source is not among those offered. */
const NO_CHOICES: SourceChoices = { value: '', label: '', operators: [], values: [] }

/** The key the button adding a rule set is registered under. */
const ADD_KEY = 'add'

/**
 * Returns the key a rule set, or one rule in it, is registered under.
 * @param set - The set's place.
 * @param rule - The rule's place in the set, none for the set itself.
 * @returns The key.
 */
function spotKey(set: number, rule?: number): string {
	return rule === undefined ? `${set}` : `${set}:${rule}`
}

/**
 * Returns the key of what takes the focus once a rule goes.
 * @param held - The rule sets held before the rule goes.
 * @param at - Where the rule sat.
 * @returns The rule taking its place or the one before it, else the set taking its emptied set's place or the one
 *   before it, else the button adding a rule set.
 */
function focusAfter(held: Rule[][], at: Spot): string {
	const left = held[at.set].length - 1
	if (left > 0) {
		return spotKey(at.set, Math.min(at.rule, left - 1))
	}
	const sets = held.length - 1
	return sets > 0 ? spotKey(Math.min(at.set, sets - 1)) : ADD_KEY
}

/**
 * Returns what registers each rule set, each rule and the button adding a set, and moves the focus as they come and go.
 * @returns The call returning the ref that registers an element under a key, the call returning the element a key
 *   holds, and the call naming the key to focus once the next draft shows while a given element holds the focus.
 */
function useRuleFocus() {
	const known = useRef(new Map<string, HTMLElement>())
	const next = useRef<string | null>(null)
	useLayoutEffect(() => {
		const target = next.current
		next.current = null
		if (target !== null) {
			known.current.get(target)?.focus()
		}
	})
	const [focus] = useState(() => ({
		hold: (key: string) => (element: HTMLElement | null) => {
			if (element === null) {
				known.current.delete(key)
			} else {
				known.current.set(key, element)
			}
		},
		held: (key: string) => known.current.get(key),
		after: (key: string, from: Element | undefined) => {
			if (from?.contains(document.activeElement) === true) {
				next.current = key
			}
		},
	}))
	return focus
}

/**
 * Returns the phrase a rule operator is offered under.
 * @param operator - The operator as the server names it.
 * @returns The phrase to show, the operator itself when the admin has no name for it.
 */
export function operatorLabel(operator: string): string {
	const labels = new Map<string, string>([
		['==', __('is', 'gophenberg')],
		['!=', __('is not', 'gophenberg')],
		['<', __('is below', 'gophenberg')],
		['>', __('is above', 'gophenberg')],
		['contains', __('holds', 'gophenberg')],
		['empty', __('is empty', 'gophenberg')],
		['not_empty', __('holds anything', 'gophenberg')],
	])
	return labels.get(operator) ?? operator
}

/**
 * Returns the choice standing for a stored value.
 * @param offered - The choices the select holds.
 * @param value - The value stored.
 * @returns The matching choice, or the raw value when nothing matches.
 */
function chosen(offered: Choice[], value: string): Choice {
	return offered.find((held) => held.value === value) ?? { label: value, value }
}

/**
 * Returns the rule a new row starts as, read from the first source offered.
 * @param sources - The sources a rule may read.
 * @returns The rule to add, or nothing when no source can carry one.
 */
function blankRule(sources: SourceChoices[]): Rule | null {
	const [first] = sources
	if (first === undefined) {
		return null
	}
	const [operator] = first.operators
	const value = first.values[0]?.value ?? (first.typed === undefined ? undefined : '')
	if (operator === undefined || value === undefined) {
		return null
	}
	return { source: first.value, operator, value }
}

/**
 * Returns the rule sets with one rule replaced.
 * @param held - The rule sets held.
 * @param at - Where the rule sits.
 * @param rule - The rule to store there.
 * @returns The rule sets to hold.
 */
function withRule(held: Rule[][], at: Spot, rule: Rule): Rule[][] {
	return held.map((set, index) =>
		index === at.set ? set.map((stored, spot) => (spot === at.rule ? rule : stored)) : set,
	)
}

/**
 * Returns the rule sets with one rule gone, dropping a set the rule emptied.
 * @param held - The rule sets held.
 * @param at - Where the rule sits.
 * @returns The rule sets to hold.
 */
function withoutRule(held: Rule[][], at: Spot): Rule[][] {
	return held
		.map((set, index) => (index === at.set ? set.filter((_, spot) => spot !== at.rule) : set))
		.filter((set) => set.length > 0)
}

/**
 * Renders the rule sets, any one of which is enough, and the control adding another.
 * @param props - The rule sets held, the sources a rule may read, the words shown while none stands, whether they are
 * read only, and what to call with an edit.
 * @returns The editor element.
 */
export function RuleSets(props: RuleSetsProps) {
	const newRule = blankRule(props.sources)
	const focus = useRuleFocus()
	const shared: SpotProps = {
		...props,
		hold: focus.hold,
		after: focus.after,
		onRemove: (at) => {
			focus.after(focusAfter(props.draft, at), focus.held(spotKey(at.set, at.rule)))
			props.onDraft(withoutRule(props.draft, at))
		},
	}
	return (
		<Stack direction="column" gap="md">
			{props.draft.length === 0 && <Text>{props.empty}</Text>}
			{props.draft.map((set, at) => (
				<RuleSet key={at} set={set} at={at} {...shared} />
			))}
			<div>
				<Button
					ref={focus.hold(ADD_KEY)}
					variant="outline"
					disabled={newRule === null || props.readOnly}
					onClick={
						newRule === null
							? undefined
							: (event) => {
									focus.after(spotKey(props.draft.length), event.currentTarget)
									props.onDraft([...props.draft, [newRule]])
								}
					}
				>
					{__('Add rule set', 'gophenberg')}
				</Button>
			</div>
		</Stack>
	)
}

/**
 * Renders one set of rules an item has to meet all of, named above them.
 * @param props - The set, where it sits, the rule sets held, the sources, and what to do.
 * @returns The set element.
 */
function RuleSet(props: SpotProps & { set: Rule[]; at: number }) {
	const newRule = blankRule(props.sources)
	const name = sprintf(__('Rule set %(number)s', 'gophenberg'), { number: formatNumber(props.at + 1) })
	return (
		<Stack
			ref={props.hold(spotKey(props.at))}
			direction="column"
			gap="sm"
			role="group"
			aria-label={name}
			tabIndex={-1}
			className="godmin-rows__row"
		>
			<Text variant="body-sm">{name}</Text>
			{props.set.map((rule, spot) => (
				<RuleRow key={spot} rule={rule} spot={spot} {...props} />
			))}
			<div>
				<Button
					variant="outline"
					size="compact"
					disabled={newRule === null || props.readOnly}
					onClick={
						newRule === null
							? undefined
							: (event) => {
									props.after(spotKey(props.at, props.set.length), event.currentTarget)
									props.onDraft(props.draft.map((set, index) => (index === props.at ? [...set, newRule] : set)))
								}
					}
				>
					{__('Add condition', 'gophenberg')}
				</Button>
			</div>
		</Stack>
	)
}

/**
 * Renders one rule as the source, condition and value it reads, ending in the trash that takes it away.
 * @param props - The rule, where it sits, the rule sets held, the sources, and what to do.
 * @returns The row element.
 */
function RuleRow(props: SpotProps & { rule: Rule; at: number; spot: number }) {
	const { rule } = props
	const where = { set: props.at, rule: props.spot }
	const place = { number: formatNumber(props.spot + 1), set: formatNumber(props.at + 1) }
	const held = props.sources.find((source) => source.value === rule.source) ?? NO_CHOICES
	const sources = props.sources.map((source) => ({ value: source.value, label: source.label }))
	const operators = held.operators.map((operator) => ({ value: operator, label: operatorLabel(operator) }))

	/**
	 * Stores the rule as one of its controls changed it.
	 * @param part - The part of the rule that changed.
	 */
	function edited(part: Partial<Rule>) {
		props.onDraft(withRule(props.draft, where, { ...rule, ...part }))
	}

	/**
	 * Stores the rule under the source it moved to, starting its condition and value anewRule.
	 * @param source - The source picked.
	 */
	function moved(source: string) {
		const started = blankRule(props.sources.filter((offered) => offered.value === source))
		if (started !== null) {
			props.onDraft(withRule(props.draft, where, started))
		}
	}

	return (
		<div
			ref={props.hold(spotKey(props.at, props.spot))}
			role="group"
			aria-label={sprintf(__('Rule %(number)s of set %(set)s', 'gophenberg'), place)}
			tabIndex={-1}
			className="godmin-rows__row"
		>
			<div className="godmin-rows__line">
				<div className="godmin-rows__inputs">
					<div className="godmin-form__row">
						<SelectControl
							label={__('Source', 'gophenberg')}
							items={sources}
							value={chosen(sources, rule.source)}
							readOnly={props.readOnly}
							onValueChange={(item) => moved(chosenOf(item, sources, chosen(sources, rule.source)).value)}
						/>
						<SelectControl
							label={__('Condition', 'gophenberg')}
							items={operators}
							value={chosen(operators, rule.operator)}
							readOnly={props.readOnly}
							onValueChange={(item) =>
								edited({ operator: chosenOf(item, operators, chosen(operators, rule.operator)).value })
							}
						/>
						{needsValue(rule.operator) && (
							<RuleValue
								held={held}
								value={rule.value}
								readOnly={props.readOnly}
								onValue={(value) => edited({ value })}
							/>
						)}
					</div>
				</div>
				<RowControls
					movable={false}
					removable={!props.readOnly}
					labels={{ remove: sprintf(__('Remove rule %(number)s of set %(set)s', 'gophenberg'), place) }}
					onRemove={() => props.onRemove(where)}
				/>
			</div>
		</div>
	)
}

/** What the value control of a rule shows and edits. */
interface ValueProps {
	/** The choices of the source the rule reads. */
	held: SourceChoices
	value: string
	readOnly: boolean
	onValue: (value: string) => void
}

/**
 * Renders the value a rule compares against, picked from the values its source offers or written in.
 * @param props - The choices of the rule's source, the value stored, whether it is read only, and what to call with a
 * new value.
 * @returns The value control, a box while the source offers no value to pick and takes a written one.
 */
function RuleValue(props: ValueProps) {
	const { held } = props
	if (held.values.length === 0 && held.typed !== undefined) {
		return (
			<InputControl
				label={__('Value', 'gophenberg')}
				type={held.typed}
				autoComplete="off"
				value={props.value}
				readOnly={props.readOnly}
				onValueChange={props.onValue}
			/>
		)
	}
	const current = chosen(held.values, props.value)
	return (
		<SelectControl
			label={__('Value', 'gophenberg')}
			items={held.values}
			value={current}
			readOnly={props.readOnly}
			onValueChange={(item) => props.onValue(chosenOf(item, held.values, current).value)}
		/>
	)
}
