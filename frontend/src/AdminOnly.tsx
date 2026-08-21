// SPDX-License-Identifier: Apache-2.0

import { useSession } from '@gopherium/react-auth'
import { Navigate, Outlet } from '@tanstack/react-router'

import { isAdmin } from './users/ranks'

/**
 * Renders the administration screens for an administrator, and sends every other rank home.
 * @returns The nested screen, or the redirect standing in for it.
 */
export function AdminOnly() {
	const rank = useSession().data?.rank
	if (!isAdmin(rank)) {
		return <Navigate to="/" replace />
	}
	return <Outlet />
}
