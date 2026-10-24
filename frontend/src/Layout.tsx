// SPDX-License-Identifier: Apache-2.0

import { Text, can } from '@gophenberg/frontend-sdk'
import { Frame } from '@gopherium/godmin'
import { Navigation, useCanvas, useFrameLocation } from '@gopherium/godmin/router'
import { useSession } from '@gopherium/react-auth'
import { AccountPanel } from '@gopherium/react-auth/wpds'
import { Link, Outlet } from '@tanstack/react-router'
import { __, sprintf } from '@wordpress/i18n'

import { useContentNav, useDefaultType } from './content/nav'
import { coreNav } from './menu/coreNav'
import { plugins } from './plugins'
import { useUpdatesMenuItem } from './updates/nav'
import { UpdateNag } from './updates/UpdateNag'
import { useAppVersion } from './version'

/**
 * Returns the link to the list the root lands on, where Back on a sub menu goes.
 * @returns The default type's list once the types answer, else the root, which waits on them or reports them.
 */
function useHome() {
	const { key } = useDefaultType()
	return key === undefined ? <Link to="/" /> : <Link to="/content/$typeKey" params={{ typeKey: key }} />
}

/**
 * Renders the admin layout framing the active route.
 * @returns The layout element.
 */
export function Layout() {
	const session = useSession().data
	const version = useAppVersion().data
	return (
		<Frame.Root location={useFrameLocation()}>
			<Frame.Sidebar
				brand={<Text variant="heading-lg">{'Gophenberg'}</Text>}
				footer={<AccountPanel />}
				footerNote={version ? sprintf(__('v%(version)s', 'gophenberg'), { version }) : undefined}
			>
				<Navigation
					title={'Gophenberg'}
					items={[...useContentNav(), ...coreNav, useUpdatesMenuItem(), ...plugins.flatMap((plugin) => plugin.nav)]}
					home={useHome()}
					can={(capability) => can(session, capability)}
				/>
			</Frame.Sidebar>
			<Frame.Canvas canvas={useCanvas()} notices={<UpdateNag />}>
				<Outlet />
			</Frame.Canvas>
		</Frame.Root>
	)
}
