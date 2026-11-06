// SPDX-License-Identifier: Apache-2.0

import type { View, ViewPickerGrid } from '@gophenberg/frontend-sdk/dataviews'
import { createContext, useContext, useMemo, useState } from 'react'
import type { Dispatch, ReactNode, SetStateAction } from 'react'

import { MEDIA_SORT } from './fields'

const PREVIEW_SIZE = 170

const INITIAL_VIEW: ViewPickerGrid = {
	type: 'pickerGrid',
	fields: [],
	showTitle: false,
	titleField: 'title',
	mediaField: 'thumbnail',
	search: '',
	page: 1,
	filters: [],
	sort: { ...MEDIA_SORT },
	layout: { previewSize: PREVIEW_SIZE, density: 'compact', mediaFit: 'contain' },
}

/** A view and what changes it. */
type HeldView = readonly [View, Dispatch<SetStateAction<View>>]

const PickerView = createContext<HeldView | null>(null)

/**
 * Holds one view for every picker under it.
 * @param props - The pickers sharing the view.
 * @returns The pickers, inside the view they share.
 */
export function PickerViewProvider({ children }: { children: ReactNode }) {
	const [view, setView] = useState<View>(INITIAL_VIEW)
	const held = useMemo(() => [view, setView] as const, [view])
	return <PickerView.Provider value={held}>{children}</PickerView.Provider>
}

/**
 * Returns the view the provider above holds, or this picker's own when there is no provider.
 * @returns The view and what changes it.
 */
export function usePickerView(): HeldView {
	const own = useState<View>(INITIAL_VIEW)
	return useContext(PickerView) ?? own
}
