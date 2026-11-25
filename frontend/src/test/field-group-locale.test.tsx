// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { expect, onTestFinished, test } from 'vitest'

import { catalogFor } from '../i18n/catalog'
import { DETAILS, EXTRAS, listing, READING_TIME, serveFieldGroups, SUBTITLE } from './groupFieldsFixtures'
import { renderAt } from './render'
import { findToast } from './typeFixtures'

const BYLINE = { ...SUBTITLE, key: 'byline', label: 'Byline' }

const AUTHOR = { ...SUBTITLE, key: 'author', label: 'Author', kind: 'section', fields: [BYLINE] }

const STYLE = { ...SUBTITLE, key: 'style', label: 'Style', kind: 'choice' }

/** The rule showing a field while the reading time is above five. */
const WHILE_ABOVE_FIVE = { source: READING_TIME.key, operator: '>', value: '5' }

/** The subtitle shown by a stored rule on the reading time. */
const RULED_SUBTITLE = { ...SUBTITLE, settings: { conditions: [[WHILE_ABOVE_FIVE]] } }

serveFieldGroups()

/** The words of the fields table, an open field, its rules and choices, and the header Save in each shipped locale. */
const READERS = [
	{
		locale: 'es-ES',
		fields: 'Campos',
		columns: ['Etiqueta', 'Tipo', 'Nombre de API', 'Acciones'],
		empty: 'Todavía no hay campos',
		add: 'Añade un campo para guardar más datos de los elementos en los que aparece este grupo.',
		label: 'Etiqueta',
		tabs: ['General', 'Validación', 'Presentación', 'Lógica condicional'],
		ruled: /^Lógica condicional/,
		active: 'Activa',
		rule: 'Regla 1 del conjunto 1',
		trash: 'Quitar la regla 1 del conjunto 1',
		close: 'Cerrar campo',
		discard: 'Descartar los cambios',
		listed: 'Mostrar en la lista',
		inside: 'Subcampos de Author',
		remove: 'Borrar Author',
		lostToo: 'También se pierden los cambios sin guardar dentro de Author.',
		choice: 'Opción 1',
		choiceButtons: ['Subir la opción', 'Bajar la opción', 'Quitar la opción'],
		noChoices: 'Todavía no hay opciones.',
		save: 'Guardar',
		ran: [
			'Cambios guardados: 1 de 2.',
			'Error al guardar Subtitle.',
			'Alguien guardó el campo Subtitle después de que lo abrieras.',
		],
	},
	{
		locale: 'fr-FR',
		fields: 'Champs',
		columns: ['Libellé', 'Genre', "Nom d'API", 'Actions'],
		empty: "Aucun champ pour l'instant",
		add: "Ajoutez un champ pour enregistrer plus d'informations sur les éléments où ce groupe apparaît.",
		label: 'Libellé',
		tabs: ['Général', 'Validation', 'Présentation', 'Logique conditionnelle'],
		ruled: /^Logique conditionnelle/,
		active: 'Active',
		rule: "Règle 1 de l'ensemble 1",
		trash: "Retirer la règle 1 de l'ensemble 1",
		close: 'Fermer le champ',
		discard: 'Abandonner les modifications',
		listed: 'Afficher dans la liste',
		inside: 'Sous-champs de Author',
		remove: 'Supprimer Author',
		lostToo: 'Les modifications non enregistrées dans Author sont aussi perdues.',
		choice: 'Choix 1',
		choiceButtons: ['Monter le choix', 'Descendre le choix', 'Retirer le choix'],
		noChoices: "Aucun choix pour l'instant.",
		save: 'Enregistrer',
		ran: [
			'Modifications enregistrées : 1 sur 2.',
			"Impossible d'enregistrer Subtitle.",
			"Quelqu'un a enregistré le champ Subtitle après que vous l'avez ouvert.",
		],
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

/**
 * Opens a field from the label in its row and returns the row that opened under it.
 * @param fields - The region the fields table scrolls in.
 * @param label - The label the field shows.
 * @returns The row holding the open part.
 */
async function openIn(fields: HTMLElement, label: string) {
	const toggle = within(fields).getByRole('button', { name: label })
	await userEvent.click(toggle)
	const part = toggle.closest('tr')?.nextElementSibling as HTMLElement
	await waitFor(() => expect(part).not.toHaveAttribute('hidden'))
	return part
}

test.each([
	{ locale: 'es-ES', list: 'Grupos de campos', counts: ['1 campo', '0 campos', '2 campos'] },
	{ locale: 'fr-FR', list: 'Groupes de champs', counts: ['1 champ', '0 champ', '2 champs'] },
])('writes how many fields each group holds in the Field Groups list in $locale', async (said) => {
	const SOLE = { ...DETAILS, id: 5, key: 'sole', title: 'Sole', position: 3, fields: [SUBTITLE] }
	listing([SOLE, EXTRAS, DETAILS])
	server.use(
		http.get('/api/definitions/drift', () => HttpResponse.json({ orphans: [], collisions: [], nesting_kept: [] })),
	)
	setLocaleData(await catalogFor(said.locale), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	renderAt('/field-groups')

	const table = await screen.findByRole('region', { name: said.list })

	const written = [SOLE, EXTRAS, DETAILS].map(
		(group) =>
			(within(table).getByRole('row', { name: new RegExp(group.title) }) as HTMLTableRowElement).cells[2].textContent,
	)
	expect(written).toEqual(said.counts)
})

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

test.each(READERS)('names the tabs of an open field, Close field and Discard changes in $locale', async (said) => {
	const fields = await fieldsIn(said.locale, DETAILS, said.fields)
	const part = await openIn(fields, SUBTITLE.label)

	expect(within(part).getAllByRole('tab').map((tab) => tab.textContent)).toEqual(said.tabs)
	expect(within(part).getByRole('button', { name: said.close })).toBeInTheDocument()
	await userEvent.type(within(part).getByRole('textbox', { name: said.label }), ' line')
	expect(within(part).getByRole('button', { name: said.discard })).toBeInTheDocument()
})

test.each(READERS)('names the box putting a field in the admin list on Presentation in $locale', async (said) => {
	const fields = await fieldsIn(said.locale, DETAILS, said.fields)
	const part = await openIn(fields, SUBTITLE.label)

	await userEvent.click(within(part).getByRole('tab', { name: said.tabs[2] }))

	expect(within(part).getByRole('checkbox', { name: said.listed })).toBeInTheDocument()
})

test.each(READERS)('names the Conditional logic tab Active while the field holds a rule in $locale', async (said) => {
	listing([{ ...DETAILS, fields: [RULED_SUBTITLE, READING_TIME] }, EXTRAS])
	const fields = await fieldsIn(said.locale, DETAILS, said.fields)
	const part = await openIn(fields, SUBTITLE.label)

	const tab = within(part).getByRole('tab', { name: said.ruled })

	expect(tab).toHaveTextContent(said.tabs[3] + said.active)
	expect(within(tab).getByText(said.active)).toBeInTheDocument()
})

test.each(READERS)('names a rule row and its trash by their place on Conditional logic in $locale', async (said) => {
	listing([{ ...DETAILS, fields: [RULED_SUBTITLE, READING_TIME] }, EXTRAS])
	const fields = await fieldsIn(said.locale, DETAILS, said.fields)
	const part = await openIn(fields, SUBTITLE.label)

	await userEvent.click(within(part).getByRole('tab', { name: said.ruled }))

	const rule = within(part).getByRole('group', { name: said.rule })
	expect(within(rule).getByRole('button', { name: said.trash })).toBeInTheDocument()
})

test.each(READERS)('names the sub fields of a container and the edits its delete loses in $locale', async (said) => {
	listing([{ ...DETAILS, fields: [SUBTITLE, AUTHOR] }, EXTRAS])
	const fields = await fieldsIn(said.locale, DETAILS, said.fields)
	const container = await openIn(fields, AUTHOR.label)
	const inside = within(container).getByRole('region', { name: said.inside })
	const part = await openIn(inside, BYLINE.label)
	await userEvent.type(within(part).getByRole('textbox', { name: said.label }), ' line')

	await userEvent.click(within(fields).getByRole('button', { name: said.remove }))

	expect(await screen.findByRole('alertdialog')).toHaveTextContent(said.lostToo)
})

test.each(READERS)('names a choice row by its place and its move and remove buttons in $locale', async (said) => {
	listing([{ ...DETAILS, fields: [{ ...STYLE, settings: { choices: [{ value: 'ipa', label: 'IPA' }] } }] }, EXTRAS])
	const fields = await fieldsIn(said.locale, DETAILS, said.fields)
	const part = await openIn(fields, STYLE.label)

	const row = within(part).getByRole('group', { name: said.choice })
	said.choiceButtons.forEach((name) => expect(within(row).getByRole('button', { name })).toBeInTheDocument())
})

test.each(READERS)('says a choice field holds no choices yet in $locale', async (said) => {
	listing([{ ...DETAILS, fields: [STYLE] }, EXTRAS])
	const fields = await fieldsIn(said.locale, DETAILS, said.fields)
	const part = await openIn(fields, STYLE.label)

	expect(within(part).getByText(said.noChoices)).toBeInTheDocument()
})

test.each(READERS)('counts what the header Save stored and names the field it could not in $locale', async (said) => {
	server.use(
		http.patch('/api/groups/3/fields/subtitle', () =>
			HttpResponse.json({ error: 'content: conflicting update', code: 'content_stale_update' }, { status: 409 }),
		),
		http.patch('/api/groups/3/fields/reading-time', () => HttpResponse.json(READING_TIME)),
	)
	const fields = await fieldsIn(said.locale, DETAILS, said.fields)
	const stale = await openIn(fields, SUBTITLE.label)
	const saving = await openIn(fields, READING_TIME.label)
	await userEvent.type(within(stale).getByRole('textbox', { name: said.label }), ' line')
	await userEvent.type(within(saving).getByRole('textbox', { name: said.label }), ' in minutes')

	await userEvent.click(within(screen.getByRole('main')).getByRole('button', { name: said.save }))

	expect(await findToast(said.ran.join(' '))).toBeInTheDocument()
})
