// SPDX-License-Identifier: Apache-2.0

import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'

import { BootLoading } from '../boot'

test('ghosts the screen while the session loads', () => {
	render(<BootLoading />)

	const status = screen.getByRole('status')
	expect(status).toHaveTextContent('Loading…')
	expect(status.closest('.godmin-loading-screen')).not.toBeNull()
})
