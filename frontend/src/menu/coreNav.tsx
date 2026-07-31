// SPDX-License-Identifier: Apache-2.0

import type { NavItem } from '@gophenberg/frontend-sdk'
import { usersNavItem } from '@gopherium/react-auth/wpds'

import { postsNavItem } from '../posts/nav'

export const coreNav: NavItem[] = [postsNavItem, usersNavItem]
