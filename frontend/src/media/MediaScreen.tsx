// SPDX-License-Identifier: Apache-2.0

import { Stack, Text } from '@gophenberg/frontend-sdk'
import { DataViews } from '@gophenberg/frontend-sdk/dataviews'
import type { View } from '@gophenberg/frontend-sdk/dataviews'
import { ErrorNotice, Page } from '@gopherium/godmin'
import { useMutation, useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import type { DragEvent } from 'react'

import './media.css'
import { useMediaActions, useRefreshMedia } from './actions'
import { listMedia, mediaQueryKey, uploadMedia } from './api'
import type { MediaItem, MediaPage } from './api'
import { mediaFields } from './fields'

const PER_PAGE = 20

const PREVIEW_SIZE = 230

const EMPTY_PAGE: MediaPage = { items: [], total: 0 }

const INITIAL_VIEW: View = {
	type: 'grid',
	fields: ['dimensions', 'filesize'],
	titleField: 'title',
	mediaField: 'thumbnail',
	search: '',
	page: 1,
	perPage: PER_PAGE,
	layout: { previewSize: PREVIEW_SIZE },
}

/**
 * Returns the kind a view is filtered to.
 * @param view - The view the library is shown through.
 * @returns The kind to ask for, empty for every kind.
 */
export function filteredKind(view: View): string {
	const chosen = view.filters?.find((filter) => filter.field === 'type')?.value
	return typeof chosen === 'string' ? chosen : ''
}

/**
 * Returns the first file a drag carried.
 * @param transfer - What the drag carried, when it carried anything.
 * @returns The dropped file, or null when the drag carried none.
 */
export function droppedFile(transfer: DataTransfer | null): File | null {
	return chosenFile(transfer?.files ?? null)
}

/**
 * Returns the file a file field holds.
 * @param files - What the file field holds.
 * @returns The chosen file, or null when the field holds none.
 */
export function chosenFile(files: FileList | null): File | null {
	return files?.[0] ?? null
}

/**
 * Renders the media library screen.
 * @returns The library screen element.
 */
export function MediaScreen() {
	const [view, setView] = useState<View>(INITIAL_VIEW)
	const [selection, setSelection] = useState<string[]>([])
	const [refusal, setRefusal] = useState('')
	const actions = useMediaActions()
	const refresh = useRefreshMedia()
	const kind = filteredKind(view)
	const media = useQuery({
		queryKey: [...mediaQueryKey, kind, view.search, view.page],
		queryFn: () => listMedia({ type: kind, search: view.search, page: view.page }),
	})
	const upload = useMutation({
		mutationFn: (chosen: File) => uploadMedia(chosen),
		onSuccess: async (outcome) => {
			if (outcome.kind === 'refused') {
				setRefusal(outcome.reason)
				return
			}
			setRefusal('')
			await refresh()
		},
		onError: () => setRefusal('The server could not be reached, so nothing was uploaded.'),
	})
	/**
	 * Uploads a file the administrator handed the library.
	 * @param chosen - The file to upload, or null when none was handed over.
	 */
	function accept(chosen: File | null) {
		if (chosen !== null) {
			upload.mutate(chosen)
		}
	}
	const page = media.data ?? EMPTY_PAGE
	return (
		<Page title="Media" actions={<UploadControl onChoose={accept} busy={upload.isPending} />}>
			<Stack direction="column" gap="md">
				{refusal !== '' && <ErrorNotice>{refusal}</ErrorNotice>}
				{media.isError ? (
					<ErrorNotice>The media library could not be loaded.</ErrorNotice>
				) : (
					<div
						className="gophenberg-media__drop"
						data-testid="media-drop"
						onDragOver={(event: DragEvent<HTMLDivElement>) => event.preventDefault()}
						onDrop={(event: DragEvent<HTMLDivElement>) => {
							event.preventDefault()
							accept(droppedFile(event.dataTransfer))
						}}
					>
						<DataViews
							data={page.items}
							fields={mediaFields}
							actions={actions}
							view={view}
							onChangeView={setView}
							selection={selection}
							onChangeSelection={setSelection}
							isLoading={media.isPending}
							getItemId={(item: MediaItem) => String(item.id)}
							searchLabel="Search media"
							config={{ perPageSizes: [PER_PAGE] }}
							paginationInfo={{
								totalItems: page.total,
								totalPages: Math.max(1, Math.ceil(page.total / PER_PAGE)),
							}}
							defaultLayouts={{
								grid: { fields: ['dimensions', 'filesize'] },
								table: {
									fields: ['filename', 'mimeType', 'dimensions', 'filesize', 'date'],
								},
							}}
							empty={<Text>No media has been uploaded yet.</Text>}
						/>
					</div>
				)}
			</Stack>
		</Page>
	)
}

/**
 * Renders the control adding a file to the library.
 * @param props - The handler taking the chosen file and whether one is in flight.
 * @returns The upload element.
 */
function UploadControl({
	onChoose,
	busy,
}: {
	onChoose: (chosen: File | null) => void
	busy: boolean
}) {
	return (
		<Stack direction="row" gap="sm" align="center">
			<label htmlFor="media-file">Add media</label>
			<input
				id="media-file"
				type="file"
				disabled={busy}
				onChange={(event) => {
					onChoose(chosenFile(event.target.files))
					event.target.value = ''
				}}
			/>
		</Stack>
	)
}
