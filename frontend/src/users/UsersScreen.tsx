// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, SelectControl, Stack, Text } from '@gophenberg/frontend-sdk'
import { __, _x, sprintf } from '@wordpress/i18n'
import { fetchUsers, setUserDisabled, setUserRole, usersQueryKey } from '@gopherium/react-auth/admin'
import type { User } from '@gopherium/react-auth/admin'
import { useSession } from '@gopherium/react-auth'
import { ErrorNotice, LoadingRows, Page } from '@gopherium/godmin'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { roleLabel, roleOptions } from './roles'

/**
 * Renders the account administration screen.
 * @returns The users screen element.
 */
export function UsersScreen() {
	const currentUserId = useSession().data?.id
	const users = useQuery({
		queryKey: usersQueryKey,
		queryFn: ({ signal }) => fetchUsers(signal),
	})
	return (
		<Page
			title={__('Users', 'gophenberg')}
			actions={
				<Button render={<Link to="/users/new" />}>{__('New user', 'gophenberg')}</Button>
			}
		>
			<UsersBody users={users.data} failed={users.isError} currentUserId={currentUserId} />
		</Page>
	)
}

/**
 * Renders the account list, or the state standing in for it.
 * @param props - The accounts, whether the read failed, and the signed-in account.
 * @returns The list element, or the state element.
 */
function UsersBody({
	users,
	failed,
	currentUserId,
}: {
	users: User[] | undefined
	failed: boolean
	currentUserId: string | undefined
}): ReactNode {
	if (failed) {
		return <ErrorNotice>{__('Users could not be loaded.', 'gophenberg')}</ErrorNotice>
	}
	if (users === undefined) {
		return <LoadingRows label={__('Loading users.', 'gophenberg')} />
	}
	return (
		<div
			className="godmin-table-scroll godmin-arrival"
			role="region"
			aria-label={__('Users', 'gophenberg')}
			tabIndex={0}
		>
			<table className="godmin-table">
				<thead>
					<tr>
						<th scope="col">{_x('Name', 'person', 'gophenberg')}</th>
						<th scope="col">{__('Email', 'gophenberg')}</th>
						<th scope="col">{_x('Role', 'account', 'gophenberg')}</th>
						<th scope="col">{_x('Status', 'account', 'gophenberg')}</th>
						<th scope="col" className="godmin-table__actions">
							{__('Actions', 'gophenberg')}
						</th>
					</tr>
				</thead>
				<tbody>
					{users.map((user) => (
						<UserRow key={user.id} user={user} isSelf={user.id === currentUserId} />
					))}
				</tbody>
			</table>
		</div>
	)
}

/**
 * Renders one account with its status and the control letting it in or out.
 * @param props - The account and whether it is the signed-in one.
 * @returns The table row element.
 */
function UserRow({ user, isSelf }: { user: User, isSelf: boolean }) {
	return (
		<tr>
			<td>{user.name}</td>
			<td>{user.email}</td>
			<td>{isSelf ? roleLabel(user.role) : <UserRole user={user} />}</td>
			<td>
				<UserStatus disabled={user.disabled} />
			</td>
			<td className="godmin-table__actions">{isSelf ? null : <UserToggle user={user} />}</td>
		</tr>
	)
}

/**
 * Renders the control writing the role an account holds.
 * @param props - The account the control acts on.
 * @returns The role control element.
 */
function UserRole({ user }: { user: User }) {
	const client = useQueryClient()
	const write = useMutation({
		mutationFn: (role: string) => setUserRole(user.id, role),
		onSuccess: () => client.invalidateQueries({ queryKey: usersQueryKey }),
	})
	const options = roleOptions()
	const held = options.find((option) => option.value === user.role)
	return (
		<Stack direction="column" gap="xs">
			<SelectControl
				label={sprintf(_x('Role of %(name)s', 'account', 'gophenberg'), { name: user.name })}
				hideLabelFromVision
				size="compact"
				items={options}
				value={held}
				onValueChange={(item) => item?.value != null && write.mutate(item.value)}
			/>
			{write.isError && <Text role="alert">{__('Update failed.', 'gophenberg')}</Text>}
		</Stack>
	)
}

/**
 * Renders whether an account may log in.
 * @param props - Whether the account is shut out.
 * @returns The status badge element.
 */
function UserStatus({ disabled }: { disabled: boolean }) {
	return (
		<Badge intent={disabled ? 'draft' : 'stable'}>
			{disabled ? __('Disabled', 'gophenberg') : __('Active', 'gophenberg')}
		</Badge>
	)
}

/**
 * Renders the control letting an account in or shutting it out.
 * @param props - The account the control acts on.
 * @returns The toggle element.
 */
function UserToggle({ user }: { user: User }) {
	const client = useQueryClient()
	const toggle = useMutation({
		mutationFn: () => setUserDisabled(user.id, !user.disabled),
		onSuccess: () => client.invalidateQueries({ queryKey: usersQueryKey }),
	})
	const verb = user.disabled ? __('Enable', 'gophenberg') : __('Disable', 'gophenberg')
	const named = user.disabled
		? sprintf(__('Enable %(name)s', 'gophenberg'), { name: user.name })
		: sprintf(__('Disable %(name)s', 'gophenberg'), { name: user.name })
	return (
		<Stack direction="column" gap="xs">
			<Button
				variant="outline"
				aria-label={named}
				loading={toggle.isPending}
				onClick={() => toggle.mutate()}
			>
				{verb}
			</Button>
			{toggle.isError && <Text role="alert">{__('Update failed.', 'gophenberg')}</Text>}
		</Stack>
	)
}
