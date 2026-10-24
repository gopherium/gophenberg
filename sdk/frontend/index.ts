// SPDX-License-Identifier: Apache-2.0

import type { MenuItem } from '@gopherium/godmin/router'
import type { AnyRoute } from '@tanstack/react-router'

export type {
	BulkRun,
	BulkWords,
	CanvasMode,
	ConfirmBodyProps,
	FormBodyProps,
	FormKind,
	ListRun,
	ListRunState,
	RenameBodyProps,
	RunConfirmProps,
	RunSettled,
	ToasterHandle,
} from '@gopherium/godmin'
export {
	ConfirmBody,
	FormBody,
	MenuCount,
	NavigationScreen,
	RenameBody,
	RunConfirm,
	useBulkRun,
	useListRun,
	useToaster,
} from '@gopherium/godmin'
export type { MenuItem } from '@gopherium/godmin/router'
export { DrilldownItem, NavigationItem, NavigationList } from '@gopherium/godmin/router'

export interface FrontendPlugin {
	id: string
	routes: (parent: AnyRoute) => AnyRoute[]
	nav: MenuItem[]
}

export {
	AlertDialog,
	Badge,
	Button,
	Icon,
	IconButton,
	InputControl,
	Link,
	Notice,
	SelectControl,
	Skeleton,
	Spinner,
	Stack,
	Tabs,
	Text,
	VisuallyHidden,
} from '@wordpress/ui'
export {
	commentAuthorAvatar as authorIcon,
	chevronLeft as backIcon,
	backup as backupIcon,
	brush as brushIcon,
	category as categoryIcon,
	closeSmall as closeIcon,
	customPostType as customPostTypeIcon,
	chevronDown as downIcon,
	language as languageIcon,
	list as listIcon,
	listView as listViewIcon,
	media as mediaIcon,
	page as pageIcon,
	post as postIcon,
	redo as redoIcon,
	cloudUpload as saveIcon,
	settings as settingsIcon,
	drawerRight as sidebarIcon,
	drawerLeft as sidebarRtlIcon,
	trash as trashIcon,
	undo as undoIcon,
	chevronUp as upIcon,
	update as updateIcon,
	upload as uploadIcon,
} from '@wordpress/icons'
export { Dialog } from './dialog'
export { TextareaControl } from './TextareaControl'
export {
	CheckboxControl,
	DropZone,
	RadioControl,
	RangeControl,
	__experimentalStyleProvider as StyleProvider,
} from '@wordpress/components'
export { speak } from '@wordpress/a11y'
export {
	ACCESS_ADMIN,
	ADMIN,
	AUTHOR,
	CHANGE_OTHERS_WORK,
	EDITOR,
	MANAGE_SETTINGS,
	MANAGE_THEMES,
	MANAGE_TYPES,
	MANAGE_USERS,
	MEMBER,
	UPDATE_CORE,
	sessionMayChange,
} from './capabilities'
export { can, useSession } from '@gopherium/react-auth'
export type { User as Session } from '@gopherium/react-auth'
