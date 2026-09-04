// SPDX-License-Identifier: Apache-2.0

import { Button, Dialog, InputControl, SelectControl, Stack, Text } from '@gophenberg/frontend-sdk'
import { __, sprintf } from '@wordpress/i18n'
import { useMutation } from '@tanstack/react-query'
import { useState } from 'react'

import { conditionsOf, multipleOf, needsValue, operatorsFor } from './conditions'
import { setFieldSettingsInGroup } from './groups'
import { chosenOf } from './select'
import { pairsOf } from './types'
import type { ConditionRule } from './conditions'
import type { Choice } from './select'
import type { ContentField } from './types'

/** The phrase each operator is offered under. */
const OPERATOR_LABELS: Record<string, string> = {
	'==': __('is', 'gophenberg'),
	'!=': __('is not', 'gophenberg'),
	'<': __('is below', 'gophenberg'),
	'>': __('is above', 'gophenberg'),
	contains: __('holds', 'gophenberg'),
	empty: __('is empty', 'gophenberg'),
	not_empty: __('holds anything', 'gophenberg'),
}

/** The values a boolean source offers. */
const BOOLEANS: Choice[] = [
	{ value: 'true', label: __('Yes', 'gophenberg') },
	{ value: 'false', label: __('No', 'gophenberg') },
]

/**
 * Returns the sibling fields a condition on the field may read.
 * @param siblings - The fields standing at the same depth.
 * @param field - The field the conditions belong to.
 * @returns The fields offered as sources.
 */
export function conditionSources(siblings: ContentField[], field: ContentField): ContentField[] {
	return siblings.filter(
		(held) => held.key !== field.key && operatorsFor(held.kind, multipleOf(held)).length > 0,
	)
}

/**
 * Returns the values a source offers, none when it takes a typed value.
 * @param source - The field a rule reads.
 * @returns The choices offered.
 */
function valuesOf(source: ContentField): Choice[] {
	if (source.kind === 'boolean') {
		return BOOLEANS
	}
	return source.kind === 'choice' ? pairsOf(source.settings) : []
}

/**
 * Returns the rule a fresh row starts as.
 * @param sources - The fields offered as sources.
 * @returns The rule to add.
 */
function blankRule(sources: ContentField[]): ConditionRule {
	const [first] = sources
	const offered = operatorsFor(first.kind, multipleOf(first))
	const values = valuesOf(first)
	return { source: first.key, operator: offered[0], value: values.length > 0 ? values[0].value : '' }
}

/**
 * Returns the input shape a source's value is written in.
 * @param kind - The kind the source holds.
 * @returns The input type.
 */
function shapeOf(kind: string): string {
	return kind === 'number' ? 'number' : 'text'
}

/**
 * Renders one condition as the source, operator and value it reads.
 * @param props - The rule, where it sits, the sources, and what to do.
 * @returns The row element.
 */
function ConditionRow(props: {
	rule: ConditionRule
	sources: ContentField[]
	onChange: (rule: ConditionRule) => void
	onRemove: () => void
}) {
	const held = props.sources.find((source) => source.key === props.rule.source) ?? props.sources[0]
	const sources = props.sources.map((source) => ({ value: source.key, label: source.label }))
	const operators = operatorsFor(held.kind, multipleOf(held)).map((operator) => ({
		value: operator,
		label: OPERATOR_LABELS[operator],
	}))
	const values = valuesOf(held)
	const chosen = (offered: Choice[], value: string) =>
		offered.find((one) => one.value === value) ?? { value, label: value }
	return (
		<Stack direction="row" gap="sm" align="end">
			<SelectControl
				label={__('Source', 'gophenberg')}
				items={sources}
				value={chosen(sources, props.rule.source)}
				onValueChange={(item) =>
					props.onChange(
						blankRule(
							props.sources.filter(
								(source) => source.key === chosenOf(item, sources, sources[0]).value,
							),
						),
					)
				}
			/>
			<SelectControl
				label={__('Condition', 'gophenberg')}
				items={operators}
				value={chosen(operators, props.rule.operator)}
				onValueChange={(item) =>
					props.onChange({
						...props.rule,
						operator: chosenOf(item, operators, operators[0]).value,
					})
				}
			/>
			{needsValue(props.rule.operator) && values.length > 0 && (
				<SelectControl
					label={__('Value', 'gophenberg')}
					items={values}
					value={chosen(values, props.rule.value)}
					onValueChange={(item) =>
						props.onChange({ ...props.rule, value: chosenOf(item, values, values[0]).value })
					}
				/>
			)}
			{needsValue(props.rule.operator) && values.length === 0 && (
				<InputControl
					label={__('Value', 'gophenberg')}
					type={shapeOf(held.kind)}
					autoComplete="off"
					value={props.rule.value}
					onValueChange={(written) => props.onChange({ ...props.rule, value: written })}
				/>
			)}
			<Button variant="outline" onClick={props.onRemove}>
				{__('Remove', 'gophenberg')}
			</Button>
		</Stack>
	)
}

/**
 * Renders the rule sets a field is shown under and the controls editing them.
 * @param props - The rules held, the sources offered, and what to do.
 * @returns The body element.
 */
function ConditionsBody(props: {
	draft: ConditionRule[][]
	sources: ContentField[]
	onDraft: (next: ConditionRule[][]) => void
}) {
	return (
		<Stack direction="column" gap="md">
			{props.draft.length === 0 && (
				<Text>{__('Without a rule this field always shows.', 'gophenberg')}</Text>
			)}
			{props.draft.map((set, at) => (
				<Stack
					key={at}
					direction="column"
					gap="sm"
					role="group"
					aria-label={sprintf(__('Rule set %(number)d', 'gophenberg'), { number: at + 1 })}
				>
					{set.map((rule, spot) => (
						<ConditionRow
							key={spot}
							rule={rule}
							sources={props.sources}
							onChange={(next) =>
								props.onDraft(
									props.draft.map((held, index) =>
										index === at ? held.map((one, i) => (i === spot ? next : one)) : held,
									),
								)
							}
							onRemove={() =>
								props.onDraft(
									props.draft
										.map((held, index) =>
											index === at ? held.filter((_, i) => i !== spot) : held,
										)
										.filter((held) => held.length > 0),
								)
							}
						/>
					))}
					<Button
						variant="outline"
						onClick={() =>
							props.onDraft(
								props.draft.map((held, index) =>
									index === at ? [...held, blankRule(props.sources)] : held,
								),
							)
						}
					>
						{__('Add condition', 'gophenberg')}
					</Button>
				</Stack>
			))}
			<Button variant="outline" onClick={() => props.onDraft([...props.draft, [blankRule(props.sources)]])}>
				{__('Add rule set', 'gophenberg')}
			</Button>
		</Stack>
	)
}

/**
 * Renders the control editing the rules a field is shown under.
 * @param props - The group, the field, its siblings, its path, and what to report.
 * @returns The control and its dialog, or nothing when no sibling can be read.
 */
export function FieldConditions(props: {
	group: number
	field: ContentField
	siblings: ContentField[]
	path?: string
	onDone: (said: string) => Promise<void>
	onRefused: (cause: unknown) => Promise<void>
}) {
	const sources = conditionSources(props.siblings, props.field)
	const [open, setOpen] = useState(false)
	const [draft, setDraft] = useState<ConditionRule[][]>([])
	const [opened, setOpened] = useState(props.field)
	const asking = sprintf(__('Rules showing %(field)s', 'gophenberg'), { field: props.field.label })
	const save = useMutation({
		mutationFn: () =>
			setFieldSettingsInGroup(
				props.group,
				props.path ?? opened.key,
				storedConditions(opened.settings, draft),
				opened.updatedAt,
			),
		onSuccess: async () => {
			setOpen(false)
			await props.onDone(sprintf(__('%(field)s shows by its rules now.', 'gophenberg'), {
				field: props.field.label,
			}))
		},
		onError: async (cause) => {
			await props.onRefused(cause)
			setOpen(false)
		},
	})

	/**
	 * Opens the dialog on the stored rules, or closes it on the edits.
	 * @param next - Whether the dialog is opening.
	 */
	function change(next: boolean) {
		if (next) {
			setDraft(conditionsOf(props.field))
			setOpened(props.field)
		}
		setOpen(next)
	}

	if (sources.length === 0) {
		return null
	}
	return (
		<>
			<Button variant="outline" size="compact" aria-label={asking} onClick={() => change(true)}>
				{__('Rules', 'gophenberg')}
			</Button>
			<Dialog.Root open={open} onOpenChange={change}>
				<Dialog.Popup>
					<Dialog.Header>
						<Dialog.Title>{asking}</Dialog.Title>
						<Dialog.CloseIcon />
					</Dialog.Header>
					<Dialog.Content>
						<ConditionsBody draft={draft} sources={sources} onDraft={setDraft} />
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

/**
 * Returns the settings to store, carrying the rules beside whatever else the field holds.
 * @param held - The settings the field stands on.
 * @param draft - The rules the operator wrote.
 * @returns The settings to send.
 */
function storedConditions(held: Record<string, unknown>, draft: ConditionRule[][]): Record<string, unknown> {
	const settings = { ...held }
	delete settings.conditions
	if (draft.length === 0) {
		return settings
	}
	return { ...settings, conditions: draft }
}
