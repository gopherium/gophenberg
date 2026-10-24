// SPDX-License-Identifier: Apache-2.0

import { formatDate, formatNumber, formatTime } from '@gopherium/gottext'
import { _n, sprintf } from '@wordpress/i18n'

/** How many milliseconds a minute lasts. */
const MINUTE = 60_000

/** How many milliseconds an hour lasts. */
const HOUR = 60 * MINUTE

/** How many milliseconds a day lasts. */
const DAY = 24 * HOUR

/**
 * Returns a moment as the date and the time of day the site format writes it with.
 * @param at - The moment the server stored.
 * @returns The date and the time, named for the sentence they fill.
 */
export function dateAndTime(at: string): { date: string, time: string } {
	return { date: formatDate(at), time: formatTime(at) }
}

/**
 * Returns how long until a moment, counted the way WordPress counts a time difference.
 * @param at - The moment ahead.
 * @param now - The moment it is now, in milliseconds.
 * @returns The minutes, hours or days left, one minute at the least.
 */
export function timeUntil(at: string, now: number): string {
	const left = new Date(at).getTime() - now
	if (left < HOUR) {
		const minutes = wholeCount(left / MINUTE)
		return sprintf(_n('%s minute', '%s minutes', minutes, 'gophenberg'), formatNumber(minutes))
	}
	if (left < DAY) {
		const hours = wholeCount(left / HOUR)
		return sprintf(_n('%s hour', '%s hours', hours, 'gophenberg'), formatNumber(hours))
	}
	const days = wholeCount(left / DAY)
	return sprintf(_n('%s day', '%s days', days, 'gophenberg'), formatNumber(days))
}

/**
 * Returns an amount rounded to a whole count of one or more, as WordPress rounds a time difference.
 * @param amount - The amount, in minutes, hours or days.
 * @returns The whole count.
 */
function wholeCount(amount: number): number {
	return Math.max(1, Math.round(amount))
}
