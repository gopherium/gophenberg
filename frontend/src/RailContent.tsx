// SPDX-License-Identifier: Apache-2.0

import { Stack, Text } from '@gophenberg/frontend-sdk'
import { AccountPanel } from '@gopherium/react-auth/wpds'
import { Link, useRouterState } from '@tanstack/react-router'
import { __ } from '@wordpress/i18n'

import { MainMenu } from './menu/MainMenu'
import { useAppVersion } from './version'

/**
 * Renders the navigation region's contents.
 * @returns The rail content element.
 */
export function RailContent() {
	const matches = useRouterState({ select: (state) => state.matches })
	const Sidebar = [...matches].reverse().find((match) => match.staticData.Sidebar)
		?.staticData.Sidebar
	const version = useAppVersion().data
	return (
		<>
			<Stack direction="column" gap="lg">
				<Link to="/" className="gophenberg-rail__brand">
					<Text variant="heading-lg">{'Gophenberg'}</Text>
				</Link>
				<nav aria-label={__('Navigation', 'gophenberg')}>
					{Sidebar ? <Sidebar /> : <MainMenu />}
				</nav>
			</Stack>
			<AccountPanel className="gophenberg-rail__account" />
			{version ? (
				<Text className="gophenberg-rail__version">
					v{version}
				</Text>
			) : null}
		</>
	)
}
