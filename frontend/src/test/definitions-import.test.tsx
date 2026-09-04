// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { fireEvent, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test } from 'vitest'

import { chosenDefinitions } from '../content/ImportDefinitions'
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
		{ action: 'delete', subject: 'type', key: 'note', label: 'Note', reason: 'removed' },
	],
	warnings: [
		{ code: 'route_word_changed', key: 'recipe' },
		{ code: 'root_moved', key: 'recipe' },
	],
}

/** Serves the given answer from the plan endpoint. */
function planning(answer: Parameters<typeof http.post>[1]) {
	server.use(http.post('/api/definitions/plan', answer))
}

/** Opens the import dialog and hands it a definitions file carrying the text. */
async function importing(text: string) {
	renderAt('/field-groups')
	await userEvent.click(await screen.findByRole('button', { name: 'Import definitions' }))
	await userEvent.upload(
		screen.getByLabelText('Definitions file'),
		new File([text], 'definitions.json', { type: 'application/json' }),
	)
}

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
	expect(screen.getByText('recipe answers on a new address, and stored links move with it.')).toBeInTheDocument()
	expect(
		screen.getByText('recipe becomes the type the site opens on, and the old one takes an address.'),
	).toBeInTheDocument()
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
			HttpResponse.json({ applied: [], skipped: [PLAN.changes[3]] }),
		),
	)

	await importing('{"format":"1.0.0"}')
	await userEvent.click(await screen.findByRole('button', { name: 'Apply' }))

	expect(
		await screen.findByText('These were left alone, because nobody confirmed losing them.'),
	).toBeInTheDocument()
})

test('puts the plan away when the admin closes it', async () => {
	planning(() => HttpResponse.json(PLAN))

	await importing('{"format":"1.0.0"}')
	await screen.findByText('Add Event details')
	const [, footerClose] = screen.getAllByRole('button', { name: 'Close' })
	await userEvent.click(footerClose as HTMLElement)

	expect(screen.queryByText('Add Event details')).not.toBeInTheDocument()
})

test('sends the file the admin chose exactly as it was written', async () => {
	let sent = ''
	planning(async ({ request }) => {
		sent = await request.text()
		return HttpResponse.json({ changes: [], warnings: [] })
	})

	await importing('{"format":"1.0.0","types":[],"groups":[]}')

	expect(await screen.findByText('This file matches the site, so there is nothing to change.')).toBeInTheDocument()
	expect(sent).toBe('{"format":"1.0.0","types":[],"groups":[]}')
})

test('says what the server refused about a definitions file', async () => {
	planning(() =>
		HttpResponse.json(
			{ error: 'too big', code: 'definitions_too_large', meta: { max: 262144 } },
			{ status: 413 },
		),
	)

	await importing('{"format":"1.0.0"}')

	expect(await screen.findByRole('alert')).toHaveTextContent(/262144/)
})

test('says a definitions file was turned away even when the answer carries no reason', async () => {
	planning(() => new HttpResponse('gateway said no', { status: 502 }))

	await importing('{"format":"1.0.0"}')

	expect(await screen.findByRole('alert')).toBeInTheDocument()
})
