// SPDX-License-Identifier: Apache-2.0

import { commentAuthorAvatar, Icon } from '@gophenberg/frontend-sdk'
import type { Field, Filter } from '@gophenberg/frontend-sdk/dataviews'
import { InitialsAvatar } from '@gopherium/godmin'
import { __ } from '@wordpress/i18n'
import { z } from 'zod'

/** The author a listed item carries. */
interface Authored {
	/** The id of the account that wrote the item. */
	authorId: string
	/** The name of that account. */
	authorName: string
	/** The address of that account's picture, empty when it has none. */
	authorAvatar: string
}

/** The part of a listing request an author filter narrows it by. */
interface AuthorTerms {
	/** The accounts whose items the list keeps. */
	author?: string[]
	/** The accounts whose items the list leaves out. */
	authorExclude?: string[]
}

const authorsSchema = z.object({ items: z.array(z.object({ id: z.string(), name: z.string() })) })

/** The shape of an account id. */
const ACCOUNT_ID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

/**
 * Returns every account that may write, as the choices of the author filter.
 * @returns Each account as the id the filter sends and the name it shows.
 */
async function authorElements(): Promise<{ value: string, label: string }[]> {
	const response = await fetch('/api/authors')
	if (!response.ok) {
		throw new Error(`listing authors failed with status ${response.status}`)
	}
	return authorsSchema.parse(await response.json()).items.map((author) => ({ value: author.id, label: author.name }))
}

/**
 * Renders the author of an item as WordPress draws one, its avatar before its name, the author icon with no avatar.
 * @param props - The row being rendered.
 * @returns The author cell.
 */
function AuthorCell({ item }: { item: Authored }) {
	return (
		<span className="gophenberg-author">
			{item.authorAvatar === '' ? (
				<span className="gophenberg-author__icon">
					<Icon icon={commentAuthorAvatar} />
				</span>
			) : (
				<span className="gophenberg-author__avatar">
					<InitialsAvatar name={item.authorName} size={16} src={item.authorAvatar} />
				</span>
			)}
			<span className="gophenberg-author__name">{item.authorName}</span>
		</span>
	)
}

/**
 * Returns the column naming the author of each item, filtered by the accounts it keeps or leaves out.
 * @returns The author column.
 */
export function authorField<Item extends Authored>(): Field<Item> {
	return {
		id: 'author',
		label: __('Author', 'gophenberg'),
		render: AuthorCell,
		getElements: authorElements,
		filterBy: { operators: ['isAny', 'isNone'] },
	}
}

/**
 * Reports whether a filter value names accounts, each by an id in the shape account ids take.
 * @param value - The value the filter holds.
 * @returns Whether the value is a list of account ids.
 */
export function namesAccounts(value: unknown): boolean {
	return Array.isArray(value) && value.every((id) => ACCOUNT_ID.test(String(id)))
}

/**
 * Returns the part of a listing request an author filter narrows it by.
 * @param filter - The author filter the view carries.
 * @returns The accounts whose items the list keeps, or the ones it leaves out.
 */
export function authorNarrowing(filter: Filter): AuthorTerms {
	const ids = filter.value as string[]
	return filter.operator === 'isAny' ? { author: ids } : { authorExclude: ids }
}
