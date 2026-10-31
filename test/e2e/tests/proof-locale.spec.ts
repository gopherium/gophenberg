// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'

import { proofLocale } from '../env'

test.describe('the admin read in the proof locale', () => {
	test('names the main menu in the reader language', async ({ page }) => {
		await page.goto('/admin/')

		const nav = page.getByRole('region', { name: 'Navegación' })
		await expect(nav.getByRole('link', { name: 'Entradas' })).toBeVisible()
		await expect(nav.getByRole('link', { name: 'Medios' })).toBeVisible()
		await expect(nav.getByRole('link', { name: 'Temas' })).toBeVisible()
		await expect(nav.getByRole('link', { name: 'Tipos de contenido' })).toBeVisible()
		await expect(nav.getByRole('link', { name: 'Idioma' })).toBeVisible()
		await expect(nav.getByRole('link', { name: 'Usuarios' })).toBeVisible()
	})

	test('opens the default list from the root in the reader language', async ({ page }) => {
		await page.goto('/admin/')

		await expect(page.getByRole('main').getByRole('button', { name: 'Añadir entrada', exact: true })).toBeVisible()
		await expect(page).toHaveURL(/\/admin\/content\/post$/)
	})

	test('names the content list and its controls in the reader language', async ({ page }) => {
		await page.goto('/admin/content/post')

		await expect(page.getByRole('heading', { level: 2, name: 'Entradas' })).toBeVisible()
		await expect(page.getByText('Gestiona las entradas de este sitio.')).toBeVisible()
		await expect(page.getByRole('main').getByRole('button', { name: 'Añadir entrada', exact: true })).toBeVisible()
		await expect(page.getByRole('button', { name: 'Ver opciones' })).toBeVisible()
		await expect(page.getByRole('searchbox', { name: 'Buscar entradas…', exact: true })).toBeVisible()
		await expect(page.getByRole('button', { name: 'Añadir filtro' })).toBeVisible()
	})

	test('names the month of the date chip calendar in the reader language, its weeks starting on Monday', async ({
		page,
	}) => {
		const filters = JSON.stringify([{ field: 'date', operator: 'after', value: '2026-07-20T10:00:00.000Z' }])
		await page.goto(`/admin/content/post?filters=${encodeURIComponent(filters)}`)

		await page.getByRole('button', { name: 'Filtrar', exact: true }).click()
		await page.getByRole('button', { name: /^Fecha es después de/ }).click()

		const calendar = page.getByRole('grid')
		await expect(calendar).toHaveAccessibleName('julio de 2026')
		await expect(calendar.locator('th').first()).toHaveAttribute('aria-label', 'lunes')
	})

	test('writes the list dates abbreviated in the reader language, as WordPress es_ES does', async ({ page }) => {
		await page.goto(`/admin/content/post?search=${encodeURIComponent('Welcome to Gophenberg')}`)

		const welcome = page.getByRole('row').filter({ has: page.getByRole('link', { name: 'Welcome to Gophenberg' }) })
		await expect(welcome.getByText(/^Publicado: \d{1,2} (Ene|Feb|Mar|Abr|May|Jun|Jul|Ago|Sep|Oct|Nov|Dic) \d{4} \d{2}:\d{2}$/))
			.toBeVisible()
	})

	test('names the content types screen in the reader language', async ({ page }) => {
		await page.goto('/admin/content-types')

		await expect(page.getByRole('heading', { name: 'Tipos de contenido' })).toBeVisible()
		await expect(page.getByRole('button', { name: 'Añadir tipo de contenido' })).toBeVisible()
		await expect(page.getByRole('region', { name: 'Tipos de contenido' }).getByText('Entradas', { exact: true }))
			.toBeVisible()
	})

	test('names the media screen in the reader language', async ({ page }) => {
		await page.goto('/admin/media')

		await expect(page.getByRole('heading', { name: 'Medios' })).toBeVisible()
		await expect(page.getByRole('main').getByRole('button', { name: 'Añadir medios' })).toBeVisible()
	})

	test('names the themes screen in the reader language', async ({ page }) => {
		await page.goto('/admin/themes')

		await expect(page.getByRole('heading', { name: 'Temas' })).toBeVisible()
		await expect(page.getByRole('main').getByRole('button', { name: 'Subir tema' })).toBeVisible()
	})

	test('names the users screen and its list in the reader language', async ({ page }) => {
		await page.goto('/admin/users')

		await expect(page.getByRole('heading', { name: 'Usuarios' })).toBeVisible()
		await expect(page.getByText('Gestiona quién puede entrar y qué puede hacer.')).toBeVisible()
		await expect(page.getByRole('main').getByRole('link', { name: 'Añadir usuario' })).toBeVisible()
		await expect(page.getByRole('searchbox', { name: 'Buscar usuarios…', exact: true })).toBeVisible()
		for (const column of ['Nombre', 'Estado', 'Rol', 'Entradas', 'Creado']) {
			await expect(page.getByRole('columnheader', { name: column, exact: true })).toBeVisible()
		}
	})

	test('names Change role of the users screen in the reader language', async ({ page }) => {
		await page.goto(`/admin/users?search=${encodeURIComponent('member@example.com')}`)
		const row = page.getByRole('row').filter({ hasText: 'member@example.com' })

		await row.getByRole('button', { name: 'Acciones' }).click()
		await page.getByRole('menuitem', { name: 'Cambiar el rol…', exact: true }).click()

		const dialog = page.getByRole('dialog', { name: 'Cambiar el rol' })
		await expect(dialog.getByText('Elige el rol de Member.')).toBeVisible()
		await expect(dialog.getByRole('combobox', { name: 'Rol' })).toHaveText('Miembro')
		await expect(dialog.getByRole('button', { name: 'Cambiar el rol', exact: true })).toBeVisible()
	})

	test('names Add User in the reader language', async ({ page }) => {
		await page.goto('/admin/users/new')

		await expect(page.getByRole('heading', { name: 'Añadir usuario' })).toBeVisible()
		await expect(page.getByText('Crea un nuevo usuario y añádelo a este sitio.')).toBeVisible()
		await expect(page.getByRole('textbox', { name: 'Correo electrónico (Obligatorio)' })).toBeVisible()
		await expect(page.getByRole('textbox', { name: 'Nombre (Obligatorio)' })).toBeVisible()
		await expect(page.getByRole('button', { name: 'Generar contraseña' })).toBeVisible()
		await expect(page.getByRole('button', { name: 'Añadir usuario', exact: true })).toBeVisible()
	})

	test('names the language screen and reports the chosen language', async ({ page }) => {
		await page.goto('/admin/language')

		await expect(page.getByRole('heading', { name: 'Idioma' })).toBeVisible()
		await expect(page.getByText('Elige el idioma en el que se lee el panel de administración.')).toBeVisible()
	})

	test('names the editor chrome in the reader language', async ({ page }) => {
		await page.goto('/admin/content/post')
		await page.getByRole('main').getByRole('button', { name: 'Añadir entrada', exact: true }).click()

		await expect(page.getByRole('button', { name: 'Guardar el borrador' })).toBeVisible()
		await expect(page.getByRole('button', { name: 'Publicar' })).toBeVisible()
		await expect(page.getByTestId('document-bar')).toHaveText(/·\s*Entrada$/)
	})

	test('leaves the English account reading English', async ({ browser }) => {
		const english = await browser.newContext({ storageState: proofLocale.englishState })
		const page = await english.newPage()

		await page.goto('/admin/')

		await expect(page.getByRole('main').getByRole('button', { name: 'Add Post', exact: true })).toBeVisible()
		await english.close()
	})
})
