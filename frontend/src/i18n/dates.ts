// SPDX-License-Identifier: Apache-2.0

import { getSettings, setSettings } from '@wordpress/date'
import type { L10nSettings } from '@wordpress/date'
import { __, _x } from '@wordpress/i18n'

/** The days a week may start on, Sunday first, as the date settings number them. */
const WEEK_DAYS = [0, 1, 2, 3, 4, 5, 6] as const

/**
 * Returns the names of the months in the loaded catalogue, January first.
 * @returns The twelve month names.
 */
function monthNames(): string[] {
	return [
		__('January', 'gophenberg'),
		__('February', 'gophenberg'),
		__('March', 'gophenberg'),
		__('April', 'gophenberg'),
		__('May', 'gophenberg'),
		__('June', 'gophenberg'),
		__('July', 'gophenberg'),
		__('August', 'gophenberg'),
		__('September', 'gophenberg'),
		__('October', 'gophenberg'),
		__('November', 'gophenberg'),
		__('December', 'gophenberg'),
	]
}

/**
 * Returns the short names of the months in the loaded catalogue, January first.
 * @returns The twelve month abbreviations.
 */
function monthAbbreviations(): string[] {
	return [
		_x('Jan', 'January abbreviation', 'gophenberg'),
		_x('Feb', 'February abbreviation', 'gophenberg'),
		_x('Mar', 'March abbreviation', 'gophenberg'),
		_x('Apr', 'April abbreviation', 'gophenberg'),
		_x('May', 'May abbreviation', 'gophenberg'),
		_x('Jun', 'June abbreviation', 'gophenberg'),
		_x('Jul', 'July abbreviation', 'gophenberg'),
		_x('Aug', 'August abbreviation', 'gophenberg'),
		_x('Sep', 'September abbreviation', 'gophenberg'),
		_x('Oct', 'October abbreviation', 'gophenberg'),
		_x('Nov', 'November abbreviation', 'gophenberg'),
		_x('Dec', 'December abbreviation', 'gophenberg'),
	]
}

/**
 * Returns the names of the weekdays in the loaded catalogue, Sunday first.
 * @returns The seven weekday names.
 */
function weekdayNames(): string[] {
	return [
		__('Sunday', 'gophenberg'),
		__('Monday', 'gophenberg'),
		__('Tuesday', 'gophenberg'),
		__('Wednesday', 'gophenberg'),
		__('Thursday', 'gophenberg'),
		__('Friday', 'gophenberg'),
		__('Saturday', 'gophenberg'),
	]
}

/**
 * Returns the short names of the weekdays in the loaded catalogue, Sunday first.
 * @returns The seven weekday abbreviations.
 */
function weekdayAbbreviations(): string[] {
	return [
		__('Sun', 'gophenberg'),
		__('Mon', 'gophenberg'),
		__('Tue', 'gophenberg'),
		__('Wed', 'gophenberg'),
		__('Thu', 'gophenberg'),
		__('Fri', 'gophenberg'),
		__('Sat', 'gophenberg'),
	]
}

/**
 * Returns the day a week starts on as the loaded catalogue names it, Monday when it names no day.
 * @returns The first day of the week, Sunday being 0.
 */
function startOfWeek(): L10nSettings['startOfWeek'] {
	const named = _x('1', 'start of week', 'gophenberg')
	return WEEK_DAYS.find((day) => String(day) === named) ?? 1
}

/**
 * Hands the date package the names, the formats and the first weekday of the reader language, and the reader's zone.
 * @param locale - The language the admin reads in.
 */
export function startDates(locale: string): void {
	const held = getSettings()
	setSettings({
		...held,
		formats: {
			...held.formats,
			datetime: _x('F j, Y g:i a', 'date and time format', 'gophenberg'),
			datetimeAbbreviated: _x('M j, Y g:i a', 'abbreviated date and time format', 'gophenberg'),
		},
		l10n: {
			...held.l10n,
			locale,
			months: monthNames(),
			monthsShort: monthAbbreviations(),
			weekdays: weekdayNames(),
			weekdaysShort: weekdayAbbreviations(),
			startOfWeek: startOfWeek(),
		},
		timezone: {
			...held.timezone,
			offset: 0 - new Date().getTimezoneOffset() / 60,
			string: Intl.DateTimeFormat().resolvedOptions().timeZone,
		},
	})
}
