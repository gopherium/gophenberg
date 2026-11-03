// SPDX-License-Identifier: Apache-2.0

import { MANAGE_SETTINGS, SelectControl, Stack, Text, can } from '@gophenberg/frontend-sdk'
import { __, sprintf } from '@wordpress/i18n'
import { Page, useToaster } from '@gopherium/godmin'
import { useSession } from '@gopherium/react-auth'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'


import { chooseLocale, chooseSiteLocale, fetchLocale, fetchSiteLocale } from './api'
import { DEFAULT_LOCALE } from './catalog'

/** What the site default select offers when the site follows the reader's browser. */
const followBrowser = { label: __('Follow the browser', 'gophenberg'), value: '' }

/** The query naming the language the admin reads in. */
const localeQueryKey = ['locale']

/** The query naming the language the site chose for itself. */
const siteLocaleQueryKey = ['site-locale']

/**
 * Returns the options a language select offers.
 * @param supported - The languages the site answers in.
 * @returns The options to render.
 */
function optionsOf(supported: string[]): { label: string, value: string }[] {
	return supported.map((locale) => ({ label: locale, value: locale }))
}

/**
 * Renders the screen where a reader picks their language and an administrator
 * picks the one the site answers in.
 * @returns The language screen element.
 */
export function LanguageScreen() {
	const client = useQueryClient()
	const session = useSession().data
	const toaster = useToaster()
	const answered = useQuery({ queryKey: localeQueryKey, queryFn: fetchLocale })
	const site = useQuery({ queryKey: siteLocaleQueryKey, queryFn: fetchSiteLocale })
	const supported = answered.data?.supported ?? [DEFAULT_LOCALE]
	const mine = useMutation({
		mutationFn: chooseLocale,
		onSuccess: () => client.invalidateQueries({ queryKey: localeQueryKey }),
		onError: (err: Error) =>
			toaster.show(sprintf(__('Could not save your language. %s', 'gophenberg'), err.message)),
	})
	const theirs = useMutation({
		mutationFn: chooseSiteLocale,
		onSuccess: () => client.invalidateQueries({ queryKey: siteLocaleQueryKey }),
		onError: (err: Error) =>
			toaster.show(sprintf(__('Could not save the site default. %s', 'gophenberg'), err.message)),
	})
	const options = optionsOf(supported)
	const chosen = options.find((held) => held.value === answered.data?.locale) ?? options[0]
	const siteOptions = [followBrowser, ...options]
	const siteChosen = siteOptions.find((held) => held.value === site.data) ?? followBrowser
	return (
		<Page
			title={__('Language', 'gophenberg')}
			headingLevel={2}
			subtitle={__('Choose the language the admin reads in.', 'gophenberg')}
		>
			<Stack direction="column" gap="md">
				<SelectControl
					label={__('Your language', 'gophenberg')}
					items={options}
					value={chosen}
					onValueChange={(item) => item?.value != null && mine.mutate(item.value)}
				/>
				<Text variant="body-sm">
					{__('A reload shows the admin in the language you chose.', 'gophenberg')}
				</Text>
				{can(session, MANAGE_SETTINGS) && (
					<SelectControl
						label={__('The site default', 'gophenberg')}
						items={siteOptions}
						value={siteChosen}
						placeholder={followBrowser.label}
						onValueChange={(item) => item?.value != null && theirs.mutate(item.value)}
					/>
				)}
			</Stack>
		</Page>
	)
}
