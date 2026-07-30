// SPDX-License-Identifier: Apache-2.0

import { render, screen } from '@testing-library/react'
import { expect, test } from 'vitest'

test('queries ignore the a11y live region duplicating announced text', () => {
	render(
		<div>
			<p>Post saved</p>
			<div className="a11y-speak-region">Post saved</div>
		</div>,
	)

	expect(screen.getByText('Post saved')).toBeInTheDocument()
})
