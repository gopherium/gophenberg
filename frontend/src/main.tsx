// SPDX-License-Identifier: Apache-2.0

import { AdminRoot, Toaster } from '@gopherium/godmin'
import { Text } from '@gophenberg/frontend-sdk'
import { AuthGate, createAuthQueryClient } from '@gopherium/react-auth'
import { LoginScreen } from '@gopherium/react-auth/wpds'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import '@gopherium/godmin/base.css'
import '@gopherium/react-auth/wpds/style.css'
import './index.css'
import { BootLoading } from './boot'
import { createAppRouter } from './router'

const queryClient = createAuthQueryClient()
const router = createAppRouter()

createRoot(document.getElementById('root')!).render(
	<StrictMode>
		<QueryClientProvider client={queryClient}>
			<AdminRoot>
				<AuthGate
					loginScreen={(onLogin) => (
						<LoginScreen brand="Gophenberg" onLogin={onLogin} />
					)}
					loading={<BootLoading />}
					error={<Text role="alert">Something went wrong.</Text>}
				>
					<Toaster>
						<RouterProvider router={router} />
					</Toaster>
				</AuthGate>
			</AdminRoot>
		</QueryClientProvider>
	</StrictMode>,
)
