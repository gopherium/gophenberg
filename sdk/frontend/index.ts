// SPDX-License-Identifier: Apache-2.0

import type { AnyRoute } from '@tanstack/react-router'
import type { ComponentProps, ComponentType, ReactElement } from 'react'

export type CanvasMode = 'padded' | 'bleed'

declare module '@tanstack/react-router' {
	interface StaticDataRouteOption {
		Sidebar?: ComponentType
		canvas?: CanvasMode
	}
}

export interface NavItem {
	label: string
	to: string
	icon: ReactElement<ComponentProps<'svg'>>
}

export interface FrontendPlugin {
	id: string
	routes: (parent: AnyRoute) => AnyRoute[]
	nav: NavItem[]
}

export {
	AlertDialog,
	Badge,
	Button,
	Icon,
	Notice,
	Stack,
	Text,
} from '@wordpress/ui'
export { ThemeProvider } from '@wordpress/theme'
export { SidebarNavigationScreen } from './SidebarNavigationScreen'
