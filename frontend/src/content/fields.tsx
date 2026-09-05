// SPDX-License-Identifier: Apache-2.0

import { Badge, Stack, Text } from '@gophenberg/frontend-sdk'
import type { Field } from '@gophenberg/frontend-sdk/dataviews'
import { __, _x } from '@wordpress/i18n'
import { Link } from '@tanstack/react-router'

import { formatDate } from '@gopherium/gottext'
import type { Post } from './api'
import { pairsOf } from './types'
import type { ContentField, ContentType } from './types'

/** What a column id carries before the key of the field it shows. */
export const fieldColumnPrefix = 'field.'

/**
 * Returns the label shown beside the title of a post that is not published.
 * @param status - The status the post holds.
 * @returns The label, or nothing when the status needs no marking.
 */
function stateLabel(status: string): string | null {
	switch (status) {
		case 'draft':
			return _x('Draft', 'post status', 'gophenberg')
		case 'pending':
			return __('Pending', 'gophenberg')
		case 'private':
			return __('Private', 'gophenberg')
		case 'scheduled':
			return __('Scheduled', 'gophenberg')
		case 'trash':
			return _x('Trash', 'post status', 'gophenberg')
		default:
			return null
	}
}

/**
 * Returns the date a listing shows for a post, and what that date means.
 * @param post - The post to describe.
 * @returns The caption and the date to show.
 */
function dateLine(post: Post): { caption: string, at: string } {
	return post.publishedAt !== null
		? { caption: _x('Published', 'date caption', 'gophenberg'), at: post.publishedAt }
		: { caption: __('Last Modified', 'gophenberg'), at: post.updatedAt }
}

/**
 * Renders the title of a post as a link to its editor.
 * @param props - The row being rendered.
 * @returns The title cell.
 */
function TitleCell({ item }: { item: Post }) {
	const state = stateLabel(item.status)
	return (
		<Stack direction="row" gap="xs" align="center">
			<Link to="/content/$typeKey/$postId/edit" params={{ typeKey: item.type, postId: item.id }}>
				{item.title === '' ? __('(no title)', 'gophenberg') : item.title}
			</Link>
			{state !== null && <Badge>{state}</Badge>}
		</Stack>
	)
}

/**
 * Renders the author of a post.
 * @param props - The row being rendered.
 * @returns The author cell.
 */
function AuthorCell({ item }: { item: Post }) {
	return <Text>{item.authorName}</Text>
}

/**
 * Renders the date of a post with the caption explaining it.
 * @param props - The row being rendered.
 * @returns The date cell.
 */
function DateCell({ item }: { item: Post }) {
	const { caption, at } = dateLine(item)
	return (
		<Text>
			{caption} {formatDate(at)}
		</Text>
	)
}

const builtInFields: Field<Post>[] = [
	{
		id: 'title',
		label: __('Title', 'gophenberg'),
		render: TitleCell,
		enableSorting: true,
		enableHiding: false,
	},
	{ id: 'author', label: __('Author', 'gophenberg'), render: AuthorCell, enableSorting: false },
	{ id: 'date', label: _x('Date', 'column', 'gophenberg'), render: DateCell, enableSorting: true },
]

/**
 * Returns the value a listed field holds on a post, as the column shows it.
 * @param declared - The field the column stands for.
 * @param held - The value the post holds under it.
 * @returns The text of the cell.
 */
function shownValue(declared: ContentField, held: unknown): string {
	if (held === undefined || held === null) {
		return ''
	}
	if (declared.kind === 'boolean') {
		return held === true ? __('Yes', 'gophenberg') : __('No', 'gophenberg')
	}
	if (declared.kind === 'date' && typeof held === 'string') {
		return formatDate(held)
	}
	if (declared.kind === 'choice') {
		return chosenLabels(declared, held)
	}
	return String(held)
}

/**
 * Returns the labels a choice field gives the values a post holds.
 * @param declared - The choice field the column stands for.
 * @param held - The value or values the post holds.
 * @returns The labels, joined by a comma when the field holds several.
 */
function chosenLabels(declared: ContentField, held: unknown): string {
	const pairs = pairsOf(declared.settings)
	const chosen = Array.isArray(held) ? held : [held]
	return chosen
		.map((one) => pairs.find((pair) => pair.value === one)?.label ?? String(one))
		.join(', ')
}

/**
 * Returns the elements a column offers as a filter, none for a kind that offers no chip.
 * @param declared - The field the column stands for.
 * @returns The elements, or nothing when the kind offers no chip.
 */
function filterElements(declared: ContentField) {
	if (declared.kind === 'boolean') {
		return [
			{ value: 'true', label: __('Yes', 'gophenberg') },
			{ value: 'false', label: __('No', 'gophenberg') },
		]
	}
	if (declared.kind === 'choice') {
		const pairs = pairsOf(declared.settings)
		return pairs.length > 0 ? pairs : undefined
	}
	return undefined
}

/**
 * Returns the column showing what a listed field holds.
 * @param declared - The field the column stands for.
 * @returns The column.
 */
function fieldColumn(declared: ContentField): Field<Post> {
	const elements = filterElements(declared)
	return {
		id: `${fieldColumnPrefix}${declared.key}`,
		label: declared.label,
		enableSorting: false,
		getValue: ({ item }: { item: Post }) => shownValue(declared, item.fields?.[declared.key]),
		...(elements === undefined ? {} : { elements, filterBy: { operators: ['is' as const], isPrimary: true } }),
	}
}

/**
 * Returns the terms a view's chips narrow the listing by, keyed by the field each names.
 * @param filters - The filters the view carries.
 * @returns The terms, keyed by field key.
 */
export function fieldTerms(
	filters: { field: string, value: unknown }[] | undefined,
): Record<string, string> {
	const terms: Record<string, string> = {}
	for (const filter of filters ?? []) {
		if (filter.field.startsWith(fieldColumnPrefix) && filter.value !== undefined) {
			terms[filter.field.slice(fieldColumnPrefix.length)] = String(filter.value)
		}
	}
	return terms
}

/**
 * Returns the columns a content type's listing shows, one per field it marks for the list.
 * @param listed - The type being listed.
 * @returns The columns.
 */
export function postFields(listed: ContentType): Field<Post>[] {
	const marked = listed.fields.filter((declared) => declared.settings.listed === true)
	return [...builtInFields, ...marked.map(fieldColumn)]
}
