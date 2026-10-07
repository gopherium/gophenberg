// SPDX-License-Identifier: Apache-2.0

/** The translation calls whose last argument names the text domain. */
const TRANSLATIONS = new Set<string | undefined>(['__', '_x', '_n', '_nx'])

/** Why a mutant of a text domain is left out. */
const REASON = 'The text domain argument of a translation call.'

/** The part of a syntax tree node the ignorer reads. */
interface TreeNode {
	type: string
	name?: string
	callee?: TreeNode
	arguments?: TreeNode[]
}

/** The part of a syntax tree path the ignorer reads. */
interface TreePath {
	node: TreeNode
	parentPath: TreePath | null
}

/**
 * Reports whether a node is a call to a translation function.
 * @param node - The node to read, nothing at the top of the tree.
 * @returns Whether the node is a translation call.
 */
function isTranslationCall(node: TreeNode | undefined): node is TreeNode & { arguments: TreeNode[] } {
	return TRANSLATIONS.has(node?.callee?.name)
}

/**
 * Returns why Stryker leaves a path unmutated, when the path is the text domain of a translation call.
 * @param path - The path Stryker is about to mutate.
 * @returns The reason, or nothing to mutate the path.
 */
export function textDomainReason(path: TreePath): string | undefined {
	const call = path.parentPath?.node
	const domain = path.node.type === 'StringLiteral' && isTranslationCall(call) && call.arguments.at(-1) === path.node
	return domain ? REASON : undefined
}

/** The Stryker plugins this module declares. */
export const strykerPlugins = [{ kind: 'Ignore', name: 'text-domain', value: { shouldIgnore: textDomainReason } }]
