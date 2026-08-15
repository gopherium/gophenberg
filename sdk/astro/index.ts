// SPDX-License-Identifier: Apache-2.0

export { parseBlocks, toBlockNode, verbatimSegments } from './blocks.ts'
export { GophenbergClient } from './client.ts'
export { siteAssetUrls } from './assets.ts'
export {
	contentApiPath,
	generator,
	isBlockName,
	kitFeatureVersion,
	kitName,
	kitVersion,
} from './kit.ts'
export { defineTheme } from './theme.ts'
export { gophenbergLoader } from './loader.ts'
export { relatedFields, relatedItems } from './related.ts'

export type { BlockComponentMap, BlockNode, BlockProps, BlockSegment, ParsedBlock } from './blocks.ts'
export type { ClientOptions, ListQuery, ResolveOptions } from './client.ts'
export type {
	ContentType,
	ContentTypeField,
	Handshake,
	Page,
	Post,
	PostSummary,
	RelatedItem,
	Resolved,
} from './content.ts'
export type { SiteAssetUrls } from './assets.ts'
export type { LivePost, LoaderOptions, PostCollectionFilter, PostEntryFilter } from './loader.ts'
export type { RelatedField } from './related.ts'
export type { GophenbergTheme, ThemeLayouts, ThemeSeo } from './theme.ts'
