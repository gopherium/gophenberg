// SPDX-License-Identifier: Apache-2.0

import { __ } from '@wordpress/i18n'

import { holdsRows } from './types'
import type { Choice } from './select'
import type { ChoicePair, ContentField } from './types'

/** The tab of an open field a setting shows in. */
export type SettingTab = 'general' | 'validation' | 'presentation'

/** One setting a kind takes, as its control edits it, and the tab it shows in. */
export interface SettingControl {
	name: string
	label: string
	shape: 'text' | 'textarea' | 'number' | 'choice'
	tab: SettingTab
}

/**
 * Returns the settings a field takes, in the order the controls show them.
 * @param field - The declared field to tune.
 * @returns The settings to offer.
 */
export function settingsOffered(field: ContentField): SettingControl[] {
	const kind = field.kind
	const held: SettingControl[] = [
		{ name: 'instructions', label: __('Instructions', 'gophenberg'), shape: 'textarea', tab: 'presentation' },
	]
	if (kind === 'media' && field.many) {
		held.push({ name: 'min', label: __('Fewest files', 'gophenberg'), shape: 'number', tab: 'validation' })
		held.push({ name: 'max', label: __('Most files', 'gophenberg'), shape: 'number', tab: 'validation' })
	}
	if (kind === 'text') {
		held.push({ name: 'default', label: __('Default', 'gophenberg'), shape: 'text', tab: 'general' })
		held.push({ name: 'placeholder', label: __('Placeholder', 'gophenberg'), shape: 'text', tab: 'presentation' })
		held.push({ name: 'maxlength', label: __('Longest', 'gophenberg'), shape: 'number', tab: 'validation' })
	}
	if (kind === 'number') {
		held.push({ name: 'default', label: __('Default', 'gophenberg'), shape: 'number', tab: 'general' })
		held.push({ name: 'min', label: __('Lowest', 'gophenberg'), shape: 'number', tab: 'validation' })
		held.push({ name: 'max', label: __('Highest', 'gophenberg'), shape: 'number', tab: 'validation' })
		held.push({ name: 'step', label: __('Steps of', 'gophenberg'), shape: 'number', tab: 'presentation' })
	}
	if (kind === 'boolean') {
		held.push({ name: 'default', label: __('Default', 'gophenberg'), shape: 'choice', tab: 'general' })
	}
	if (kind === 'choice') {
		held.push({ name: 'multiple', label: __('Many values', 'gophenberg'), shape: 'choice', tab: 'general' })
		held.push({ name: 'allow_custom', label: __('Allow custom', 'gophenberg'), shape: 'choice', tab: 'presentation' })
		held.push({ name: 'allow_null', label: __('Allow empty', 'gophenberg'), shape: 'choice', tab: 'validation' })
	}
	if (holdsRows(kind)) {
		held.push({ name: 'min', label: __('Fewest rows', 'gophenberg'), shape: 'number', tab: 'validation' })
		held.push({ name: 'max', label: __('Most rows', 'gophenberg'), shape: 'number', tab: 'validation' })
	}
	return held
}

/**
 * Returns the settings with the complete choice pairs the editor holds written in.
 * @param settings - The settings as the controls stored them.
 * @param choicePairs - The pairs the editor rows hold.
 * @returns The settings to store, with no choices key when no pair is complete.
 */
export function pairedSettings(settings: Record<string, unknown>, choicePairs: ChoicePair[]): Record<string, unknown> {
	const completePairs = choicePairs.filter((pair) => pair.value !== '' && pair.label !== '')
	const settingsWithChoices = { ...settings }
	if (completePairs.length === 0) {
		delete settingsWithChoices.choices
		return settingsWithChoices
	}
	settingsWithChoices.choices = completePairs
	return settingsWithChoices
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
 * Reports whether a control still holds the text of the value the field carries.
 * @param controlText - The text the control holds.
 * @param storedValue - The value the field carries.
 * @returns Whether the control is untouched.
 */
function isUntouchedControl(controlText: string, storedValue: unknown): boolean {
	return storedValue !== undefined && storedValue !== null && controlText === String(storedValue)
}

/**
 * Returns the settings to store, untouched controls as stored and every emptied control left out.
 * @param controls - The setting controls the kind takes.
 * @param controlTexts - The text the controls hold.
 * @param currentSettings - The settings the field carries.
 * @returns The settings as the registry stores them.
 */
export function storedSettings(
	controls: SettingControl[],
	controlTexts: Record<string, string>,
	currentSettings: Record<string, unknown>,
): Record<string, unknown> {
	const settings: Record<string, unknown> = { ...currentSettings }
	for (const control of controls) {
		if (isUntouchedControl(controlTexts[control.name], currentSettings[control.name])) {
			continue
		}
		const trimmedText = controlTexts[control.name].trim()
		if (trimmedText === '') {
			delete settings[control.name]
			continue
		}
		settings[control.name] = settingValueOf(control.shape, trimmedText)
	}
	return settings
}

/**
 * Returns the text of a control as the value of the shape its setting stores.
 * @param shape - The shape the setting takes.
 * @param text - The text the control holds.
 * @returns The value to store.
 */
function settingValueOf(shape: SettingControl['shape'], text: string): unknown {
	if (shape === 'number') {
		return Number(text)
	}
	if (shape === 'choice') {
		return text === 'true'
	}
	return text
}

/** The kinds the admin list can show as a column. */
export const LISTABLE = ['text', 'number', 'boolean', 'date', 'choice']
