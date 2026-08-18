// SPDX-License-Identifier: Apache-2.0

import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { expect, test, vi } from 'vitest'

import {
	localeFor,
	localeOf,
	meaningfulChange,
	poeditorAt,
	supportedLocales,
	translated,
	withPluralRuleOf,
} from '../../scripts/poeditor.ts'
import { repositoryRoot } from '../../scripts/pot.ts'

const HEADER = `msgid ""
msgstr ""
"Language: es-ES\\n"
"Content-Type: text/plain; charset=UTF-8\\n"
"PO-Revision-Date: 2026-08-18 09:00+0000\\n"
`

/**
 * Returns a catalogue carrying one message.
 * @param stamp - The revision date the export was stamped with.
 * @param msgstr - The translation the catalogue holds.
 * @returns The catalogue as PO text.
 */
function catalogue(stamp: string, msgstr: string): string {
	return `${HEADER.replace('2026-08-18 09:00+0000', stamp)}
msgid "Older posts"
msgstr "${msgstr}"
`
}

test('reports no change when only the export stamp moved', () => {
	const before = catalogue('2026-08-18 09:00+0000', 'Entradas anteriores')
	const after = catalogue('2026-08-19 10:30+0000', 'Entradas anteriores')

	expect(meaningfulChange(before, after)).toBe(false)
})

test('reports a change when a translation moved', () => {
	const before = catalogue('2026-08-18 09:00+0000', 'Entradas anteriores')
	const after = catalogue('2026-08-18 09:00+0000', 'Entradas previas')

	expect(meaningfulChange(before, after)).toBe(true)
})

test('reports a change when a message arrives that was not there', () => {
	const before = catalogue('2026-08-18 09:00+0000', 'Entradas anteriores')
	const after = `${before}
msgid "Newer posts"
msgstr "Entradas más recientes"
`

	expect(meaningfulChange(before, after)).toBe(true)
})

test('reports a change when a translation was emptied', () => {
	const before = catalogue('2026-08-18 09:00+0000', 'Entradas anteriores')
	const after = catalogue('2026-08-18 09:00+0000', '')

	expect(meaningfulChange(before, after)).toBe(true)
})

test('reports a change against a catalogue that does not exist yet', () => {
	expect(meaningfulChange(undefined, catalogue('2026-08-18 09:00+0000', 'Hola'))).toBe(true)
})

test('reads a language the platform names in its own casing', () => {
	expect(localeOf('es-es')).toBe('es-ES')
	expect(localeOf('en')).toBe('en')
	expect(localeOf('pt-br')).toBe('pt-BR')
})

test('asks the platform for the languages a project carries', async () => {
	const sent: Array<{ url: string, body: string }> = []
	const fetched = vi.fn(async (url: string, init: { body: URLSearchParams }) => {
		sent.push({ url, body: init.body.toString() })
		return {
			ok: true,
			json: async () => ({ response: { status: 'success' }, result: { languages: [{ code: 'es-es' }] } }),
		}
	})

	const held = await poeditorAt('a-token', '12345', fetched as never).languages()

	expect(held).toEqual(['es-ES'])
	expect(sent[0].url).toContain('languages/list')
	expect(sent[0].body).toContain('api_token=a-token')
	expect(sent[0].body).toContain('id=12345')
})

test('refuses a platform answer that reports a failure', async () => {
	const fetched = vi.fn(async () => ({
		ok: true,
		json: async () => ({ response: { status: 'fail', message: 'invalid token' } }),
	}))

	await expect(poeditorAt('bad', '1', fetched as never).languages()).rejects.toThrow(/invalid token/)
})

test('refuses a platform answer that did not arrive', async () => {
	const fetched = vi.fn(async () => ({ ok: false, status: 503, json: async () => ({}) }))

	await expect(poeditorAt('a', '1', fetched as never).languages()).rejects.toThrow(/503/)
})

test('downloads the export the platform prepared', async () => {
	const asked: string[] = []
	const fetched = vi.fn(async (url: string, init?: { body?: URLSearchParams }) => {
		asked.push(url)
		if (init?.body !== undefined) {
			expect(init.body.toString()).toContain('language=es-es')
			return {
				ok: true,
				json: async () => ({ response: { status: 'success' }, result: { url: 'https://held/es.po' } }),
			}
		}
		return { ok: true, text: async () => 'msgid ""\nmsgstr ""\n' }
	})

	const held = await poeditorAt('t', '1', fetched as never).exportPo('es-ES')

	expect(held).toContain('msgid')
	expect(asked[1]).toBe('https://held/es.po')
})

test('refuses an export the platform named no address for', async () => {
	const fetched = vi.fn(async () => ({
		ok: true,
		json: async () => ({ response: { status: 'success' }, result: {} }),
	}))

	await expect(poeditorAt('t', '1', fetched as never).exportPo('es-ES')).rejects.toThrow(/named no export/)
})

test('refuses an export that could not be downloaded', async () => {
	const fetched = vi.fn(async (_url: string, init?: { body?: URLSearchParams }) =>
		init?.body !== undefined
			? { ok: true, json: async () => ({ response: { status: 'success' }, result: { url: 'https://held/es.po' } }) }
			: { ok: false, status: 404 },
	)

	await expect(poeditorAt('t', '1', fetched as never).exportPo('es-ES')).rejects.toThrow(/404/)
})

test('reads a platform answer that carries no result at all', async () => {
	const fetched = vi.fn(async () => ({ ok: true, json: async () => ({ response: { status: 'success' } }) }))

	expect(await poeditorAt('t', '1', fetched as never).languages()).toEqual([])
})

test('names no reason when the platform refused without one', async () => {
	const fetched = vi.fn(async () => ({ ok: true, json: async () => ({ response: { status: 'fail' } }) }))

	await expect(poeditorAt('t', '1', fetched as never).languages()).rejects.toThrow(/no reason given/)
})

test('reads the languages the application says it answers in', () => {
	expect(supportedLocales(repositoryRoot())).toEqual(['en-US', 'es-ES'])
})

test('matches a language named exactly as the application names it', () => {
	expect(localeFor('es-ES', ['en-US', 'es-ES'])).toBe('es-ES')
})

test('matches a bare language against the one region the application supports', () => {
	expect(localeFor('es', ['en-US', 'es-ES'])).toBe('es-ES')
})

test('refuses a bare language when two regions of it are supported', () => {
	expect(localeFor('es', ['es-ES', 'es-MX'])).toBeUndefined()
})

test('refuses a language the application does not answer in', () => {
	expect(localeFor('fr', ['en-US', 'es-ES'])).toBeUndefined()
})

test('reports a catalogue nobody has translated yet', () => {
	const bare = `msgid ""
msgstr ""
"Language: fr\\n"

msgid "Older posts"
msgstr ""
`

	expect(translated(bare)).toBe(0)
})

test('counts the messages a catalogue answers', () => {
	const held = `msgid ""
msgstr ""
"Language: es-ES\\n"

msgid "Older posts"
msgstr "Entradas anteriores"
`

	expect(translated(held)).toBe(1)
})

test('keeps the plural rule the committed catalogue declares', () => {
	const ours = `msgid ""
msgstr ""
"Language: es-ES\\n"
"Plural-Forms: nplurals=2; plural=(n != 1);\\n"
`
	const theirs = `msgid ""
msgstr ""
"Language: es-ES\\n"
"Plural-Forms: nplurals=3; plural=(n==1 ? 0 : n==2 ? 1 : 2);\\n"

msgid "%d post"
msgid_plural "%d posts"
msgstr[0] "%d entrada"
msgstr[1] "%d entradas"
msgstr[2] ""
`

	const held = withPluralRuleOf(ours, theirs)

	expect(held).toContain('nplurals=2')
	expect(held).not.toContain('nplurals=3')
	expect(held).not.toMatch(/msgstr\[2\]/)
})

test('refuses a source that declares no supported languages', () => {
	const root = mkdtempSync(join(tmpdir(), 'gophenberg-locale-'))
	mkdirSync(join(root, 'internal', 'content'), { recursive: true })
	writeFileSync(join(root, 'internal', 'content', 'locale.go'), 'package content\n')

	expect(() => supportedLocales(root)).toThrow(/no supported languages/)
})

test('takes the platform plural rule when the catalogue declares none', () => {
	const ours = 'msgid ""\nmsgstr ""\n"Language: es-ES\\n"\n'
	const theirs = 'msgid ""\nmsgstr ""\n"Plural-Forms: nplurals=2; plural=(n != 1);\\n"\n'

	expect(withPluralRuleOf(ours, theirs)).toBe(theirs)
})

test('keeps one form when the committed rule names no count', () => {
	const ours = 'msgid ""\nmsgstr ""\n"Plural-Forms: plural=(n != 1);\\n"\n'
	const theirs = `msgid ""
msgstr ""

msgid "%d post"
msgid_plural "%d posts"
msgstr[0] "%d entrada"
msgstr[1] "%d entradas"
`

	expect(withPluralRuleOf(ours, theirs)).not.toMatch(/msgstr\[1\]/)
})
