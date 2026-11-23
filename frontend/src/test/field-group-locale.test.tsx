// SPDX-License-Identifier: Apache-2.0

import { screen, within } from '@testing-library/react'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { expect, onTestFinished, test } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { DETAILS, EXTRAS, serveFieldGroups } from './groupFieldsFixtures'
import { renderAt } from './render'

serveFieldGroups()

/** The words of the fields table on a group's page in each shipped locale. */
const READERS = [
	{
		locale: 'es-ES',
		fields: 'Campos',
		columns: ['Etiqueta', 'Tipo', 'Nombre de API', 'Acciones'],
		empty: 'Todavía no hay campos',
		add: 'Añade un campo para guardar más datos de los elementos en los que aparece este grupo.',
	},
	{
		locale: 'fr-FR',
		fields: 'Champs',
		columns: ['Libellé', 'Genre', "Nom d'API", 'Actions'],
		empty: "Aucun champ pour l'instant",
		add: "Ajoutez un champ pour enregistrer plus d'informations sur les éléments où ce groupe apparaît.",
	},
]

/**
 * Renders a group's page in the locale and returns the region its fields table scrolls in.
 * @param locale - The language to read in.
 * @param group - The group whose page to render.
 * @param fields - The name the region carries in the locale.
 * @returns The region.
 */
async function fieldsIn(locale: string, group: { id: number; title: string }, fields: string) {
	setLocaleData(await catalogFor(locale), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt(`/field-groups/${group.id}`)
	const main = await screen.findByRole('main')
	await within(main).findByRole('heading', { level: 2, name: group.title })
	return within(main).getByRole('region', { name: fields })
}

test.each(READERS)('heads the fields table with Label, Kind, API name and Actions in $locale', async (said) => {
	const fields = await fieldsIn(said.locale, DETAILS, said.fields)

	const headers = within(fields).getAllByRole('columnheader')
	expect(headers.map((header) => header.textContent)).toEqual(said.columns)
})

test.each(READERS)('says a group holds no fields yet and what a field adds in $locale', async (said) => {
	const fields = await fieldsIn(said.locale, EXTRAS, said.fields)

	expect(within(fields).getByRole('heading', { name: said.empty })).toBeInTheDocument()
	expect(within(fields).getByText(said.add)).toBeInTheDocument()
})
