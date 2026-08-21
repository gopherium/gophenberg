// SPDX-License-Identifier: Apache-2.0

import { Button, InputControl, SelectControl, Stack } from '@gophenberg/frontend-sdk'
import { __, _x } from '@wordpress/i18n'
import {
	EmailTakenError,
	ValidationError,
	createUser,
	usersQueryKey,
} from '@gopherium/react-auth/admin'
import { ErrorNotice, Page } from '@gopherium/godmin'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'

import { NARROWEST, rankOptions } from './ranks'

/**
 * Returns the message shown under the form when a creation fails.
 * @param error - The failure the create raised.
 * @returns The message to show.
 */
export function createFailureMessage(error: unknown): string {
	if (error instanceof EmailTakenError) {
		return __('That email is already in use.', 'gophenberg')
	}
	if (error instanceof ValidationError) {
		return error.message
	}
	return __('The user could not be created.', 'gophenberg')
}

/**
 * Renders the account creation screen.
 * @returns The new user screen element.
 */
export function NewUserScreen() {
	const client = useQueryClient()
	const navigate = useNavigate()
	const [email, setEmail] = useState('')
	const [name, setName] = useState('')
	const [password, setPassword] = useState('')
	const [rank, setRank] = useState(NARROWEST)
	const create = useMutation({
		mutationFn: () => createUser({ email: email.trim(), name: name.trim(), password, rank }),
		onSuccess: async () => {
			await client.invalidateQueries({ queryKey: usersQueryKey })
			await navigate({ to: '/users' })
		},
	})
	const incomplete = email.trim() === '' || name.trim() === '' || password === ''
	return (
		<Page title={__('New user', 'gophenberg')}>
			<form
				onSubmit={(event) => {
					event.preventDefault()
					create.mutate()
				}}
			>
				<Stack direction="column" gap="md">
					<InputControl
						label={__('Email', 'gophenberg')}
						type="email"
						autoComplete="off"
						value={email}
						onValueChange={setEmail}
					/>
					<InputControl
						label={_x('Name', 'person', 'gophenberg')}
						autoComplete="off"
						value={name}
						onValueChange={setName}
					/>
					<InputControl
						label={__('Password', 'gophenberg')}
						type="password"
						autoComplete="new-password"
						value={password}
						onValueChange={setPassword}
					/>
					<SelectControl
						label={_x('Role', 'account', 'gophenberg')}
						items={rankOptions()}
						value={rankOptions().find((option) => option.value === rank)}
						onValueChange={(item) => item?.value != null && setRank(item.value)}
					/>
					<Button
						type="submit"
						disabled={incomplete || create.isPending}
						loading={create.isPending}
					>
						{__('Create user', 'gophenberg')}
					</Button>
					{create.isError && <ErrorNotice>{createFailureMessage(create.error)}</ErrorNotice>}
				</Stack>
			</form>
		</Page>
	)
}
