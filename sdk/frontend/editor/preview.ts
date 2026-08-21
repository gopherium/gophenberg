// SPDX-License-Identifier: Apache-2.0

import * as blockEditor from '@wordpress/block-editor'
import type { Block } from '@wordpress/blocks'
import type { ComponentType } from 'react'

/** BlockPreview renders blocks without a form, typed here because the package types omit it. */
export const BlockPreview = (
	blockEditor as unknown as {
		BlockPreview: ComponentType<{ blocks: Block[], viewportWidth?: number }>
	}
).BlockPreview
