// SPDX-License-Identifier: Apache-2.0

import { Stack } from '@gophenberg/frontend-sdk'
import { __, _x } from '@wordpress/i18n'

import { conditionsOf, multipleOf, operatorsFor } from './conditions'
import { RuleSets } from './RuleSets'
import { pairsOf } from './types'
import type { ConditionRule } from './conditions'
import type { FieldDraft } from './fieldDraft'
import type { SourceChoices } from './RuleSets'
import type { Choice } from './select'
import type { ContentField } from './types'

/**
 * Returns the values a source offers to pick, none when it takes a written value.
 * @param source - The field a rule reads.
 * @returns The choices offered.
 */
function valuesOf(source: ContentField): Choice[] {
	if (source.kind === 'boolean') {
		return [
			{ value: 'true', label: __('Yes', 'gophenberg') },
			{ value: 'false', label: __('No', 'gophenberg') },
		]
	}
	return source.kind === 'choice' ? pairsOf(source.settings) : []
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
 * Returns the sibling fields a condition on the field may read, each with the conditions and values it offers.
 * @param siblings - The fields standing at the same depth.
 * @param field - The field the conditions belong to.
 * @returns The sources offered, none for a sibling of a kind no rule reads.
 */
export function conditionSources(siblings: ContentField[], field: ContentField): SourceChoices[] {
	return siblings
		.filter((held) => held.key !== field.key)
		.map((held) => ({
			value: held.key,
			label: held.label,
			operators: operatorsFor(held.kind, multipleOf(held)),
			values: valuesOf(held),
			typed: shapeOf(held.kind),
		}))
		.filter((source) => source.operators.length > 0)
}

/**
 * Returns the rules a draft shows its field under.
 * @param draft - The draft held.
 * @returns The rule sets, any one of which is enough.
 */
function draftedRules(draft: FieldDraft): ConditionRule[][] {
	return conditionsOf({ ...draft.base, settings: draft.settings })
}

/**
 * Returns the settings of a draft holding the rules given, the rules left out once none stands.
 * @param settings - The settings the draft holds.
 * @param rules - The rule sets the reader left.
 * @returns The settings.
 */
function ruledSettings(settings: Record<string, unknown>, rules: ConditionRule[][]): Record<string, unknown> {
	const held = { ...settings }
	delete held.conditions
	return rules.length === 0 ? held : { ...held, conditions: rules }
}

/**
 * Renders the name of the Conditional logic tab, with the Active dot while the draft holds a rule.
 * @param props - The draft held.
 * @returns The tab's name.
 */
export function ConditionsTabName(props: { draft: FieldDraft }) {
	return (
		<Stack direction="row" gap="sm" align="center" render={<span />}>
			{__('Conditional logic', 'gophenberg')}
			{draftedRules(props.draft).length > 0 && (
				<span className="gophenberg-conditions__active">{_x('Active', 'conditional logic', 'gophenberg')}</span>
			)}
		</Stack>
	)
}

/** What the rules of an open field show and edit. */
interface ConditionsProps {
	draft: FieldDraft
	/** The sibling fields a rule may read. */
	sources: SourceChoices[]
	readOnly: boolean
	onDraft: (next: FieldDraft) => void
}

/**
 * Renders the rules a field is shown under, read from its draft and written back into it.
 * @param props - The draft, the sources a rule may read, whether they are read only, and what changes the draft.
 * @returns The rule sets editor.
 */
export function FieldConditions(props: ConditionsProps) {
	const { draft } = props
	return (
		<RuleSets
			draft={draftedRules(draft)}
			sources={props.sources}
			empty={__('Without a rule this field always shows.', 'gophenberg')}
			readOnly={props.readOnly}
			onDraft={(rules) => props.onDraft({ ...draft, settings: ruledSettings(draft.settings, rules) })}
		/>
	)
}
