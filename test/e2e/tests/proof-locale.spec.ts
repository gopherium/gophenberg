// SPDX-License-Identifier: Apache-2.0

import { expect, test } from '@playwright/test'

import { proofLocale } from '../env'

test.describe('the admin read in the proof locale', () => {
	test('names the main menu in the reader language', async ({ page }) => {
		await page.goto('/admin/')

		const nav = page.getByRole('navigation', { name: 'Navegación' })
		await expect(nav.getByRole('link', { name: 'Medios' })).toBeVisible()
		await expect(nav.getByRole('link', { name: 'Temas' })).toBeVisible()
		await expect(nav.getByRole('link', { name: 'Tipos de contenido' })).toBeVisible()
		await expect(nav.getByRole('link', { name: 'Idioma' })).toBeVisible()
		await expect(nav.getByRole('link', { name: 'Usuarios' })).toBeVisible()
	})

	test('greets the reader in the reader language', async ({ page }) => {
		await page.goto('/admin/')

		await expect(page.getByText('Te damos la bienvenida a Gophenberg.')).toBeVisible()
	})

	test('names the content list and its controls in the reader language', async ({ page }) => {
		await page.goto('/admin/content/post')

		await expect(page.getByRole('main').getByRole('button', { name: 'Añadir nuevo', exact: true })).toBeVisible()
		await expect(page.getByRole('link', { name: 'Atrás' })).toBeVisible()
		await expect(page.getByRole('button', { name: 'Ver opciones' })).toBeVisible()
		await expect(page.getByRole('searchbox', { name: 'Buscar posts…', exact: true })).toBeVisible()
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
		await expect(page.getByRole('button', { name: 'Añadir tipo nuevo' })).toBeVisible()
	})

	test('names the media screen in the reader language', async ({ page }) => {
		await page.goto('/admin/media')

		await expect(page.getByRole('heading', { name: 'Medios' })).toBeVisible()
		await expect(page.getByText('Añadir medio')).toBeVisible()
	})

	test('names the themes screen in the reader language', async ({ page }) => {
		await page.goto('/admin/themes')

		await expect(page.getByRole('heading', { name: 'Temas' })).toBeVisible()
		await expect(page.getByText('Archivo del tema')).toBeVisible()
	})

	test('names the users screen in the reader language', async ({ page }) => {
		await page.goto('/admin/users')

		await expect(page.getByRole('heading', { name: 'Usuarios' })).toBeVisible()
		await expect(page.getByRole('columnheader', { name: 'Estado' })).toBeVisible()
	})

	test('names the language screen and reports the chosen language', async ({ page }) => {
		await page.goto('/admin/language')

		await expect(page.getByRole('heading', { name: 'Idioma' })).toBeVisible()
		await expect(page.getByText('Elige el idioma en el que se lee el panel de administración.')).toBeVisible()
	})

	test('names the editor chrome in the reader language', async ({ page }) => {
		await page.goto('/admin/content/post')
		await page.getByRole('main').getByRole('button', { name: 'Añadir nuevo', exact: true }).click()

		await expect(page.getByRole('button', { name: 'Guardar el borrador' })).toBeVisible()
		await expect(page.getByRole('button', { name: 'Publicar' })).toBeVisible()
	})

	test('leaves the English account reading English', async ({ browser }) => {
		const english = await browser.newContext({ storageState: proofLocale.englishState })
		const page = await english.newPage()

		await page.goto('/admin/')

		await expect(page.getByText('Welcome to Gophenberg.')).toBeVisible()
		await english.close()
	})
})
