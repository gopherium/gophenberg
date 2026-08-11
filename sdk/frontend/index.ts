// SPDX-License-Identifier: Apache-2.0

import type { AnyRoute } from '@tanstack/react-router'
import type { ComponentProps, ComponentType, ReactElement } from 'react'

export type { CanvasMode } from '@gopherium/godmin'

declare module '@tanstack/react-router' {
	interface StaticDataRouteOption {
		Sidebar?: ComponentType
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
	Dialog,
	Icon,
	IconButton,
	InputControl,
	Notice,
	SelectControl,
	Skeleton,
	Stack,
	Tabs,
	Text,
} from '@wordpress/ui'
export {
	chevronLeft as backIcon,
	listView as listViewIcon,
	redo as redoIcon,
	undo as undoIcon,
} from '@wordpress/icons'
export { TextareaControl } from './TextareaControl'
