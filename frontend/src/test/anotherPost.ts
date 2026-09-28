// SPDX-License-Identifier: Apache-2.0

import { act, screen, waitFor } from '@testing-library/react'
import { expect } from 'vitest'

import type { renderRoutedAt } from './render'
import { storedPostWithId } from './postFixture'

/** Another post the editor can open beside the one under test. */
export const OTHER_POST = { ...storedPostWithId('019fb000-0000-7000-8000-000000000002'), title: 'Another post' }

/** The router the admin runs on in a test. */
export type Router = ReturnType<typeof renderRoutedAt>['router']

/**
 * Leaves the editor for another post and waits until that post shows.
 * @param router - The router the admin runs on.
 */
export async function openAnotherPost(router: Router) {
	await act(async () => {
		await router.navigate({
			to: '/content/$typeKey/$postId/edit',
			params: { typeKey: 'post', postId: OTHER_POST.id },
		})
	})
	await waitFor(() => expect(screen.getByRole('textbox', { name: 'Title' })).toHaveValue(OTHER_POST.title))
}

/**
 * Goes back to the page the author came from.
 * @param router - The router the admin runs on.
 */
export async function comeBack(router: Router) {
	await act(async () => {
		router.history.back()
	})
}
