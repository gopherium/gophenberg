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
