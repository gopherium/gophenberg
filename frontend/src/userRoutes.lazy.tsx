// SPDX-License-Identifier: Apache-2.0

import { createLazyRoute } from '@tanstack/react-router'

import { NewUserScreen } from './users/NewUserScreen'
import { UsersScreen } from './users/UsersScreen'

export const UsersLazyRoute = createLazyRoute('/users')({
	component: UsersScreen,
})

export const NewUserLazyRoute = createLazyRoute('/users/new')({
	component: NewUserScreen,
})
