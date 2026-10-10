// SPDX-License-Identifier: Apache-2.0

import { Badge } from '@gophenberg/frontend-sdk'
import type { Field, Filter, View } from '@gophenberg/frontend-sdk/dataviews'
import { InitialsAvatar } from '@gopherium/godmin'
import { formatDate } from '@gopherium/gottext'
import { dateI18n, getDate, getSettings } from '@wordpress/date'
import { __, _x, sprintf } from '@wordpress/i18n'
import type { ComponentProps } from 'react'

import { everyDecimal } from '../i18n/numbers'
import type { Post, PostQuery } from './api'
import { authorElements } from './authors'
import { pairsOf } from './types'
import type { ContentField, ContentType } from './types'

/** What a column id carries before the key of the field it shows. */
const fieldColumnPrefix = 'field.'

/** The order a list opens in, and the one it keeps when the address names a column it never sorts by. */
export const OPENING_SORT = { field: 'date', direction: 'desc' } as const

/** The colour a status badge is drawn in. */
type BadgeIntent = ComponentProps<typeof Badge>['intent']

/**
 * Returns the words and the colour of the badge each status is drawn with, read fresh so the loaded catalogue answers.
 * @returns The badge of each status, keyed by the status.
 */
function statusBadges(): Record<string, { label: string, intent: BadgeIntent }> {
	return {
		draft: { label: _x('Draft', 'post status', 'gophenberg'), intent: 'low' },
		scheduled: { label: __('Scheduled', 'gophenberg'), intent: 'informational' },
		pending: { label: __('Pending Review', 'gophenberg'), intent: 'informational' },
		private: { label: __('Private', 'gophenberg'), intent: 'draft' },
		published: { label: _x('Published', 'post status', 'gophenberg'), intent: 'stable' },
		trash: { label: _x('Trash', 'post status', 'gophenberg'), intent: 'none' },
	}
}

/**
 * Renders the status of a post as a badge kept on one line.
 * @param props - The row being rendered.
 * @returns The status cell.
 */
function StatusCell({ item }: { item: Post }) {
	const badge = statusBadges()[item.status]
	return (
		<Badge className="gophenberg-status" intent={badge.intent}>
			{badge.label}
		</Badge>
	)
}

/**
 * Returns the words a date cell reads, a caption per status before the moment and none in the trash.
 * @param status - The status the post holds.
 * @param when - The moment as the list writes it.
 * @returns The words of the cell.
 */
function datedAs(status: string, when: string): string {
	switch (status) {
		case 'published':
			return sprintf(__('Published: %s', 'gophenberg'), when)
		case 'scheduled':
			return sprintf(__('Scheduled: %s', 'gophenberg'), when)
		case 'trash':
			return when
		default:
			return sprintf(__('Modified: %s', 'gophenberg'), when)
	}
}

/**
 * Renders the moment a list dates a post by, abbreviated as WordPress lists dates, with the caption explaining it.
 * @param props - The row being rendered.
 * @returns The date cell as plain text kept on one line, or nothing for a post that carries no moment.
 */
function DateCell({ item }: { item: Post }) {
	if (item.date === '') {
		return null
	}
	const written = dateI18n(getSettings().formats.datetimeAbbreviated, getDate(item.date))
	return <span>{datedAs(item.status, written)}</span>
}

/**
 * Renders the author of a post, its initials before its name as WordPress draws an author.
 * @param props - The row being rendered.
 * @returns The author cell.
 */
function AuthorCell({ item }: { item: Post }) {
	return (
		<span className="gophenberg-author">
			<span className="gophenberg-author__avatar">
				<InitialsAvatar name={item.authorName} size={16} />
			</span>
			<span>{item.authorName}</span>
		</span>
	)
}

/**
 * Returns the columns every content type's list offers, in the order WordPress lists them.
 * @returns The title, excerpt, author, status, date and slug columns.
 */
function builtInFields(): Field<Post>[] {
	return [
		{
			id: 'title',
			label: __('Title', 'gophenberg'),
			getValue: ({ item }) => (item.title === '' ? __('(no title)', 'gophenberg') : item.title),
			enableSorting: true,
			enableHiding: false,
		},
		{ id: 'excerpt', label: __('Excerpt', 'gophenberg'), enableSorting: false },
		{
			id: 'author',
			label: __('Author', 'gophenberg'),
			render: AuthorCell,
			getElements: authorElements,
			filterBy: { operators: ['isAny', 'isNone'] },
		},
		{ id: 'status', label: _x('Status', 'post', 'gophenberg'), render: StatusCell, enableSorting: false },
		{
			id: 'date',
			label: _x('Date', 'column', 'gophenberg'),
			type: 'datetime',
			render: DateCell,
			format: { datetime: getSettings().formats.datetime, weekStartsOn: getSettings().l10n.startOfWeek },
			filterBy: { operators: ['before', 'after'] },
		},
		{ id: 'slug', label: __('Slug', 'gophenberg') },
	]
}

/**
 * Returns the column naming the parent of each item of a type that nests.
 * @returns The parent column.
 */
function parentColumn(): Field<Post> {
	return {
		id: 'parent',
		label: __('Parent', 'gophenberg'),
		getValue: ({ item }) => (item.parentTitle === '' ? __('None', 'gophenberg') : item.parentTitle),
	}
}

/**
 * Returns how deep an item of a type sits in its tree, a top level item 0.
 * @param listed - The type the list shows.
 * @returns The reader of an item's level, from the slashes of its path under the type's route word.
 */
export function levelOf(listed: ContentType): (post: Post) => number {
	const under = listed.routeWord === '' ? 0 : 1
	return (post) => post.path.split('/').length - 1 - under
}

/** The columns a list shows by default on a phone, which read alone. */
export const PHONE_COLUMNS = ['status', 'date']

/**
 * Returns the ids of the columns a list shows before a reader picks any, the fields a type marks for the list last.
 * @param columns - The columns the list offers.
 * @returns The author, the status, the date and every listed field.
 */
export function openingColumns(columns: Field<Post>[]): string[] {
	const listed = columns.filter((column) => column.id.startsWith(fieldColumnPrefix)).map((column) => column.id)
	return ['author', 'status', 'date', ...listed]
}

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
	const write = valueWriters[declared.kind]
	return write === undefined ? String(held) : write(held, declared)
}

/**
 * Returns the word a switch is read under.
 * @param held - The value the post holds.
 * @returns Yes when the switch is on, No otherwise.
 */
function switchWord(held: unknown): string {
	return held === true ? __('Yes', 'gophenberg') : __('No', 'gophenberg')
}

/**
 * Returns a day as the site writes it, or the value as it stands when it is not a day.
 * @param held - The value the post holds.
 * @returns The day, or the value.
 */
function writtenDay(held: unknown): string {
	return typeof held === 'string' ? formatDate(held) : String(held)
}

/**
 * Returns a number as the site writes it, or the value as it stands when it is not a number.
 * @param held - The value the post holds.
 * @returns The number, or the value.
 */
function writtenNumber(held: unknown): string {
	return typeof held === 'number' ? everyDecimal(held) : String(held)
}

/**
 * Returns the words a link is read under, its address when it carries none.
 * @param held - The value the post holds.
 * @returns The title, or the address.
 */
function linkTitle(held: unknown): string {
	const link = held as { url: string; title: string }
	return link.title === '' ? link.url : link.title
}

/**
 * Returns the labels a choice field gives the values a post holds.
 * @param held - The value or values the post holds.
 * @param declared - The choice field the column stands for.
 * @returns The labels, joined by a comma when the field holds several.
 */
function chosenLabels(held: unknown, declared: ContentField): string {
	const pairs = pairsOf(declared.settings)
	const chosen = Array.isArray(held) ? held : [held]
	return chosen
		.map((one) => pairs.find((pair) => pair.value === one)?.label ?? String(one))
		.join(', ')
}

/** The writer of a listed value, keyed by the kind of field holding it. */
const valueWriters: Record<string, ((held: unknown, declared: ContentField) => string) | undefined> = {
	boolean: switchWord,
	date: writtenDay,
	number: writtenNumber,
	choice: chosenLabels,
	link: linkTitle,
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
		...(elements === undefined ? {} : { elements, filterBy: { operators: ['is' as const] } }),
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

/** The shape of an account id. */
const ACCOUNT_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

/**
 * Reports whether a filter value names accounts, each by an id in the shape account ids take.
 * @param value - The value the filter holds.
 * @returns Whether the value is a list of account ids.
 */
function namesAccounts(value: unknown): boolean {
	return Array.isArray(value) && value.every((id) => ACCOUNT_ID.test(String(id)))
}

/**
 * Reports whether a filter value is a moment the server reads, one falling in a year of four digits.
 * @param value - The value the filter holds.
 * @returns Whether the value is text a date can be read from, in the years 0 to 9999.
 */
function isMoment(value: unknown): boolean {
	const year = typeof value === 'string' ? new Date(value).getUTCFullYear() : Number.NaN
	return year >= 0 && year <= 9999
}

/** How each filtering column that offers no elements of its own checks the value a filter holds. */
const valueChecks: Record<string, (value: unknown) => boolean> = { author: namesAccounts, date: isMoment }

/** The part of a listing request a filter of each column narrows it by, keyed by the column, none for a field. */
const narrowings: Record<string, ((filter: Filter) => PostQuery) | undefined> = {
	author: (filter) => {
		const ids = filter.value as string[]
		return filter.operator === 'isAny' ? { author: ids } : { authorExclude: ids }
	},
	date: (filter) => {
		const at = new Date(String(filter.value)).toISOString()
		return filter.operator === 'before' ? { before: at } : { after: at }
	},
}

/** A view as the columns of its list can draw it, every filter it holds one the list offers. */
export type OfferedView = View & { filters: Filter[] }

/**
 * Returns the part of a listing request the filters of a view narrow it by.
 * @param filters - The filters the view carries, each one the list offers.
 * @returns The field terms, the authors kept or left out and the dates the filters name.
 */
export function narrowedBy(filters: Filter[]): PostQuery {
	let narrowed: PostQuery = { fields: fieldTerms(filters) }
	for (const filter of filters) {
		const narrow = narrowings[filter.field]
		if (narrow !== undefined && filter.value !== undefined) {
			narrowed = { ...narrowed, ...narrow(filter) }
		}
	}
	return narrowed
}

/**
 * Reports whether a column takes the value a filter holds.
 * @param column - The column the filter narrows by.
 * @param value - The value the filter holds.
 * @returns Whether the value is one of the column's elements, or passes the check of a column offering none.
 */
function takes(column: Field<Post>, value: unknown): boolean {
	if (column.elements !== undefined) {
		return column.elements.some((element) => element.value === value)
	}
	return valueChecks[column.id](value)
}

/**
 * Reports whether a filter names a column the list filters by, with an operator and a value that column offers.
 * @param filter - The filter the view carries.
 * @param columns - The columns the list shows.
 * @returns Whether the list can apply the filter, one still waiting for its value included.
 */
function offers(filter: Filter, columns: Field<Post>[]): boolean {
	const column = columns.find((held) => held.id === filter.field)
	if (column === undefined) {
		return false
	}
	const operators: string[] = (column.filterBy || {}).operators ?? []
	return operators.includes(filter.operator) && (filter.value === undefined || takes(column, filter.value))
}

/**
 * Returns the view as the columns can draw it, a sort or a filter naming what the list never offers dropped.
 * @param view - The view the address and the reader hold.
 * @param columns - The columns the list shows.
 * @returns The view the list shows and asks the server for.
 */
export function offeredView(view: View, columns: Field<Post>[]): OfferedView {
	const sorted = columns.some((column) => column.id === view.sort?.field && column.enableSorting !== false)
	return {
		...view,
		sort: sorted ? view.sort : { ...OPENING_SORT },
		filters: (view.filters ?? []).filter((filter) => offers(filter, columns)),
	}
}

/**
 * Returns the columns a content type's listing shows, one per field it marks for the list.
 * @param listed - The type being listed.
 * @returns The columns.
 */
export function postFields(listed: ContentType): Field<Post>[] {
	const marked = listed.fields.filter((declared) => declared.settings.listed === true)
	const nested = listed.hierarchical ? [parentColumn()] : []
	return [...builtInFields(), ...nested, ...marked.map(fieldColumn)]
}
