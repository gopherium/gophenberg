// SPDX-License-Identifier: Apache-2.0

declare module '@wordpress/block-library' {
	export interface CoreBlockModule {
		metadata: { name: string }
		init: () => void
	}
	export function __experimentalGetCoreBlocks(): CoreBlockModule[]
	export function registerCoreBlocks(blocks?: CoreBlockModule[]): void
}

declare module '@wordpress/block-editor' {
	import type { ComponentType, ReactNode } from 'react'

	export interface BlockEditorSettings {
		hasFixedToolbar?: boolean
		bodyPlaceholder?: string
		maxWidth?: number
		styles?: { css: string }[]
		__experimentalBlockPatterns?: unknown[]
	}
	export const BlockEditorProvider: ComponentType<{
		value?: unknown[]
		onInput?: (blocks: unknown[]) => void
		onChange?: (blocks: unknown[]) => void
		settings?: BlockEditorSettings
		children?: ReactNode
	}>
	export const BlockCanvas: ComponentType<{
		height?: string
		styles?: { css: string }[]
		children?: ReactNode
	}>
	export const BlockList: ComponentType<Record<string, never>>
	export const BlockToolbar: ComponentType<{ hideDragHandle?: boolean }>
	export const BlockInspector: ComponentType<Record<string, never>>
	export const WritingFlow: ComponentType<{ children?: ReactNode }>
	export const ObserveTyping: ComponentType<{ children?: ReactNode }>
}
