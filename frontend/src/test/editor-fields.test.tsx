// SPDX-License-Identifier: Apache-2.0

import { http, HttpResponse, server } from '@gophenberg/frontend-sdk/testing'
import { fireEvent, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeAll, beforeEach, expect, test } from 'vitest'

import { renderAt } from './render'
import { storedPost } from './postFixture'

const EDITOR_PATH = `/content/post/${storedPost.id}/edit`

const STAMP = '2026-08-01T10:00:00Z'

const TYPE_WITH_FIELDS = {
	key: 'post',
	singular_label: 'Post',
	plural_label: 'Posts',
	route_word: '',
	hierarchical: false,
	revisions: true,
	revision_cap: 100,
	page_kind: 'single',
	default: true,
	active: true,
	created_at: '2026-08-01T10:00:00Z',
	updated_at: '2026-08-01T10:00:00Z',
	fields: [
		{ key: 'color', label: 'Color', kind: 'text', many: false, required: false, updated_at: STAMP },
		{ key: 'doors', label: 'Doors', kind: 'number', many: false, required: false, updated_at: STAMP },
		{ key: 'boxed', label: 'Boxed', kind: 'boolean', many: false, required: false, updated_at: STAMP },
	],
}

beforeAll(async () => {
	await import('../content/EditorScreen')
}, 120000)

beforeEach(() => {
	server.use(
		http.get('/api/types', () => HttpResponse.json({ items: [TYPE_WITH_FIELDS] })),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { color: 'red' } }),
		),
	)
})

test('shows a control for every scalar field the type declares', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByLabelText('Color')).toBeInTheDocument()
	expect(screen.getByLabelText('Doors')).toBeInTheDocument()
	expect(screen.getByLabelText('Boxed')).toBeInTheDocument()
})

test('shows the value the post holds', async () => {
	renderAt(EDITOR_PATH)

	expect(await screen.findByLabelText('Color')).toHaveValue('red')
})

test('shows no panel when the type declares no fields', async () => {
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [{ ...TYPE_WITH_FIELDS, fields: [] }] }),
		),
	)
	renderAt(EDITOR_PATH)
	await screen.findByRole('tab', { name: 'Document' })

	expect(screen.queryByLabelText('Color')).not.toBeInTheDocument()
})

/**
 * Serves a type whose only field is the one described.
 * @param field - The field the type declares.
 */
function declaring(field: Record<string, unknown>) {
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({
				items: [{ ...TYPE_WITH_FIELDS, fields: [{ updated_at: STAMP, ...field }] }],
			}),
		),
	)
}

const A_TEXT_FIELD = {
	key: 'color',
	label: 'Color',
	kind: 'text',
	many: false,
	required: false,
	updated_at: STAMP,
}

test('names the ceiling under a number typed above its max', async () => {
	declaring({
		key: 'doors',
		label: 'Doors',
		kind: 'number',
		many: false,
		required: false,
		settings: { max: 10 },
	})
	renderAt(EDITOR_PATH)

	const control = await screen.findByLabelText('Doors')
	await userEvent.type(control, '50')
	await userEvent.tab()

	expect(
		await screen.findByText('Doors goes no higher than 10. Lower the value and save again.'),
	).toBeInTheDocument()

	await userEvent.clear(control)
	await userEvent.type(control, '5')
	await userEvent.tab()

	await waitFor(() =>
		expect(
			screen.queryByText('Doors goes no higher than 10. Lower the value and save again.'),
		).toBeNull(),
	)
})

test('leaves a sibling its own complaint while a number sits outside its max', async () => {
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({
				items: [
					{
						...TYPE_WITH_FIELDS,
						fields: [
							{
								key: 'doors',
								label: 'Doors',
								kind: 'number',
								many: false,
								required: false,
								settings: { max: 10 },
								updated_at: STAMP,
							},
							{
								key: 'color',
								label: 'Color',
								kind: 'text',
								many: false,
								required: true,
								settings: { maxlength: 8 },
								updated_at: STAMP,
							},
						],
					},
				],
			}),
		),
	)
	renderAt(EDITOR_PATH)
	const doors = await screen.findByLabelText('Doors')
	const color = screen.getByLabelText(/Color/)

	await userEvent.type(doors, '50')
	await userEvent.clear(color)
	await userEvent.tab()

	expect(
		await screen.findByText('Doors goes no higher than 10. Lower the value and save again.'),
	).toBeInTheDocument()
	expect(color).toBeRequired()
	expect(color).toHaveAttribute('maxlength', '8')
	expect((color as HTMLInputElement).validationMessage).not.toBe('')
})

test('shows a checkbox per listed answer and keeps a stored stray checked', async () => {
	declaring({
		key: 'styles',
		label: 'Styles',
		kind: 'choice',
		many: false,
		required: false,
		settings: {
			presentation: 'checkbox',
			multiple: true,
			choices: [
				{ value: 'ipa', label: 'IPA' },
				{ value: 'stout', label: 'Stout' },
			],
		},
	})
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { styles: ['stout', 'homebrew'] } }),
		),
	)
	renderAt(EDITOR_PATH)

	const stout = await screen.findByRole('checkbox', { name: 'Stout' })
	expect(stout).toBeChecked()
	expect(screen.getByRole('checkbox', { name: 'homebrew' })).toBeChecked()
	const ipa = screen.getByRole('checkbox', { name: 'IPA' })
	expect(ipa).not.toBeChecked()

	await userEvent.click(ipa)
	await userEvent.click(stout)

	expect(ipa).toBeChecked()
	expect(stout).not.toBeChecked()
})

test('offers every checkbox unticked when the item holds no answer yet', async () => {
	declaring({
		key: 'styles',
		label: 'Styles',
		kind: 'choice',
		many: false,
		required: false,
		settings: {
			presentation: 'checkbox',
			multiple: true,
			choices: [
				{ value: 'ipa', label: 'IPA' },
				{ value: 'stout', label: 'Stout' },
			],
		},
	})
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('checkbox', { name: 'IPA' })).not.toBeChecked()
	expect(screen.getByRole('checkbox', { name: 'Stout' })).not.toBeChecked()
})

test('shows the instructions a field carries under its control', async () => {
	declaring({ ...A_TEXT_FIELD, settings: { instructions: 'Name the colour.' } })
	renderAt(EDITOR_PATH)

	expect(await screen.findByText('Name the colour.')).toBeInTheDocument()
})

test('shows the placeholder a field carries in its empty control', async () => {
	declaring({ ...A_TEXT_FIELD, settings: { placeholder: 'red' } })
	renderAt(EDITOR_PATH)

	expect(await screen.findByLabelText('Color')).toHaveAttribute('placeholder', 'red')
})

test('holds a control to the length its field allows', async () => {
	declaring({ ...A_TEXT_FIELD, settings: { maxlength: 5 } })
	renderAt(EDITOR_PATH)

	expect(await screen.findByLabelText('Color')).toHaveAttribute('maxlength', '5')
})

test('shows the instructions a media field carries, which no control builds for it', async () => {
	declaring({
		key: 'cover',
		label: 'Cover',
		kind: 'media',
		many: false,
		required: false,
		settings: { instructions: 'Pick a wide one.' },
	})
	renderAt(EDITOR_PATH)

	expect(await screen.findByText('Pick a wide one.')).toBeInTheDocument()
})

test('offers a choice field its listed pairs', async () => {
	declaring({
		key: 'style',
		label: 'Style',
		kind: 'choice',
		many: false,
		required: false,
		settings: {
			choices: [
				{ value: 'ipa', label: 'IPA' },
				{ value: 'stout', label: 'Stout' },
			],
		},
	})
	renderAt(EDITOR_PATH)

	const control = await screen.findByLabelText('Style')
	await userEvent.click(control)

	expect(await screen.findByRole('option', { name: 'IPA' })).toBeInTheDocument()
	expect(screen.getByRole('option', { name: 'Stout' })).toBeInTheDocument()
})

test('offers a radio presentation one circle per pair', async () => {
	declaring({
		key: 'style',
		label: 'Style',
		kind: 'choice',
		many: false,
		required: false,
		settings: {
			presentation: 'radio',
			choices: [
				{ value: 'ipa', label: 'IPA' },
				{ value: 'stout', label: 'Stout' },
			],
		},
	})
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('radio', { name: 'IPA' })).toBeInTheDocument()
	expect(screen.getByRole('radio', { name: 'Stout' })).toBeInTheDocument()
})

test('offers a textarea variant a box holding lines', async () => {
	declaring({
		key: 'notes',
		label: 'Notes',
		kind: 'text',
		many: false,
		required: false,
		settings: { variant: 'textarea' },
	})
	renderAt(EDITOR_PATH)

	const control = await screen.findByLabelText('Notes')
	expect(control.tagName).toBe('TEXTAREA')
})

test('offers an email variant the input that says so', async () => {
	declaring({
		key: 'contact',
		label: 'Contact',
		kind: 'text',
		many: false,
		required: false,
		settings: { variant: 'email' },
	})
	renderAt(EDITOR_PATH)

	expect(await screen.findByLabelText('Contact')).toHaveAttribute('type', 'email')
})

test('offers a bounded range a slider and an unbounded one the plain input', async () => {
	declaring({
		key: 'rating',
		label: 'Rating',
		kind: 'number',
		many: false,
		required: false,
		settings: { presentation: 'range', min: 1, max: 10 },
	})
	renderAt(EDITOR_PATH)

	expect(await screen.findByRole('slider', { name: 'Rating' })).toBeInTheDocument()
})

test('sends the number the slider was dragged to', async () => {
	declaring({
		key: 'rating',
		label: 'Rating',
		kind: 'number',
		many: false,
		required: false,
		settings: { presentation: 'range', min: 1, max: 10 },
	})
	const sent: unknown[] = []
	server.use(
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...storedPost, fields: { rating: 7 } })
		}),
	)
	renderAt(EDITOR_PATH)
	const slider = await screen.findByRole('slider', { name: 'Rating' })

	fireEvent.change(slider, { target: { value: '7' } })
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { rating: 7 } })
})

test('sends the edited values when the post is saved', async () => {
	const sent: unknown[] = []
	server.use(
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push(await request.json())
			return HttpResponse.json({ ...storedPost, fields: { color: 'blue' } })
		}),
	)
	renderAt(EDITOR_PATH)
	const control = await screen.findByLabelText('Color')

	await userEvent.clear(control)
	await userEvent.type(control, 'blue')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0]).toMatchObject({ fields: { color: 'blue' } })
})

test('sends no field values when only the title moved', async () => {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: { color: 'red' } })
		}),
	)
	renderAt(EDITOR_PATH)
	await screen.findByLabelText('Color')

	await userEvent.type(screen.getByRole('textbox', { name: 'Title' }), ' again')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0].fields).toEqual({})
})

test('sends no value for a field the type stopped declaring', async () => {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.get('/api/types', () =>
			HttpResponse.json({ items: [{ ...TYPE_WITH_FIELDS, fields: [] }] }),
		),
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { color: 'red' } }),
		),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: { color: 'red' } })
		}),
	)
	renderAt(EDITOR_PATH)

	await userEvent.type(await screen.findByRole('textbox', { name: 'Title' }), ' again')
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0].fields).toEqual({})
})

test('clears a number field when its control is emptied', async () => {
	const sent: Record<string, unknown>[] = []
	server.use(
		http.get(`/api/content/${storedPost.id}`, () =>
			HttpResponse.json({ ...storedPost, fields: { doors: 4 } }),
		),
		http.patch(`/api/content/${storedPost.id}`, async ({ request }) => {
			sent.push((await request.json()) as Record<string, unknown>)
			return HttpResponse.json({ ...storedPost, fields: {} })
		}),
	)
	renderAt(EDITOR_PATH)
	const control = await screen.findByLabelText('Doors')

	await userEvent.clear(control)
	await userEvent.click(screen.getByRole('button', { name: 'Save draft' }))

	await waitFor(() => expect(sent).toHaveLength(1))
	expect(sent[0].fields).toEqual({ doors: null })
})
