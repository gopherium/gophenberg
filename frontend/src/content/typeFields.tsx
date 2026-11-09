// SPDX-License-Identifier: Apache-2.0

import { Badge, Stack, VisuallyHidden } from '@gophenberg/frontend-sdk'
import type { Field } from '@gophenberg/frontend-sdk/dataviews'
import type { ViewGuard } from '@gopherium/godmin'
import { listDate } from '@gopherium/godmin/dates'
import { __, _x, sprintf } from '@wordpress/i18n'

import { pluralOf, singularOf, typeText } from './typeText'
import type { ContentType } from './types'

/** The order the list opens in, the newest type first, and the one it keeps for a sort it never offers. */
export const TYPE_SORT = { field: 'created', direction: 'desc' } as const

/** The order the list opens in, which every column check falls back to. */
export const TYPE_VIEW_GUARD: ViewGuard = { opening: TYPE_SORT }

/** The columns the table shows beside the type before a reader picks any. */
export const TYPE_COLUMNS = ['description', 'address', 'status']

/** The columns the phone list shows beside the type, the ones that read alone. */
export const TYPE_PHONE_COLUMNS = ['status']

/** The dash a column shows for a value a type leaves empty, as ACF's types list shows one. */
const EMPTY_MARK = '—'

/** One mark the Status column draws and its filter picks from. */
interface Mark {
	/** The value the filter holds for the mark. */
	value: string
	/** Whether a type carries the mark. */
	holds: (type: ContentType) => boolean
}

/** The marks a type may carry, in the order the Status column draws them. */
const MARKS: Mark[] = [
	{ value: 'default', holds: (type) => type.isDefault },
	{ value: 'nests', holds: (type) => type.hierarchical },
	{ value: 'closed', holds: (type) => !type.active },
]

/**
 * Returns the words of each mark, read fresh so the loaded catalogue answers.
 * @returns The words, keyed by the value of the mark.
 */
function markLabels(): Record<string, string> {
	return {
		default: __('Default', 'gophenberg'),
		nests: __('Nests', 'gophenberg'),
		closed: __('Closed', 'gophenberg'),
	}
}

/** The start of the filter value naming the plugin that declared a type. */
const PLUGIN_MARK = 'plugin:'

/**
 * Returns the address a type answers under, as the list shows it.
 * @param type - The type the address belongs to.
 * @returns The address, or the root marker.
 */
function addressOf(type: ContentType): string {
	return type.routeWord === '' ? '/' : `/${type.routeWord}`
}

/**
 * Returns the plugin that declared a type, empty when the site made it.
 * @param type - The type.
 * @returns The plugin's id, or an empty string.
 */
export function originOf(type: ContentType): string {
	return type.origin ?? ''
}

/**
 * Returns the plugins that declared the types given, each once and in order.
 * @param types - The types.
 * @returns The plugins' ids.
 */
export function originsOf(types: readonly ContentType[]): string[] {
	return [...new Set(types.map(originOf).filter((origin) => origin !== ''))].toSorted()
}

/**
 * Returns the words naming the plugin that declared a type.
 * @param plugin - The plugin's id.
 * @returns The words.
 */
function fromPlugin(plugin: string): string {
	return sprintf(__('From %(plugin)s', 'gophenberg'), { plugin })
}

/**
 * Returns the filter values of the marks a type carries, the plugin that declared it last.
 * @param type - The type.
 * @returns The values.
 */
function statusOf(type: ContentType): string[] {
	const marks = MARKS.filter((mark) => mark.holds(type)).map((mark) => mark.value)
	return [...marks, ...originsOf([type]).map((plugin) => PLUGIN_MARK + plugin)]
}

/**
 * Renders the badges naming what stands out about a type, each kept on one line.
 * @param props - The row being rendered.
 * @returns The badges.
 */
function StatusCell({ item }: { item: ContentType }) {
	const labels = markLabels()
	const origin = originOf(item)
	return (
		<Stack direction="row" gap="xs" wrap="wrap">
			{MARKS.filter((mark) => mark.holds(item)).map((mark) => (
				<Badge key={mark.value} className="gophenberg-status" intent={mark.value === 'closed' ? 'none' : undefined}>
					{labels[mark.value]}
				</Badge>
			))}
			{origin === '' ? null : <Badge className="gophenberg-status">{fromPlugin(origin)}</Badge>}
		</Stack>
	)
}

/**
 * Returns the description a type holds as the reader reads it.
 * @param type - The type.
 * @returns The shipped words in the reader's language while it holds them, else the stored description.
 */
function descriptionOf(type: ContentType): string {
	return typeText(type, 'description', type.description)
}

/**
 * Renders the description of a type, or a dash a screen reader reads as no description, as ACF lists its types.
 * @param props - The row being rendered.
 * @returns The description cell.
 */
function DescriptionCell({ item }: { item: ContentType }) {
	if (item.description.trim() === '') {
		return (
			<>
				<span aria-hidden="true">{EMPTY_MARK}</span>
				<VisuallyHidden>{__('No description', 'gophenberg')}</VisuallyHidden>
			</>
		)
	}
	return <span>{descriptionOf(item)}</span>
}

/**
 * Renders the moment a type was registered, as WordPress lists dates, or nothing for a type that carries none.
 * @param props - The row being rendered.
 * @returns The date cell.
 */
function CreatedCell({ item }: { item: ContentType }) {
	return item.createdAt === undefined ? null : <span>{listDate(item.createdAt)}</span>
}

/**
 * Renders the name of a type as text marked with its key.
 * @param props - The row being rendered.
 * @returns The plural name.
 */
function TitleCell({ item }: { item: ContentType }) {
	return <span data-item-id={item.key}>{pluralOf(item)}</span>
}

/**
 * Returns the columns of the types list, in the order the table draws them.
 * @param origins - The plugins that declared a type, which the Status filter offers after the marks.
 * @returns The type, its key, its singular name kept for the search alone, description, address, status and creation.
 */
export function typeFields(origins: readonly string[]): Field<ContentType>[] {
	return [
		{
			id: 'title',
			label: __('Type', 'gophenberg'),
			getValue: ({ item }) => pluralOf(item),
			render: TitleCell,
			enableHiding: false,
			enableGlobalSearch: true,
		},
		{ id: 'key', label: __('Key', 'gophenberg'), enableSorting: false, enableGlobalSearch: true },
		{
			id: 'singular',
			label: __('Singular name', 'gophenberg'),
			getValue: ({ item }) => singularOf(item),
			enableHiding: false,
			enableSorting: false,
			enableGlobalSearch: true,
			filterBy: false,
		},
		{
			id: 'description',
			label: __('Description', 'gophenberg'),
			getValue: ({ item }) => descriptionOf(item),
			render: DescriptionCell,
			enableSorting: false,
			enableGlobalSearch: true,
		},
		{ id: 'address', label: __('Address', 'gophenberg'), getValue: ({ item }) => addressOf(item) },
		{
			id: 'status',
			label: _x('Status', 'content type', 'gophenberg'),
			type: 'array',
			getValue: ({ item }) => statusOf(item),
			render: StatusCell,
			elements: [
				...Object.entries(markLabels()).map(([value, label]) => ({ value, label })),
				...origins.map((plugin) => ({ value: PLUGIN_MARK + plugin, label: fromPlugin(plugin) })),
			],
			filterBy: { operators: ['isAny', 'isNone'] },
			enableSorting: false,
		},
		{
			id: 'created',
			label: __('Created', 'gophenberg'),
			type: 'datetime',
			getValue: ({ item }) => item.createdAt,
			render: CreatedCell,
			filterBy: false,
		},
	]
}
