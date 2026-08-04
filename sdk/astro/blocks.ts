// SPDX-License-Identifier: Apache-2.0

import type { PostSummary } from './content.ts'

/** One parsed block with its raw inner HTML and its children. */
export interface BlockNode {
	name: string | null
	attrs: Record<string, unknown>
	innerHTML: string
	innerContent: (string | null)[]
	innerBlocks: BlockNode[]
}

/** What every block component receives. */
export interface BlockProps {
	block: BlockNode
	post: PostSummary
}
