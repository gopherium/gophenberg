// SPDX-License-Identifier: Apache-2.0

import { Tabs } from '@gophenberg/frontend-sdk'
import { BlockInspector } from '@gophenberg/frontend-sdk/editor'

import { DocumentPanels } from './DocumentPanels'
import type { EditorBuffer } from './useEditorBuffer'

/**
 * Renders the editor sidebar holding the document and block panels.
 * @param props - The post being edited and the buffer its panels drive.
 * @returns The sidebar element.
 */
export function EditorSidebar({ postId, buffer }: { postId: string, buffer: EditorBuffer }) {
	return (
		<Tabs.Root defaultValue="document">
			<Tabs.List>
				<Tabs.Tab value="document">Document</Tabs.Tab>
				<Tabs.Tab value="block">Block</Tabs.Tab>
			</Tabs.List>
			<Tabs.Panel value="document">
				<DocumentPanels postId={postId} buffer={buffer} />
			</Tabs.Panel>
			<Tabs.Panel value="block">
				<BlockInspector />
			</Tabs.Panel>
		</Tabs.Root>
	)
}
