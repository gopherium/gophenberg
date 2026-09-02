// SPDX-License-Identifier: Apache-2.0

import { Button, InputControl, Skeleton, Stack, Text } from '@gophenberg/frontend-sdk'
import { __ } from '@wordpress/i18n'
import { ErrorNotice, Page } from '@gopherium/godmin'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'

import { chooseSiteSettings, fetchSiteSettings } from './api'
import type { SiteSettings } from './api'

/** The query naming the settings the site chose for itself. */
const settingsQueryKey = ['site-settings']

/**
 * Returns the number the typed text names, or nothing when it names none.
 * @param typed - The text an administrator typed.
 * @returns The number, or undefined.
 */
function numberTyped(typed: string): number | undefined {
	return typed === '' ? undefined : Number(typed)
}

/**
 * Renders the fields an administrator edits, starting from what the site holds.
 * @param props - The settings the site holds.
 * @returns The settings form element.
 */
function SettingsForm({ held }: { held: SiteSettings }) {
	const client = useQueryClient()
	const [notice, setNotice] = useState('')
	const [perPage, setPerPage] = useState(String(held.content_per_page))
	const [quality, setQuality] = useState(String(held.jpeg_quality))
	const save = useMutation({
		mutationFn: chooseSiteSettings,
		onSuccess: async () => {
			setNotice('')
			await client.invalidateQueries({ queryKey: settingsQueryKey })
		},
		onError: (err: Error) => setNotice(err.message),
	})
	return (
		<Stack direction="column" gap="md">
			{notice !== '' && <ErrorNotice>{notice}</ErrorNotice>}
			<InputControl
				label={__('Posts per page', 'gophenberg')}
				type="number"
				autoComplete="off"
				value={perPage}
				onValueChange={setPerPage}
			/>
			<Text variant="body-sm">
				{__('A theme naming its own page size uses that instead.', 'gophenberg')}
			</Text>
			<InputControl
				label={__('Picture quality', 'gophenberg')}
				type="number"
				autoComplete="off"
				value={quality}
				onValueChange={setQuality}
			/>
			<Text variant="body-sm">
				{__('A higher quality stores larger files, and it applies to pictures uploaded from now on.', 'gophenberg')}
			</Text>
			<Button
				loading={save.isPending}
				onClick={() =>
					save.mutate({
						content_per_page: numberTyped(perPage),
						jpeg_quality: numberTyped(quality),
					})
				}
			>
				{__('Save', 'gophenberg')}
			</Button>
		</Stack>
	)
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
			subtitle={__('Choose how the site serves its content.', 'gophenberg')}
		>
			{held.data ? <SettingsForm held={held.data} /> : <Skeleton />}
		</Page>
	)
}
