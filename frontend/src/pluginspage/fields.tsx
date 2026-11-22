// SPDX-License-Identifier: Apache-2.0

import type { Field } from '@gophenberg/frontend-sdk/dataviews'
import { Badge } from '@wordpress/ui'

import { PausedReason } from './PausedReason'
import type { PausedRecord } from './PausedReason'

/** The state an installed plugin stands in, as the site's plugin records hold it. */
export type PluginStatus = 'active' | 'inactive' | 'paused'

/** One installed plugin as the list shows it, its name and description already in the reader's language. */
export interface InstalledPlugin extends PausedRecord {
	/** The plugin's id, as its manifest names it. */
	id: string
	/** The plugin's name. */
	name: string
	/** What the plugin does, in one short sentence, empty when its manifest says nothing. */
	description: string
	/** The version its manifest names, empty when it names none. */
	version: string
	/** The state the plugin stands in. */
	status: PluginStatus
}

/** The id of each field the list offers. */
type PluginField = 'name' | 'paused_reason' | 'description' | 'status' | 'version'

/** The words the fields read, each one in the reader's language. */
export interface FieldWords {
	/** The label of each field, keyed by its id. */
	labels: Record<PluginField, string>
	/** The words of each state's badge. */
	statuses: Record<PluginStatus, string>
	/** Returns the sentence of the step a plugin paused at, holding one `%s` where its own error goes. */
	pausedAt: (step: string) => string
}

/** The badge intent each state is drawn with, as the WPDS badge guidance names a plugin's status. */
const STATUS_INTENTS: Record<PluginStatus, 'stable' | 'high' | 'none'> = {
	active: 'stable',
	paused: 'high',
	inactive: 'none',
}

/**
 * Returns the name field, the title marked with its plugin so a later dialog finds the row again.
 * @param label - The field's label.
 * @returns The field.
 */
function nameField(label: string): Field<InstalledPlugin> {
	return {
		id: 'name',
		label,
		type: 'text',
		enableGlobalSearch: true,
		enableHiding: false,
		filterBy: false,
		render: ({ item }) => <span data-item-id={item.id}>{item.name}</span>,
	}
}

/**
 * Returns the paused reason field, the line under the name, drawn only for a paused plugin.
 * @param words - The field words.
 * @returns The field.
 */
function reasonField(words: FieldWords): Field<InstalledPlugin> {
	return {
		id: 'paused_reason',
		label: words.labels.paused_reason,
		type: 'text',
		enableSorting: false,
		filterBy: false,
		render: ({ item }) => <PausedReason plugin={item} sentenceOf={words.pausedAt} />,
	}
}

/**
 * Returns the description field, plain words a search reads, in a column of its own.
 * @param label - The field's label.
 * @returns The field.
 */
function descriptionField(label: string): Field<InstalledPlugin> {
	return { id: 'description', label, type: 'text', enableSorting: false, enableGlobalSearch: true, filterBy: false }
}

/**
 * Returns the version field, the version as the manifest writes it.
 * @param label - The field's label.
 * @returns The field.
 */
function versionField(label: string): Field<InstalledPlugin> {
	return { id: 'version', label, type: 'text', enableSorting: false, filterBy: false }
}

/**
 * Returns the status field, a text badge in the intent the state takes.
 * @param words - The field words.
 * @returns The field.
 */
function statusField(words: FieldWords): Field<InstalledPlugin> {
	return {
		id: 'status',
		label: words.labels.status,
		type: 'text',
		enableSorting: false,
		filterBy: false,
		render: ({ item }) => <Badge intent={STATUS_INTENTS[item.status]}>{words.statuses[item.status]}</Badge>,
	}
}

/**
 * Returns every field the plugins list offers, in the order the table draws them.
 * @param words - The field words.
 * @returns The name, the paused reason, the description, the status and the version.
 */
export function pluginFields(words: FieldWords): Field<InstalledPlugin>[] {
	return [
		nameField(words.labels.name),
		reasonField(words),
		descriptionField(words.labels.description),
		statusField(words),
		versionField(words.labels.version),
	]
}
