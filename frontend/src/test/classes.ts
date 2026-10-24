// SPDX-License-Identifier: Apache-2.0

import { render } from '@testing-library/react'
import type { ReactElement } from 'react'

/**
 * Returns the classes WPDS draws an element with when it renders on its own.
 * @param element - The element to render.
 * @returns The class attribute of the rendered element.
 */
export function classesOf(element: ReactElement): string {
	const { container, unmount } = render(element)
	const classes = (container.firstElementChild as HTMLElement).className
	unmount()
	return classes
}
