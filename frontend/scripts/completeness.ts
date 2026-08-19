// SPDX-License-Identifier: Apache-2.0

import { po } from 'gettext-parser'

/** The metadata entry, which carries headers rather than a message. */
const METADATA = ''

/**
 * Returns the key a message waits under, its context and message joined.
 * @param context - The context telling two senses of one word apart, if any.
 * @param msgid - The source message.
 * @returns The lookup key.
 */
function keyOf(context: string, msgid: string): string {
	return context === '' ? msgid : `${context}${msgid}`
}

/**
 * Returns every key a catalogue carries a filled translation for.
 * @param source - The catalogue as PO text.
 * @returns The keys answered.
 */
function answered(source: string): Set<string> {
	const held = new Set<string>()
	for (const [context, entries] of Object.entries(po.parse(source).translations)) {
		for (const [msgid, entry] of Object.entries(entries)) {
			if (msgid === METADATA) {
				continue
			}
			if (entry.msgstr.length > 0 && entry.msgstr.every((form) => form !== '')) {
				held.add(keyOf(context, msgid))
			}
		}
	}
	return held
}

/**
 * Returns every message a catalogue carries that the template does not name.
 * @param source - The catalogue as PO text.
 * @param template - The template naming every message the catalogue may carry.
 * @returns The keys carried without a place in the template, in the order the catalogue holds them.
 */
export function orphaned(source: string, template: string): string[] {
	const named = po.parse(template).translations
	const carried: string[] = []
	for (const [context, entries] of Object.entries(po.parse(source).translations)) {
		for (const msgid of Object.keys(entries)) {
			if (msgid !== METADATA && named[context]?.[msgid] === undefined) {
				carried.push(keyOf(context, msgid))
			}
		}
	}
	return carried
}

/** A placeholder naming what goes into it. */
const NAMED = /%\(([A-Za-z_][A-Za-z0-9_]*)\)[bcdieEfgGosuxX]/g

/** A placeholder naming nothing. */
const BARE = /%(?:\d+\$)?[bcdieEfgGosuxX]/

/**
 * Returns the placeholders a message names, each once.
 * @param message - The message to read.
 * @returns The names, in a settled order.
 */
function placeholders(message: string): string[] {
	return [...new Set([...message.matchAll(NAMED)].map((found) => found[1]))].sort()
}

/**
 * Returns whether a message carries a placeholder naming nothing.
 * @param message - The message to read.
 * @returns Whether a bare placeholder is present.
 */
function carriesBare(message: string): boolean {
	return BARE.test(message.replace(NAMED, ''))
}

/**
 * Returns whether a translated form answers its message with the placeholders that message names.
 * @param form - The translated form.
 * @param message - The message the form answers.
 * @returns Whether the form matches.
 */
function answersPlaceholders(form: string, message: string): boolean {
	if (form === '' || carriesBare(form)) {
		return form === ''
	}
	const named = placeholders(form)
	const wanted = placeholders(message)
	return named.length === wanted.length && named.every((held, at) => held === wanted[at])
}

/**
 * Returns every message whose translation names placeholders its message does not.
 * @param source - The catalogue as PO text.
 * @param template - The template naming every message and the placeholders it carries.
 * @returns The keys whose translation would not render, in the order the template holds them.
 */
export function mismatched(source: string, template: string): string[] {
	const held = po.parse(source).translations
	const broken: string[] = []
	for (const [context, entries] of Object.entries(po.parse(template).translations)) {
		for (const [msgid, entry] of Object.entries(entries)) {
			const answer = held[context]?.[msgid]
			if (msgid === METADATA || answer === undefined) {
				continue
			}
			const plural = entry.msgid_plural ?? msgid
			const matches = answer.msgstr.every((form, at) =>
				answersPlaceholders(form, at === 0 ? msgid : plural))
			if (!matches) {
				broken.push(keyOf(context, msgid))
			}
		}
	}
	return broken
}

/**
 * Returns every message of a catalogue that still waits for a translation.
 * @param source - The catalogue as PO text.
 * @param template - The template naming every message that must be answered, the catalogue itself by default.
 * @returns The keys still waiting, in the order the template holds them.
 */
export function untranslated(source: string, template: string = source): string[] {
	const held = answered(source)
	const waiting: string[] = []
	for (const [context, entries] of Object.entries(po.parse(template).translations)) {
		for (const msgid of Object.keys(entries)) {
			if (msgid === METADATA) {
				continue
			}
			const key = keyOf(context, msgid)
			if (!held.has(key)) {
				waiting.push(key)
			}
		}
	}
	return waiting
}
