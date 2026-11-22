// SPDX-License-Identifier: Apache-2.0

import { Button, InputControl, Text, ValidatedInputControl } from '@gophenberg/frontend-sdk'
import { Page, PageBreadcrumb, PageBreadcrumbs, useRejectedField, useToaster } from '@gopherium/godmin'
import type { RejectedField } from '@gopherium/godmin'
import { useSession } from '@gopherium/react-auth'
import { ValidationError, fieldError, invite, usersQueryKey } from '@gopherium/react-auth/admin'
import type { Invitation, NewInvite } from '@gopherium/react-auth/admin'
import { usersMenuItem } from '@gopherium/react-auth/wpds'
import { RoleSelect, addUserErrorMessageOf } from '@gopherium/react-auth/wpds/users'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { __, _x, sprintf } from '@wordpress/i18n'
import { useEffect, useId, useRef, useState } from 'react'
import type { ComponentProps, RefObject } from 'react'

import { useListSettings } from '../settings/useListSettings'
import { accountErrorMessages } from './errorMessages'
import { roleLabels, startingRole } from './roles'

/** The fields of the form a server's rejection can stand under. */
type InviteField = 'email' | 'name'

/** What a field the form needs filled takes beside the props of the WPDS field. */
type RequiredFieldProps = Omit<
	ComponentProps<typeof ValidatedInputControl>,
	'value' | 'required' | 'customValidity' | 'ref'
> & {
	/** The value the field holds. */
	value: string
	/** The server's rejection of the value the field holds, if any, and the ref that ties it to the field. */
	rejected: RejectedField
	/** The words the field shows under it while empty. */
	empty: string
}

/**
 * Renders a field the form needs filled, its message under it once the reader leaves it or sends the form.
 * @param props - The value, the server's rejection of it, the words for an empty field and the field's own props.
 * @returns The field.
 */
function RequiredField({ rejected: { ref, validity }, empty, ...field }: RequiredFieldProps) {
	const blank = field.value.trim() === '' ? { type: 'invalid' as const, message: empty } : undefined
	return <ValidatedInputControl {...field} ref={ref} required customValidity={validity ?? blank} />
}

/**
 * Returns the words a failed invitation reads as, the server's own only for a rejection it named no code for.
 * @param error - The mutation error.
 * @returns The words, empty when the failure has none worth showing.
 */
function failureWords(error: unknown): string {
	const about = fieldError(error)
	if (about !== undefined) {
		return about.message
	}
	const unnamed = error instanceof ValidationError && error.code === undefined ? error.message : ''
	return addUserErrorMessageOf(error, accountErrorMessages(), unnamed)
}

/**
 * Returns the field of the form a failed invitation is about.
 * @param error - The mutation error.
 * @returns The field, none when the failure is about no field the form holds.
 */
function rejectedField(error: unknown): InviteField | undefined {
	const field = fieldError(error)?.field
	return field === 'email' || field === 'name' ? field : undefined
}

/**
 * Returns the toast of a failed invitation.
 * @param words - What the failure reads as, empty when it has no words worth showing.
 * @returns The toast, the words after the failure itself.
 */
function failureToast(words: string): string {
	return words === ''
		? __('Could not send the invitation.', 'gophenberg')
		: sprintf(__('Could not send the invitation. %s', 'gophenberg'), words)
}

/**
 * Returns the toast of an invitation that came back once the reader had left the form.
 * @param invitation - What the server answered.
 * @returns The toast, where to get an activation link when no mail server delivered it.
 */
function leftToast(invitation: Invitation): string {
	return invitation.delivered
		? __('Invitation sent.', 'gophenberg')
		: __('No mail server is configured. Use Resend invitation in Users to get an activation link.', 'gophenberg')
}

/**
 * Returns whether the component calling it left the page, set once it unmounts.
 * @returns The ref, true once the component left.
 */
function useLeft(): RefObject<boolean> {
	const left = useRef(false)
	useEffect(() => {
		left.current = false
		return () => {
			left.current = true
		}
	}, [])
	return left
}

/**
 * Renders the invitation form.
 * @param props - What runs with the activation link when no mail server delivered the invitation.
 * @returns The form.
 */
function InvitationForm({ onUndelivered }: { onUndelivered: (link: string) => void }) {
	const client = useQueryClient()
	const navigate = useNavigate()
	const toaster = useToaster()
	const left = useLeft()
	const grantable = useSession().data?.grantable ?? []
	const defaultRole = useListSettings().data?.default_role
	const [email, setEmail] = useState('')
	const [name, setName] = useState('')
	const [picked, setPicked] = useState<string>()
	const rejected: Record<InviteField, RejectedField> = { email: useRejectedField(email), name: useRejectedField(name) }
	const role = picked ?? startingRole(grantable, defaultRole)
	const send = useMutation({
		mutationFn: (sent: NewInvite) => invite({ ...sent, email: sent.email.trim(), name: sent.name.trim() }),
		onSuccess: async (invitation) => {
			await client.invalidateQueries({ queryKey: usersQueryKey })
			if (left.current) {
				toaster.show(leftToast(invitation))
				return
			}
			if (!invitation.delivered) {
				onUndelivered(invitation.activation_link)
				return
			}
			await navigate({ to: '/users' })
			toaster.show(__('Invitation sent.', 'gophenberg'))
		},
		onError: (error, sent) => {
			const field = rejectedField(error)
			const words = failureWords(error)
			if (field === undefined || !rejected[field].reject(words, sent[field])) {
				toaster.show(failureToast(words))
			}
		},
	})
	return (
		<form
			className="godmin-form"
			onSubmit={(event) => {
				event.preventDefault()
				send.mutate({ email, name, role })
			}}
		>
			<RequiredField
				label={__('Email', 'gophenberg')}
				type="email"
				autoComplete="off"
				value={email}
				onValueChange={setEmail}
				rejected={rejected.email}
				empty={__('Please enter an email address.', 'gophenberg')}
			/>
			<RequiredField
				label={_x('Name', 'person', 'gophenberg')}
				autoComplete="off"
				value={name}
				onValueChange={setName}
				rejected={rejected.name}
				empty={__('Please provide a name.', 'gophenberg')}
			/>
			<RoleSelect grantable={grantable} value={role} onChange={setPicked} roleLabels={roleLabels()} />
			<Button type="submit" loading={send.isPending}>
				{__('Send invitation', 'gophenberg')}
			</Button>
		</form>
	)
}

/**
 * Renders the activation link to deliver by hand in place of the form, the focus on its field.
 * @param props - The link the invitation answered.
 * @returns The link and the way back to the list.
 */
function ActivationLink({ link }: { link: string }) {
	const navigate = useNavigate()
	const field = useRef<HTMLInputElement>(null)
	const words = useId()
	useEffect(() => {
		field.current?.focus()
	}, [])
	return (
		<div className="godmin-form">
			<Text id={words} role="status">
				{__('No mail server is configured. Deliver the activation link by hand.', 'gophenberg')}
			</Text>
			<InputControl
				ref={field}
				label={__('Activation link', 'gophenberg')}
				aria-describedby={words}
				readOnly
				value={link}
			/>
			<Button onClick={() => navigate({ to: '/users' })}>{__('Done', 'gophenberg')}</Button>
		</div>
	)
}

/**
 * Renders the account creation screen, the form giving way to the activation link when no mail server delivered it.
 * @returns The new user screen element.
 */
export function NewUserScreen() {
	const [link, setLink] = useState<string>()
	return (
		<Page
			title={__('Add User', 'gophenberg')}
			subtitle={__('Create a brand new user and add them to this site.', 'gophenberg')}
			headingLevel={2}
			breadcrumbs={
				<PageBreadcrumbs label={__('Breadcrumbs', 'gophenberg')}>
					<PageBreadcrumb render={<Link to="/users" activeOptions={{ exact: true }} />}>
						{usersMenuItem.label}
					</PageBreadcrumb>
				</PageBreadcrumbs>
			}
		>
			{link === undefined ? <InvitationForm onUndelivered={setLink} /> : <ActivationLink link={link} />}
		</Page>
	)
}
