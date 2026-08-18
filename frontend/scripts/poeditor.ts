// SPDX-License-Identifier: AGPL-3.0-or-later

import { po } from 'gettext-parser'

/** Where the translation platform answers. */
const API = 'https://api.poeditor.com/v2'

/** What one language's export carries, keyed by context and message. */
type Answers = Record<string, Record<string, string[]>>

/** What the platform answers a request with. */
interface Answer {
	response: { status: string, message?: string }
	result?: { languages?: { code: string }[], url?: string }
}

/** What this repository reads a translation platform through. */
export interface Poeditor {
	languages: () => Promise<string[]>
	exportPo: (locale: string) => Promise<string>
}

/**
 * Returns the language a platform code names, in the casing a catalogue file uses.
 * @param code - The code as the platform writes it.
 * @returns The language, with its region upper cased.
 */
export function localeOf(code: string): string {
	const [language, region] = code.split('-')
	return region === undefined ? language : `${language}-${region.toUpperCase()}`
}

/**
 * Returns the translations a catalogue holds, without the headers an export restamps.
 * @param source - The catalogue as PO text.
 * @returns The translations, keyed by context and message.
 */
function answersOf(source: string): Answers {
	const held: Answers = {}
	for (const [context, entries] of Object.entries(po.parse(source).translations)) {
		held[context] = {}
		for (const [msgid, entry] of Object.entries(entries)) {
			if (msgid !== '') {
				held[context][msgid] = entry.msgstr
			}
		}
	}
	return held
}

/**
 * Reports whether an incoming catalogue says anything the current one does not.
 * @param current - The catalogue as committed, or nothing when none is committed yet.
 * @param incoming - The catalogue the platform exported.
 * @returns True when a translation differs, false when only the export stamp moved.
 */
export function meaningfulChange(current: string | undefined, incoming: string): boolean {
	if (current === undefined) {
		return true
	}
	return JSON.stringify(answersOf(current)) !== JSON.stringify(answersOf(incoming))
}

/**
 * Returns the platform's answer to one call, refusing anything that is not a success.
 * @param fetched - How a request is sent.
 * @param path - The call to make.
 * @param form - The values the call carries.
 * @returns The result the platform answered with.
 */
async function ask(
	fetched: typeof fetch,
	path: string,
	form: URLSearchParams,
): Promise<NonNullable<Answer['result']>> {
	const response = await fetched(`${API}/${path}`, { method: 'POST', body: form })
	if (!response.ok) {
		throw new Error(`the translation platform answered ${response.status}`)
	}
	const answered = (await response.json()) as Answer
	if (answered.response.status !== 'success') {
		throw new Error(`the translation platform refused: ${answered.response.message ?? 'no reason given'}`)
	}
	return answered.result ?? {}
}

/**
 * Returns the reader of one translation platform project.
 * @param token - The credential the platform answers to.
 * @param project - The project the translations live in.
 * @param fetched - How a request is sent, the runtime's own by default.
 * @returns The reader.
 */
export function poeditorAt(token: string, project: string, fetched: typeof fetch = fetch): Poeditor {
	/**
	 * Returns the values every call carries.
	 * @returns The credential and the project.
	 */
	function credentials(): URLSearchParams {
		return new URLSearchParams({ api_token: token, id: project })
	}
	return {
		languages: async () => {
			const held = await ask(fetched, 'languages/list', credentials())
			return (held.languages ?? []).map((named) => localeOf(named.code))
		},
		exportPo: async (locale: string) => {
			const form = credentials()
			form.set('language', locale.toLowerCase())
			form.set('type', 'po')
			const held = await ask(fetched, 'projects/export', form)
			if (held.url === undefined) {
				throw new Error(`the translation platform named no export for ${locale}`)
			}
			const downloaded = await fetched(held.url)
			if (!downloaded.ok) {
				throw new Error(`the export for ${locale} answered ${downloaded.status}`)
			}
			return downloaded.text()
		},
	}
}
