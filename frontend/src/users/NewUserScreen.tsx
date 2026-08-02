// SPDX-License-Identifier: Apache-2.0

import { Button, InputControl, Stack } from '@gophenberg/frontend-sdk'
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

/**
 * Returns the message shown under the form when a creation fails.
 * @param error - The failure the create raised.
 * @returns The message to show.
 */
export function createFailureMessage(error: unknown): string {
	if (error instanceof EmailTakenError) {
		return 'That email is already in use.'
	}
	if (error instanceof ValidationError) {
		return error.message
	}
	return 'The user could not be created.'
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
	const create = useMutation({
		mutationFn: () => createUser({ email: email.trim(), name: name.trim(), password }),
		onSuccess: async () => {
			await client.invalidateQueries({ queryKey: usersQueryKey })
			await navigate({ to: '/users' })
		},
	})
	const incomplete = email.trim() === '' || name.trim() === '' || password === ''
	return (
		<Page title="New user">
			<form
				onSubmit={(event) => {
					event.preventDefault()
					create.mutate()
				}}
			>
				<Stack direction="column" gap="md">
					<InputControl
						label="Email"
						type="email"
						autoComplete="off"
						value={email}
						onValueChange={setEmail}
					/>
					<InputControl label="Name" autoComplete="off" value={name} onValueChange={setName} />
					<InputControl
						label="Password"
						type="password"
						autoComplete="new-password"
						value={password}
						onValueChange={setPassword}
					/>
					<Button type="submit" disabled={incomplete || create.isPending}>
						Create user
					</Button>
					{create.isError && <ErrorNotice>{createFailureMessage(create.error)}</ErrorNotice>}
				</Stack>
			</form>
		</Page>
	)
}
