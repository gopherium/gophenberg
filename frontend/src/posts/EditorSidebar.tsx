// SPDX-License-Identifier: Apache-2.0

import { Stack, Tabs, Text } from '@gophenberg/frontend-sdk'
import { BlockInspector } from '@gophenberg/frontend-sdk/editor'

const STATUS_LABELS: Record<string, string> = {
	draft: 'Draft',
	pending: 'Pending',
	private: 'Private',
	published: 'Published',
	trash: 'Trash',
}

/**
 * Returns the label a post status is shown under.
 * @param status - The status the post holds.
 * @returns The label to show.
 */
function labelFor(status: string): string {
	return STATUS_LABELS[status] ?? status
}

/**
 * Renders the editor sidebar holding the document and block panels.
 * @param props - The status of the post being edited.
 * @returns The sidebar element.
 */
export function EditorSidebar({ status }: { status: string }) {
	return (
		<Tabs.Root defaultValue="document">
			<Tabs.List>
				<Tabs.Tab value="document">Document</Tabs.Tab>
				<Tabs.Tab value="block">Block</Tabs.Tab>
			</Tabs.List>
			<Tabs.Panel value="document">
				<Stack direction="column" gap="xs">
					<Text>{labelFor(status)}</Text>
				</Stack>
			</Tabs.Panel>
			<Tabs.Panel value="block">
				<BlockInspector />
			</Tabs.Panel>
		</Tabs.Root>
	)
}
