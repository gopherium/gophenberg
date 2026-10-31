// SPDX-License-Identifier: Apache-2.0

import { Button, ValidatedInputControl } from '@gophenberg/frontend-sdk'
import { Page, useToaster } from '@gopherium/godmin'
import { useSession } from '@gopherium/react-auth'
import { createUser, fieldError, usersQueryKey } from '@gopherium/react-auth/admin'
import type { UserField } from '@gopherium/react-auth/admin'
import { PasswordField, usePasswordField } from '@gopherium/react-auth/wpds'
import type { PasswordFieldState } from '@gopherium/react-auth/wpds'
import { RoleSelect, errorMessageOf } from '@gopherium/react-auth/wpds/users'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { __, _x } from '@wordpress/i18n'
import { useEffect, useRef, useState } from 'react'
import type { ComponentProps } from 'react'

import { useListSettings } from '../settings/useListSettings'
import { roleLabels, startingRole } from './roles'

/** The length of every password Add User generates. */
const GENERATED_PASSWORD_LENGTH = 24

/** What the reader typed in each field, as one send carries it. */
type Typed = Record<UserField, string>

/** The message under one field, shown while the field holds the value it is about. */
interface FieldMessage {
	field: UserField
	message: string
	value: string
}

/** What a field the form needs filled takes beside the props of the WPDS field. */
type RequiredFieldProps = Omit<
	ComponentProps<typeof ValidatedInputControl>,
	'value' | 'required' | 'customValidity'
> & {
	/** The value the field holds. */
	value: string
	/** The server's message about the value the field holds, if any. */
	refused?: string
	/** The words the field shows under it while empty. */
	empty: string
}

/**
 * Renders a field the form needs filled, its message under it once the reader leaves it or sends the form.
 * @param props - The value, the server's message about it, the words for an empty field and the field's own props.
 * @returns The field.
 */
function RequiredField({ refused, empty, ...field }: RequiredFieldProps) {
	const message = refused ?? (field.value.trim() === '' ? empty : undefined)
	return (
		<ValidatedInputControl
			{...field}
			required
			customValidity={message === undefined ? undefined : { type: 'invalid', message }}
		/>
	)
}

/**
 * Returns the words under a weak password Add User was pressed on, while the field still holds it unconfirmed.
 * @param pressed - The weak password Add User was last pressed on, if any.
 * @param password - The password the field holds and what the meter knows of it.
 * @returns The words, none once the password changed or its use is confirmed.
 */
function unconfirmedMessage(pressed: string | undefined, password: PasswordFieldState): string | undefined {
	return pressed === password.value && !password.ready
		? __('Please confirm the use of a weak password.', 'gophenberg')
		: undefined
}

/**
 * Renders the account creation screen.
 * @returns The new user screen element.
 */
export function NewUserScreen() {
	const client = useQueryClient()
	const navigate = useNavigate()
	const toaster = useToaster()
	const grantable = useSession().data?.grantable ?? []
	const defaultRole = useListSettings().data?.default_role
	const [email, setEmail] = useState('')
	const [name, setName] = useState('')
	const [picked, setPicked] = useState<string>()
	const [refusal, setRefusal] = useState<FieldMessage>()
	const [pressedWeak, setPressedWeak] = useState<string>()
	const form = useRef<HTMLFormElement>(null)
	useEffect(() => {
		if (refusal !== undefined) {
			form.current?.reportValidity()
		}
	}, [refusal])
	const password = usePasswordField({ length: GENERATED_PASSWORD_LENGTH, userInputs: [name, email] })
	const role = picked ?? startingRole(grantable, defaultRole)
	const typed: Typed = { email, name, password: password.value }
	const messageOf = (field: UserField) =>
		refusal?.field === field && refusal.value === typed[field] ? refusal.message : undefined
	const create = useMutation({
		mutationFn: (sent: Typed) =>
			createUser({ email: sent.email, name: sent.name.trim(), password: sent.password, role }),
		onSuccess: async () => {
			await client.invalidateQueries({ queryKey: usersQueryKey })
			await navigate({ to: '/users' })
			toaster.show(__('New user created.', 'gophenberg'))
		},
		onError: (error, sent) => {
			const about = fieldError(error)
			if (about === undefined) {
				toaster.show(errorMessageOf(error, {}, __('The user could not be created.', 'gophenberg')))
				return
			}
			setRefusal({ ...about, value: sent[about.field] })
		},
	})
	const submit = () => {
		if (password.value.trim() === '') {
			setRefusal({ field: 'password', message: __('Please enter a password.', 'gophenberg'), value: password.value })
		} else if (password.ready) {
			password.mask(true)
			create.mutate(typed)
		} else {
			setPressedWeak(password.value)
		}
	}
	return (
		<Page
			title={__('Add User', 'gophenberg')}
			subtitle={__('Create a brand new user and add them to this site.', 'gophenberg')}
			headingLevel={2}
		>
			<form
				ref={form}
				className="godmin-form"
				onSubmit={(event) => {
					event.preventDefault()
					submit()
				}}
			>
				<RequiredField
					label={__('Email', 'gophenberg')}
					type="email"
					autoComplete="off"
					value={email}
					onValueChange={setEmail}
					refused={messageOf('email')}
					empty={__('Please enter an email address.', 'gophenberg')}
				/>
				<RequiredField
					label={_x('Name', 'person', 'gophenberg')}
					autoComplete="off"
					value={name}
					onValueChange={setName}
					refused={messageOf('name')}
					empty={__('Please provide a name.', 'gophenberg')}
				/>
				<PasswordField
					password={password}
					generateFirst
					labelPosition="top"
					readOnly={create.isPending}
					help={messageOf('password') ?? unconfirmedMessage(pressedWeak, password)}
				/>
				<RoleSelect grantable={grantable} value={role} onChange={setPicked} roleLabels={roleLabels()} />
				<Button type="submit" loading={create.isPending}>
					{__('Add User', 'gophenberg')}
				</Button>
			</form>
		</Page>
	)
}
