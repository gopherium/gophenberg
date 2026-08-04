// SPDX-License-Identifier: Apache-2.0

import { describe, expect, test, vi } from 'vitest'

import { gophenberg } from '../integration.ts'
import { kitName } from '../kit.ts'

/** The theme plugin as the recorder captured it. */
interface RecordedPlugin {
	name: string
	resolveId(id: string): string | undefined
	load(id: string): string | undefined
}

/** What a recorded config:setup call captured. */
interface SetupRecord {
	updates: Record<string, unknown>[]
	routes: { pattern: string; entrypoint: string; prerender?: boolean }[]
	plugins: RecordedPlugin[]
}

/**
 * Runs the integration's config:setup hook against doubles and records what it asked for.
 * @param options - What the theme passed to the integration.
 * @param root - The project root the config reports.
 * @returns What the hook asked the build to do.
 */
function runSetup(options?: { theme?: string }, root = '/themes/starter'): SetupRecord {
	const record: SetupRecord = { updates: [], routes: [], plugins: [] }
	const updateConfig = vi.fn((update: Record<string, unknown>) => {
		record.updates.push(update)
		const vite = update.vite as { plugins?: RecordedPlugin[] } | undefined
		record.plugins.push(...(vite?.plugins ?? []))
		return update
	})
	const injectRoute = vi.fn((route: { pattern: string; entrypoint: string; prerender?: boolean }) => {
		record.routes.push(route)
	})
	const setup = gophenberg(options).hooks['astro:config:setup']
	setup?.({
		config: { root: new URL(`file://${root}/`) },
		updateConfig,
		injectRoute,
	} as never)
	return record
}

describe('the integration itself', () => {
	test('names the package it comes from', () => {
		expect(gophenberg().name).toBe(kitName)
	})

	test('hooks the two moments it needs', () => {
		const hooks = Object.keys(gophenberg().hooks)

		expect(hooks).toContain('astro:config:setup')
		expect(hooks).toContain('astro:config:done')
	})
})

describe('the build profile it pins', () => {
	test('asks for a server rendering on demand with everything bundled', () => {
		const [applied] = runSetup().updates

		expect(applied.output).toBe('server')
		expect((applied.vite as { ssr: { noExternal: boolean } }).ssr.noExternal).toBe(true)
	})

	test('refuses a config that dropped the profile', () => {
		const done = gophenberg().hooks['astro:config:done']

		expect(() => done?.({ config: { output: 'static' } } as never)).toThrow(/output/)
	})

	test('accepts a config that kept it', () => {
		const done = gophenberg().hooks['astro:config:done']

		expect(() => done?.({ config: { output: 'server' } } as never)).not.toThrow()
	})
})

describe('the routes it injects', () => {
	test('serves the addresses the feed already links', () => {
		const patterns = runSetup().routes.map((route) => route.pattern)

		expect(patterns).toContain('/')
		expect(patterns).toContain('/[type]/[slug]')
		expect(patterns).toContain('/[type]/page/[page]')
		expect(patterns).toContain('/404')
	})

	test('serves the readiness probe the supervisor waits on', () => {
		const patterns = runSetup().routes.map((route) => route.pattern)

		expect(patterns).toContain('/_gophenberg/health')
	})

	test('renders every route on demand', () => {
		for (const route of runSetup().routes) {
			expect(route.prerender).toBe(false)
		}
	})

	test('points each route at a file the kit ships', () => {
		for (const route of runSetup().routes) {
			expect(route.entrypoint).toContain('@gophenberg/astro/routes/')
		}
	})
})

describe('the theme it resolves', () => {
	test('reads the default path when the theme names none', () => {
		const [plugin] = runSetup().plugins

		expect(plugin).toBeDefined()
		expect(plugin.resolveId('virtual:gophenberg/theme')).toBeTruthy()
	})

	test('serves the theme through a module the routes import', () => {
		const [plugin] = runSetup().plugins
		const resolved = plugin.resolveId('virtual:gophenberg/theme') as string

		const code = plugin.load(resolved) as string

		expect(code).toContain('/themes/starter/src/theme.ts')
		expect(code).toContain('export')
	})

	test('reads the path a theme named instead', () => {
		const [plugin] = runSetup({ theme: './src/custom-theme.ts' }).plugins
		const resolved = plugin.resolveId('virtual:gophenberg/theme') as string

		expect(plugin.load(resolved)).toContain('/themes/starter/src/custom-theme.ts')
	})

	test('leaves every other module alone', () => {
		const [plugin] = runSetup().plugins

		expect(plugin.resolveId('some/other/module')).toBeUndefined()
		expect(plugin.load('some/other/module')).toBeUndefined()
	})
})
