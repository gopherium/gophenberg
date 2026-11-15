// SPDX-License-Identifier: Apache-2.0

import { Button, SelectControl, Stack, Text } from '@gophenberg/frontend-sdk'
import { ErrorNotice } from '@gopherium/godmin'
import { formatNumber } from '@gopherium/gottext'
import { __, sprintf } from '@wordpress/i18n'

import { changedElsewhere, linkPart } from './groupDraft'
import { backlinkSources, pickOf, typeSource } from './groups'
import { SourceOfLinks } from './SourceOfLinks'
import { chosenOf } from './select'
import { pluralOf } from './typeText'
import type { DraftPart } from './groupDraft'
import type { BacklinkPick, FieldGroup, Location, LocationRule, RuleSource } from './groups'
import type { Choice } from './select'
import type { ContentField, ContentType } from './types'

/**
 * Returns the phrase a rule operator is offered under.
 * @param operator - The operator as the server names it.
 * @returns The phrase to show, the operator itself when the admin has no name for it.
 */
export function operatorLabel(operator: string): string {
	if (operator === '==') {
		return __('is', 'gophenberg')
	}
	if (operator === '!=') {
		return __('is not', 'gophenberg')
	}
	return operator
}

/**
 * Returns the name a rule source is offered under.
 * @param source - The source as the server names it.
 * @returns The name to show, the source itself when the admin has no name for it.
 */
function sourceLabel(source: string): string {
	return source === typeSource ? __('Content type', 'gophenberg') : source
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
 * Returns a type the content type source offers under the plural name the reader reads it by.
 * @param offered - The type key the server offers and the plural name it carries.
 * @param types - The registered types, each naming the plugin that declared it.
 * @returns The choice the value select holds, as the server sent it while the type is not among the registered ones.
 */
function namedType(offered: Choice, types: ContentType[]): Choice {
	const found = types.find((held) => held.key === offered.value)
	if (found === undefined) {
		return offered
	}
	return { ...offered, label: pluralOf({ key: offered.value, pluralLabel: offered.label, origin: found.origin }) }
}

/**
 * Returns the sources the server offers, each type among the values named as the reader reads it.
 * @param sources - The sources the server offers.
 * @param types - The registered types, each naming the plugin that declared it.
 * @returns The sources the selects offer.
 */
export function namedSources(sources: RuleSource[], types: ContentType[]): RuleSource[] {
	return sources.map((offered) =>
		offered.source === typeSource
			? { ...offered, values: offered.values.map((value) => namedType(value, types)) }
			: offered,
	)
}

/**
 * Returns the rule a new row starts as.
 * @param sources - The sources the server offers.
 * @returns The rule to add, or nothing when no source can carry one.
 */
function blankRule(sources: RuleSource[]): LocationRule | null {
	const [first] = sources
	if (first === undefined) {
		return null
	}
	const [operator] = first.operators
	const [value] = first.values
	if (operator === undefined || value === undefined) {
		return null
	}
	return { source: first.source, operator, value: value.value }
}

/**
 * Returns the rules with one row replaced.
 * @param held - The rules held.
 * @param at - Where the row sits.
 * @param rule - The rule to store there.
 * @returns The rules to hold.
 */
function withRule(held: Location, at: { set: number; rule: number }, rule: LocationRule): Location {
	return held.map((set, index) =>
		index === at.set ? set.map((stored, spot) => (spot === at.rule ? rule : stored)) : set,
	)
}

/**
 * Returns the rules with one row gone, dropping a set the row emptied.
 * @param held - The rules held.
 * @param at - Where the row sits.
 * @returns The rules to hold.
 */
function withoutRule(held: Location, at: { set: number; rule: number }): Location {
	return held
		.map((set, index) => (index === at.set ? set.filter((_, spot) => spot !== at.rule) : set))
		.filter((set) => set.length > 0)
}

/**
 * Renders the pickers naming what each Linked from field of the group reads.
 * @param props - The group, the groups a relation may stand in, the pairs picked so far, whether the pickers are read
 *   only, the keys of the fields whose pick the saved version won over, and what to call with a pick.
 * @returns The pickers element, or nothing when the group holds no Linked from field.
 */
export function LinkedSources(props: {
	held: FieldGroup
	groups: FieldGroup[]
	picked: Record<string, BacklinkPick>
	readOnly: boolean
	lost: DraftPart[]
	onPick: (field: ContentField, pick: BacklinkPick) => void
}) {
	const reading = props.held.fields.filter((field) => field.kind === 'backlinks')
	if (reading.length === 0) {
		return null
	}
	return (
		<Stack direction="column" gap="sm">
			<Text>{__('What each Linked from field reads', 'gophenberg')}</Text>
			{reading.map((field) => {
				const pick = props.picked[field.key] ?? pickOf(field)
				return (
					<Stack key={field.key} direction="column" gap="sm" role="group" aria-label={field.label}>
						<Text variant="body-sm">{field.label}</Text>
						{props.lost.includes(linkPart(field.key)) && <Text variant="body-sm">{changedElsewhere()}</Text>}
						{pick === null ? (
							<Text variant="body-sm">
								{__(
									'It reads a relation inside a container, and only an import can point it elsewhere.',
									'gophenberg',
								)}
							</Text>
						) : (
							<Stack direction="row" gap="sm" align="end" wrap="wrap">
								<SourceOfLinks
									sources={backlinkSources(props.groups, pick.group, pick.field)}
									readOnly={props.readOnly}
									onGroup={(group) => props.onPick(field, { ...pick, group })}
									onField={(through) => props.onPick(field, { ...pick, field: through })}
								/>
							</Stack>
						)}
					</Stack>
				)
			})}
		</Stack>
	)
}

/** What an edited rule row reports back. */
interface Drafting {
	draft: Location
	sources: RuleSource[]
	readOnly: boolean
	onDraft: (next: Location) => void
}

/**
 * Renders the rule sets and the control adding another.
 * @param props - The rules held, the sources, whether the sources failed or the body is read only, and what to do.
 * @returns The body element.
 */
export function RulesBody(props: Drafting & { failed: boolean }) {
	if (props.failed) {
		return <ErrorNotice>{__('The rules could not be loaded.', 'gophenberg')}</ErrorNotice>
	}
	const fresh = blankRule(props.sources)
	return (
		<Stack direction="column" gap="md" align="start">
			{props.draft.length === 0 && (
				<Text>{__('Without a rule this group appears nowhere.', 'gophenberg')}</Text>
			)}
			{props.draft.map((set, at) => (
				<RuleSet key={at} set={set} at={at} {...props} />
			))}
			<Button
				variant="outline"
				disabled={fresh === null || props.readOnly}
				onClick={() => fresh !== null && props.onDraft([...props.draft, [fresh]])}
			>
				{__('Add rule set', 'gophenberg')}
			</Button>
		</Stack>
	)
}

/**
 * Renders one set of conditions an item has to meet all of, named above them.
 * @param props - The set, where it sits, the rules held, the sources, and what to do.
 * @returns The set element.
 */
function RuleSet(props: Drafting & { set: LocationRule[]; at: number }) {
	const fresh = blankRule(props.sources)
	const name = sprintf(__('Rule set %(number)s', 'gophenberg'), { number: formatNumber(props.at + 1) })
	return (
		<Stack direction="column" gap="sm" align="start" role="group" aria-label={name}>
			<Text variant="body-sm">{name}</Text>
			{props.set.map((rule, spot) => (
				<RuleRow key={spot} rule={rule} spot={spot} {...props} />
			))}
			<Button
				variant="outline"
				size="compact"
				disabled={fresh === null || props.readOnly}
				onClick={() =>
					fresh !== null &&
					props.onDraft(props.draft.map((set, index) => (index === props.at ? [...set, fresh] : set)))
				}
			>
				{__('Add condition', 'gophenberg')}
			</Button>
		</Stack>
	)
}

/**
 * Renders one condition as the source, operator and value it reads.
 * @param props - The rule, where it sits, the rules held, the sources, and what to do.
 * @returns The row element.
 */
function RuleRow(props: Drafting & { rule: LocationRule; at: number; spot: number }) {
	const { rule } = props
	const where = { set: props.at, rule: props.spot }
	const held = props.sources.find((source) => source.source === rule.source)
	const sources = props.sources.map((source) => ({ value: source.source, label: sourceLabel(source.source) }))
	const operators = (held?.operators ?? []).map((operator) => ({
		value: operator,
		label: operatorLabel(operator),
	}))
	const values = held?.values ?? []

	/**
	 * Stores the rule as one of its selects changed it.
	 * @param part - The part of the rule that changed.
	 */
	function edited(part: Partial<LocationRule>) {
		props.onDraft(withRule(props.draft, where, { ...rule, ...part }))
	}

	/**
	 * Stores the rule under the source it moved to, starting its condition and value afresh.
	 * @param source - The source picked.
	 */
	function moved(source: string) {
		const started = blankRule(props.sources.filter((offered) => offered.source === source))
		if (started !== null) {
			props.onDraft(withRule(props.draft, where, started))
		}
	}

	return (
		<Stack
			direction="row"
			gap="sm"
			align="end"
			wrap="wrap"
			role="group"
			aria-label={sprintf(__('Rule %(number)s of set %(set)s', 'gophenberg'), {
				number: formatNumber(props.spot + 1),
				set: formatNumber(props.at + 1),
			})}
		>
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
			<SelectControl
				label={__('Value', 'gophenberg')}
				items={values}
				value={chosen(values, rule.value)}
				readOnly={props.readOnly}
				onValueChange={(item) => edited({ value: chosenOf(item, values, chosen(values, rule.value)).value })}
			/>
			<Button
				variant="outline"
				disabled={props.readOnly}
				onClick={() => props.onDraft(withoutRule(props.draft, where))}
			>
				{__('Remove', 'gophenberg')}
			</Button>
		</Stack>
	)
}
