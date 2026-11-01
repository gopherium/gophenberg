// SPDX-License-Identifier: Apache-2.0

import { DropZone, mediaIcon, uploadIcon } from '@gophenberg/frontend-sdk'
import { DataViews } from '@gophenberg/frontend-sdk/dataviews'
import type { Field, View } from '@gophenberg/frontend-sdk/dataviews'
import {
	ErrorNotice,
	FileButton,
	ListEmpty,
	ListNoResults,
	Page,
	bulkNotes,
	focusInDialog,
	focusedTrigger,
	offeredSize,
	offeredView,
	paginationOf,
	runEach,
	sizedView,
	useFocusBack,
	useListRun,
	useToaster,
	viewNarrows,
} from '@gopherium/godmin'
import type { BulkFailure, BulkWords, OfferedView, Trigger } from '@gopherium/godmin'
import { openOnTap, useListView } from '@gopherium/godmin/router'
import type { ListView } from '@gopherium/godmin/router'
import { formatNumber } from '@gopherium/gottext'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import type { UseQueryResult } from '@tanstack/react-query'
import { __, _n, _x, sprintf } from '@wordpress/i18n'
import { useMemo, useRef, useState } from 'react'
import type { RefObject } from 'react'

import type { ListSettings } from '../settings/api'
import { useListSettings } from '../settings/useListSettings'
import './media.css'
import { useMayChange, useMediaActions, useMediaTools, useRefreshMedia, viewFile } from './actions'
import { listMedia, mediaQueryKey, uploadMedia } from './api'
import type { MediaItem, MediaPage, MediaQuery } from './api'
import { MEDIA_SORT, MEDIA_VIEW_GUARD, libraryFields, mediaListing } from './fields'
import { OpenMediaDialog } from './MediaModals'
import type { MediaTools, OpenedDialog } from './MediaModals'

/** The width a card of the grid opens at, the one DataViews draws by default. */
const PREVIEW_SIZE = 230

/** The items the library holds before its first page arrives. */
const NO_ITEMS: MediaItem[] = []

/**
 * Returns the reason a file was not stored, already in the reader's words.
 * @param failure - The file and what its upload failed with.
 * @returns The reason.
 */
function reasonOf(failure: BulkFailure<File>): string {
	return (failure.error as Error).message
}

/**
 * Returns the notice naming the files an upload could not store, one line per file when several were sent.
 * @param failures - The files that failed and what each failed with.
 * @param asked - How many files the upload was given.
 * @returns The reason alone for one file sent, else a line naming each failed file with its reason.
 */
function refusedLines(failures: BulkFailure<File>[], asked: number): string {
	if (asked === 1) {
		return reasonOf(failures[0])
	}
	const line = __('%(file)s: %(reason)s', 'gophenberg')
	return failures.map((failure) => sprintf(line, { file: failure.item.name, reason: reasonOf(failure) })).join('\n')
}

/**
 * Returns the words an upload to the library reads as.
 * @returns The toast naming the one file stored or counting the files stored, and the notice for the refused ones.
 */
function uploadWords(): BulkWords<File> {
	return {
		done: (count, only) =>
			only === undefined
				? sprintf(_n('Uploaded %s file.', 'Uploaded %s files.', count, 'gophenberg'), formatNumber(count))
				: sprintf(__('"%s" uploaded.', 'gophenberg'), only.name),
		failed: refusedLines,
	}
}

/** The uploads running at once, how many files they were given and the files they could not store between them. */
interface UploadBatch {
	runs: number
	asked: number
	failures: BulkFailure<File>[]
}

/**
 * Returns the batch once another upload joins it, a fresh one when no upload was running.
 * @param held - The batch before the upload starts.
 * @param files - The files the upload was given.
 * @returns The batch counting the upload and its files.
 */
function joined(held: UploadBatch, files: File[]): UploadBatch {
	if (held.runs === 0) {
		return { runs: 1, asked: files.length, failures: [] }
	}
	return { ...held, runs: held.runs + 1, asked: held.asked + files.length }
}

/**
 * Returns the batch once one of its uploads ended.
 * @param held - The batch before the upload ended.
 * @param failures - The files the upload could not store.
 * @returns The batch holding one upload less and every failure so far.
 */
function ended(held: UploadBatch, failures: BulkFailure<File>[]): UploadBatch {
	return { ...held, runs: held.runs - 1, failures: [...held.failures, ...failures] }
}

/**
 * Returns the notice a finished batch reads as.
 * @param held - The batch whose last upload ended.
 * @returns The notice naming the refused files, none when every file was stored.
 */
function batchNotice(held: UploadBatch): string | undefined {
	return held.failures.length === 0 ? undefined : refusedLines(held.failures, held.asked)
}

/**
 * Holds the uploads to the library: a request per file, a toast per upload, one notice per batch running together.
 * @param region - The list region focused once a file landed.
 * @returns Whether an upload runs, the notice of the last uploads, and the handler taking the files.
 */
function useUpload(region: RefObject<HTMLDivElement | null>) {
	const toaster = useToaster()
	const refresh = useRefreshMedia()
	const batch = useRef<UploadBatch>({ runs: 0, asked: 0, failures: [] })
	const [busy, setBusy] = useState(false)
	const [notice, setNotice] = useState<string>()
	const finish = (failures: BulkFailure<File>[]) => {
		batch.current = ended(batch.current, failures)
		if (batch.current.runs === 0) {
			setBusy(false)
			setNotice(batchNotice(batch.current))
		}
	}
	const upload = async (files: File[]) => {
		batch.current = joined(batch.current, files)
		setBusy(true)
		setNotice(undefined)
		const outcome = await runEach(files, uploadMedia)
		if (outcome.done > 0) {
			await refresh()
		}
		const toast = bulkNotes(files, outcome, uploadWords()).toast
		if (toast !== undefined) {
			toaster.show(toast)
		}
		finish(outcome.failures)
		if (outcome.done > 0 && !focusInDialog()) {
			region.current?.focus()
		}
	}
	return { busy, notice, upload: (files: File[]) => void upload(files) }
}

/**
 * Returns the words of the header button adding files to the library.
 * @returns The button's words in the reader's language.
 */
function addLabel(): string {
	return __('Add Media File', 'gophenberg')
}

/**
 * Renders what the library shows when it holds no item.
 * @param props - Whether a search or a filter narrowed the library, whether an upload runs and what takes the files.
 * @returns The empty state, offering the add button when nothing narrowed the library.
 */
function MediaEmpty({ narrowed, busy, upload }: { narrowed: boolean, busy: boolean, upload: (files: File[]) => void }) {
	if (narrowed) {
		return <ListNoResults />
	}
	return (
		<ListEmpty
			icon={mediaIcon}
			title={__('No media items yet', 'gophenberg')}
			actions={
				<FileButton multiple size="default" icon={uploadIcon} busy={busy} inputLabel={addLabel()} onChoose={upload}>
					{addLabel()}
				</FileButton>
			}
		/>
	)
}

/**
 * Returns the listing request a view of the library stands for.
 * @param view - The view the list shows.
 * @param settings - The list settings, absent when they could not be read.
 * @returns The request, leaving the page size to the server while the settings name none.
 */
function listingOf(view: OfferedView<View>, settings: ListSettings | undefined): MediaQuery {
	return { ...mediaListing(view), ...sizedView(view, settings?.list_page_sizes) }
}

/**
 * Holds the view of the library in the address, sized by the settings once they are known, and reads its page.
 * @returns The list view, the fields, the view the fields can draw, the settings and the page query.
 */
function useMediaList() {
	const settings = useListSettings().data
	const fields = useMemo(() => libraryFields(), [])
	const list = useListView<View>({
		fields: ['author'],
		phoneFields: ['author'],
		titleField: 'title',
		mediaField: 'thumbnail',
		sort: { ...MEDIA_SORT },
		perPage: settings?.list_page_size,
		layouts: { wide: 'grid', phone: 'list' },
		layoutSettings: { grid: { layout: { previewSize: PREVIEW_SIZE } } },
	})
	const offered = offeredView(list.view, fields, MEDIA_VIEW_GUARD)
	const view = {
		...offered,
		perPage: offeredSize(offered.perPage, settings?.list_page_sizes, settings?.list_page_size),
	}
	const asked = listingOf(view, settings)
	const media = useQuery({
		queryKey: [...mediaQueryKey, 'library', asked],
		queryFn: () => listMedia(asked),
		placeholderData: keepPreviousData,
	})
	return { list, fields, view, settings, media }
}

/**
 * Returns the rows, the paging and the page sizes DataViews draws the page of the library with.
 * @param page - The page the server served, absent until it arrives.
 * @param settings - The list settings, absent when they could not be read.
 * @returns The props of the page.
 */
function pageProps(page: MediaPage | undefined, settings: ListSettings | undefined) {
	return {
		data: page?.items ?? NO_ITEMS,
		paginationInfo: paginationOf(page === undefined ? undefined : { total: page.total, limit: page.perPage }),
		config: { perPageSizes: settings?.list_page_sizes ?? [] },
	}
}

/**
 * Returns the handler opening Edit on a file a tap landed on, for a file the session may change, and the file itself
 * in a new tab otherwise, as View does.
 * @param page - The page the list shows, which holds every row a tap can land on.
 * @param mayChange - Whether the session may change a file.
 * @param onEdit - Opens Edit on a file.
 * @returns The handler, given the id of the file tapped.
 */
function openTapped(
	page: MediaPage | undefined,
	mayChange: (item: MediaItem) => boolean,
	onEdit: (item: MediaItem) => void,
) {
	return (id: string) => {
		const tapped = (page as MediaPage).items.find((item) => String(item.id) === id) as MediaItem
		if (mayChange(tapped)) {
			onEdit(tapped)
		} else {
			viewFile(tapped)
		}
	}
}

/**
 * Renders the items of the library, or the failure of their read.
 * @param props - The page query, the list view, the view the fields draw, the fields, the settings, what the actions
 *   need from the screen, whether the session may change a file, the handler opening a dialog on one, whether an upload
 *   runs and what takes the files.
 * @returns The list, or the failure notice.
 */
function MediaList(props: {
	media: UseQueryResult<MediaPage>
	list: ListView<View>
	view: OfferedView<View>
	fields: Field<MediaItem>[]
	settings: ListSettings | undefined
	tools: MediaTools
	mayChange: (item: MediaItem) => boolean
	open: (opened: OpenedDialog) => void
	busy: boolean
	upload: (files: File[]) => void
}) {
	const actions = useMediaActions(props.tools, props.mayChange, props.open)
	const { media, list, view } = props
	const edit = (item: MediaItem, trigger: Trigger) => props.open({ action: 'edit', item, trigger })
	if (media.isError) {
		return <ErrorNotice>{__('The media library could not be loaded.', 'gophenberg')}</ErrorNotice>
	}
	return (
		<DataViews
			{...pageProps(media.data, props.settings)}
			empty={<MediaEmpty narrowed={viewNarrows(view)} busy={props.busy} upload={props.upload} />}
			fields={props.fields}
			actions={actions}
			view={view}
			onChangeView={list.onChangeView}
			defaultLayouts={list.defaultLayouts}
			selection={list.selection}
			onChangeSelection={openOnTap(list, openTapped(media.data, props.mayChange, (item) => edit(item, 'row')))}
			isItemClickable={props.mayChange}
			onClickItem={(item: MediaItem) => edit(item, focusedTrigger())}
			isLoading={media.isPending || media.isPlaceholderData}
			getItemId={(item: MediaItem) => String(item.id)}
			searchLabel={sprintf(__('Search %(type)s…', 'gophenberg'), {
				type: _x('Media', 'admin section', 'gophenberg').toLowerCase(),
			})}
		/>
	)
}

/**
 * Renders the media library screen.
 * @returns The library screen element.
 */
export function MediaScreen() {
	const { list, fields, view, settings, media } = useMediaList()
	const { run, region } = useListRun('media', list.onChangeSelection)
	const tools = useMediaTools(run)
	const mayChange = useMayChange()
	const [opened, setOpened] = useState<OpenedDialog>()
	const focusBack = useFocusBack(media.isFetching, region)
	const { busy, notice, upload } = useUpload(region)
	const title = _x('Media', 'admin section', 'gophenberg')
	const uploadLabel = addLabel()
	return (
		<Page
			title={title}
			headingLevel={2}
			subtitle={__('Manage the images and files this site holds.', 'gophenberg')}
			actions={
				<FileButton multiple icon={uploadIcon} busy={busy} inputLabel={uploadLabel} onChoose={upload}>
					{uploadLabel}
				</FileButton>
			}
			list
			listLabel={title}
			listRef={region}
		>
			{notice === undefined ? null : (
				<div className="gophenberg-media__notice">
					<ErrorNotice>{notice}</ErrorNotice>
				</div>
			)}
			<MediaList
				media={media}
				list={list}
				view={view}
				fields={fields}
				settings={settings}
				tools={tools}
				mayChange={mayChange}
				open={setOpened}
				busy={busy}
				upload={upload}
			/>
			<DropZone className="godmin-list-overlay" label={__('Drop files to upload', 'gophenberg')} onFilesDrop={upload} />
			<OpenMediaDialog opened={opened} tools={tools} onClose={() => setOpened(undefined)} onBack={focusBack} />
		</Page>
	)
}
