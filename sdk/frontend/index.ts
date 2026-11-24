// SPDX-License-Identifier: Apache-2.0

import type { MenuItem } from '@gopherium/godmin/router'
import type { AnyRoute } from '@tanstack/react-router'

export type {
	BulkRun,
	BulkWords,
	CanvasMode,
	ConfirmDialogProps,
	DialogAnswer,
	DialogRun,
	FormDialogProps,
	ListRun,
	ListRunState,
	RenameDialogProps,
	RunSettled,
	ToasterHandle,
	Trigger,
	UnsavedText,
} from '@gopherium/godmin'
export {
	ConfirmDialog,
	FormDialog,
	ListEmpty,
	ListNoResults,
	MenuCount,
	NavigationScreen,
	RenameDialog,
	focusInDialog,
	focusedTrigger,
	markLatest,
	useBulkRun,
	useDialogRun,
	useFocusBack,
	useListRun,
	useToaster,
	useUnsavedText,
	viewNarrows,
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
	Checkbox,
	Dialog,
	Icon,
	IconButton,
	InputControl,
	Link,
	Notice,
	Popover,
	SelectControl,
	Skeleton,
	Spinner,
	Stack,
	Tabs,
	Text,
	Tooltip,
	ValidatedInputControl,
	ValidatedTextareaControl,
	VisuallyHidden,
} from '@wordpress/ui'
export {
	audio,
	backup,
	brush,
	category,
	check,
	chevronDown,
	chevronLeft,
	chevronUp,
	closeSmall,
	cloudUpload,
	commentAuthorAvatar,
	customPostType,
	drawerLeft,
	drawerRight,
	file,
	image,
	language,
	list,
	listView,
	media,
	notAllowed,
	page,
	plugins,
	post,
	redo,
	settings,
	trash,
	undo,
	update,
	upload,
	video,
} from '@wordpress/icons'
export { TextareaControl } from './TextareaControl'
export { CheckboxControl, DropZone, RadioControl, RangeControl } from '@wordpress/components'
export { speak } from '@wordpress/a11y'
export {
	ACCESS_ADMIN,
	ACTIVATE_PLUGINS,
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
