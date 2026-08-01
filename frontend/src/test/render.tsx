// SPDX-License-Identifier: Apache-2.0

import { SnackbarProvider } from '@gophenberg/frontend-sdk'
import { createAuthQueryClient } from '@gopherium/react-auth'
import type { User } from '@gopherium/react-auth'
import { defaultUser, seedSession } from '@gopherium/react-auth/testing'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createMemoryHistory } from '@tanstack/react-router'
import { render } from '@testing-library/react'

import { createAppRouter } from '../router'
import { versionQueryKey } from '../version'

export function renderAt(
	path: string,
	user: User | null = defaultUser,
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
		createMemoryHistory({ initialEntries: [path] }),
	)
	render(
		<QueryClientProvider client={client}>
			<SnackbarProvider>
				<RouterProvider router={router} />
			</SnackbarProvider>
		</QueryClientProvider>,
	)
	return client
}
