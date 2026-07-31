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
}).lazy(() => import('./posts/postsRoutes.lazy').then((module) => module.PostsLazyRoute))

const usersRoute = createRoute({
	getParentRoute: () => framedRoute,
	path: '/users',
}).lazy(() => import('./userRoutes.lazy').then((module) => module.UsersLazyRoute))

const newUserRoute = createRoute({
	getParentRoute: () => framedRoute,
	path: '/users/new',
}).lazy(() => import('./userRoutes.lazy').then((module) => module.NewUserLazyRoute))

const editorRoute = createRoute({
	getParentRoute: () => rootRoute,
	path: '/posts/$postId/edit',
}).lazy(() => import('./posts/postsRoutes.lazy').then((module) => module.EditorLazyRoute))

const routeTree = rootRoute.addChildren([
	framedRoute.addChildren([
		homeRoute,
		postsRoute,
		usersRoute,
		newUserRoute,
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
	return createRouter({ routeTree, history })
}

declare module '@tanstack/react-router' {
	interface Register {
		router: ReturnType<typeof createAppRouter>
	}
}
