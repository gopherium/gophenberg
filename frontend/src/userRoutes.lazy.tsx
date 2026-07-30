// SPDX-License-Identifier: Apache-2.0

import { createLazyRoute } from '@tanstack/react-router'

import { NewUserRoute, UsersRoute } from './userRoutes'

export const UsersLazyRoute = createLazyRoute('/users')({
	component: UsersRoute,
})

export const NewUserLazyRoute = createLazyRoute('/users/new')({
	component: NewUserRoute,
})
