// SPDX-License-Identifier: Apache-2.0

import { render, screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { TextareaControl } from '../index'

test('describes a textarea with the words given under it', () => {
	render(<TextareaControl label="Instructions" value="" description="Shown under the box." onValueChange={vi.fn()} />)

	expect(screen.getByRole('textbox', { name: 'Instructions' })).toHaveAccessibleDescription('Shown under the box.')
})

test('leaves a textarea given no words under it without a description', () => {
	render(<TextareaControl label="Instructions" value="" onValueChange={vi.fn()} />)

	expect(screen.getByRole('textbox', { name: 'Instructions' })).not.toHaveAccessibleDescription()
})

test('keeps a textarea marked read only from taking typing', () => {
	render(<TextareaControl label="Instructions" value="Shown under the box." readOnly onValueChange={vi.fn()} />)

	expect(screen.getByRole('textbox', { name: 'Instructions' })).toHaveAttribute('readonly')
})

test('lets a textarea not marked read only take typing', () => {
	render(<TextareaControl label="Instructions" value="" onValueChange={vi.fn()} />)

	expect(screen.getByRole('textbox', { name: 'Instructions' })).not.toHaveAttribute('readonly')
})
