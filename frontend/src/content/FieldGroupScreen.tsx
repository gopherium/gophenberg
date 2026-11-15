// SPDX-License-Identifier: Apache-2.0

import { Notice } from '@gophenberg/frontend-sdk'
import { ErrorNotice, LoadingScreen, Page, PageBreadcrumb, PageBreadcrumbs } from '@gopherium/godmin'
import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from '@tanstack/react-router'
import { __, sprintf } from '@wordpress/i18n'

import { GroupFields } from './GroupFieldsDialog'
import { groupsQueryKey, listGroups } from './groups'
import { groupsMenuItem, typesQueryKey } from './nav'
import { listTypes } from './types'
import type { FieldGroup } from './groups'
import type { ContentType } from './types'

/**
 * Returns the title the page of a group carries: the group's own, else the name of the kind of record.
 * @param held - The group the address names, none while the groups load or when no group holds the address.
 * @returns The page title.
 */
function titleOf(held: FieldGroup | undefined): string {
	return held === undefined || held.title === '' ? __('Field Group', 'gophenberg') : held.title
}

/**
 * Renders the page of one field group, its fields in the main work.
 * @returns The field group screen element.
 */
export function FieldGroupScreen() {
	const { groupId } = useParams({ from: '/framed/admin/field-groups/$groupId' })
	const groups = useQuery({ queryKey: groupsQueryKey, queryFn: listGroups })
	const types = useQuery({ queryKey: typesQueryKey, queryFn: listTypes })
	const held = groups.data?.find((listed) => String(listed.id) === groupId)
	return (
		<Page
			title={titleOf(held)}
			headingLevel={2}
			breadcrumbs={
				<PageBreadcrumbs label={__('Breadcrumbs', 'gophenberg')}>
					<PageBreadcrumb render={<Link to="/field-groups" activeOptions={{ exact: true }} />}>
						{groupsMenuItem.label}
					</PageBreadcrumb>
				</PageBreadcrumbs>
			}
		>
			<GroupWork held={held} groups={groups.data} failed={groups.isError} types={types.data ?? []} />
		</Page>
	)
}

/** What the main work of a group's page reads. */
interface GroupWorkProps {
	held?: FieldGroup
	groups?: FieldGroup[]
	failed: boolean
	types: ContentType[]
}

/**
 * Renders the main work of a group's page in whichever state the read of the groups is in.
 * @param props - The group the address names, every group read, whether the read failed, and the types.
 * @returns The loading screen, a notice, or the fields of the group under the notice of a failed read.
 */
function GroupWork(props: GroupWorkProps) {
	const failure = <ErrorNotice>{__('The field groups could not be loaded.', 'gophenberg')}</ErrorNotice>
	if (props.groups === undefined) {
		return props.failed ? failure : <LoadingScreen label={__('Loading field groups.', 'gophenberg')} />
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
	const origin = props.held.origin ?? ''
	if (origin !== '') {
		return <PluginGroupNotice held={props.held} origin={origin} />
	}
	return (
		<>
			{props.failed && failure}
			<GroupFields held={props.held} groups={props.groups} types={props.types} />
		</>
	)
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
