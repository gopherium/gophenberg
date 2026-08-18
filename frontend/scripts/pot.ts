// SPDX-License-Identifier: Apache-2.0

import { GettextExtractor, JsExtractors } from 'gettext-extractor'
import { po, type GetTextTranslation } from 'gettext-parser'
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'

/** The text domain every Gophenberg owned string names. */
const DOMAIN = 'gophenberg'

/** The globs holding every string the admin ships, read from the repository root. */
const SOURCES = ['frontend/src/**/*.{ts,tsx}', 'sdk/frontend/**/*.{ts,tsx}']

/** The paths inside those globs that ship no string. */
const IGNORED = [
	'**/node_modules/**',
	'**/*.d.ts',
	'**/test/**',
	'**/*.test.ts',
	'**/*.test.tsx',
	'**/stubs/**',
	'**/scripts/**',
]

/** The comment positions the extractor leaves out of the template. */
const NO_COMMENTS = { otherLineLeading: false, sameLineLeading: false, sameLineTrailing: false }

/** The header the generated template carries, in this order. */
const HEADERS = {
	'Project-Id-Version': DOMAIN,
	'MIME-Version': '1.0',
	'Content-Type': 'text/plain; charset=UTF-8',
	'Content-Transfer-Encoding': '8bit',
	'Plural-Forms': 'nplurals=2; plural=(n != 1);',
	'X-Domain': DOMAIN,
}


/** The directories a source walk skips whole. */
const SKIPPED_DIRS = new Set(['node_modules', 'dist', 'coverage'])

/** The directories the Go side's messages live under. */
const GO_ROOTS = ['internal', 'cmd', 'plugins']

/** The shape a Go template asks the translator with. */
const TEMPLATE_CALL = /\bT\.Get "((?:[^"\\]|\\.)*)"/g

/** The shape Go source marks a message with. */
const MARKER_CALL = /\bMsgid\("((?:[^"\\]|\\.)*)"\)/g

/**
 * Returns every file under a directory, skipping installed and built trees.
 * @param dir - The directory to walk.
 * @returns The files found, as absolute paths.
 */
function sourceFiles(dir: string): string[] {
	const held: string[] = []
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		if (entry.isDirectory()) {
			if (!SKIPPED_DIRS.has(entry.name)) {
				held.push(...sourceFiles(join(dir, entry.name)))
			}
			continue
		}
		held.push(join(dir, entry.name))
	}
	return held
}

/**
 * Returns the message a captured Go quoted string holds.
 * @param raw - The captured text between the quotes.
 * @param file - The file the capture came from.
 * @returns The message with its escapes resolved.
 */
export function goString(raw: string, file: string): string {
	try {
		return JSON.parse(`"${raw}"`) as string
	} catch {
		throw new Error(`the string ${raw} in ${file} holds an escape this extractor cannot read`)
	}
}

/**
 * Adds the messages one file declares to the given set.
 * @param file - The file to read.
 * @param held - The set the messages land in.
 */
function collect(file: string, held: Set<string>): void {
	if (file.endsWith('.html')) {
		for (const found of readFileSync(file, 'utf8').matchAll(TEMPLATE_CALL)) {
			held.add(goString(found[1], file))
		}
		return
	}
	if (!file.endsWith('.go') || file.endsWith('_test.go')) {
		return
	}
	for (const found of readFileSync(file, 'utf8').matchAll(MARKER_CALL)) {
		held.add(goString(found[1], file))
	}
}

/**
 * Returns every message the Go side declares, through its markers and its templates.
 * @param root - The repository root the walk starts from.
 * @param dirs - The directories to walk, the Go side's by default.
 * @returns The messages found.
 */
export function goMessages(root: string, dirs: string[] = GO_ROOTS): string[] {
	const held = new Set<string>()
	for (const dir of dirs) {
		for (const file of sourceFiles(join(root, dir))) {
			collect(file, held)
		}
	}
	return [...held]
}

/** A message one of the four gettext calls declared. */
export interface Found {
	text: string
	textPlural: string | null
	context: string | null
}

/**
 * Returns the repository root the source globs resolve against.
 * @returns The absolute path of the repository root.
 */
export function repositoryRoot(): string {
	return join(import.meta.dirname, '..', '..')
}

/**
 * Returns the key an entry is ordered by.
 * @param entry - The entry to key.
 * @returns The context and message joined.
 */
function keyOf(entry: GetTextTranslation): string {
	return `${entry.msgctxt ?? ''} ${entry.msgid}`
}

/**
 * Returns the ordering placing entries by context then message, by code unit.
 * @param left - The entry on the left.
 * @param right - The entry on the right.
 * @returns A negative number when the left entry sorts first.
 */
function byKey(left: GetTextTranslation, right: GetTextTranslation): number {
	return Number(keyOf(left) > keyOf(right)) - Number(keyOf(left) < keyOf(right))
}

/**
 * Returns every translatable message the given sources declare.
 * @param root - The repository root the globs resolve against.
 * @param sources - The globs to read, the admin sources by default.
 * @returns The messages the extractor found.
 */
export function messages(root: string, sources: string[] = SOURCES): Found[] {
	const extractor = new GettextExtractor()
	const parser = extractor.createJsParser([
		JsExtractors.callExpression(['__', 'i18n.__'], {
			arguments: { text: 0 },
			comments: NO_COMMENTS,
		}),
		JsExtractors.callExpression(['_x', 'i18n._x'], {
			arguments: { text: 0, context: 1 },
			comments: NO_COMMENTS,
		}),
		JsExtractors.callExpression(['_n', 'i18n._n'], {
			arguments: { text: 0, textPlural: 1 },
			comments: NO_COMMENTS,
		}),
		JsExtractors.callExpression(['_nx', 'i18n._nx'], {
			arguments: { text: 0, textPlural: 1, context: 3 },
			comments: NO_COMMENTS,
		}),
	])
	for (const pattern of sources) {
		parser.parseFilesGlob(pattern, { cwd: root, ignore: IGNORED, absolute: true })
	}
	return extractor.getMessages() as Found[]
}

/**
 * Returns the catalogue template built from the given sources.
 * @param root - The repository root the globs resolve against.
 * @param sources - The globs to read, the admin sources by default.
 * @returns The template as UTF-8 bytes.
 */
export function pot(root: string, sources: string[] = SOURCES): Buffer {
	const translations: Record<string, Record<string, GetTextTranslation>> = {}
	for (const text of goMessages(root)) {
		translations[''] ??= {}
		translations[''][text] = { msgid: text, msgstr: [''] }
	}
	for (const message of messages(root, sources)) {
		const context = message.context ?? ''
		translations[context] ??= {}
		translations[context][message.text] = {
			msgctxt: message.context ?? undefined,
			msgid: message.text,
			msgid_plural: message.textPlural ?? undefined,
			msgstr: message.textPlural ? ['', ''] : [''],
		}
	}
	return po.compile(
		{ charset: 'UTF-8', headers: HEADERS, translations },
		{ sort: byKey, foldLength: 0, eol: '\n' },
	)
}
