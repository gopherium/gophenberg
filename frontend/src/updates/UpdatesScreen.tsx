// SPDX-License-Identifier: Apache-2.0

import { Skeleton, Stack, Text } from '@gophenberg/frontend-sdk'
import { ErrorNotice, Page } from '@gopherium/godmin'
import { __, sprintf } from '@wordpress/i18n'

import { CheckLines } from './CheckLines'
import { BackupsLine, CoreOffer } from './CoreOffer'
import { useUpdatesScreen } from './useUpdatesScreen'
import type { UpdatesScreenState } from './useUpdatesScreen'

/**
 * Returns what the screen shows under its title: the checks, the policy, the offer and the backups.
 * @param state - The screen's state.
 * @returns The body element.
 */
function updatesBody(state: UpdatesScreenState) {
	const { overview } = state
	if (state.unread) {
		return (
			<ErrorNotice>
				{__('The updates could not be read, so nothing can be updated here yet.', 'gophenberg')}
			</ErrorNotice>
		)
	}
	if (overview === undefined) {
		return <Skeleton />
	}
	return (
		<Stack direction="column" gap="lg">
			<CheckLines overview={overview} />
			<Text render={<p />}>
				{__('This site will not receive automatic updates for new versions of Gophenberg.', 'gophenberg')}
			</Text>
			<CoreOffer overview={overview} view={state.view} onStart={state.start} onDismiss={state.dismiss} />
			<BackupsLine backups={overview.backups} />
		</Stack>
	)
}

/**
 * Renders the Updates screen, where an update_core holder checks for and installs a new version of Gophenberg.
 * @returns The page element.
 */
export function UpdatesScreen() {
	const state = useUpdatesScreen()
	const running = state.overview?.running
	return (
		<Page
			headingLevel={2}
			title={__('Gophenberg Updates', 'gophenberg')}
			subtitle={
				running === undefined
					? undefined
					: sprintf(__('Current version: %(version)s', 'gophenberg'), { version: running })
			}
		>
			{updatesBody(state)}
		</Page>
	)
}
