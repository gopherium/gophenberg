// SPDX-License-Identifier: Apache-2.0

import { Stack, Text, ThemeProvider } from '@gophenberg/frontend-sdk'
import { AccountPanel } from '@gopherium/react-auth/wpds'
import { Link, Outlet, useRouterState } from '@tanstack/react-router'

import { MainMenu } from './menu/MainMenu'
import { useAppVersion } from './version'

const CHROME_COLOR = { background: '#1e1e1e' }
const CANVAS_COLOR = { background: '#ffffff' }

/**
 * Renders the admin layout: a dark navigation chrome holding the branding and
 * either the main menu or the active section's sidebar screen, wrapped around a
 * light canvas showing the active route.
 * @returns The layout element framing the current route.
 */
export function Layout() {
	const matches = useRouterState({ select: (state) => state.matches })
	const sidebarMatch = [...matches]
		.reverse()
		.find((match) => match.staticData.Sidebar)
	const Sidebar = sidebarMatch?.staticData.Sidebar
	const version = useAppVersion().data
	return (
		<ThemeProvider color={CHROME_COLOR}>
			<div className="gophenberg-layout">
				<div className="gophenberg-layout__sidebar">
					<Stack direction="column" gap="lg">
						<Link to="/" className="gophenberg-layout__brand">
							<Text variant="heading-lg" render={<h1 />}>
								Gophenberg
							</Text>
						</Link>
						<nav aria-label="Navigation">
							{Sidebar ? <Sidebar /> : <MainMenu />}
						</nav>
					</Stack>
					<AccountPanel className="gophenberg-layout__account" />
					{version ? (
						<Text className="gophenberg-layout__version">v{version}</Text>
					) : null}
				</div>
				<ThemeProvider color={CANVAS_COLOR}>
					<main className="gophenberg-layout__canvas">
						<Outlet />
					</main>
				</ThemeProvider>
			</div>
		</ThemeProvider>
	)
}
