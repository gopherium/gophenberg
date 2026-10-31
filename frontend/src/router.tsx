// SPDX-License-Identifier: Apache-2.0

import { ErrorNotice, LoadingScreen, Page } from '@gopherium/godmin'
import { listSearch, routerDefaults } from '@gopherium/godmin/router'
import {
	Navigate,
	createRootRoute,
	createRoute,
	createRouter,
	lazyRouteComponent,
} from '@tanstack/react-router'
import type { RouterHistory } from '@tanstack/react-router'
import { __ } from '@wordpress/i18n'

import { MANAGE_SETTINGS, MANAGE_THEMES, MANAGE_TYPES, MANAGE_USERS, UPDATE_CORE } from '@gophenberg/frontend-sdk'

import { adminBasepath } from './basepath'
import { CapabilityGate } from './CapabilityGate'
import { Layout } from './Layout'
import { plugins } from './plugins'
import { useDefaultType } from './content/nav'
import { statusSearch } from './content/StatusTabs'

const rootRoute = createRootRoute()

const framedRoute = createRoute({
	getParentRoute: () => rootRoute,
	id: 'framed',
	component: Layout,
})

/**
 * Sends the root to the list of the default content type, replacing the root in the history.
 * @returns The redirect once a default type is held, else the registry failure under a title, or the loading spinner.
 */
function DefaultList() {
	const landing = useDefaultType()
	if (landing.key !== undefined) {
		return <Navigate to="/content/$typeKey" params={{ typeKey: landing.key }} replace />
	}
	if (landing.failed) {
		return (
			<Page title={__('Content', 'gophenberg')} headingLevel={2}>
				<ErrorNotice>{__('The content types could not be loaded.', 'gophenberg')}</ErrorNotice>
			</Page>
		)
	}
	return <LoadingScreen label={__('Loading the screen.', 'gophenberg')} />
}

const homeRoute = createRoute({
	getParentRoute: () => framedRoute,
	path: '/',
	component: DefaultList,
})

const contentRoute = createRoute({
	getParentRoute: () => framedRoute,
	path: '/content/$typeKey',
	validateSearch: (raw: Record<string, unknown>) => ({ ...listSearch(raw), ...statusSearch(raw) }),
	component: lazyRouteComponent(() => import('./content/PostsScreen'), 'PostsScreen'),
})

const adminRoute = createRoute({
	getParentRoute: () => framedRoute,
	id: 'admin',
	component: CapabilityGate,
})

const contentTypesRoute = createRoute({
	getParentRoute: () => adminRoute,
	path: '/content-types',
	validateSearch: listSearch,
	staticData: { capability: MANAGE_TYPES },
	component: lazyRouteComponent(() => import('./content/TypesScreen'), 'TypesScreen'),
})

const fieldGroupsRoute = createRoute({
	getParentRoute: () => adminRoute,
	path: '/field-groups',
	staticData: { capability: MANAGE_TYPES },
	component: lazyRouteComponent(() => import('./content/GroupsScreen'), 'GroupsScreen'),
})

const usersRoute = createRoute({
	getParentRoute: () => adminRoute,
	path: '/users',
	validateSearch: listSearch,
	staticData: { capability: MANAGE_USERS },
	component: lazyRouteComponent(() => import('./users/UsersScreen'), 'UsersScreen'),
})

const newUserRoute = createRoute({
	getParentRoute: () => adminRoute,
	path: '/users/new',
	staticData: { capability: MANAGE_USERS },
	component: lazyRouteComponent(() => import('./users/NewUserScreen'), 'NewUserScreen'),
})

const mediaRoute = createRoute({
	getParentRoute: () => framedRoute,
	path: '/media',
	validateSearch: listSearch,
	component: lazyRouteComponent(() => import('./media/MediaScreen'), 'MediaScreen'),
})

const languageRoute = createRoute({
	getParentRoute: () => framedRoute,
	path: '/language',
	component: lazyRouteComponent(() => import('./i18n/LanguageScreen'), 'LanguageScreen'),
})

const themesRoute = createRoute({
	getParentRoute: () => adminRoute,
	path: '/themes',
	staticData: { capability: MANAGE_THEMES },
	component: lazyRouteComponent(() => import('./themes/ThemesScreen'), 'ThemesScreen'),
})

const settingsRoute = createRoute({
	getParentRoute: () => adminRoute,
	path: '/settings',
	staticData: { capability: MANAGE_SETTINGS },
	component: lazyRouteComponent(() => import('./settings/SettingsScreen'), 'SettingsScreen'),
})

const updatesRoute = createRoute({
	getParentRoute: () => adminRoute,
	path: '/update-core',
	staticData: { capability: UPDATE_CORE },
	component: lazyRouteComponent(() => import('./updates/UpdatesScreen'), 'UpdatesScreen'),
})

const editorRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: '/content/$typeKey/$postId/edit',
	component: lazyRouteComponent(() => import('./content/EditorScreen'), 'EditorScreen'),
})

const routeTree = rootRoute.addChildren([
	framedRoute.addChildren([
		homeRoute,
		contentRoute,
		mediaRoute,
		languageRoute,
		adminRoute.addChildren([
			contentTypesRoute,
			fieldGroupsRoute,
			usersRoute,
			newUserRoute,
			themesRoute,
			settingsRoute,
			updatesRoute,
		]),
		...plugins.flatMap((plugin) => plugin.routes(framedRoute)),
	]),
	editorRoute,
])

/**
 * Creates the application router with the assembled route tree.
 * @param history - Optional router history instance for controlling navigation state.
 * @returns The configured TanStack router bound to the route tree.
 */
export function createAppRouter(history?: RouterHistory) {
	return createRouter({
		...routerDefaults(),
		routeTree,
		history,
		basepath: adminBasepath,
	})
}

declare module '@tanstack/react-router' {
	interface Register {
		router: ReturnType<typeof createAppRouter>
	}
}
