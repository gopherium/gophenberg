// SPDX-License-Identifier: Apache-2.0

import { Button, Dialog, SelectControl, Stack, Text } from '@gophenberg/frontend-sdk'
import { ErrorNotice } from '@gopherium/godmin'
import { __, sprintf } from '@wordpress/i18n'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useState } from 'react'

import { groupErrorMessage, listRuleSources, ruleSourcesQueryKey, typeSource, updateGroup } from './groups'
import { chosenOf } from './select'
import type { FieldGroup, Location, LocationRule, RuleSource } from './groups'
import type { Choice } from './select'

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
 * Renders the control editing where a group appears.
 * @param props - The group and the reporter.
 * @returns The control and its dialog.
 */
export function RulesDialog(props: { held: FieldGroup; onDone: (said: string) => void }) {
	const [open, setOpen] = useState(false)
	const [notice, setNotice] = useState('')
	const [draft, setDraft] = useState<Location>(props.held.location)
	const sources = useQuery({ queryKey: ruleSourcesQueryKey, queryFn: listRuleSources })
	const save = useMutation({
		mutationFn: () => updateGroup(props.held.id, { location: draft }),
		onSuccess: () => {
			setOpen(false)
			props.onDone(sprintf(__('%(group)s now appears where you said.', 'gophenberg'), {
				group: props.held.title,
			}))
		},
		onError: (cause) => setNotice(groupErrorMessage(cause)),
	})

	/**
	 * Opens the dialog on the stored rules, or closes it on the edits.
	 * @param next - Whether the dialog is opening.
	 */
	function change(next: boolean) {
		if (next) {
			setDraft(props.held.location)
			setNotice('')
		}
		setOpen(next)
	}

	return (
		<>
			<Button variant="outline" onClick={() => change(true)}>
				{__('Rules', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={change}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>
							{sprintf(__('Where %(group)s appears', 'gophenberg'), { group: props.held.title })}
						</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<Stack direction="column" gap="md">
							{notice !== '' && <ErrorNotice>{notice}</ErrorNotice>}
							<RulesBody
								draft={draft}
								sources={sources.data ?? []}
								failed={sources.isError}
								onDraft={setDraft}
							/>
						</Stack>
					</Dialog.Content>
					<Dialog.Footer>
						<Button variant="outline" onClick={() => change(false)}>
							{__('Cancel', 'gophenberg')}
						</Button>
						<Button loading={save.isPending} onClick={() => save.mutate()}>
							{__('Save rules', 'gophenberg')}
						</Button>
					</Dialog.Footer>
				</Dialog.Popup>
			</Dialog.Root>
		</>
	)
}

/** What an edited rule row reports back. */
interface Drafting {
	draft: Location
	sources: RuleSource[]
	onDraft: (next: Location) => void
}

/**
 * Renders the rule sets and the control adding another.
 * @param props - The rules held, the sources offered, whether the sources failed, and what to do.
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
				disabled={fresh === null}
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
			aria-label={sprintf(__('Rule set %(number)d', 'gophenberg'), { number: props.at + 1 })}
		>
			{props.set.map((rule, spot) => (
				<RuleRow key={spot} rule={rule} spot={spot} {...props} />
			))}
			<Button
				variant="outline"
				disabled={fresh === null}
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
			aria-label={sprintf(__('Rule %(number)d of set %(set)d', 'gophenberg'), {
				number: props.spot + 1,
				set: props.at + 1,
			})}
		>
			<SelectControl
				label={__('Source', 'gophenberg')}
				items={sources}
				value={chosen(sources, rule.source)}
				onValueChange={(item) => moved(chosenOf(item, sources, chosen(sources, rule.source)).value)}
			/>
			<SelectControl
				label={__('Condition', 'gophenberg')}
				items={operators}
				value={chosen(operators, rule.operator)}
				onValueChange={(item) =>
					edited({ operator: chosenOf(item, operators, chosen(operators, rule.operator)).value })
				}
			/>
			<SelectControl
				label={__('Value', 'gophenberg')}
				items={values}
				value={chosen(values, rule.value)}
				onValueChange={(item) => edited({ value: chosenOf(item, values, chosen(values, rule.value)).value })}
			/>
			<Button variant="outline" onClick={() => props.onDraft(withoutRule(props.draft, where))}>
				{__('Remove', 'gophenberg')}
			</Button>
		</Stack>
	)
}
