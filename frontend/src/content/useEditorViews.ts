// SPDX-License-Identifier: Apache-2.0

import { useCallback, useState } from 'react'

import type { PreviewWidth } from './editorSetup'

export interface EditorViews {
	listOpen: boolean
	toggleList: () => void
	preview: PreviewWidth
	setPreview: (width: PreviewWidth) => void
}

/**
 * Holds the views the editor chrome offers over the post being edited.
 * @returns The state of the block list and of the preview width.
 */
export function useEditorViews(): EditorViews {
	const [listOpen, setListOpen] = useState(false)
	const [preview, setPreview] = useState<PreviewWidth>('desktop')
	const toggleList = useCallback(() => setListOpen((open) => !open), [])
	return { listOpen, toggleList, preview, setPreview }
}
