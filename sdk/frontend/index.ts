// SPDX-License-Identifier: Apache-2.0

import type { AnyRoute } from '@tanstack/react-router'
import type { ComponentProps, ComponentType, ReactElement } from 'react'

import type { Capability } from './capabilities'

export type { CanvasMode } from '@gopherium/godmin'

declare module '@tanstack/react-router' {
	interface StaticDataRouteOption {
		Sidebar?: ComponentType
		capability?: Capability
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
	chevronDown as downIcon,
	listView as listViewIcon,
	redo as redoIcon,
	undo as undoIcon,
	chevronUp as upIcon,
} from '@wordpress/icons'
export { TextareaControl } from './TextareaControl'
export { RangeControl } from '@wordpress/components'
export {
	ADMIN,
	AUTHOR,
	CHANGE_OTHERS_WORK,
	EDITOR,
	MANAGE_SETTINGS,
	MANAGE_THEMES,
	MANAGE_TYPES,
	MANAGE_USERS,
	can,
	sessionMayChange,
} from './capabilities'
export { useSession } from '@gopherium/react-auth'
export type { User as Session } from '@gopherium/react-auth'
