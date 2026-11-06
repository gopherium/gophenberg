// SPDX-License-Identifier: Apache-2.0

import { useRegistry, useSelect } from '@wordpress/data'
import type { select } from '@wordpress/data'
import { useEffect } from 'react'

/** The store blocks leave their messages in. */
const NOTICES_STORE = 'core/notices'

/** What a message offers the reader, as a block writes it. */
interface OfferedAction {
	label: string
	onClick?: () => void
	url?: string
	openInNewTab?: boolean
}

/** One message a block left. */
interface BlockNotice {
	id: string
	content: string
	actions: OfferedAction[]
}

/** What the store of messages answers. */
interface NoticesReader {
	getNotices: () => BlockNotice[]
}

/** What the store of messages takes. */
interface NoticesWriter {
	removeNotice: (id: string) => Promise<unknown>
}

/** What a message offers the reader, as the hook hands it on. */
interface BlockNoticeAction {
	label: string
	onAct: () => void
}

/**
 * Returns the messages the blocks left so far.
 * @param read - Reads a store of the block editor.
 * @returns The messages.
 */
function noticesFrom(read: typeof select): BlockNotice[] {
	return (read(NOTICES_STORE) as unknown as NoticesReader).getNotices()
}

/**
 * Opens the link an action offers, in a new tab when it asks for one.
 * @param offered - The action.
 */
function follow({ url, openInNewTab }: OfferedAction) {
	if (url === undefined) {
		return
	}
	if (openInNewTab === true) {
		window.open(url, '_blank', 'noopener,noreferrer')
		return
	}
	window.location.assign(url)
}

/**
 * Returns the action a message offers.
 * @param notice - The message.
 * @returns The first action, undefined when it offers none.
 */
function actionOf(notice: BlockNotice): BlockNoticeAction | undefined {
	const [offered] = notice.actions
	if (offered === undefined) {
		return undefined
	}
	return {
		label: offered.label,
		onAct: () => {
			offered.onClick?.()
			follow(offered)
		},
	}
}

/**
 * Hands each message the blocks leave to what draws it, taking it off the store of the registry it is drawn under.
 * @param show - Takes the words of a message and what it offers.
 */
export function useBlockNotices(show: (words: string, action?: BlockNoticeAction) => void) {
	const registry = useRegistry()
	const left = useSelect((read) => noticesFrom(read as typeof select), [])
	useEffect(() => {
		for (const notice of noticesFrom(registry.select as typeof select)) {
			show(notice.content, actionOf(notice))
			void (registry.dispatch(NOTICES_STORE) as unknown as NoticesWriter).removeNotice(notice.id)
		}
	}, [left, registry, show])
}
