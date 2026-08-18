// SPDX-License-Identifier: Apache-2.0

import { Frame } from '@gopherium/godmin'
import { __ } from '@wordpress/i18n'
import { useCanvas, useFrameLocation } from '@gopherium/godmin/router'
import { Outlet } from '@tanstack/react-router'

import { RailContent } from './RailContent'

const CHROME_COLOR = { background: '#1e1e1e' }
const CANVAS_COLOR = { background: '#ffffff' }

/**
 * Renders the admin layout framing the active route.
 * @returns The layout element.
 */
export function Layout() {
	return (
		<Frame.Root
			location={useFrameLocation()}
			chromeColor={CHROME_COLOR}
			canvasColor={CANVAS_COLOR}
		>
			<Frame.Rail menuLabel={__('Open navigation', 'gophenberg')}>
				<RailContent />
			</Frame.Rail>
			<Frame.Canvas canvas={useCanvas()}>
				<Outlet />
			</Frame.Canvas>
		</Frame.Root>
	)
}
