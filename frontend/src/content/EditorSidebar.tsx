// SPDX-License-Identifier: Apache-2.0

import { IconButton, Tabs, closeIcon } from '@gophenberg/frontend-sdk'
import { BlockInspector } from '@gophenberg/frontend-sdk/editor'
import { __ } from '@wordpress/i18n'

import { DocumentPanels } from './DocumentPanels'
import type { EditorBuffer } from './useEditorBuffer'

/**
 * Renders the editor sidebar holding the document and block panels.
 * @param props - The post being edited, the buffer its panels drive, what parks its words now and what closing does.
 * @returns The sidebar element.
 */
export function EditorSidebar({
	postId,
	buffer,
	parkNow,
	onClose,
}: {
	postId: string
	buffer: EditorBuffer
	parkNow: () => Promise<() => void>
	onClose: () => void
}) {
	return (
		<Tabs.Root defaultValue="document">
			<div className="gophenberg-editor__sidebar-header">
				<Tabs.List className="gophenberg-editor__sidebar-tabs">
					<Tabs.Tab value="document">{__('Document', 'gophenberg')}</Tabs.Tab>
					<Tabs.Tab value="block">{__('Block', 'gophenberg')}</Tabs.Tab>
				</Tabs.List>
				<IconButton
					label={__('Close Settings', 'gophenberg')}
					icon={closeIcon}
					variant="minimal"
					tone="neutral"
					size="compact"
					onClick={onClose}
				/>
			</div>
			<Tabs.Panel value="document" className="gophenberg-editor__sidebar-panel">
				<DocumentPanels postId={postId} buffer={buffer} parkNow={parkNow} />
			</Tabs.Panel>
			<Tabs.Panel value="block">
				<BlockInspector />
			</Tabs.Panel>
		</Tabs.Root>
	)
}
