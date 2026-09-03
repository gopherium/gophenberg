// SPDX-License-Identifier: Apache-2.0

import { LoadingScreen } from '@gopherium/godmin'
import {
	createRootRoute,
	createRoute,
	createRouter,
	lazyRouteComponent,
} from '@tanstack/react-router'
import type { RouterHistory } from '@tanstack/react-router'
import { __ } from '@wordpress/i18n'

import { MANAGE_SETTINGS, MANAGE_THEMES, MANAGE_TYPES, MANAGE_USERS } from '@gophenberg/frontend-sdk'

import { adminBasepath } from './basepath'
import { CapabilityGate } from './CapabilityGate'
import { Home } from './Home'
import { Layout } from './Layout'
import { plugins } from './plugins'
import { ContentSidebar } from './content/ContentSidebar'

const rootRoute = createRootRoute()

const framedRoute = createRoute({
	getParentRoute: () => rootRoute,
	id: 'framed',
	component: Layout,
})

const homeRoute = createRoute({
	getParentRoute: () => framedRoute,
	path: '/',
	component: Home,
})

const contentRoute = createRoute({
	getParentRoute: () => framedRoute,
	path: '/content/$typeKey',
	staticData: { Sidebar: ContentSidebar },
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
		]),
		...plugins.flatMap((plugin) => plugin.routes(framedRoute)),
	]),
	editorRoute,
])

/**
 * Renders the ghost a route shows while its chunk or data arrives.
 * @returns The pending element.
 */
function RoutePending() {
	return <LoadingScreen label={__('Loading the screen.', 'gophenberg')} />
}

/**
 * Creates the application router with the assembled route tree.
 * @param history - Optional router history instance for controlling navigation state.
 * @returns The configured TanStack router bound to the route tree.
 */
export function createAppRouter(history?: RouterHistory) {
	return createRouter({
		routeTree,
		history,
		basepath: adminBasepath,
		defaultPendingComponent: RoutePending,
		defaultPendingMs: 200,
		defaultPendingMinMs: 0,
	})
}

declare module '@tanstack/react-router' {
	interface Register {
		router: ReturnType<typeof createAppRouter>
	}
}
