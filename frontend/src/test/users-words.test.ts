// SPDX-License-Identifier: Apache-2.0

import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { expect, test } from 'vitest'

import { unreviewed } from '@gopherium/gottext/build'

import { repositoryRoot } from '../../scripts/config.ts'

/**
 * Returns the compiled catalogue the admin ships for a language.
 * @param locale - The language, such as es-ES.
 * @returns The answers, keyed by the context and the message.
 */
function shipped(locale: string): Record<string, string[]> {
	return JSON.parse(readFileSync(join(repositoryRoot(), 'frontend', 'src', 'languages', `${locale}.json`), 'utf8'))
}

/**
 * Returns the messages of a catalogue still waiting for review.
 * @param locale - The language, such as es-ES.
 * @returns The keys of the answers that carry the fuzzy flag.
 */
function waiting(locale: string): string[] {
	return unreviewed(readFileSync(join(repositoryRoot(), 'languages', `${locale}.po`), 'utf8'))
}

/** The words WordPress's own Spanish and French catalogues give the strings the Users screens show. */
const WORDPRESS = [
	{ key: 'Add User', es: ['Añadir usuario'], fr: ['Ajouter un compte'] },
	{
		key: 'Create a brand new user and add them to this site.',
		es: ['Crea un nuevo usuario y añádelo a este sitio.'],
		fr: ['Créer un nouveau compte et l’ajouter à ce site.'],
	},
	{ key: 'New user created.', es: ['Nuevo usuario creado.'], fr: ['Le nouveau compte a été créé.'] },
	{
		key: '%s post by this author',
		es: ['%s entrada de este autor', '%s de entradas de este autor', '%s entradas de este autor'],
		fr: [
			'%s publication par cet auteur/autrice',
			'%s de publications par cet auteur/autrice',
			'%s publications par cet auteur/autrice',
		],
	},
]

test.each(WORDPRESS)('ships $key in the Spanish and the French WordPress gives it', ({ key, es, fr }) => {
	expect([shipped('es-ES')[key], shipped('fr-FR')[key]]).toEqual([es, fr])
})

test.each(WORDPRESS)('ships $key reviewed in both languages', ({ key }) => {
	expect(['es-ES', 'fr-FR'].filter((locale) => waiting(locale).includes(key))).toEqual([])
})
