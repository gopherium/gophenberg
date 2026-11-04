// SPDX-License-Identifier: Apache-2.0

import { speak } from '@gophenberg/frontend-sdk'
import { bulkNotes } from '@gopherium/godmin'
import { __ } from '@wordpress/i18n'
import { useCallback, useRef, useState } from 'react'

import { useRefreshMedia } from './actions'
import { uploadMedia } from './api'
import type { MediaItem } from './api'
import { typeAllowed } from './editorMedia'
import { fileReason, uploadWords } from './uploadWords'

/** One file of an upload, as the status lists it. */
export interface UploadRow {
	id: string
	batch: number
	name: string
	status: 'uploading' | 'uploaded' | 'error'
	error: string
}

/** The words of a toast waiting to be raised, a new object each run. */
export interface UploadNotice {
	words: string
}

/** The batches still running, the files they were given and how many of those landed. */
interface Run {
	open: number
	files: File[]
	landed: number
}

/** What the picker tells the upload, and is told by it. */
interface UploadRunOptions {
	allowedTypes: string[] | undefined
	onLanded: (ids: string[]) => void
}

/** The run before any file was given. */
const NO_RUN: Run = { open: 0, files: [], landed: 0 }

/**
 * Returns the words for a file refused before anything is sent.
 * @param file - The file.
 * @param allowedTypes - The types the block accepts.
 * @returns The words, empty when the file may go.
 */
function whyRefused(file: File, allowedTypes: string[] | undefined): string {
	if (!typeAllowed(file.type, allowedTypes)) {
		return __('Sorry, this file type is not supported here.', 'gophenberg')
	}
	if (file.size === 0) {
		return __('This file is empty.', 'gophenberg')
	}
	return ''
}

/**
 * Returns one row per file of a batch, uploading or holding the words the check refused it with.
 * @param files - The files of the batch.
 * @param batch - The number of the batch.
 * @param allowedTypes - The types the block accepts.
 * @returns The rows.
 */
function rowsFor(files: File[], batch: number, allowedTypes: string[] | undefined): UploadRow[] {
	return files.map((file, at) => {
		const error = whyRefused(file, allowedTypes)
		return { id: `${batch}-${at}`, batch, name: file.name, status: error === '' ? 'uploading' : 'error', error }
	})
}

/**
 * Speaks the lines of the rows the check refused as one message.
 * @param rows - The rows of a batch.
 */
function speakRefused(rows: UploadRow[]) {
	const lines = rows.filter((row) => row.status === 'error').map((row) => fileReason(row.name, row.error))
	if (lines.length > 0) {
		speak(lines.join(' '), 'assertive')
	}
}

/**
 * Returns the rows with one turned to an error.
 * @param rows - The rows held.
 * @param id - The row that failed.
 * @param reason - Why it failed, in the reader's words.
 * @returns The rows.
 */
function refusedRow(rows: UploadRow[], id: string, reason: string): UploadRow[] {
	return rows.map((row) => (row.id === id ? { ...row, status: 'error', error: reason } : row))
}

/**
 * Returns the rows with those of a batch still uploading turned uploaded.
 * @param rows - The rows held.
 * @param batch - The batch that ended.
 * @returns The rows.
 */
function batchEnded(rows: UploadRow[], batch: number): UploadRow[] {
	return rows.map((row) => (row.batch === batch && row.status === 'uploading' ? { ...row, status: 'uploaded' } : row))
}

/**
 * Returns the rows less the finished ones.
 * @param rows - The rows held.
 * @returns The rows still uploading or holding an error.
 */
function withoutUploaded(rows: UploadRow[]): UploadRow[] {
	return rows.filter((row) => row.status !== 'uploaded')
}

/**
 * Reports whether the answer to one file holds a stored item.
 * @param answer - The stored item, undefined when the file was not stored.
 * @returns Whether it does.
 */
function isStored(answer: MediaItem | undefined): answer is MediaItem {
	return answer !== undefined
}

/**
 * Returns the identities of the stored items the block's types take.
 * @param stored - The items that landed.
 * @param allowedTypes - The types the block accepts.
 * @returns The identities, as a selection holds them.
 */
function tickable(stored: MediaItem[], allowedTypes: string[] | undefined): string[] {
	return stored.filter((item) => typeAllowed(item.mimeType, allowedTypes)).map((item) => String(item.id))
}

/**
 * Returns the run with one more batch and its files.
 * @param run - The run held.
 * @param files - The files of the batch.
 * @returns The run.
 */
function joinedRun(run: Run, files: File[]): Run {
	return { ...run, open: run.open + 1, files: [...run.files, ...files] }
}

/**
 * Returns the run with one batch less and what landed counted.
 * @param run - The run held.
 * @param landed - How many files of the batch landed.
 * @returns The run.
 */
function endedRun(run: Run, landed: number): Run {
	return { ...run, open: run.open - 1, landed: run.landed + landed }
}

/**
 * Returns the words of the toast a finished run raises.
 * @param run - The run that finished.
 * @returns The words, undefined when nothing landed.
 */
function toastOf(run: Run): string | undefined {
	return bulkNotes(run.files, { asked: run.files.length, done: run.landed, failures: [] }, uploadWords()).toast
}

/**
 * Holds what the status of the picker lists and runs every upload started from the picker.
 * @param options - The types the block accepts, and the handler told which files to tick.
 * @returns The rows, the handlers changing them and the toast waiting to be raised.
 */
export function useUploadRun({ allowedTypes, onLanded }: UploadRunOptions) {
	const refresh = useRefreshMedia()
	const [rows, setRows] = useState<UploadRow[]>([])
	const [toast, setToast] = useState<UploadNotice | null>(null)
	const run = useRef(NO_RUN)
	const batches = useRef(0)
	const panelOpen = useRef(false)
	/**
	 * Turns a row whose request failed to an error and speaks its line, once, whether or not the panel shows it.
	 * @param row - The row that failed.
	 * @param reason - Why it failed, in the reader's words.
	 */
	function fail(row: UploadRow, reason: string) {
		setRows((held) => refusedRow(held, row.id, reason))
		speak(fileReason(row.name, reason), 'assertive')
	}
	/**
	 * Sends one file unless the check refused it.
	 * @param row - The row of the file.
	 * @param file - The file.
	 * @returns The stored item, undefined when the file was not stored.
	 */
	async function send(row: UploadRow, file: File): Promise<MediaItem | undefined> {
		if (row.status === 'error') {
			return undefined
		}
		return uploadMedia(file).catch((failure: Error) => {
			fail(row, failure.message)
			return undefined
		})
	}
	/**
	 * Counts a batch out, and on the last one raises the toast and clears the finished rows unless a panel shows them.
	 * @param landed - How many files of the batch landed.
	 */
	function finish(landed: number) {
		run.current = endedRun(run.current, landed)
		if (run.current.open > 0) {
			return
		}
		const words = toastOf(run.current)
		run.current = NO_RUN
		if (words !== undefined) {
			setToast({ words })
		}
		if (!panelOpen.current) {
			setRows(withoutUploaded)
		}
	}
	/**
	 * Runs one batch of files from the pick to its end.
	 * @param files - The files picked or dropped.
	 */
	async function upload(files: File[]) {
		batches.current += 1
		const batch = batches.current
		const fresh = rowsFor(files, batch, allowedTypes)
		run.current = joinedRun(run.current, files)
		setRows((held) => [...held, ...fresh])
		speakRefused(fresh)
		const answers = await Promise.all(fresh.map((row, at) => send(row, files[at])))
		const stored = answers.filter(isStored)
		setRows((held) => batchEnded(held, batch))
		onLanded(tickable(stored, allowedTypes))
		if (stored.length > 0) {
			void refresh()
		}
		finish(stored.length)
	}
	/** Takes one row off the list. */
	const dismiss = useCallback((id: string) => {
		setRows((held) => held.filter((row) => row.id !== id))
	}, [])
	/** Forgets the toast once it was raised. */
	const toastShown = useCallback(() => setToast(null), [])
	/** Follows the panel, clearing the finished rows when it closes. */
	const onPanelChange = useCallback((open: boolean) => {
		panelOpen.current = open
		if (!open) {
			setRows(withoutUploaded)
		}
	}, [])
	return { rows, upload: (files: File[]) => void upload(files), dismiss, toast, toastShown, onPanelChange }
}

/** What an upload run hands the dialog. */
export type UploadRun = ReturnType<typeof useUploadRun>
