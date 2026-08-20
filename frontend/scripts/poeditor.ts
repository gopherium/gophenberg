// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { join } from 'node:path'

import { po } from 'gettext-parser'
import type { GetTextTranslation, GetTextTranslations } from 'gettext-parser'

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
	const doubled = localeOf(`${code}-${code}`)
	return supported.includes(doubled) ? doubled : undefined
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
 * Returns how many translated forms a catalogue carries.
 * @param source - The catalogue as PO text.
 * @returns The count of answered forms across every message.
 */
export function answeredForms(source: string): number {
	let held = 0
	for (const entries of Object.values(po.parse(source).translations)) {
		for (const [msgid, entry] of Object.entries(entries)) {
			if (msgid !== '') {
				held += entry.msgstr.filter((form) => form !== '').length
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
	const counted = /nplurals\s*=\s*(\d+)/.exec(rule)
	const forms = counted === null ? 0 : Number(counted[1])
	if (forms < 1) {
		throw new Error('the committed catalogue declares a plural rule naming no usable count')
	}
	for (const entries of Object.values(held.translations)) {
		for (const entry of Object.values(entries)) {
			if (entry.msgstr.length > forms) {
				entry.msgstr = entry.msgstr.slice(0, forms)
			}
		}
	}
	return po.compile(held, { foldLength: 0, eol: '\n' }).toString()
}

/**
 * Returns an incoming catalogue restoring every committed answer it lost.
 * @param current - The catalogue as committed.
 * @param incoming - The catalogue the platform exported.
 * @param template - The template naming every message the catalogue may carry.
 * @returns The incoming catalogue, keeping each committed answer the export holds empty or lacks.
 */
export function keepingAnswers(current: string, incoming: string, template: string): string {
	const named = po.parse(template).translations
	const ours = po.parse(current).translations
	const held = po.parse(incoming)
	for (const [context, entries] of Object.entries(ours)) {
		for (const [msgid, entry] of Object.entries(entries)) {
			if (msgid === '' || named[context]?.[msgid] === undefined || !answered(entry)) {
				continue
			}
			restoring(held, context, msgid, entry)
		}
	}
	return po.compile(held, { foldLength: 0, eol: '\n' }).toString()
}

/**
 * Reports whether a catalogue entry carries any translated form.
 * @param entry - The entry to read.
 * @returns True when a form holds text.
 */
function answered(entry: GetTextTranslation): boolean {
	return entry.msgstr.some((form) => form !== '')
}

/**
 * Returns the forms a catalogue keeps, taking each answered form over an empty one.
 * @param ours - The forms as committed.
 * @param theirs - The forms the platform exported.
 * @returns The forms, answered wherever either side answers them.
 */
function mergedForms(ours: string[], theirs: string[]): string[] {
	const held: string[] = []
	for (let at = 0; at < Math.max(ours.length, theirs.length); at += 1) {
		const arrived = theirs[at] ?? ''
		held.push(arrived === '' ? (ours[at] ?? '') : arrived)
	}
	return held
}

/**
 * Writes one committed answer into a catalogue that arrived without every form of it.
 * @param held - The catalogue the platform exported, as parsed.
 * @param context - The context the answer sits under.
 * @param msgid - The message the answer belongs to.
 * @param entry - The committed answer.
 */
function restoring(
	held: GetTextTranslations,
	context: string,
	msgid: string,
	entry: GetTextTranslation,
): void {
	const arrived = held.translations[context]?.[msgid]
	if (arrived === undefined) {
		held.translations[context] ??= {}
		held.translations[context][msgid] = entry
		return
	}
	arrived.msgid_plural ??= entry.msgid_plural
	arrived.msgstr = mergedForms(entry.msgstr, arrived.msgstr)
}

/**
 * Returns an incoming catalogue without any message the template does not name.
 * @param incoming - The catalogue the platform exported.
 * @param template - The template naming every message the catalogue may carry.
 * @returns The incoming catalogue, trimmed to the template.
 */
export function namedByTemplate(incoming: string, template: string): string {
	const named = po.parse(template).translations
	const held = po.parse(incoming)
	for (const [context, entries] of Object.entries(held.translations)) {
		for (const msgid of Object.keys(entries)) {
			if (msgid !== '' && named[context]?.[msgid] === undefined) {
				delete held.translations[context][msgid]
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
	result?: { languages?: { code: string }[], url?: string, terms?: { deleted?: number } }
}

/** What this repository reads a translation platform through. */
export interface Poeditor {
	languages: () => Promise<string[]>
	exportPo: (locale: string) => Promise<string>
	uploadTerms: (source: string) => Promise<void>
}

/** What this repository retires a platform's absent terms through. */
export interface Retiring {
	retireTerms: (source: string) => Promise<number>
}

/**
 * Returns how many messages a template names.
 * @param source - The template as POT text.
 * @returns The count of named messages.
 */
function namedMessages(source: string): number {
	let held = 0
	for (const entries of Object.values(po.parse(source).translations)) {
		for (const msgid of Object.keys(entries)) {
			if (msgid !== '') {
				held += 1
			}
		}
	}
	return held
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
 * Returns the reader and retirer of one translation platform project.
 * @param token - The credential the platform answers to.
 * @param project - The project the translations live in.
 * @param fetched - How a request is sent, the runtime's own by default.
 * @returns The reader, carrying the retirement its own interface names.
 */
export function poeditorAt(
	token: string,
	project: string,
	fetched: typeof fetch = fetch,
): Poeditor & Retiring {
	/**
	 * Returns the values every call carries.
	 * @returns The credential and the project.
	 */
	function credentials(): URLSearchParams {
		return new URLSearchParams({ api_token: token, id: project })
	}
	/**
	 * Sends the template to the platform, saying whether absent terms retire.
	 * @param source - The catalogue template as POT text.
	 * @param retiring - Whether terms the template does not name are deleted.
	 * @returns The result the platform answered with.
	 */
	async function sendTemplate(source: string, retiring: boolean): Promise<NonNullable<Answer['result']>> {
		const form = new FormData()
		form.set('api_token', token)
		form.set('id', project)
		form.set('updating', 'terms')
		if (retiring) {
			form.set('sync_terms', '1')
		}
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
		return answered.result ?? {}
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
			await sendTemplate(source, false)
		},
		/**
		 * Deletes from the platform every term the template does not name.
		 * @param source - The catalogue template as POT text.
		 * @returns How many terms the platform deleted.
		 */
		retireTerms: async (source: string) => {
			if (namedMessages(source) === 0) {
				throw new Error('the template names no messages, so retiring would empty the project')
			}
			return (await sendTemplate(source, true)).terms?.deleted ?? 0
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
