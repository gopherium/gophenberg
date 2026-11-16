// SPDX-License-Identifier: Apache-2.0

import { Button, Notice } from '@gophenberg/frontend-sdk'
import { ErrorNotice, LoadingScreen, Page, PageBreadcrumb, PageBreadcrumbs } from '@gopherium/godmin'
import { UnsavedChangesDialog } from '@gopherium/godmin/router'
import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { __, sprintf } from '@wordpress/i18n'
import { useId } from 'react'
import type { ReactNode } from 'react'

import { GroupDetails, SaveNotices, useGroupDetails } from './GroupDetails'
import { GroupFields } from './GroupFieldsDialog'
import { shadowings, shadowingSentence } from './GroupsScreen'
import { groupsQueryKey, listGroups } from './groups'
import { groupsMenuItem, typesQueryKey } from './nav'
import { listTypes } from './types'
import type { FieldGroup } from './groups'
import type { Shadowing } from './GroupsScreen'
import type { ContentType } from './types'

/**
 * Returns the title the page of a group carries: the group's own, else the name of the kind of record.
 * @param title - The title the group holds or the reader typed, none while the groups load or when no group holds
 * the address.
 * @returns The page title, the name of the kind of record while the title is blank.
 */
function titleOf(title: string | undefined): string {
	return title === undefined || title.trim() === '' ? __('Field Group', 'gophenberg') : title
}

/**
 * Renders the page of one field group, read afresh for each address.
 * @returns The field group screen element.
 */
export function FieldGroupScreen() {
	const { groupId } = useParams({ from: '/framed/admin/field-groups/$groupId' })
	return <GroupScreen key={groupId} groupId={groupId} />
}

/**
 * Renders the page of the group an address names in whichever state the read of the groups is in.
 * @param props - The id the address names.
 * @returns The page element.
 */
function GroupScreen(props: { groupId: string }) {
	const groups = useQuery({ queryKey: groupsQueryKey, queryFn: listGroups })
	const types = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })
	const listed = groups.data ?? []
	const held = listed.find((group) => String(group.id) === props.groupId)
	const origin = held?.origin ?? ''
	if (held !== undefined && origin === '') {
		return (
			<SiteGroupPage
				held={held}
				groups={listed}
				types={types.data}
				failed={groups.isError}
				typesFailed={types.isError}
			/>
		)
	}
	return (
		<GroupPage title={titleOf(held?.title)}>
			<GroupState held={held} origin={origin} loaded={groups.data !== undefined} failed={groups.isError} />
		</GroupPage>
	)
}

/** What the frame of a group's page holds beside its main work. */
interface GroupPageProps {
	title: string
	actions?: ReactNode
	aside?: ReactNode
	children: ReactNode
}

/**
 * Renders the frame of a group's page: the breadcrumb back to the list, the title, the header actions and the details.
 * @param props - The title, the header actions and the details, both absent when the page edits nothing, and the
 * main work.
 * @returns The page element.
 */
function GroupPage(props: GroupPageProps) {
	return (
		<Page
			title={props.title}
			headingLevel={2}
			breadcrumbs={
				<PageBreadcrumbs label={__('Breadcrumbs', 'gophenberg')}>
					<PageBreadcrumb render={<Link to="/field-groups" activeOptions={{ exact: true }} />}>
						{groupsMenuItem.label}
					</PageBreadcrumb>
				</PageBreadcrumbs>
			}
			actions={props.actions}
			aside={props.aside}
			asideLabel={props.aside === undefined ? undefined : __('Field group settings', 'gophenberg')}
		>
			{props.children}
		</Page>
	)
}

/**
 * Returns the overlaps a group takes part in, as the group serving a field or as the group losing it.
 * @param held - The group of the page.
 * @param groups - Every group read.
 * @param types - The registered types the rules may reach.
 * @returns The overlaps naming the group.
 */
function overlapsOf(held: FieldGroup, groups: FieldGroup[], types: ContentType[]): Shadowing[] {
	return shadowings(groups, types).filter((overlap) => overlap.winnerID === held.id || overlap.loserID === held.id)
}

/** What the page of a group the site declared reads. */
interface SiteGroupProps {
	held: FieldGroup
	groups: FieldGroup[]
	types: ContentType[] | undefined
	failed: boolean
	typesFailed: boolean
}

/**
 * Renders the page of a group the site declared: its fields, its details beside them and the header Save.
 * @param props - The group, every group read, the types last read, and whether the last read of each failed.
 * @returns The page element.
 */
function SiteGroupPage(props: SiteGroupProps) {
	const form = useId()
	const types = props.types ?? []
	const details = useGroupDetails(props.held, props.groups)
	const overlaps = overlapsOf(props.held, props.groups, types)
	const losing = new Set(overlaps.filter((overlap) => overlap.loserID === props.held.id).map((overlap) => overlap.key))
	const blank = details.draft.title.trim() === ''
	return (
		<GroupPage
			title={titleOf(details.draft.title)}
			actions={
				<Button
					type="submit"
					form={form}
					size="compact"
					disabled={!details.changed || blank || details.saving}
					loading={details.saving}
				>
					{__('Save', 'gophenberg')}
				</Button>
			}
			aside={<GroupDetails form={form} held={props.held} groups={props.groups} types={types} details={details} />}
		>
			<SaveNotices held={props.held} details={details} />
			{overlaps.map((overlap) => (
				<Notice.Root key={`${overlap.key}:${overlap.winnerID}:${overlap.loserID}`} intent="warning">
					<Notice.Description>{shadowingSentence(overlap)}</Notice.Description>
				</Notice.Root>
			))}
			{props.failed && <ReadFailure />}
			{props.typesFailed && <TypesFailure read={props.types !== undefined} />}
			<GroupFields held={props.held} groups={props.groups} types={types} shadowed={losing} />
			<UnsavedChangesDialog
				when={details.changed}
				title={__('Discard changes?', 'gophenberg')}
				description={__('The changes to this field group are not saved.', 'gophenberg')}
				discardLabel={__('Discard', 'gophenberg')}
				keepLabel={__('Keep editing', 'gophenberg')}
			/>
		</GroupPage>
	)
}

/**
 * Renders the notice that the field groups could not be read.
 * @returns The notice element.
 */
function ReadFailure() {
	return <ErrorNotice>{__('The field groups could not be loaded.', 'gophenberg')}</ErrorNotice>
}

/**
 * Renders the notice that the content types could not be read, for the page of one group.
 * @param props - Whether an earlier read of the types held, so what the page shows may only be out of date.
 * @returns The notice element.
 */
function TypesFailure(props: { read: boolean }) {
	const lost = __(
		'The content types could not be loaded, so where this group appears, which fields it shadows ' +
			'and what a relation can point at are not shown.',
		'gophenberg',
	)
	const stale = __(
		'The content types could not be refreshed, so where this group appears, which fields it shadows ' +
			'and what a relation can point at may be out of date.',
		'gophenberg',
	)
	return <ErrorNotice>{props.read ? stale : lost}</ErrorNotice>
}

/** What the page of a group with nothing to edit reads. */
interface GroupStateProps {
	held?: FieldGroup
	origin: string
	loaded: boolean
	failed: boolean
}

/**
 * Renders the main work of a page with nothing to edit in whichever state the read of the groups is in.
 * @param props - The group the address names, the plugin that declared it, whether the groups were read, and whether
 * the read failed.
 * @returns The loading screen, the notice of a failed read or of a missing group, or the notice of a plugin group.
 */
function GroupState(props: GroupStateProps) {
	if (!props.loaded) {
		return props.failed ? <ReadFailure /> : <LoadingScreen label={__('Loading field groups.', 'gophenberg')} />
	}
	if (props.held === undefined) {
		return (
			<ErrorNotice>
				{__(
					'This field group does not exist, and it may have been deleted. Go back to the list to see what is there.',
					'gophenberg',
				)}
			</ErrorNotice>
		)
	}
	return <PluginGroupNotice held={props.held} origin={props.origin} />
}

/**
 * Renders the notice a group a plugin declared shows in place of its fields.
 * @param props - The group and the plugin that declared it.
 * @returns The notice element.
 */
function PluginGroupNotice(props: { held: FieldGroup; origin: string }) {
	return (
		<Notice.Root intent="info">
			<Notice.Description>
				{sprintf(
					__(
						'%(group)s comes from %(plugin)s, so it is read only. Its fields show from the Field Groups list.',
						'gophenberg',
					),
					{ group: props.held.title, plugin: props.origin },
				)}
			</Notice.Description>
		</Notice.Root>
	)
}
