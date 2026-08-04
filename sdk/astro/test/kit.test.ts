// SPDX-License-Identifier: Apache-2.0

import { describe, expect, test } from 'vitest'

import { contentApiPath, generator, isBlockName, kitFeatureVersion, kitName } from '../kit.ts'

describe('kit identity', () => {
	test('names the package a theme depends on', () => {
		expect(kitName).toBe('@gophenberg/astro')
	})

	test('addresses the content API a theme reads through', () => {
		expect(contentApiPath).toBe('/api/content/v1')
	})
})

describe('what a themed page reports', () => {
	test('names the product and drops the patch version', () => {
		expect(generator).toBe('Gophenberg 0.0')
	})

	test('reports the feature version the readiness probe answers with', () => {
		expect(kitFeatureVersion).toBe('0.0')
	})
})

describe('isBlockName', () => {
	test('accepts the names the editor serializes', () => {
		expect(isBlockName('core/paragraph')).toBe(true)
		expect(isBlockName('paragraph')).toBe(true)
		expect(isBlockName('my-plugin/my-block')).toBe(true)
	})

	test('rejects anything that is not a block name', () => {
		expect(isBlockName('')).toBe(false)
		expect(isBlockName('Core/Paragraph')).toBe(false)
		expect(isBlockName('core/')).toBe(false)
		expect(isBlockName('/paragraph')).toBe(false)
		expect(isBlockName('core/para/graph')).toBe(false)
		expect(isBlockName('9lives')).toBe(false)
	})
})
