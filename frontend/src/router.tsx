// SPDX-License-Identifier: Apache-2.0

import { LoadingScreen } from '@gopherium/godmin'
import {
	createRootRoute,
	createRoute,
	createRouter,
	lazyRouteComponent,
} from '@tanstack/react-router'
import type { RouterHistory } from '@tanstack/react-router'

import { adminBasepath } from './basepath'
import { Home } from './Home'
import { Layout } from './Layout'
import { plugins } from './plugins'
import { PostsSidebar } from './posts/PostsSidebar'

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

const postsRoute = createRoute({
	getParentRoute: () => framedRoute,
	path: '/posts',
	staticData: { Sidebar: PostsSidebar },
	component: lazyRouteComponent(() => import('./posts/PostsScreen'), 'PostsScreen'),
})

const usersRoute = createRoute({
	getParentRoute: () => framedRoute,
	path: '/users',
	component: lazyRouteComponent(() => import('./users/UsersScreen'), 'UsersScreen'),
})

const newUserRoute = createRoute({
	getParentRoute: () => framedRoute,
	path: '/users/new',
	component: lazyRouteComponent(() => import('./users/NewUserScreen'), 'NewUserScreen'),
})

const mediaRoute = createRoute({
	getParentRoute: () => framedRoute,
	path: '/media',
	component: lazyRouteComponent(() => import('./media/MediaScreen'), 'MediaScreen'),
})

const themesRoute = createRoute({
	getParentRoute: () => framedRoute,
	path: '/themes',
	component: lazyRouteComponent(() => import('./themes/ThemesScreen'), 'ThemesScreen'),
})

const editorRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: '/posts/$postId/edit',
	component: lazyRouteComponent(() => import('./posts/EditorScreen'), 'EditorScreen'),
})

const routeTree = rootRoute.addChildren([
	framedRoute.addChildren([
		homeRoute,
		postsRoute,
		mediaRoute,
		usersRoute,
		newUserRoute,
		themesRoute,
		...plugins.flatMap((plugin) => plugin.routes(framedRoute)),
	]),
	editorRoute,
])

/**
 * Renders the ghost a route shows while its chunk or data arrives.
 * @returns The pending element.
 */
function RoutePending() {
	return <LoadingScreen label="Loading the screen." />
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
