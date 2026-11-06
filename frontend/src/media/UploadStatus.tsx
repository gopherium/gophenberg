// SPDX-License-Identifier: Apache-2.0

import { Button, checkIcon, downIcon, Icon, Notice, Popover, Spinner } from '@gophenberg/frontend-sdk'
import { formatNumber } from '@gopherium/gottext'
import { __, _n, sprintf } from '@wordpress/i18n'
import { useEffect, useRef, useState } from 'react'
import type { RefObject } from 'react'

import { fileReason } from './uploadWords'
import type { UploadRow } from './useUploadRun'

/** What one row of the panel draws and what it asks for. */
interface RowProps {
	row: UploadRow
	onDismiss: (id: string) => void
}

/** What the status draws and whom it tells. */
interface UploadStatusProps {
	rows: UploadRow[]
	frame: RefObject<HTMLDivElement | null>
	onDismiss: (id: string) => void
	onPanelChange: (open: boolean) => void
}

/**
 * Returns the words of the trigger and of the panel's heading, for files uploading, errors or a finished run.
 * @param rows - The rows listed.
 * @returns The two sets of words.
 */
function statusWords(rows: UploadRow[]): { trigger: string, heading: string } {
	const uploading = rows.filter((row) => row.status === 'uploading').length
	if (uploading > 0) {
		return {
			trigger: sprintf(_n('Uploading %s file', 'Uploading %s files', uploading, 'gophenberg'), formatNumber(uploading)),
			heading: __('Uploading', 'gophenberg'),
		}
	}
	const errors = rows.filter((row) => row.status === 'error').length
	if (errors > 0) {
		return {
			trigger: sprintf(_n('%s upload error', '%s upload errors', errors, 'gophenberg'), formatNumber(errors)),
			heading: __('Upload errors', 'gophenberg'),
		}
	}
	return { trigger: __('Upload complete', 'gophenberg'), heading: __('Upload complete', 'gophenberg') }
}

/**
 * Moves the focus to an element on screen.
 * @param target - The ref holding the element.
 */
function focusOn(target: RefObject<HTMLDivElement | null>) {
	(target.current as HTMLDivElement).focus()
}

/**
 * Moves the focus to the frame unless it already sits inside.
 * @param frame - The ref holding the frame of the dialog.
 */
function bringFocusInside(frame: RefObject<HTMLDivElement | null>) {
	if (!(frame.current as HTMLDivElement).contains(document.activeElement)) {
		focusOn(frame)
	}
}

/**
 * Opens the panel when errors go from none to some, a mount already holding one included.
 * @param hasErrors - Whether any row holds an error.
 * @param frame - The ref holding the frame of the dialog.
 * @param open - Opens the panel.
 */
function useOpenOnFirstError(
	hasErrors: boolean,
	frame: RefObject<HTMLDivElement | null>,
	open: (next: boolean) => void,
) {
	const hadErrors = useRef(false)
	useEffect(() => {
		if (hasErrors && !hadErrors.current) {
			bringFocusInside(frame)
			open(true)
		}
		hadErrors.current = hasErrors
	}, [hasErrors, frame, open])
}

/**
 * Renders the row of a file that failed.
 * @param props - The row and the handler taking it away.
 * @returns The notice with its reason and its close button.
 */
function FailedRow({ row, onDismiss }: RowProps) {
	return (
		<Notice.Root intent="error" icon={null} className="gophenberg-media-picker__error">
			<Notice.Description>{fileReason(row.name, row.error)}</Notice.Description>
			<Notice.CloseIconButton label={__('Close', 'gophenberg')} onClick={() => onDismiss(row.id)} />
		</Notice.Root>
	)
}

/**
 * Renders one file in the panel.
 * @param props - The row and the handler taking it away.
 * @returns The list item, a spinner or a check with the name, or the failed row.
 */
function StatusRow({ row, onDismiss }: RowProps) {
	if (row.status === 'error') {
		return (
			<li className="gophenberg-media-picker__row">
				<FailedRow row={row} onDismiss={onDismiss} />
			</li>
		)
	}
	return (
		<li className="gophenberg-media-picker__row">
			{row.status === 'uploading' ? <Spinner /> : <Icon icon={checkIcon} size={16} />}
			<span className="gophenberg-media-picker__filename" title={row.name}>
				{row.name}
			</span>
		</li>
	)
}

/**
 * Renders the status of the uploads over the foot of the list, with the panel listing each file.
 * @param props - The rows, the frame of the dialog and the handlers told what happens to the panel.
 * @returns The spinner, the trigger and the panel.
 */
export function UploadStatus({ rows, frame, onDismiss, onPanelChange }: UploadStatusProps) {
	const [open, setOpen] = useState(false)
	const panel = useRef<HTMLDivElement>(null)
	const words = statusWords(rows)
	useOpenOnFirstError(rows.some((row) => row.status === 'error'), frame, setOpen)
	useEffect(() => {
		if (!open) {
			return undefined
		}
		onPanelChange(true)
		return () => onPanelChange(false)
	}, [open, onPanelChange])
	/**
	 * Follows the panel, moving the focus to the frame first when the rows and their trigger go with this close.
	 * @param next - Whether the panel is to be open.
	 */
	function changeOpen(next: boolean) {
		if (!next && rows.every((row) => row.status === 'uploaded')) {
			focusOn(frame)
		}
		setOpen(next)
	}
	/**
	 * Takes a row away and moves the focus to the panel, or to the frame when this was the last row.
	 * @param id - The row.
	 */
	function dismiss(id: string) {
		onDismiss(id)
		focusOn(rows.length > 1 ? panel : frame)
	}
	return (
		<div className="gophenberg-media-picker__status">
			{rows.some((row) => row.status === 'uploading') && <Spinner />}
			<Popover.Root open={open} onOpenChange={changeOpen}>
				<Popover.Trigger render={<Button variant="minimal" tone="neutral" size="compact" />}>
					{words.trigger}
					<Button.Icon icon={downIcon} />
				</Popover.Trigger>
				<Popover.Popup
					ref={panel}
					initialFocus={panel}
					className="gophenberg-media-picker__panel"
					positioner={<Popover.Positioner side="top" align="start" />}
				>
					<div className="gophenberg-media-picker__panel-header">
						<Popover.Title render={<h3 />}>{words.heading}</Popover.Title>
					</div>
					<ul className="gophenberg-media-picker__rows">
						{rows.map((row) => (
							<StatusRow key={row.id} row={row} onDismiss={dismiss} />
						))}
					</ul>
				</Popover.Popup>
			</Popover.Root>
		</div>
	)
}
