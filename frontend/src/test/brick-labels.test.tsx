// SPDX-License-Identifier: Apache-2.0

import { setViewport } from '@gopherium/godmin/testing'
import { useToaster } from '@gopherium/godmin'
import { setLocaleData } from '@wordpress/i18n'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { AdminToaster } from '../toasts'
import { renderAt } from './render'

setLocaleData(
	{
		Dismiss: ['Descartar'],
		Back: ['Atras'],
		'Open navigation': ['Abrir navegacion'],
	},
	'gophenberg',
)

/**
 * Renders a control raising one toast inside the admin toast region.
 * @returns The rendered region.
 */
function renderToast() {
	/**
	 * Renders the control raising the toast.
	 * @returns The control element.
	 */
	function Raise() {
		const toaster = useToaster()
		return <button onClick={() => toaster.show('Saved.')}>raise</button>
	}
	return render(
		<AdminToaster>
			<Raise />
		</AdminToaster>,
	)
}

test('names the menu button in the language the reader loaded', async () => {
	setViewport({ matches: true })

	renderAt('/')

	expect(
		await screen.findByRole('button', { name: 'Abrir navegacion' }),
	).toBeInTheDocument()
})

test('names the back link in the language the reader loaded', async () => {
	setViewport({ matches: false })

	renderAt('/content/post')

	expect(await screen.findByRole('link', { name: 'Atras' })).toBeInTheDocument()
})

test('names the dismiss control in the language the reader loaded', async () => {
	renderToast()

	await userEvent.click(screen.getByRole('button', { name: 'raise' }))

	expect(
		await screen.findByRole('button', { name: 'Descartar' }),
	).toBeInTheDocument()
})
