// SPDX-License-Identifier: Apache-2.0

import { __ } from '@wordpress/i18n'

import { holdsRows } from './types'
import type { Choice } from './select'
import type { ChoicePair, ContentField } from './types'

/** One setting a kind takes, as its control edits it. */
interface SettingControl {
	name: string
	label: string
	shape: 'text' | 'number' | 'choice'
}

/**
 * Returns the settings a field takes, in the order the controls show them.
 * @param field - The declared field to tune.
 * @returns The settings to offer.
 */
export function settingsOffered(field: ContentField): SettingControl[] {
	const kind = field.kind
	const held: SettingControl[] = [
		{ name: 'instructions', label: __('Instructions', 'gophenberg'), shape: 'text' },
	]
	if (kind === 'media' && field.many) {
		held.push({ name: 'min', label: __('Fewest files', 'gophenberg'), shape: 'number' })
		held.push({ name: 'max', label: __('Most files', 'gophenberg'), shape: 'number' })
	}
	if (kind === 'text') {
		held.push({ name: 'default', label: __('Default', 'gophenberg'), shape: 'text' })
		held.push({ name: 'placeholder', label: __('Placeholder', 'gophenberg'), shape: 'text' })
		held.push({ name: 'maxlength', label: __('Longest', 'gophenberg'), shape: 'number' })
	}
	if (kind === 'number') {
		held.push({ name: 'default', label: __('Default', 'gophenberg'), shape: 'number' })
		held.push({ name: 'min', label: __('Lowest', 'gophenberg'), shape: 'number' })
		held.push({ name: 'max', label: __('Highest', 'gophenberg'), shape: 'number' })
		held.push({ name: 'step', label: __('Steps of', 'gophenberg'), shape: 'number' })
	}
	if (kind === 'boolean') {
		held.push({ name: 'default', label: __('Default', 'gophenberg'), shape: 'choice' })
	}
	if (kind === 'choice') {
		held.push({ name: 'multiple', label: __('Many values', 'gophenberg'), shape: 'choice' })
		held.push({ name: 'allow_custom', label: __('Allow custom', 'gophenberg'), shape: 'choice' })
		held.push({ name: 'allow_null', label: __('Allow empty', 'gophenberg'), shape: 'choice' })
	}
	if (holdsRows(kind)) {
		held.push({ name: 'min', label: __('Fewest rows', 'gophenberg'), shape: 'number' })
		held.push({ name: 'max', label: __('Most rows', 'gophenberg'), shape: 'number' })
	}
	return held
}

/**
 * Returns the settings with the choice pairs the editor holds written in.
 * @param kind - The kind the field holds.
 * @param settings - The settings as the controls stored them.
 * @param pairs - The pairs the editor rows hold.
 * @returns The settings to store.
 */
export function pairedSettings(
	kind: string,
	settings: Record<string, unknown>,
	pairs: ChoicePair[],
): Record<string, unknown> {
	if (kind !== 'choice') {
		return settings
	}
	const settled = pairs.filter((pair) => pair.value !== '' && pair.label !== '')
	const held = { ...settings }
	if (settled.length === 0) {
		delete held.choices
		return held
	}
	held.choices = settled
	return held
}

/**
 * Returns the answers a boolean setting offers, the first one leaving it unset.
 * @returns The answers to offer.
 */
export function settingAnswers(): Choice[] {
	return [
		{ label: __('None', 'gophenberg'), value: '' },
		{ label: __('Yes', 'gophenberg'), value: 'true' },
		{ label: __('No', 'gophenberg'), value: 'false' },
	]
}

/**
 * Returns the settings as the controls hold them, every value written out.
 * @param offered - The settings the kind takes.
 * @param held - The settings the field carries.
 * @returns The text each control starts with.
 */
export function typedSettings(offered: SettingControl[], held: Record<string, unknown>): Record<string, string> {
	const written: Record<string, string> = {}
	for (const setting of offered) {
		const value = held[setting.name]
		written[setting.name] = value === undefined || value === null ? '' : String(value)
	}
	return written
}

/**
 * Returns the settings to store, leaving out every control the operator left empty.
 * @param offered - The settings the kind takes.
 * @param typed - The text the controls hold.
 * @param held - The settings the field carries.
 * @returns The settings as the registry stores them.
 */
export function storedSettings(
	offered: SettingControl[],
	typed: Record<string, string>,
	held: Record<string, unknown>,
): Record<string, unknown> {
	const settings: Record<string, unknown> = { ...held }
	for (const setting of offered) {
		const written = typed[setting.name].trim()
		if (written === '') {
			delete settings[setting.name]
			continue
		}
		settings[setting.name] = settingHeld(setting.shape, written)
	}
	return settings
}

/**
 * Returns what a control holds as the shape its setting stores.
 * @param shape - The shape the setting takes.
 * @param written - The text the control holds.
 * @returns The value to store.
 */
function settingHeld(shape: SettingControl['shape'], written: string): unknown {
	if (shape === 'number') {
		return Number(written)
	}
	if (shape === 'choice') {
		return written === 'true'
	}
	return written
}

/** The kinds the admin list can show as a column. */
export const LISTABLE = ['text', 'number', 'boolean', 'date', 'choice']
