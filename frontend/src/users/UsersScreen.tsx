// SPDX-License-Identifier: Apache-2.0

import { Badge, Button, Stack, Text } from '@gophenberg/frontend-sdk'
import { fetchUsers, setUserDisabled, usersQueryKey } from '@gopherium/react-auth/admin'
import type { User } from '@gopherium/react-auth/admin'
import { useSession } from '@gopherium/react-auth'
import { ErrorNotice, LoadingRows, Page } from '@gopherium/godmin'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import type { ReactNode } from 'react'

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
			title="Users"
			actions={<Button render={<Link to="/users/new" />}>New user</Button>}
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
		return <ErrorNotice>Users could not be loaded.</ErrorNotice>
	}
	if (users === undefined) {
		return <LoadingRows label="Loading users." />
	}
	return (
		<div className="godmin-table-scroll" role="region" aria-label="Users" tabIndex={0}>
			<table className="godmin-table">
				<thead>
					<tr>
						<th scope="col">Name</th>
						<th scope="col">Email</th>
						<th scope="col">Status</th>
						<th scope="col" className="godmin-table__actions">
							Actions
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
			<td>
				<UserStatus disabled={user.disabled} />
			</td>
			<td className="godmin-table__actions">{isSelf ? null : <UserToggle user={user} />}</td>
		</tr>
	)
}

/**
 * Renders whether an account may log in.
 * @param props - Whether the account is shut out.
 * @returns The status badge element.
 */
function UserStatus({ disabled }: { disabled: boolean }) {
	return (
		<Badge intent={disabled ? 'draft' : 'stable'}>{disabled ? 'Disabled' : 'Active'}</Badge>
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
	const verb = user.disabled ? 'Enable' : 'Disable'
	return (
		<Stack direction="column" gap="xs">
			<Button
				variant="outline"
				aria-label={`${verb} ${user.name}`}
				disabled={toggle.isPending}
				onClick={() => toggle.mutate()}
			>
				{verb}
			</Button>
			{toggle.isError && <Text role="alert">Update failed.</Text>}
		</Stack>
	)
}
