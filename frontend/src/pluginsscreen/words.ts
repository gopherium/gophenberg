// SPDX-License-Identifier: Apache-2.0

import { __, _x, sprintf } from '@wordpress/i18n'

import type { InstalledPlugin, PluginsWords } from '../pluginspage'
import type { PluginTab } from './nav'

/** One tab of the Plugins list: the state it narrows the list to, none for All, beside its label. */
interface PluginsTab {
	/** The state the tab narrows the list to, none for All. */
	status: PluginTab | undefined
	/** The words the tab shows. */
	label: string
}

/** The tabs of the Plugins list and the name of the navigation holding them. */
interface PluginsTabs {
	/** The name of the navigation holding the tabs. */
	label: string
	/** Each tab, in the order the list shows them. */
	tabs: PluginsTab[]
}

/**
 * Returns the words of each state a plugin stands in, read fresh so the loaded catalogue answers.
 * @returns The words of each state, keyed by the state.
 */
function statusWords(): PluginsWords['statuses'] {
	return {
		active: _x('Active', 'plugin status', 'gophenberg'),
		paused: _x('Paused', 'plugin status', 'gophenberg'),
		inactive: _x('Inactive', 'plugin status', 'gophenberg'),
	}
}

/**
 * Returns the sentence of each step a plugin can pause at, read fresh so the loaded catalogue answers.
 * @returns Each sentence, holding one %s where the plugin's own error goes, keyed by its step.
 */
function stepSentences(): Map<string, string> {
	return new Map<string, string>([
		['register', __('It failed while the site set it up. Error message: %s', 'gophenberg')],
		['roles', __('It failed while it declared its roles. Error message: %s', 'gophenberg')],
		['types', __('It failed while it declared its content types. Error message: %s', 'gophenberg')],
		['hooks', __('It failed while the site checked its hooks. Error message: %s', 'gophenberg')],
		['routes', __('It failed while it handed over its web addresses. Error message: %s', 'gophenberg')],
		['migrate', __('It failed while it updated its database tables. Error message: %s', 'gophenberg')],
		['start', __('It failed while it started. Error message: %s', 'gophenberg')],
	])
}

/**
 * Returns the sentence of the step a plugin paused at, the general one for a step the admin does not know.
 * @param step - The step the plugin's record names, empty when it names none.
 * @returns The sentence, holding one %s where the plugin's own error goes.
 */
function pausedSentence(step: string): string {
	return stepSentences().get(step) ?? __('It failed. Error message: %s', 'gophenberg')
}

/**
 * Returns every word the Plugins page shows, read fresh so the loaded catalogue answers.
 * @returns The words, the field labels, the badge words and the sentence of each step.
 */
export function pluginsWords(): PluginsWords {
	const title = _x('Plugins', 'admin section', 'gophenberg')
	return {
		title,
		subtitle: __('Every plugin this site holds, and whether it runs.', 'gophenberg'),
		searchLabel: sprintf(__('Search %(type)s…', 'gophenberg'), { type: title.toLowerCase() }),
		loadFailed: __('The plugins could not be loaded.', 'gophenberg'),
		noPlugins: __('No plugins yet', 'gophenberg'),
		emptyTabs: {
			active: __('No active plugins', 'gophenberg'),
			paused: __('No paused plugins', 'gophenberg'),
		},
		labels: {
			name: __('Plugin', 'gophenberg'),
			paused_reason: _x('Reason', 'plugin', 'gophenberg'),
			description: __('Description', 'gophenberg'),
			status: _x('Status', 'plugin', 'gophenberg'),
			version: __('Version', 'gophenberg'),
		},
		statuses: statusWords(),
		pausedAt: pausedSentence,
	}
}

/**
 * Returns the tabs of the Plugins list, each label read fresh so the loaded catalogue answers.
 * @returns The name of the tab navigation and each tab, All first.
 */
export function pluginsTabs(): PluginsTabs {
	const statuses = statusWords()
	return {
		label: __('Filter by status', 'gophenberg'),
		tabs: [
			{ status: undefined, label: _x('All', 'status filter', 'gophenberg') },
			{ status: 'active', label: statuses.active },
			{ status: 'paused', label: statuses.paused },
		],
	}
}

/**
 * Returns a text a plugin's manifest wrote in the reader's language, an empty one kept empty.
 * @param text - The text, in English as the manifest wrote it.
 * @returns The catalogue's text, the English when the catalogue holds none.
 */
function manifestText(text: string): string {
	return text === '' ? '' : __(text, 'gophenberg')
}

/**
 * Returns a plugin with the name and the description its manifest wrote in the reader's language.
 * @param plugin - The plugin as the server lists it.
 * @returns The plugin, its own error left as it gave it.
 */
export function translatedPlugin(plugin: InstalledPlugin): InstalledPlugin {
	return { ...plugin, name: manifestText(plugin.name), description: manifestText(plugin.description) }
}
