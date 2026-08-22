// SPDX-License-Identifier: Apache-2.0

import { useSession } from '@gopherium/react-auth'
import { Navigate, Outlet, useMatches } from '@tanstack/react-router'

import { can } from '@gophenberg/frontend-sdk'

/**
 * Renders the nested screen when the session holds every capability the route declares,
 * and sends the reader home otherwise, a route declaring none also refused.
 * @returns The nested screen, or the redirect standing in for it.
 */
export function CapabilityGate() {
	const role = useSession().data?.role
	const asked = useMatches().flatMap((match) => match.staticData.capability ?? [])
	if (asked.length === 0 || !asked.every((capability) => can(role, capability))) {
		return <Navigate to="/" replace />
	}
	return <Outlet />
}
