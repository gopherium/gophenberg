// SPDX-License-Identifier: Apache-2.0

import type { NavItem } from '@gophenberg/frontend-sdk'
import { usersNavItem } from '@gopherium/react-auth/wpds'

import { mediaNavItem } from '../media/nav'
import { postsNavItem } from '../posts/nav'
import { themesNavItem } from '../themes/nav'

export const coreNav: NavItem[] = [postsNavItem, mediaNavItem, themesNavItem, usersNavItem]
