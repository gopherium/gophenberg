// SPDX-License-Identifier: Apache-2.0

import { Button, SelectControl, Stack, Text } from '@gophenberg/frontend-sdk'
import { ErrorNotice, FormDialog } from '@gopherium/godmin'
import { formatNumber } from '@gopherium/gottext'
import { __, sprintf } from '@wordpress/i18n'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import {
	backlinkSettings,
	backlinkSources,
	groupErrorMessage,
	groupsQueryKey,
	listRuleSources,
	pickOf,
	ruleSourcesQueryKey,
	StaleWriteError,
	typeSource,
	updateGroup,
} from './groups'
import { SourceOfLinks } from './SourceOfLinks'
import { chosenOf } from './select'
import { pluralOf } from './typeText'
import type { BacklinkPick, FieldGroup, Location, LocationRule, RuleSource } from './groups'
import type { Choice } from './select'
import type { ContentField, ContentType } from './types'

/** A Linked from source the operator picked, with the stamp its field carried when they picked it. */
interface Touched extends BacklinkPick {
	updatedAt: string
}

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
function namedSources(sources: RuleSource[], types: ContentType[]): RuleSource[] {
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
 * Returns the settings naming what each Linked from field the operator touched now reads.
 * @param groups - The groups a relation may stand in.
 * @param picked - The pairs the operator picked, by field key.
 * @returns The settings to send, or nothing when no field was touched.
 */
function pointedAnew(
	groups: FieldGroup[],
	picked: Record<string, Touched>,
): Record<string, Record<string, unknown>> | undefined {
	const touched = Object.entries(picked)
	if (touched.length === 0) {
		return undefined
	}
	return Object.fromEntries(
		touched.map(([key, pick]) => [
			key,
			{ ...backlinkSettings(backlinkSources(groups, pick.group, pick.field)), updated_at: pick.updatedAt },
		]),
	)
}

/** What the control editing where a group appears takes. */
interface RulesProps {
	held: FieldGroup
	groups: FieldGroup[]
	types: ContentType[]
	onDone: (said: string) => void
	onRefused: (cause: unknown) => void
}

/**
 * Renders the control editing where a group appears and, while it asks, its form.
 * @param props - The group, the groups a relation may stand in, the registered types, and the reporters.
 * @returns The control and its form.
 */
export function RulesDialog(props: RulesProps) {
	const [asking, setAsking] = useState(false)
	const sources = useQuery({ queryKey: ruleSourcesQueryKey, queryFn: listRuleSources })
	return (
		<>
			<Button variant="outline" onClick={() => setAsking(true)}>
				{__('Rules', 'gophenberg')}
			</Button>
			{asking ? (
				<RulesForm
					{...props}
					sources={sources.data ?? []}
					failed={sources.isError}
					onClose={() => setAsking(false)}
				/>
			) : null}
		</>
	)
}

/**
 * Renders the form editing where a group appears, its draft started from the stored rules on each ask.
 * @param props - The group, the groups, the types, the reporters, the sources offered and what takes the form away.
 * @returns The form dialog, kept open with the draft after a failure other than a stale write.
 */
function RulesForm(props: RulesProps & { sources: RuleSource[]; failed: boolean; onClose: () => void }) {
	const [draft, setDraft] = useState<Location>(props.held.location)
	const [picked, setPicked] = useState<Record<string, Touched>>({})
	const client = useQueryClient()
	const save = useMutation({
		mutationFn: () =>
			updateGroup(props.held.id, { location: draft, backlinks: pointedAnew(props.groups, picked) }),
	})
	const said = sprintf(__('%(group)s now appears where you said.', 'gophenberg'), { group: props.held.title })
	const send = () =>
		save.mutateAsync().then(
			() => {
				props.onDone(said)
			},
			(cause: unknown) => {
				if (!(cause instanceof StaleWriteError)) {
					return { error: groupErrorMessage(cause) }
				}
				props.onRefused(cause)
				void client.invalidateQueries({ queryKey: groupsQueryKey })
				return undefined
			},
		)
	return (
		<FormDialog
			title={sprintf(__('Where %(group)s appears', 'gophenberg'), { group: props.held.title })}
			submitLabel={__('Save rules', 'gophenberg')}
			cancelLabel={__('Cancel', 'gophenberg')}
			onSubmit={send}
			onClose={props.onClose}
		>
			{(busy) => (
				<>
					<RulesBody
						draft={draft}
						sources={namedSources(props.sources, props.types)}
						failed={props.failed}
						readOnly={busy}
						onDraft={setDraft}
					/>
					<LinkedSources
						held={props.held}
						groups={props.groups}
						picked={picked}
						readOnly={busy}
						onPick={(field, pick) =>
							setPicked((was) => ({ ...was, [field.key]: { ...pick, updatedAt: field.updatedAt } }))
						}
					/>
				</>
			)}
		</FormDialog>
	)
}

/**
 * Renders the pickers naming what each Linked from field of the group reads.
 * @param props - The group, the groups a relation may stand in, the pairs picked so far, whether the pickers are read
 *   only, and what to call with a pick.
 * @returns The pickers element, or nothing when the group holds no Linked from field.
 */
function LinkedSources(props: {
	held: FieldGroup
	groups: FieldGroup[]
	picked: Record<string, Touched>
	readOnly: boolean
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
						{pick === null ? (
							<Text variant="body-sm">
								{__(
									'It reads a relation inside a container, and only an import can point it elsewhere.',
									'gophenberg',
								)}
							</Text>
						) : (
							<Stack direction="row" gap="sm" align="end">
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
function RulesBody(props: Drafting & { failed: boolean }) {
	if (props.failed) {
		return <ErrorNotice>{__('The rules could not be loaded.', 'gophenberg')}</ErrorNotice>
	}
	const fresh = blankRule(props.sources)
	return (
		<Stack direction="column" gap="md">
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
 * Renders one set of conditions an item has to meet all of.
 * @param props - The set, where it sits, the rules held, the sources, and what to do.
 * @returns The set element.
 */
function RuleSet(props: Drafting & { set: LocationRule[]; at: number }) {
	const fresh = blankRule(props.sources)
	return (
		<Stack
			direction="column"
			gap="sm"
			role="group"
			aria-label={sprintf(__('Rule set %(number)s', 'gophenberg'), { number: formatNumber(props.at + 1) })}
		>
			{props.set.map((rule, spot) => (
				<RuleRow key={spot} rule={rule} spot={spot} {...props} />
			))}
			<Button
				variant="outline"
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
