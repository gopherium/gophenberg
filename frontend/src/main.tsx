// SPDX-License-Identifier: Apache-2.0

import { AdminRoot } from '@gopherium/godmin'
import { Text } from '@gophenberg/frontend-sdk'
import { AuthGate, createAuthQueryClient } from '@gopherium/react-auth'
import { LoginScreen } from '@gopherium/react-auth/wpds'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from '@tanstack/react-router'
import { __ } from '@wordpress/i18n'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'

import '@gopherium/godmin/base.css'
import '@gopherium/react-auth/wpds/style.css'
import './index.css'
import { BootLoading } from './boot'
import { startLocale } from './i18n/start'
import { createAppRouter } from './router'
import { AdminToaster } from './toasts'

await startLocale()

const queryClient = createAuthQueryClient()
const router = createAppRouter()

/**
 * Renders the message the auth gate shows when the session cannot be loaded.
 * @returns The boot error element.
 */
function BootError() {
	return <Text role="alert">{__('Something went wrong.', 'gophenberg')}</Text>
}

createRoot(document.getElementById('root')!).render(
	<StrictMode>
		<QueryClientProvider client={queryClient}>
			<AdminRoot>
				<AuthGate
					loginScreen={(onLogin) => (
						<LoginScreen brand="Gophenberg" onLogin={onLogin} />
					)}
					loading={<BootLoading />}
					error={<BootError />}
				>
					<AdminToaster>
						<RouterProvider router={router} />
					</AdminToaster>
				</AuthGate>
			</AdminRoot>
		</QueryClientProvider>
	</StrictMode>,
)
