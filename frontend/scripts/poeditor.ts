// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { join } from 'node:path'

import { po } from 'gettext-parser'

/** Where the application declares the languages it answers in. */
const LOCALE_SOURCE = ['internal', 'content', 'locale.go']

/**
 * Returns the languages the application answers in, as it declares them.
 * @param root - The repository root the declaration sits under.
 * @returns The languages, the fallback first.
 */
export function supportedLocales(root: string): string[] {
	const source = readFileSync(join(root, ...LOCALE_SOURCE), 'utf8')
	const declared = /DefaultLocale\s*=\s*"([^"]+)"/.exec(source)
	const listed = /SupportedLocales\s*=\s*\[\]string\{([^}]*)\}/.exec(source)
	if (declared === null || listed === null) {
		throw new Error('the application declares no supported languages')
	}
	return listed[1]
		.split(',')
		.map((held) => held.trim())
		.filter((held) => held !== '')
		.map((held) => (held === 'DefaultLocale' ? declared[1] : held.replace(/"/g, '')))
}

/**
 * Returns the language the application knows a platform code as, if it knows one.
 * @param code - The language as the platform names it.
 * @param supported - The languages the application answers in.
 * @returns The application's own name for it, or nothing when it answers in no such language.
 */
export function localeFor(code: string, supported: string[]): string | undefined {
	if (supported.includes(code)) {
		return code
	}
	if (code.includes('-')) {
		return undefined
	}
	const sharing = supported.filter((held) => held.split('-')[0] === code)
	return sharing.length === 1 ? sharing[0] : undefined
}

/**
 * Returns how many messages a catalogue answers.
 * @param source - The catalogue as PO text.
 * @returns The count of messages carrying a translation.
 */
export function translated(source: string): number {
	let held = 0
	for (const entries of Object.values(po.parse(source).translations)) {
		for (const [msgid, entry] of Object.entries(entries)) {
			if (msgid !== '' && entry.msgstr.some((form) => form !== '')) {
				held += 1
			}
		}
	}
	return held
}

/**
 * Returns an incoming catalogue under the plural rule the committed one declares.
 * @param current - The catalogue as committed.
 * @param incoming - The catalogue the platform exported.
 * @returns The incoming catalogue, carrying the committed plural rule and no form beyond it.
 */
export function withPluralRuleOf(current: string, incoming: string): string {
	const parsed = po.parse(current)
	const rule = parsed.headers['Plural-Forms']
	if (rule === undefined) {
		return incoming
	}
	const held = po.parse(incoming)
	held.headers['Plural-Forms'] = rule
	const forms = Number(/nplurals\s*=\s*(\d+)/.exec(rule)?.[1] ?? 1)
	for (const entries of Object.values(held.translations)) {
		for (const entry of Object.values(entries)) {
			if (entry.msgstr.length > forms) {
				entry.msgstr = entry.msgstr.slice(0, forms)
			}
		}
	}
	return po.compile(held, { foldLength: 0, eol: '\n' }).toString()
}

/** How long any one call to the platform may take. */
const REQUEST_TIMEOUT = 30_000

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
	uploadTerms: (source: string) => Promise<void>
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
	for (const [context, entries] of Object.entries(po.parse(source).translations).sort()) {
		held[context] = {}
		for (const [msgid, entry] of Object.entries(entries).sort()) {
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
	const response = await fetched(`${API}/${path}`, {
		method: 'POST',
		body: form,
		signal: AbortSignal.timeout(REQUEST_TIMEOUT),
	})
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
		/**
		 * Returns every language the project carries.
		 * @returns The languages, named as a catalogue file names them.
		 */
		languages: async () => {
			const held = await ask(fetched, 'languages/list', credentials())
			return (held.languages ?? []).map((named) => localeOf(named.code))
		},
		/**
		 * Adds to the platform every term the template names, leaving translations alone.
		 * @param source - The catalogue template as POT text.
		 */
		uploadTerms: async (source: string) => {
			const form = new FormData()
			form.set('api_token', token)
			form.set('id', project)
			form.set('updating', 'terms')
			form.set('file', new Blob([source]), 'gophenberg.pot')
			const response = await fetched(`${API}/projects/upload`, {
				method: 'POST',
				body: form,
				signal: AbortSignal.timeout(REQUEST_TIMEOUT),
			})
			if (!response.ok) {
				throw new Error(`the translation platform answered ${response.status}`)
			}
			const answered = (await response.json()) as Answer
			if (answered.response.status !== 'success') {
				throw new Error(
					`the translation platform refused: ${answered.response.message ?? 'no reason given'}`,
				)
			}
		},
		/**
		 * Returns one language's catalogue as the platform exports it.
		 * @param locale - The language to export.
		 * @returns The catalogue as PO text.
		 */
		exportPo: async (locale: string) => {
			const form = credentials()
			form.set('language', locale.toLowerCase())
			form.set('type', 'po')
			const held = await ask(fetched, 'projects/export', form)
			if (held.url === undefined) {
				throw new Error(`the translation platform named no export for ${locale}`)
			}
			const downloaded = await fetched(held.url, { signal: AbortSignal.timeout(REQUEST_TIMEOUT) })
			if (!downloaded.ok) {
				throw new Error(`the export for ${locale} answered ${downloaded.status}`)
			}
			return downloaded.text()
		},
	}
}
