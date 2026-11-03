// SPDX-License-Identifier: Apache-2.0

import { Button, Text, ValidatedInputControl, speak } from '@gophenberg/frontend-sdk'
import { __, sprintf } from '@wordpress/i18n'
import { ErrorNotice, LoadingScreen, Page, useToaster } from '@gopherium/godmin'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import type { UseQueryResult } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import type { ComponentProps } from 'react'

import { errorText } from '../i18n/errors'
import { SettingError, chooseSiteSettings, fetchSiteSettings } from './api'
import type { SettingField, SettingsAsked, SiteSettings } from './api'

/** The query naming the settings the site chose for itself. */
const settingsQueryKey = ['site-settings']

/** The fewest items a listing may carry. */
const minPerPage = 1

/** The most items a listing may carry. */
const maxPerPage = 100

/** The lowest quality a picture copy may be saved at. */
const minQuality = 1

/** The highest quality a picture copy may be saved at. */
const maxQuality = 100

/** What a box holds when it names a whole number. */
const wholeNumber = /^-?\d+$/

/** The server's message about one setting, shown while its box holds the value it is about. */
interface FieldMessage {
	field: SettingField
	message: string
	value: number | undefined
}

/** What a box for a whole number takes beside the props of the WPDS field. */
type NumberBoxProps = Omit<
	ComponentProps<typeof ValidatedInputControl>,
	'value' | 'required' | 'customValidity'
> & {
	/** The text the box holds. */
	value: string
	/** The server's message about the value the box holds, if any. */
	refused?: string
}

/**
 * Returns the whole number the typed text names, or nothing when it names none.
 * @param typed - The text an administrator typed.
 * @returns The number, or undefined.
 */
function numberTyped(typed: string): number | undefined {
	return wholeNumber.test(typed) ? Number(typed) : undefined
}

/**
 * Renders a box for a whole number, its message under it once the reader leaves it or sends the form.
 * @param props - The text, the server's message about it and the field's own props.
 * @returns The box.
 */
function NumberBox({ refused, ...box }: NumberBoxProps) {
	const notWhole = numberTyped(box.value) === undefined ? __('Type a whole number.', 'gophenberg') : undefined
	const message = refused ?? notWhole
	return (
		<ValidatedInputControl
			{...box}
			customValidity={message === undefined ? undefined : { type: 'invalid', message }}
		/>
	)
}

/**
 * Shows the message of each box that fails, and speaks the one in a box that already held the focus.
 * @param form - The form holding the boxes.
 */
function reportBoxes(form: HTMLFormElement | null) {
	const box = document.activeElement
	form?.reportValidity()
	if (box === document.activeElement && box instanceof HTMLInputElement) {
		speak(box.validationMessage, 'assertive')
	}
}

/**
 * Returns a failed save as a setting error, carrying the plain words when it came from elsewhere.
 * @param error - What the save failed with.
 * @returns The setting error.
 */
function settingFailure(error: Error): SettingError {
	return error instanceof SettingError ? error : new SettingError(errorText({ error: '' }))
}

/**
 * Returns the box a failure names while that box still holds the value sent, or nothing.
 * @param field - The setting the failure names, if any.
 * @param asked - What each box holds now.
 * @param sent - What the save sent.
 * @returns The box that can show the failure.
 */
function boxStillHolding(field: SettingField | undefined, asked: SettingsAsked, sent: SettingsAsked) {
	if (field === undefined || asked[field] !== sent[field]) {
		return undefined
	}
	return field
}

/**
 * Returns the server's message about a box while the box holds the value it is about.
 * @param refused - The server's last message about one box, if any.
 * @param field - The box asking.
 * @param held - The number the box holds.
 * @returns The message, or undefined.
 */
function messageFor(refused: FieldMessage | undefined, field: SettingField, held: number | undefined) {
	return refused?.field === field && refused.value === held ? refused.message : undefined
}

/**
 * Renders the fields an administrator edits, starting from what the site holds.
 * @param props - The settings the site holds.
 * @returns The settings form element.
 */
function SettingsForm({ held }: { held: SiteSettings }) {
	const client = useQueryClient()
	const toaster = useToaster()
	const [refused, setRefused] = useState<FieldMessage>()
	const [perPage, setPerPage] = useState(String(held.content_per_page))
	const [quality, setQuality] = useState(String(held.jpeg_quality))
	const form = useRef<HTMLFormElement>(null)
	useEffect(() => {
		if (refused !== undefined) {
			reportBoxes(form.current)
		}
	}, [refused])
	const asked = { content_per_page: numberTyped(perPage), jpeg_quality: numberTyped(quality) }
	const save = useMutation({
		mutationFn: chooseSiteSettings,
		onSuccess: async () => {
			await client.invalidateQueries({ queryKey: settingsQueryKey })
		},
		onError: (error, sent) => {
			const { field: named, message } = settingFailure(error)
			const field = form.current === null ? undefined : boxStillHolding(named, asked, sent)
			if (field === undefined) {
				toaster.show(sprintf(__('Could not save the settings. %s', 'gophenberg'), message))
				return
			}
			setRefused({ field, message, value: sent[field] })
		},
	})
	const messageOf = (field: SettingField) => messageFor(refused, field, asked[field])
	const store = () => {
		if (asked.content_per_page === undefined || asked.jpeg_quality === undefined) {
			reportBoxes(form.current)
			return
		}
		save.mutate(asked)
	}
	return (
		<form
			ref={form}
			className="godmin-form"
			noValidate
			onSubmit={(event) => {
				event.preventDefault()
				store()
			}}
		>
			<NumberBox
				label={__('Posts per page', 'gophenberg')}
				type="number"
				min={minPerPage}
				max={maxPerPage}
				step={1}
				autoComplete="off"
				value={perPage}
				onValueChange={setPerPage}
				refused={messageOf('content_per_page')}
			/>
			<Text variant="body-sm">
				{__('A theme naming its own page size uses that instead.', 'gophenberg')}
			</Text>
			<NumberBox
				label={__('Picture quality', 'gophenberg')}
				type="number"
				min={minQuality}
				max={maxQuality}
				step={1}
				autoComplete="off"
				value={quality}
				onValueChange={setQuality}
				refused={messageOf('jpeg_quality')}
			/>
			<Text variant="body-sm">
				{__('A higher quality stores larger files, and it applies to pictures uploaded from now on.', 'gophenberg')}
			</Text>
			<Button type="submit" loading={save.isPending}>
				{__('Save', 'gophenberg')}
			</Button>
		</form>
	)
}

/**
 * Returns what the screen shows for the settings it asked for.
 * @param held - The query holding the site settings.
 * @returns The element to render inside the page.
 */
function settingsBody(held: UseQueryResult<SiteSettings>) {
	if (held.isError) {
		return (
			<ErrorNotice>
				{__('The settings could not be read, so nothing can be changed here yet.', 'gophenberg')}
			</ErrorNotice>
		)
	}
	if (held.data === undefined) {
		return <LoadingScreen label={__('Loading the settings.', 'gophenberg')} />
	}
	return <SettingsForm held={held.data} />
}

/**
 * Renders the screen where an administrator chooses how the site serves its content.
 * @returns The settings screen element.
 */
export function SettingsScreen() {
	const held = useQuery({ queryKey: settingsQueryKey, queryFn: fetchSiteSettings })
	return (
		<Page
			title={__('Settings', 'gophenberg')}
			headingLevel={2}
			subtitle={__('Choose how the site serves its content.', 'gophenberg')}
		>
			{settingsBody(held)}
		</Page>
	)
}
