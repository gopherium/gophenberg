// SPDX-License-Identifier: Apache-2.0

import { Stack, Text } from '@gophenberg/frontend-sdk'
import { ErrorNotice } from '@gopherium/godmin'
import { __ } from '@wordpress/i18n'

import { changedElsewhere, linkPart } from './groupDraft'
import { backlinkSources, pickOf, typeSource } from './groups'
import { RuleSets } from './RuleSets'
import { SourceOfLinks } from './SourceOfLinks'
import { pluralOf } from './typeText'
import { ownValue } from './types'
import type { DraftPart } from './groupDraft'
import type { BacklinkPick, FieldGroup, Location, RuleSource } from './groups'
import type { SourceChoices } from './RuleSets'
import type { Choice } from './select'
import type { ContentField, ContentType } from './types'

/**
 * Returns the name a rule source is offered under.
 * @param source - The source as the server names it.
 * @returns The name to show, the source itself when the admin has no name for it.
 */
function sourceLabel(source: string): string {
	return source === typeSource ? __('Content type', 'gophenberg') : source
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
 * Returns the sources the server offers under their names, each type among the values named as the reader reads it.
 * @param sources - The sources the server offers.
 * @param types - The registered types, each naming the plugin that declared it.
 * @returns The sources the rules read.
 */
function locationSources(sources: RuleSource[], types: ContentType[]): SourceChoices[] {
	return sources.map((offered) => ({
		value: offered.source,
		label: sourceLabel(offered.source),
		operators: offered.operators,
		values:
			offered.source === typeSource ? offered.values.map((value) => namedType(value, types)) : offered.values,
	}))
}

/** What the location rules of a group take. */
interface LocationRulesProps {
	draft: Location
	/** The sources the server offers. */
	sources: RuleSource[]
	types: ContentType[]
	/** Whether the sources could not be read. */
	failed: boolean
	readOnly: boolean
	onDraft: (next: Location) => void
}

/**
 * Renders the rule sets placing a group, read from the sources the server offers.
 * @param props - The rules held, the sources, the types, whether the sources failed or the rules are read only, and
 * what to call with an edit.
 * @returns The rules element, or the notice that the sources could not be read.
 */
export function LocationRules(props: LocationRulesProps) {
	if (props.failed) {
		return <ErrorNotice>{__('The rules could not be loaded.', 'gophenberg')}</ErrorNotice>
	}
	return (
		<RuleSets
			draft={props.draft}
			sources={locationSources(props.sources, props.types)}
			empty={__('Without a rule this group appears nowhere.', 'gophenberg')}
			readOnly={props.readOnly}
			onDraft={props.onDraft}
		/>
	)
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
				const pick = ownValue(props.picked, field.key) ?? pickOf(field)
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
