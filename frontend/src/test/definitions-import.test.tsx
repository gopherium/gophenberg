// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { resetLocaleData, setLocaleData } from '@wordpress/i18n'
import { expect, onTestFinished, test } from 'vitest'

import { chosenDefinitions } from '../content/ImportDefinitions'
import { catalogFor } from '../i18n/catalog'
import { gate } from './gate'
import { renderAt } from './render'

const PLAN = {
	changes: [
		{ action: 'create', subject: 'group', key: 'event-details', label: 'Event details' },
		{ action: 'create', subject: 'field', key: 'venue', group: 'event-details', label: 'Venue' },
		{ action: 'update', subject: 'type', key: 'recipe', label: 'Recipe' },
		{
			action: 'delete',
			subject: 'field',
			key: 'cook-time',
			group: 'recipe-details',
			label: 'Cook time',
			reason: 'kind_changed',
		},
		{ action: 'delete', subject: 'field', key: 'photos', label: 'Photos', reason: 'shape_changed' },
		{ action: 'delete', subject: 'field', key: 'serves', label: 'Serves', reason: 'moved' },
		{
			action: 'create',
			subject: 'field',
			key: 'prep-time',
			group: 'recipe-facts',
			label: 'Prep time',
			reason: 'carried',
			from: 'recipe-details',
		},
		{
			action: 'delete',
			subject: 'field',
			key: 'prep-time',
			group: 'recipe-details',
			label: 'Prep time',
			reason: 'carried',
		},
		{ action: 'delete', subject: 'type', key: 'note', label: 'Note', reason: 'removed' },
	],
	warnings: [
		{ code: 'route_word_changed', key: 'recipe' },
		{ code: 'root_moved', key: 'recipe' },
		{ code: 'nesting_kept', key: 'recipe' },
	],
}

/** Serves the given answer from the plan endpoint. */
function planning(answer: Parameters<typeof http.post>[1]) {
	server.use(http.post('/api/definitions/plan', answer))
}

/**
 * Opens the import dialog and hands it a definitions file carrying the text.
 * @param text - The definitions file content to upload.
 * @returns The rendered query client.
 */
async function importing(text: string) {
	const client = renderAt('/field-groups')
	await userEvent.click(await screen.findByRole('button', { name: 'Import definitions' }))
	await userEvent.upload(
		screen.getByLabelText('Definitions file'),
		new File([text], 'definitions.json', { type: 'application/json' }),
	)
	return client
}

test.each(['success', 'error'] as const)(
	'keeps and applies the latest file when an earlier plan answers with %s last',
	async (answer) => {
		const earlier = gate()
		const requested = gate()
		onTestFinished(earlier.release)
		const latest = '{"format":"1.0.0","types":[],"groups":[]}'
		let sent = ''
		planning(async ({ request }) => {
			if ((await request.text()) !== latest) {
				requested.release()
				await earlier.held
				return answer === 'success'
					? HttpResponse.json(PLAN)
					: new HttpResponse('gateway said no', { status: 502 })
			}
			return HttpResponse.json({
				changes: [{ action: 'create', subject: 'type', key: 'latest', label: 'Latest type' }],
				warnings: [],
			})
		})
		server.use(
			http.post('/api/definitions/apply', async ({ request }) => {
				sent = await request.text()
				return HttpResponse.json({ applied: [], skipped: [] })
			}),
		)

		const client = await importing('{"format":"1.0.0"}')
		await requested.held
		await userEvent.upload(
			screen.getByLabelText('Definitions file'),
			new File([latest], 'latest.json', { type: 'application/json' }),
		)
		await screen.findByText('Add Latest type')
		earlier.release()
		await waitFor(() => expect(client.isMutating()).toBe(0))

		expect(screen.getByText('Add Latest type')).toBeInTheDocument()
		expect(screen.queryByText('Add Event details')).not.toBeInTheDocument()
		expect(screen.queryByRole('alert')).not.toBeInTheDocument()
		await userEvent.click(screen.getByRole('button', { name: 'Apply' }))
		await screen.findByText('Done. The site now holds what the file describes.')
		expect(JSON.parse(sent)).toEqual({ ...JSON.parse(latest), confirm: [] })
	},
)

test('keeps the latest planning error when an earlier plan succeeds last', async () => {
	const earlier = gate()
	const requested = gate()
	onTestFinished(earlier.release)
	const latest = '{"format":"1.0.0","groups":[]}'
	planning(async ({ request }) => {
		if ((await request.text()) !== latest) {
			requested.release()
			await earlier.held
			return HttpResponse.json(PLAN)
		}
		return HttpResponse.json(
			{ error: 'too big', code: 'definitions_too_large', meta: { max: 262144 } },
			{ status: 413 },
		)
	})

	const client = await importing('{"format":"1.0.0"}')
	await requested.held
	await userEvent.upload(
		screen.getByLabelText('Definitions file'),
		new File([latest], 'latest.json', { type: 'application/json' }),
	)
	expect(await screen.findByRole('alert')).toHaveTextContent(/262\.144 bytes/)
	earlier.release()
	await waitFor(() => expect(client.isMutating()).toBe(0))

	expect(screen.getByRole('alert')).toHaveTextContent(/262\.144 bytes/)
	expect(screen.queryByText('Add Event details')).not.toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Apply' })).not.toBeInTheDocument()
})

test('sends a loss ticked on the latest plan only with the latest file when an earlier plan answers last', async () => {
	const earlier = gate()
	const requested = gate()
	onTestFinished(earlier.release)
	const latest = '{"format":"1.0.0","groups":[]}'
	let sent = ''
	planning(async ({ request }) => {
		if ((await request.text()) !== latest) {
			requested.release()
			await earlier.held
			return HttpResponse.json(PLAN)
		}
		return HttpResponse.json({ changes: [PLAN.changes[3]], warnings: [] })
	})
	server.use(
		http.post('/api/definitions/apply', async ({ request }) => {
			sent = await request.text()
			return HttpResponse.json({ applied: [], skipped: [] })
		}),
	)

	const client = await importing('{"format":"1.0.0"}')
	await requested.held
	await userEvent.upload(
		screen.getByLabelText('Definitions file'),
		new File([latest], 'latest.json', { type: 'application/json' }),
	)
	await userEvent.click(await screen.findByRole('checkbox', { name: /Remove Cook time/ }))
	earlier.release()
	await waitFor(() => expect(client.isMutating()).toBe(0))
	await userEvent.click(screen.getByRole('button', { name: 'Apply' }))
	await screen.findByText('Done. The site now holds what the file describes.')

	expect(JSON.parse(sent)).toEqual({
		...JSON.parse(latest),
		confirm: [{ subject: 'field', key: 'cook-time', group: 'recipe-details' }],
	})
})

test.each([
	{ answer: 'success', said: 'Done. The site now holds what the file describes.' },
	{ answer: 'failure', said: 'The site may already hold part of this file. Import it again to see what is left.' },
] as const)(
	'reads no file picked while an import applies and says how that import ended in $answer',
	async ({ answer, said }) => {
		const applying = gate()
		const requested = gate()
		onTestFinished(applying.release)
		const read: string[] = []
		planning(async ({ request }) => {
			read.push(await request.text())
			return HttpResponse.json(PLAN)
		})
		server.use(
			http.post('/api/definitions/apply', async () => {
				requested.release()
				await applying.held
				return answer === 'success'
					? HttpResponse.json({ applied: [], skipped: [] })
					: HttpResponse.json({ error: 'in use', code: 'type_in_use' }, { status: 422 })
			}),
		)

		const client = await importing('{"format":"1.0.0"}')
		await userEvent.click(await screen.findByRole('button', { name: 'Apply' }))
		await requested.held
		await userEvent.upload(
			screen.getByLabelText('Definitions file'),
			new File(['{"format":"1.0.0","groups":[]}'], 'other.json', { type: 'application/json' }),
		)
		applying.release()
		await waitFor(() => expect(client.isMutating()).toBe(0))

		expect(await screen.findByText(said)).toBeInTheDocument()
		expect(read).toEqual(['{"format":"1.0.0"}'])
		expect(screen.getByLabelText('Definitions file')).toBeEnabled()
	},
)

test('keeps the file field locked when the dialog is opened again while an import applies', async () => {
	const applying = gate()
	const requested = gate()
	onTestFinished(applying.release)
	planning(() => HttpResponse.json(PLAN))
	server.use(
		http.post('/api/definitions/apply', async () => {
			requested.release()
			await applying.held
			return HttpResponse.json({ applied: [], skipped: [] })
		}),
	)

	await importing('{"format":"1.0.0"}')
	await userEvent.click(await screen.findByRole('button', { name: 'Apply' }))
	await requested.held
	const [, footerClose] = screen.getAllByRole('button', { name: 'Close' })
	await userEvent.click(footerClose as HTMLElement)
	await userEvent.click(await screen.findByRole('button', { name: 'Import definitions' }))

	expect(await screen.findByLabelText('Definitions file')).toBeDisabled()
	applying.release()
	expect(await screen.findByText('Done. The site now holds what the file describes.')).toBeInTheDocument()
	await waitFor(() => expect(screen.getByLabelText('Definitions file')).toBeEnabled())
})

test('sends the file of the plan on screen when Apply is pressed the moment that plan is drawn', async () => {
	const answer = gate()
	onTestFinished(answer.release)
	const latest = '{"format":"1.0.0","types":[],"groups":[]}'
	let sent = ''
	planning(async ({ request }) => {
		if ((await request.text()) !== latest) {
			return HttpResponse.json(PLAN)
		}
		await answer.held
		return HttpResponse.json({
			changes: [{ action: 'create', subject: 'type', key: 'latest', label: 'Latest type' }],
			warnings: [],
		})
	})
	server.use(
		http.post('/api/definitions/apply', async ({ request }) => {
			sent = await request.text()
			return HttpResponse.json({ applied: [], skipped: [] })
		}),
	)
	const pressing = new MutationObserver(() => {
		if (screen.queryByText('Add Latest type') !== null) {
			pressing.disconnect()
			screen.getByRole('button', { name: 'Apply' }).click()
		}
	})
	onTestFinished(() => pressing.disconnect())

	await importing('{"format":"1.0.0"}')
	await screen.findByText('Add Event details')
	await userEvent.upload(
		screen.getByLabelText('Definitions file'),
		new File([latest], 'latest.json', { type: 'application/json' }),
	)
	pressing.observe(document.body, { childList: true, subtree: true })
	answer.release()
	await screen.findByText('Done. The site now holds what the file describes.')

	expect(JSON.parse(sent)).toEqual({ ...JSON.parse(latest), confirm: [] })
})

test.each([
	{
		way: 'Close',
		close: async () => {
			const [, footerClose] = screen.getAllByRole('button', { name: 'Close' })
			await userEvent.click(footerClose as HTMLElement)
		},
	},
	{ way: 'Escape', close: () => userEvent.keyboard('{Escape}') },
])('shows no earlier plan when the dialog is opened again after closing it with $way', async ({ close }) => {
	planning(() => HttpResponse.json(PLAN))

	await importing('{"format":"1.0.0"}')
	await userEvent.click(await screen.findByRole('checkbox', { name: /Remove Cook time/ }))
	await close()
	await userEvent.click(await screen.findByRole('button', { name: 'Import definitions' }))
	await screen.findByRole('dialog', { name: 'Import definitions' })

	expect(screen.queryByText('Add Event details')).not.toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Apply' })).not.toBeInTheDocument()
})

test('shows no plan that answers after the admin closed the dialog', async () => {
	const answer = gate()
	const requested = gate()
	onTestFinished(answer.release)
	planning(async () => {
		requested.release()
		await answer.held
		return HttpResponse.json(PLAN)
	})

	const client = await importing('{"format":"1.0.0"}')
	await requested.held
	const [, footerClose] = screen.getAllByRole('button', { name: 'Close' })
	await userEvent.click(footerClose as HTMLElement)
	answer.release()
	await waitFor(() => expect(client.isMutating()).toBe(0))
	await userEvent.click(await screen.findByRole('button', { name: 'Import definitions' }))
	await screen.findByRole('dialog', { name: 'Import definitions' })

	expect(screen.queryByText('Add Event details')).not.toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Apply' })).not.toBeInTheDocument()
})

test('shows no plan that answers after the dialog is opened again', async () => {
	const answer = gate()
	const requested = gate()
	onTestFinished(answer.release)
	planning(async () => {
		requested.release()
		await answer.held
		return HttpResponse.json(PLAN)
	})

	const client = await importing('{"format":"1.0.0"}')
	await requested.held
	const [, footerClose] = screen.getAllByRole('button', { name: 'Close' })
	await userEvent.click(footerClose as HTMLElement)
	await userEvent.click(await screen.findByRole('button', { name: 'Import definitions' }))
	await screen.findByRole('dialog', { name: 'Import definitions' })
	answer.release()
	await waitFor(() => expect(client.isMutating()).toBe(0))

	expect(screen.queryByText('Add Event details')).not.toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Apply' })).not.toBeInTheDocument()
})

test.each([
	{ answer: 'success', said: 'Done. The site now holds what the file describes.' },
	{ answer: 'failure', said: 'The site may already hold part of this file. Import it again to see what is left.' },
] as const)(
	'shows no earlier ending when the dialog is opened again after an import ended in $answer',
	async ({ answer, said }) => {
		planning(() => HttpResponse.json(PLAN))
		server.use(
			http.post('/api/definitions/apply', () =>
				answer === 'success'
					? HttpResponse.json({ applied: [], skipped: [] })
					: HttpResponse.json({ error: 'in use', code: 'type_in_use' }, { status: 422 }),
			),
		)

		await importing('{"format":"1.0.0"}')
		await userEvent.click(await screen.findByRole('button', { name: 'Apply' }))
		await screen.findByText(said)
		const [, footerClose] = screen.getAllByRole('button', { name: 'Close' })
		await userEvent.click(footerClose as HTMLElement)
		await userEvent.click(await screen.findByRole('button', { name: 'Import definitions' }))
		await screen.findByRole('dialog', { name: 'Import definitions' })

		expect(screen.queryByText(said)).not.toBeInTheDocument()
		expect(screen.queryByRole('alert')).not.toBeInTheDocument()
	},
)

test.each([
	{ answer: 'success', said: 'Done. The site now holds what the file describes.' },
	{ answer: 'failure', said: 'The site may already hold part of this file. Import it again to see what is left.' },
] as const)(
	'shows how an import ended in $answer when it ended while the dialog was closed',
	async ({ answer, said }) => {
		const applying = gate()
		const requested = gate()
		onTestFinished(applying.release)
		planning(() => HttpResponse.json(PLAN))
		server.use(
			http.post('/api/definitions/apply', async () => {
				requested.release()
				await applying.held
				return answer === 'success'
					? HttpResponse.json({ applied: [], skipped: [] })
					: HttpResponse.json({ error: 'in use', code: 'type_in_use' }, { status: 422 })
			}),
		)

		const client = await importing('{"format":"1.0.0"}')
		await userEvent.click(await screen.findByRole('button', { name: 'Apply' }))
		await requested.held
		const [, footerClose] = screen.getAllByRole('button', { name: 'Close' })
		await userEvent.click(footerClose as HTMLElement)
		applying.release()
		await waitFor(() => expect(client.isMutating()).toBe(0))
		await userEvent.click(await screen.findByRole('button', { name: 'Import definitions' }))

		expect(await screen.findByText(said)).toBeInTheDocument()
	},
)

test('shows what a chosen definitions file would change', async () => {
	planning(() => HttpResponse.json(PLAN))

	await importing('{"format":"1.0.0"}')

	expect(await screen.findByText('Add Event details')).toBeInTheDocument()
	expect(screen.getByText('Add Venue')).toBeInTheDocument()
	expect(screen.getByText('Change Recipe')).toBeInTheDocument()
	expect(screen.getByText('Remove Cook time')).toBeInTheDocument()
	expect(screen.getByText('Its kind changed, so its stored values go with it.')).toBeInTheDocument()
	expect(screen.getByText('What it points at changed, so its stored values go with it.')).toBeInTheDocument()
	expect(
		screen.getByText('It moves to another group, so its stored values do not follow.'),
	).toBeInTheDocument()
	expect(screen.getByText('It moves to another group and keeps its stored values.')).toBeInTheDocument()
	expect(screen.getByText('It moves here from another group and keeps its stored values.')).toBeInTheDocument()
	expect(screen.getByText('recipe answers on a new address, and stored links move with it.')).toBeInTheDocument()
	expect(
		screen.getByText('recipe keeps nesting, because some of its items sit inside another.'),
	).toBeInTheDocument()
	expect(
		screen.getByText('recipe becomes the type the site opens on, and the old one takes an address.'),
	).toBeInTheDocument()
})

test('names a built-in type in the plan in the words of WordPress es_ES for a Spanish reader', async () => {
	setLocaleData(await catalogFor('es-ES'), 'gophenberg')
	onTestFinished(() => resetLocaleData({}, 'gophenberg'))
	planning(() =>
		HttpResponse.json({
			changes: [
				{ action: 'update', subject: 'type', key: 'post', label: 'Post' },
				{ action: 'create', subject: 'field', key: 'post', group: 'links', label: 'Post' },
				{ action: 'delete', subject: 'type', key: 'page', label: 'Page', reason: 'removed' },
			],
			warnings: [],
		}),
	)
	renderAt('/field-groups')
	await userEvent.click(await screen.findByRole('button', { name: 'Importar definiciones' }))

	await userEvent.upload(
		screen.getByLabelText('Archivo de definiciones'),
		new File(['{"format":"1.0.0"}'], 'definitions.json', { type: 'application/json' }),
	)

	expect(await screen.findByText('Cambiar Entrada')).toBeInTheDocument()
	expect(screen.getByText('Añadir Post')).toBeInTheDocument()
	expect(screen.getByRole('checkbox', { name: 'Quitar Página' })).toBeInTheDocument()
})

test('holds no definitions file when the field holds none at all', () => {
	expect(chosenDefinitions(null)).toBeNull()
})

test('leaves the plan alone when the admin picks no file', async () => {
	planning(() => HttpResponse.json(PLAN))
	renderAt('/field-groups')
	await userEvent.click(await screen.findByRole('button', { name: 'Import definitions' }))

	fireEvent.change(screen.getByLabelText('Definitions file'), { target: { files: [] } })

	expect(screen.queryByText('Add Event details')).not.toBeInTheDocument()
})

test('applies a plan, carrying only the losses the admin confirmed', async () => {
	let sent = ''
	planning(() => HttpResponse.json(PLAN))
	server.use(
		http.post('/api/definitions/apply', async ({ request }) => {
			sent = await request.text()
			return HttpResponse.json({ applied: PLAN.changes.slice(0, 2), skipped: [] })
		}),
	)

	await importing('{"format":"1.0.0"}')
	await userEvent.click(await screen.findByRole('checkbox', { name: /Remove Cook time/ }))
	await userEvent.click(screen.getByRole('button', { name: 'Apply' }))

	expect(await screen.findByText('Done. The site now holds what the file describes.')).toBeInTheDocument()
	const body = JSON.parse(sent)
	expect(body.confirm).toEqual([{ subject: 'field', key: 'cook-time', group: 'recipe-details' }])
	expect(body.format).toBe('1.0.0')
})

test('carries nothing the admin ticked and then changed their mind about', async () => {
	let sent = ''
	planning(() => HttpResponse.json(PLAN))
	server.use(
		http.post('/api/definitions/apply', async ({ request }) => {
			sent = await request.text()
			return HttpResponse.json({ applied: [], skipped: [] })
		}),
	)

	await importing('{"format":"1.0.0"}')
	const box = await screen.findByRole('checkbox', { name: /Remove Cook time/ })
	await userEvent.click(box)
	await userEvent.click(box)
	await userEvent.click(screen.getByRole('button', { name: 'Apply' }))

	await screen.findByText('Done. The site now holds what the file describes.')
	expect(JSON.parse(sent).confirm).toEqual([])
})

test('says why an import was turned away', async () => {
	planning(() => HttpResponse.json(PLAN))
	server.use(
		http.post('/api/definitions/apply', () =>
			HttpResponse.json({ error: 'no', code: 'definition_read_only', meta: { origin: 'events' } }, { status: 422 }),
		),
	)

	await importing('{"format":"1.0.0"}')
	await userEvent.click(await screen.findByRole('button', { name: 'Apply' }))

	expect(await screen.findByRole('alert')).toHaveTextContent(/events/)
})

test('warns that part of the file may already have landed when an apply fails', async () => {
	planning(() => HttpResponse.json(PLAN))
	server.use(
		http.post('/api/definitions/apply', () =>
			HttpResponse.json({ error: 'in use', code: 'type_in_use' }, { status: 422 }),
		),
	)

	await importing('{"format":"1.0.0"}')
	await userEvent.click(await screen.findByRole('button', { name: 'Apply' }))

	expect(
		await screen.findByText('The site may already hold part of this file. Import it again to see what is left.'),
	).toBeInTheDocument()
})

test('drops a failed apply warning when another file is chosen', async () => {
	planning(() => HttpResponse.json(PLAN))
	server.use(
		http.post('/api/definitions/apply', () =>
			HttpResponse.json({ error: 'in use', code: 'type_in_use' }, { status: 422 }),
		),
	)
	const said = 'The site may already hold part of this file. Import it again to see what is left.'

	await importing('{"format":"1.0.0"}')
	await userEvent.click(await screen.findByRole('button', { name: 'Apply' }))
	await screen.findByText(said)
	server.use(http.post('/api/definitions/plan', () => new Promise(() => {})))
	await userEvent.upload(
		screen.getByLabelText('Definitions file'),
		new File(['{"format":"1.0.0","groups":[]}'], 'other.json', { type: 'application/json' }),
	)

	expect(screen.queryByText(said)).not.toBeInTheDocument()
	expect(screen.queryByRole('alert')).not.toBeInTheDocument()
})

test('says an import was turned away even when the answer carries no reason', async () => {
	planning(() => HttpResponse.json(PLAN))
	server.use(http.post('/api/definitions/apply', () => new HttpResponse('gateway said no', { status: 502 })))

	await importing('{"format":"1.0.0"}')
	await userEvent.click(await screen.findByRole('button', { name: 'Apply' }))

	expect(await screen.findByRole('alert')).toBeInTheDocument()
})

test('names what an import left alone', async () => {
	planning(() => HttpResponse.json(PLAN))
	server.use(
		http.post('/api/definitions/apply', () =>
			HttpResponse.json({
				applied: [],
				skipped: [
					PLAN.changes[3],
					{ action: 'update', subject: 'type', key: 'recipe', label: 'Recipe', reason: 'nesting_kept' },
				],
			}),
		),
	)

	await importing('{"format":"1.0.0"}')
	await userEvent.click(await screen.findByRole('button', { name: 'Apply' }))

	expect(
		await screen.findByText('These were left alone, because nobody confirmed losing them.'),
	).toBeInTheDocument()
	expect(screen.getByText('Its items sit inside one another, so it keeps nesting.')).toBeInTheDocument()
})

test('drops the plan and the ticks the moment another file is chosen', async () => {
	planning(() => HttpResponse.json(PLAN))

	await importing('{"format":"1.0.0"}')
	await screen.findByText('Add Event details')
	await userEvent.click(screen.getByRole('checkbox', { name: /Remove Cook time/ }))
	server.use(http.post('/api/definitions/plan', () => new Promise(() => {})))
	await userEvent.upload(
		screen.getByLabelText('Definitions file'),
		new File(['{"format":"1.0.0","groups":[]}'], 'other.json', { type: 'application/json' }),
	)

	expect(screen.queryByText('Add Event details')).not.toBeInTheDocument()
	expect(screen.queryByRole('button', { name: 'Apply' })).not.toBeInTheDocument()
})

test('puts the plan away when the admin closes it', async () => {
	planning(() => HttpResponse.json(PLAN))

	await importing('{"format":"1.0.0"}')
	await screen.findByText('Add Event details')
	const [, footerClose] = screen.getAllByRole('button', { name: 'Close' })
	await userEvent.click(footerClose as HTMLElement)

	expect(screen.queryByText('Add Event details')).not.toBeInTheDocument()
})

test.each([
	{ kind: 'spaces between its keys', file: '{ "format": "1.0.0", "types": [], "groups": [] }' },
	{
		kind: 'a key written twice',
		file: '{"format":"1.0.0","types":[{"key":"first"}],"types":[{"key":"second"}],"groups":[]}',
	},
	{
		kind: 'a key written in two cases',
		file: '{"format":"1.0.0","types":[{"key":"first"}],"Types":[{"key":"second"}],"types":[{"key":"third"}],"groups":[]}',
	},
])('plans a file with $kind the way the import sends it', async ({ file }) => {
	let planned = ''
	let applied = ''
	planning(async ({ request }) => {
		planned = await request.text()
		return HttpResponse.json({
			changes: [{ action: 'create', subject: 'type', key: 'latest', label: 'Latest type' }],
			warnings: [],
		})
	})
	server.use(
		http.post('/api/definitions/apply', async ({ request }) => {
			applied = await request.text()
			return HttpResponse.json({ applied: [], skipped: [] })
		}),
	)

	await importing(file)
	await userEvent.click(await screen.findByRole('button', { name: 'Apply' }))
	await screen.findByText('Done. The site now holds what the file describes.')

	expect(applied).toBe(`${planned.slice(0, -1)},"confirm":[]}`)
})

test('says when a definitions file matches the site', async () => {
	planning(() => HttpResponse.json({ changes: [], warnings: [] }))

	await importing('{"format":"1.0.0","types":[],"groups":[]}')

	expect(await screen.findByText('This file matches the site, so there is nothing to change.')).toBeInTheDocument()
})

test('hands a file that is not JSON to the server as it was written', async () => {
	let sent = ''
	planning(async ({ request }) => {
		sent = await request.text()
		return HttpResponse.json({ error: 'malformed json', code: 'body_malformed' }, { status: 400 })
	})

	await importing('not a definitions file')

	expect(await screen.findByRole('alert')).toBeInTheDocument()
	expect(sent).toBe('not a definitions file')
})

test('says what the server refused about a definitions file', async () => {
	planning(() =>
		HttpResponse.json(
			{ error: 'too big', code: 'definitions_too_large', meta: { max: 262144 } },
			{ status: 413 },
		),
	)

	await importing('{"format":"1.0.0"}')

	expect(await screen.findByRole('alert')).toHaveTextContent(/262\.144 bytes/)
})

test('says a definitions file was turned away even when the answer carries no reason', async () => {
	planning(() => new HttpResponse('gateway said no', { status: 502 }))

	await importing('{"format":"1.0.0"}')

	expect(await screen.findByRole('alert')).toBeInTheDocument()
})
