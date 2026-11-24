// SPDX-License-Identifier: Apache-2.0

import { Badge, Collapsible, Icon, Stack, VisuallyHidden, downIcon, listIcon } from '@gophenberg/frontend-sdk'
import { ErrorNotice, ListEmpty, RowControls, SectionTitle, useToaster } from '@gopherium/godmin'
import { __, _x, sprintf } from '@wordpress/i18n'
import { useQueryClient } from '@tanstack/react-query'
import { useLayoutEffect, useRef, useState } from 'react'
import type { ReactNode, RefObject } from 'react'

import { groupErrorMessage, groupsQueryKey, reorderFieldsInGroup, reorderSubFields, StaleWriteError } from './groups'
import { AddField } from './AddField'
import { DeleteConfirm, deleteName } from './DeleteField'
import { holdsDraft } from './fieldDraft'
import { CarryField } from './MoveField'
import { OpenField } from './OpenField'
import { pickedLabel } from './types'
import { useFieldOrder } from './useFieldOrder'
import { typesQueryKey } from './nav'
import type { FieldWork, FieldWrite, Reporter } from './fieldControls'
import type { FieldGroup } from './groups'
import type { ContentField, ContentType } from './types'

/** Where the fields of a group stand: the group, every group, the types, and the keys an earlier group serves. */
interface FieldsPlace {
	held: FieldGroup
	groups: FieldGroup[]
	types: ContentType[]
	shadowed: Set<string>
}

/** Where the fields of a group stand and what the page holds for the fields it edits in place. */
interface FieldsWork extends FieldsPlace {
	work: FieldWork
}

/** The columns of the fields table: Label, Kind, API name and the actions. */
const COLUMNS = 4

/** Finds the row of each field one table holds, the first row of each of its row groups. */
const FIELD_ROWS = ':scope > table > tbody > tr:first-child'

/** Finds the trash among the controls of a field's row. */
const TRASH = '.godmin-rows__remove'

/** Finds the region godmin's toasts show in. */
const TOASTS = '.godmin-toasts'

/**
 * Returns the toast naming a field write that failed, its reason left to fill in.
 * @param write - The kind of write that failed.
 * @returns The toast before the reason is written in.
 */
function failedWrite(write: FieldWrite) {
	const sentences = {
		save: __('Could not save the field. %s', 'gophenberg'),
		add: __('Could not add the field. %s', 'gophenberg'),
		order: __('Could not order the fields. %s', 'gophenberg'),
		delete: __('Could not delete the field. %s', 'gophenberg'),
	}
	return sentences[write]
}

/**
 * Renders the fields a group holds, the controls managing them and the notice naming the last failed write.
 * @param props - The group, the other groups, the types, the keys an earlier group serves in its place, and what the
 * page holds for the fields it edits in place.
 * @returns The fields element.
 */
export function GroupFields(props: FieldsWork) {
	const client = useQueryClient()
	const toaster = useToaster()
	const [notice, setNotice] = useState('')

	/**
	 * Reports what a field write did in a toast, when it names it, and refreshes what the screen holds.
	 * @param said - The sentence naming what was done, none for a write that shows no toast.
	 */
	async function done(said?: string) {
		setNotice('')
		if (said !== undefined) {
			toaster.show(said)
		}
		await Promise.all([
			client.invalidateQueries({ queryKey: groupsQueryKey }),
			client.invalidateQueries({ queryKey: typesQueryKey }),
		])
	}

	/**
	 * Reports why a field write was turned away, in a toast naming what failed and above the fields.
	 * @param cause - What the write failed with.
	 * @param write - The kind of write that failed, a field save when none is given.
	 */
	async function refused(cause: unknown, write: FieldWrite = 'save') {
		const said = groupErrorMessage(cause)
		setNotice(said)
		toaster.show(sprintf(failedWrite(write), said))
		if (cause instanceof StaleWriteError) {
			await client.invalidateQueries({ queryKey: groupsQueryKey })
		}
	}

	return (
		<Stack direction="column" gap="md">
			{notice !== '' && <ErrorNotice>{notice}</ErrorNotice>}
			<FieldsBody
				held={props.held}
				groups={props.groups}
				types={props.types}
				shadowed={props.shadowed}
				work={props.work}
				onDone={done}
				onRefused={refused}
			/>
		</Stack>
	)
}

/**
 * Renders the fields a group holds and the control declaring another.
 * @param props - Where the fields stand, what the page holds for them, and what to report.
 * @returns The body element.
 */
function FieldsBody(props: Reporter & FieldsWork) {
	return (
		<Stack direction="column" gap="lg">
			<FieldList
				held={props.held}
				groups={props.groups}
				types={props.types}
				shadowed={props.shadowed}
				work={props.work}
				onDone={props.onDone}
				onRefused={props.onRefused}
			/>
			<Stack direction="column" gap="sm">
				<SectionTitle>{__('Add a field', 'gophenberg')}</SectionTitle>
				<AddField
					group={props.held.id}
					types={props.types}
					groups={props.groups}
					onDone={props.onDone}
					onRefused={props.onRefused}
				/>
			</Stack>
		</Stack>
	)
}

/**
 * Renders the region the fields table scrolls in, holding the table or what shows while the group holds no field.
 * @param props - Where the fields stand, what the page holds for them, and what to report.
 * @returns The region element.
 */
function FieldList(props: Reporter & FieldsWork) {
	const region = useRef<HTMLDivElement>(null)
	const onGone = useFocusKept(region, props.held.fields)
	const empty = props.held.fields.length === 0
	return (
		<div
			ref={region}
			className="godmin-table-scroll"
			role="region"
			aria-label={__('Fields', 'gophenberg')}
			tabIndex={empty ? -1 : 0}
		>
			{empty ? (
				<NoFields />
			) : (
				<FieldTable
					held={props.held}
					groups={props.groups}
					types={props.types}
					shadowed={props.shadowed}
					work={props.work}
					above=""
					fields={props.held.fields}
					onGone={onGone}
					onDone={props.onDone}
					onRefused={props.onRefused}
				/>
			)}
		</div>
	)
}

/** A field whose delete landed: its key and its place in its table. */
interface Gone {
	key: string
	at: number
}

/**
 * Reports whether the focus sits where no reader put it: on the page or in the toast region.
 * @returns Whether the focus is loose.
 */
function focusLoose(): boolean {
	const now = document.activeElement
	return now === null || now === document.body || now.closest(TOASTS) !== null
}

/**
 * Returns the trash of the field standing at a place in a table, the last field's when the place is past the end.
 * @param region - The region the table scrolls in, none once it left the page.
 * @param at - The place.
 * @returns The trash, none while the table holds no field.
 */
function trashAt(region: HTMLElement | null, at: number): HTMLElement | null {
	const rows = [...(region?.querySelectorAll(FIELD_ROWS) ?? [])]
	return rows[Math.min(at, rows.length - 1)]?.querySelector<HTMLElement>(TRASH) ?? null
}

/**
 * Moves a loose focus to the trash of the field now standing where a deleted one stood, else to the region.
 * @param region - The region the table scrolls in, none once it left with its last field.
 * @param at - The place the deleted field stood at.
 * @param onEmpty - What takes the focus once the region left, nothing for a region that stays.
 */
function keepFocus(region: HTMLElement | null, at: number, onEmpty?: () => void) {
	if (!focusLoose()) {
		return
	}
	const target = trashAt(region, at) ?? region
	if (target === null) {
		onEmpty?.()
	} else {
		target.focus()
	}
}

/**
 * Keeps the focus in a fields table once a deleted field left it, and returns what notes a delete that landed.
 * @param region - The region the table scrolls in.
 * @param fields - The fields the table holds, as last read.
 * @param onEmpty - What takes the focus once the region left with its last field, nothing for a region that stays.
 * @returns What notes the key and the place of a field whose delete landed.
 */
function useFocusKept(region: RefObject<HTMLDivElement | null>, fields: ContentField[], onEmpty?: () => void) {
	const gone = useRef<Gone | null>(null)
	useLayoutEffect(() => {
		const left = gone.current
		if (left === null || fields.some((field) => field.key === left.key)) {
			return
		}
		gone.current = null
		keepFocus(region.current, left.at, onEmpty)
	}, [region, fields, onEmpty])
	return (key: string, at: number) => {
		gone.current = { key, at }
	}
}

/**
 * Renders what the fields table shows while the group holds no field.
 * @returns The empty state.
 */
function NoFields() {
	return (
		<ListEmpty
			icon={listIcon}
			title={__('No fields yet', 'gophenberg')}
			description={__('Add a field to store more about the items this group shows on.', 'gophenberg')}
		/>
	)
}

/** One level of the fields table: the dotted path holding its fields, empty at the group's top, and those fields. */
interface Level {
	above: string
	fields: ContentField[]
	/** Notes the key and the place of a field whose delete landed, so the focus stays in the table. */
	onGone: (key: string, at: number) => void
}

/**
 * Renders the table of the fields one level holds, one row group per field, its arrows moving a field at once.
 * @param props - Where the fields stand, what the page holds for them, the level, and what to report.
 * @returns The table element.
 */
function FieldTable(props: Reporter & FieldsWork & Level) {
	const client = useQueryClient()
	const { above } = props
	const group = props.held.id
	const order = useFieldOrder(props.fields, {
		store: (keys) => (above === '' ? reorderFieldsInGroup(group, keys) : reorderSubFields(group, above, keys)),
		onStored: () => props.onDone(),
		onFailed: async (cause) => {
			await props.onRefused(cause, 'order')
			await client.invalidateQueries({ queryKey: groupsQueryKey })
		},
	})

	return (
		<table className="godmin-table">
			<thead>
				<tr>
					<th scope="col">{__('Label', 'gophenberg')}</th>
					<th scope="col">{_x('Kind', 'field type', 'gophenberg')}</th>
					<th scope="col">{__('API name', 'gophenberg')}</th>
					<th scope="col" className="godmin-table__actions">
						<VisuallyHidden>{__('Actions', 'gophenberg')}</VisuallyHidden>
					</th>
				</tr>
			</thead>
			{order.rows.map((field, index) => (
				<FieldRows
					key={field.key}
					held={props.held}
					groups={props.groups}
					types={props.types}
					shadowed={props.shadowed}
					work={props.work}
					above={props.above}
					fields={props.fields}
					field={field}
					at={index}
					count={order.rows.length}
					onMove={(offset) => order.move(index, offset)}
					onGone={props.onGone}
					onDone={props.onDone}
					onRefused={props.onRefused}
				/>
			))}
		</table>
	)
}

/** One field of a table: the field, its place, how many fields the table holds, and the move it asks for. */
interface Placed {
	field: ContentField
	at: number
	count: number
	onMove: (offset: number) => void
}

/**
 * Moves the focus to a field's label when it sits inside the field's open part.
 * @param part - The open part of the field, none before it mounted.
 * @param toggle - The label that opens and closes the field, none before it mounted.
 */
function focusToggleFrom(part: HTMLDivElement | null, toggle: HTMLButtonElement | null) {
	if (part?.contains(document.activeElement)) {
		toggle?.focus()
	}
}

/**
 * Moves the focus to a field's label once something other than its own controls closed it, such as a save.
 * @param open - Whether the field stands open.
 * @param part - The open part of the field.
 * @param toggle - The label that opens and closes the field.
 */
function useFocusBackOnClose(
	open: boolean,
	part: RefObject<HTMLDivElement | null>,
	toggle: RefObject<HTMLButtonElement | null>,
) {
	useLayoutEffect(() => {
		if (!open) {
			focusToggleFrom(part.current, toggle.current)
		}
	}, [open, part, toggle])
}

/**
 * Renders the row group of one field: its row, then the row that opens under it.
 * @param props - Where the fields stand, what the page holds for them, the level, the field, its place, the table's
 * length, its move, and what to report.
 * @returns The row group element.
 */
function FieldRows(props: Reporter & FieldsWork & Level & Placed) {
	const { field, work } = props
	const path = props.above === '' ? field.key : `${props.above}.${field.key}`
	const [deleting, setDeleting] = useState(false)
	const toggle = useRef<HTMLButtonElement>(null)
	const part = useRef<HTMLDivElement>(null)
	const open = work.opened.has(path)
	const drafted = holdsDraft(work.changed, path)
	useFocusBackOnClose(open, part, toggle)

	/**
	 * Opens or closes the field, first moving the focus to its label when it sat inside the closing part.
	 * @param next - Whether the field stands open.
	 */
	function change(next: boolean) {
		if (!next) {
			focusToggleFrom(part.current, toggle.current)
		}
		work.onOpen(path, next)
	}

	/** Drops what the page held for the deleted field and keeps the focus in its table. */
	function deleted() {
		work.onDrop(path)
		props.onGone(field.key, props.at)
	}

	return (
		<Collapsible.Root render={<tbody />} open={open} onOpenChange={change}>
			<tr>
				<td>
					<LabelCell
						group={props.held.id}
						groups={props.groups}
						field={field}
						path={path}
						drafted={drafted}
						shadowed={props.above === '' && props.shadowed.has(field.key)}
						onDone={props.onDone}
						onRefused={props.onRefused}
					>
						<Collapsible.Trigger ref={toggle} className="godmin-table__toggle">
							<Icon icon={downIcon} className="godmin-table__toggle-icon" />
							{field.label}
						</Collapsible.Trigger>
					</LabelCell>
				</td>
				<td>
					<Badge intent="stable">{pickedLabel(field)}</Badge>
				</td>
				<td>
					<code>{field.key}</code>
				</td>
				<td className="godmin-table__actions">
					<RowControls
						at={props.at}
						count={props.count}
						labels={{
							moveUp: sprintf(__('Move %(field)s up', 'gophenberg'), { field: field.label }),
							moveDown: sprintf(__('Move %(field)s down', 'gophenberg'), { field: field.label }),
							remove: deleteName(field),
						}}
						onMove={props.onMove}
						onRemove={() => setDeleting(true)}
					/>
					{deleting && (
						<DeleteConfirm
							group={props.held.id}
							field={field}
							path={props.above === '' ? undefined : path}
							held={drafted}
							onDeleted={deleted}
							onDone={props.onDone}
							onRefused={props.onRefused}
							onClose={() => setDeleting(false)}
						/>
					)}
				</td>
			</tr>
			<Collapsible.Panel render={<tr />} className="godmin-table__detail" hiddenUntilFound>
				<td colSpan={COLUMNS}>
					<div className="godmin-table__detail-motion">
						<div>
							<div ref={part} className="godmin-table__detail-body">
								<OpenField
									group={props.held.id}
									groups={props.groups}
									types={props.types}
									field={field}
									path={path}
									siblings={props.fields}
									work={work}
									nested={
										<SubFieldList
											held={props.held}
											groups={props.groups}
											types={props.types}
											shadowed={props.shadowed}
											work={work}
											container={field}
											path={path}
											onEmpty={() => toggle.current?.focus()}
											onDone={props.onDone}
											onRefused={props.onRefused}
										/>
									}
									onClose={() => change(false)}
									onDone={props.onDone}
									onRefused={props.onRefused}
								/>
							</div>
						</div>
					</div>
				</td>
			</Collapsible.Panel>
		</Collapsible.Root>
	)
}

/** What the Label cell of a field shows and acts on. */
interface LabelProps extends Reporter {
	group: number
	groups: FieldGroup[]
	field: ContentField
	path: string
	/** Whether the field, or a field inside it, holds unsaved changes. */
	drafted: boolean
	shadowed: boolean
	/** The label that opens and closes the field. */
	children: ReactNode
}

/**
 * Renders the Label cell of a field: its label as the toggle opening the field, then Move and its badges.
 * @param props - Where the field stands, the field, its path, its badges, the toggle, and what to report.
 * @returns The cell's content.
 */
function LabelCell(props: LabelProps) {
	const { field } = props
	return (
		<Stack direction="column" gap="xs" align="start" className="gophenberg-fields__label">
			{props.children}
			<CarryField
				group={props.group}
				field={field}
				path={props.path}
				groups={props.groups}
				held={props.drafted}
				onDone={props.onDone}
				onRefused={props.onRefused}
			/>
			<FieldBadges field={field} drafted={props.drafted} shadowed={props.shadowed} />
		</Stack>
	)
}

/**
 * Renders the badges a field carries: Required, Unsaved changes while it holds a draft, and Shadowed.
 * @param props - The field, whether it or a field inside it holds a draft, and whether an earlier group serves its key.
 * @returns The badges, or nothing when the field carries none.
 */
function FieldBadges(props: { field: ContentField; drafted: boolean; shadowed: boolean }) {
	if (!props.field.required && !props.drafted && !props.shadowed) {
		return null
	}
	return (
		<Stack direction="row" gap="xs" align="center" wrap="wrap">
			{props.field.required && <Badge>{__('Required', 'gophenberg')}</Badge>}
			{props.drafted && <Badge intent="draft">{__('Unsaved changes', 'gophenberg')}</Badge>}
			{props.shadowed && <Badge intent="high">{__('Shadowed', 'gophenberg')}</Badge>}
		</Stack>
	)
}

/** The container whose sub fields a nested table shows, its path, and what takes the focus once its last one left. */
interface Container {
	container: ContentField
	path: string
	onEmpty: () => void
}

/**
 * Renders the region the sub fields of an open container scroll in, holding a table of the same rows.
 * @param props - Where the fields stand, what the page holds for them, the container, its path, what takes the focus
 * once its last sub field left, and what to report.
 * @returns The region, or nothing while the container holds no field.
 */
function SubFieldList(props: Reporter & FieldsWork & Container) {
	const region = useRef<HTMLDivElement>(null)
	const onGone = useFocusKept(region, props.container.fields, props.onEmpty)
	if (props.container.fields.length === 0) {
		return null
	}
	return (
		<div
			ref={region}
			className="godmin-table-scroll"
			role="region"
			aria-label={sprintf(__('Sub fields of %(field)s', 'gophenberg'), { field: props.container.label })}
			tabIndex={0}
		>
			<FieldTable
				held={props.held}
				groups={props.groups}
				types={props.types}
				shadowed={props.shadowed}
				work={props.work}
				above={props.path}
				fields={props.container.fields}
				onGone={onGone}
				onDone={props.onDone}
				onRefused={props.onRefused}
			/>
		</div>
	)
}
