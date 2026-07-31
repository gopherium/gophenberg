// SPDX-License-Identifier: Apache-2.0

import {
	createRootRoute,
	createRoute,
	createRouter,
} from '@tanstack/react-router'
import type { RouterHistory } from '@tanstack/react-router'

import { Home } from './Home'
import { Layout } from './Layout'
import { plugins } from './plugins'
import { PostsSidebar } from './posts/PostsSidebar'

const rootRoute = createRootRoute({
	component: Layout,
})

const homeRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: '/',
	component: Home,
})

const postsRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: '/posts',
	staticData: { Sidebar: PostsSidebar },
}).lazy(() => import('./posts/postsRoutes.lazy').then((module) => module.PostsLazyRoute))

const usersRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: '/users',
}).lazy(() => import('./userRoutes.lazy').then((module) => module.UsersLazyRoute))

const newUserRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: '/users/new',
}).lazy(() => import('./userRoutes.lazy').then((module) => module.NewUserLazyRoute))

const routeTree = rootRoute.addChildren([
	homeRoute,
	postsRoute,
	usersRoute,
	newUserRoute,
	...plugins.flatMap((plugin) => plugin.routes(rootRoute)),
])

/**
 * Creates the application router with the assembled route tree.
 * @param history - Optional router history instance for controlling navigation state.
 * @returns The configured TanStack router bound to the route tree.
 */
export function createAppRouter(history?: RouterHistory) {
	return createRouter({ routeTree, history })
}

declare module '@tanstack/react-router' {
	interface Register {
		router: ReturnType<typeof createAppRouter>
	}
}
