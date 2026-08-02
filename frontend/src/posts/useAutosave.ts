// SPDX-License-Identifier: Apache-2.0

import { useEffect, useRef } from 'react'

import { autosavePost } from './api'
import type { AutosaveBuffer } from './api'

const AUTOSAVE_INTERVAL = 60000

const AUTOSAVE_TARGET_POST = 'post'

/**
 * Parks the editing buffer on the server while it holds unsaved words.
 * @param postId - The post being edited.
 * @param buffer - The words held, and whether they are unsaved.
 */
export function useAutosave(
	postId: string,
	buffer: AutosaveBuffer & {
		dirty: boolean
		saving: boolean
		version: string
		adoptVersion: (version: string) => void
	},
): void {
	const latest = useRef(buffer)
	latest.current = buffer
	useEffect(() => {
		/**
		 * Parks the buffer when it holds unsaved words and no write is in flight.
		 * @param keepalive - Whether the request should outlive the page.
		 */
		function park(keepalive: boolean) {
			const held = latest.current
			if (!held.dirty || held.saving) {
				return
			}
			void autosavePost(
				postId,
				{ title: held.title, content: held.content, excerpt: held.excerpt },
				held.version,
				keepalive,
			)
				.then((outcome) => {
					if (outcome.target === AUTOSAVE_TARGET_POST) {
						held.adoptVersion(outcome.savedAt)
					}
				})
				.catch(() => {})
		}
		const timer = setInterval(() => park(false), AUTOSAVE_INTERVAL)
		const flush = () => park(true)
		window.addEventListener('beforeunload', flush)
		return () => {
			clearInterval(timer)
			window.removeEventListener('beforeunload', flush)
		}
	}, [postId])
}
