// SPDX-License-Identifier: Apache-2.0

import { createAuthQueryClient } from '@gopherium/react-auth'
import type { User } from '@gopherium/react-auth'
import { defaultUser, seedSession } from '@gopherium/react-auth/testing'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createMemoryHistory } from '@tanstack/react-router'
import { render } from '@testing-library/react'

import { adminBasepath } from '../basepath'
import { createAppRouter } from '../router'
import { AdminToaster } from '../toasts'
import { versionQueryKey } from '../version'

/** The account tests render as, an administrator unless a test names another role. */
export const adminUser: User = { ...defaultUser, role: 'admin' }

/** Renders the application at an app-relative path with a seeded session. */
export function renderAt(
	path: string,
	user: User | null = adminUser,
	version: string | null = '0.0.0',
) {
	return renderRoutedAt(path, user, version).client
}

/**
 * Renders the admin at the path and returns the query client and the router behind it.
 * @param path - The address to render at.
 * @param user - The signed in user, none for a signed out admin.
 * @param version - The version the admin reports, none to leave it unasked.
 * @returns The query client and the router.
 */
export function renderRoutedAt(
	path: string,
	user: User | null = adminUser,
	version: string | null = '0.0.0',
) {
	const client = createAuthQueryClient({
		queries: { retry: false, staleTime: Infinity },
	})
	seedSession(client, user)
	if (version !== null) {
		client.setQueryData(versionQueryKey, version)
	}
	const router = createAppRouter(
		createMemoryHistory({ initialEntries: [adminBasepath + path] }),
	)
	render(
		<QueryClientProvider client={client}>
			<AdminToaster>
				<RouterProvider router={router} />
			</AdminToaster>
		</QueryClientProvider>,
	)
	return { client, router }
}
