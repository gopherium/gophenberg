// SPDX-License-Identifier: Apache-2.0

import { __ } from '@wordpress/i18n'

import { backlinkSettings, backlinkSources, pickOf } from './groups'
import type { BacklinkPick, FieldGroup, GroupEdit, Location } from './groups'
import type { ContentField } from './types'

/** A Linked from source picked in the details, with the source and the stamp its field held at the first pick. */
interface LinkPick extends BacklinkPick {
	was: BacklinkPick | null
	updatedAt: string
}

/** The parts of a group its details edit, as the reader leaves them. */
export interface GroupDraft {
	title: string
	active: boolean
	location: Location
	links: Record<string, LinkPick>
}

/** One part of a group a draft may change, a Linked from pick named by its field. */
export type DraftPart = 'title' | 'active' | 'location' | `link:${string}`

/** A draft moved onto the version saved elsewhere, with the parts where the saved value won over the draft. */
export interface Rebased {
	draft: GroupDraft
	lost: DraftPart[]
}

/**
 * Returns the draft a group's details start from, the group as it was read with no Linked from source picked.
 * @param group - The group as it was read.
 * @returns The draft.
 */
export function draftOf(group: FieldGroup): GroupDraft {
	return { title: group.title, active: group.active, location: group.location, links: {} }
}

/**
 * Returns the part a Linked from pick goes under.
 * @param key - The key of the Linked from field.
 * @returns The part name.
 */
export function linkPart(key: string): DraftPart {
	return `link:${key}`
}

/**
 * Returns the sentence saying a part was changed by someone else too, so the draft took the saved value.
 * @returns The sentence to show under the part.
 */
export function changedElsewhere(): string {
	return __('Changed elsewhere too. Your edit was not kept.', 'gophenberg')
}

/**
 * Reports whether two picks name the same group and relation.
 * @param one - The first pick, none for a source the pickers cannot point.
 * @param other - The second pick, none for a source the pickers cannot point.
 * @returns Whether they match.
 */
function samePick(one: BacklinkPick | null, other: BacklinkPick | null): boolean {
	return one?.group === other?.group && one?.field === other?.field
}

/**
 * Reports whether two locations hold the same rules in the same order.
 * @param one - The first location.
 * @param other - The second location.
 * @returns Whether they match.
 */
function sameLocation(one: Location, other: Location): boolean {
	return JSON.stringify(one) === JSON.stringify(other)
}

/**
 * Reports whether a pick points its field somewhere other than the field read when it was first picked.
 * @param pick - The pick held.
 * @returns Whether the pick moves the source.
 */
function moves(pick: LinkPick): boolean {
	return !samePick(pick, pick.was)
}

/**
 * Returns the parts a draft holds that differ from the group it started from.
 * @param base - The group the draft started from.
 * @param draft - The draft held.
 * @returns The changed parts, none once every change is undone.
 */
export function changedParts(base: FieldGroup, draft: GroupDraft): DraftPart[] {
	const parts: DraftPart[] = []
	if (draft.title !== base.title) {
		parts.push('title')
	}
	if (draft.active !== base.active) {
		parts.push('active')
	}
	if (!sameLocation(draft.location, base.location)) {
		parts.push('location')
	}
	for (const [key, pick] of Object.entries(draft.links)) {
		if (moves(pick)) {
			parts.push(linkPart(key))
		}
	}
	return parts
}

/**
 * Returns the draft holding a pick, its field's source and stamp kept from the first pick of that field.
 * @param draft - The draft held.
 * @param field - The Linked from field as the details show it.
 * @param pick - The group and the relation picked.
 * @returns The draft to hold.
 */
export function withPick(draft: GroupDraft, field: ContentField, pick: BacklinkPick): GroupDraft {
	const first = draft.links[field.key] ?? { was: pickOf(field), updatedAt: field.updatedAt }
	const held: LinkPick = { group: pick.group, field: pick.field, was: first.was, updatedAt: first.updatedAt }
	return { ...draft, links: { ...draft.links, [field.key]: held } }
}

/**
 * Returns the Linked from field of a group under a key.
 * @param group - The group to look in.
 * @param key - The key of the field.
 * @returns The field, or nothing when the group holds no Linked from field under the key.
 */
function linkedField(group: FieldGroup, key: string): ContentField | undefined {
	return group.fields.find((field) => field.key === key && field.kind === 'backlinks')
}

/**
 * Returns the draft holding only the picks whose field the group as last read still holds as a Linked from field.
 * @param draft - The draft held.
 * @param live - The group as last read.
 * @returns The draft with every pick of a field gone from the group dropped.
 */
export function withLivePicks(draft: GroupDraft, live: FieldGroup): GroupDraft {
	const links = Object.entries(draft.links).filter(([key]) => linkedField(live, key) !== undefined)
	return { ...draft, links: Object.fromEntries(links) }
}

/**
 * Returns the pick a draft holds for a Linked from field while it points the field somewhere new.
 * @param draft - The draft held.
 * @param key - The key of the Linked from field.
 * @returns The pick, or nothing when the draft leaves the field reading what it read.
 */
function movingPick(draft: GroupDraft, key: string): LinkPick | null {
	const pick = draft.links[key]
	return pick === undefined || !moves(pick) ? null : pick
}

/**
 * Returns the parts one draft holds otherwise than another.
 * @param one - The first draft.
 * @param other - The second draft.
 * @returns The parts that differ, none when both drafts hold the same.
 */
export function editedParts(one: GroupDraft, other: GroupDraft): DraftPart[] {
	const parts: DraftPart[] = []
	if (one.title !== other.title) {
		parts.push('title')
	}
	if (one.active !== other.active) {
		parts.push('active')
	}
	if (!sameLocation(one.location, other.location)) {
		parts.push('location')
	}
	for (const key of new Set([...Object.keys(one.links), ...Object.keys(other.links)])) {
		if (!samePick(movingPick(one, key), movingPick(other, key))) {
			parts.push(linkPart(key))
		}
	}
	return parts
}

/**
 * Returns the stamp a pick sends: the newest its field was read with while its source reads as at the pick.
 * @param live - The group as last read.
 * @param key - The key of the Linked from field.
 * @param pick - The pick held.
 * @returns The field's newest stamp, or the stamp of the pick once someone pointed the field elsewhere.
 */
function linkStamp(live: FieldGroup, key: string, pick: LinkPick): string {
	const field = linkedField(live, key)
	if (field === undefined || !samePick(pickOf(field), pick.was)) {
		return pick.updatedAt
	}
	return field.updatedAt
}

/**
 * Returns the group edit naming only the changed parts of a draft, each Linked from pick with the stamp it sends.
 * @param draft - The draft held.
 * @param parts - The parts the draft changed.
 * @param live - The group as last read.
 * @param groups - The groups a relation may stand in.
 * @returns The edit to send.
 */
export function editOf(draft: GroupDraft, parts: DraftPart[], live: FieldGroup, groups: FieldGroup[]): GroupEdit {
	const named = new Set(parts)
	const links = Object.entries(draft.links).filter(([key]) => named.has(linkPart(key)))
	return {
		title: named.has('title') ? draft.title : undefined,
		active: named.has('active') ? draft.active : undefined,
		location: named.has('location') ? draft.location : undefined,
		backlinks:
			links.length === 0
				? undefined
				: Object.fromEntries(
					links.map(([key, pick]) => [
						key,
						{
							...backlinkSettings(backlinkSources(groups, pick.group, pick.field)),
							updated_at: linkStamp(live, key, pick),
						},
					]),
				),
	}
}

/** One part of a group as the draft started from it, as the draft holds it and as it was saved elsewhere. */
interface Sides<T> {
	base: T
	mine: T
	theirs: T
}

/**
 * Returns the value a part takes once the draft moves onto the saved version, naming the part when the saved value won.
 * @param part - The name of the part.
 * @param sides - The part as the draft started from it, as the draft holds it and as it was saved.
 * @param same - What tells two values of the part apart.
 * @param lost - The parts named so far, where a part the saved value won over goes.
 * @returns The value the draft holds.
 */
function settled<T>(part: DraftPart, sides: Sides<T>, same: (one: T, other: T) => boolean, lost: DraftPart[]): T {
	if (same(sides.mine, sides.base)) {
		return sides.theirs
	}
	if (same(sides.theirs, sides.base) || same(sides.theirs, sides.mine)) {
		return sides.mine
	}
	lost.push(part)
	return sides.theirs
}

/**
 * Returns the Linked from field a group holds under a key with the source it reads, while the pickers can point it.
 * @param group - The group as saved elsewhere.
 * @param key - The key of the Linked from field.
 * @returns The field and its source, or nothing when the group holds no such field or the pickers cannot point it.
 */
function savedLink(group: FieldGroup, key: string): { field: ContentField; source: BacklinkPick } | null {
	const field = linkedField(group, key)
	if (field === undefined) {
		return null
	}
	const source = pickOf(field)
	return source === null ? null : { field, source }
}

/**
 * Returns the picks of a draft moved onto the saved version, each kept pick on the source and stamp saved.
 * @param links - The picks the draft holds.
 * @param saved - The group as saved elsewhere.
 * @param lost - The parts named so far, where a pick the saved source won over goes.
 * @returns The picks the draft keeps.
 */
function rebasedLinks(links: Record<string, LinkPick>, saved: FieldGroup, lost: DraftPart[]): Record<string, LinkPick> {
	const kept: Record<string, LinkPick> = {}
	const moving = Object.entries(links).filter(([, pick]) => moves(pick))
	for (const [key, pick] of moving) {
		const theirs = savedLink(saved, key)
		if (theirs === null) {
			continue
		}
		if (samePick(theirs.source, pick.was) || samePick(theirs.source, pick)) {
			kept[key] = { group: pick.group, field: pick.field, was: theirs.source, updatedAt: theirs.field.updatedAt }
		} else {
			lost.push(linkPart(key))
		}
	}
	return kept
}

/**
 * Returns the draft moved onto the version saved elsewhere, with the parts where the saved value won.
 * @param base - The group the draft started from.
 * @param draft - The draft held.
 * @param saved - The group as saved elsewhere.
 * @returns The draft standing on the saved version, and the parts where the saved value won.
 */
export function rebased(base: FieldGroup, draft: GroupDraft, saved: FieldGroup): Rebased {
	const lost: DraftPart[] = []
	const plain = (one: unknown, other: unknown) => one === other
	const moved: GroupDraft = {
		title: settled('title', { base: base.title, mine: draft.title, theirs: saved.title }, plain, lost),
		active: settled('active', { base: base.active, mine: draft.active, theirs: saved.active }, plain, lost),
		location: settled(
			'location',
			{ base: base.location, mine: draft.location, theirs: saved.location },
			sameLocation,
			lost,
		),
		links: rebasedLinks(draft.links, saved, lost),
	}
	return { draft: moved, lost }
}
