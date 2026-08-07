// SPDX-License-Identifier: Apache-2.0

import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'
import type { ComponentType } from 'react'

import { createAppRouter } from '../router'

test('holds a loading ghost for routes whose chunk is still arriving', () => {
	const Pending = createAppRouter().options.defaultPendingComponent as ComponentType
	render(<Pending />)

	const status = screen.getByRole('status')
	expect(status).toHaveTextContent('Loading the screen.')
	expect(status.closest('.godmin-loading-screen')).not.toBeNull()
})

test('lets the ghost own the anti flash delay instead of router timers', () => {
	const options = createAppRouter().options

	expect(options.defaultPendingMs).toBe(0)
	expect(options.defaultPendingMinMs).toBe(0)
})
