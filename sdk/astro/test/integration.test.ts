// SPDX-License-Identifier: Apache-2.0

import { mkdirSync, mkdtempSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { pathToFileURL } from 'node:url'

import { afterAll, beforeAll, describe, expect, test, vi } from 'vitest'

import { gophenberg } from '../integration.ts'
import { kitName } from '../kit.ts'

/** The manifest the package ships. */
const manifest = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8')) as {
	exports: Record<string, string>
}

/** The root of the fixture theme the setup runs against. */
let starterRoot: string

beforeAll(() => {
	starterRoot = mkdtempSync(join(tmpdir(), 'gophenberg-starter-'))
	mkdirSync(join(starterRoot, 'src'))
	writeFileSync(join(starterRoot, 'src', 'theme.ts'), 'export default {}\n')
	writeFileSync(join(starterRoot, 'src', 'custom-theme.ts'), 'export default {}\n')
})

afterAll(() => {
	rmSync(starterRoot, { recursive: true, force: true })
})

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
function runSetup(options?: { theme?: string }, root = starterRoot): SetupRecord {
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
		config: { root: pathToFileURL(`${root}/`) },
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

	test('brings the node adapter the artifact runs on', () => {
		const [applied] = runSetup().updates
		const integrations = (applied.integrations ?? []) as { name: string }[]

		expect(integrations.map((added) => added.name)).toContain('@astrojs/node')
	})

	test('refuses a config that dropped the profile', () => {
		const done = gophenberg().hooks['astro:config:done']

		expect(() => done?.({ config: { output: 'static' } } as never)).toThrow(/output/)
	})

	test('accepts a config that kept it', () => {
		const done = gophenberg().hooks['astro:config:done']
		const kept = {
			output: 'server',
			image: { service: { entrypoint: 'astro/assets/services/noop' } },
			vite: { ssr: { noExternal: true } },
		}

		expect(() => done?.({ config: kept } as never)).not.toThrow()
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

	test('renders each address through the file that answers it', () => {
		const served = runSetup().routes.map((route) => [route.pattern, route.entrypoint])

		expect(Object.fromEntries(served)).toEqual({
			'/': `${kitName}/routes/archive.astro`,
			'/[type]/page/[page]': `${kitName}/routes/archive.astro`,
			'/[type]/[slug]': `${kitName}/routes/post.astro`,
			'/404': `${kitName}/routes/not-found.astro`,
			'/_gophenberg/health': `${kitName}/routes/health.ts`,
		})
	})

	test('points each route at a file the kit ships', () => {
		const shipped = readdirSync(new URL('../routes', import.meta.url))

		for (const route of runSetup().routes) {
			expect(route.entrypoint.startsWith(`${kitName}/routes/`)).toBe(true)
			expect(shipped).toContain(route.entrypoint.slice(`${kitName}/routes/`.length))
		}
	})

	test('points each route at a file the package exports under the same address', () => {
		for (const route of runSetup().routes) {
			const subpath = `.${route.entrypoint.slice(kitName.length)}`

			expect(manifest.exports[subpath]).toBe(subpath)
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

		expect(code).toBe(`export { default as theme } from ${JSON.stringify(`${starterRoot}/src/theme.ts`)}`)
	})

	test('emits the module the ambient declaration promises the routes', () => {
		const declaration = readFileSync(new URL('../virtual.d.ts', import.meta.url), 'utf8')
		const [, declaredModule] = declaration.match(/declare module '([^']+)'/) as RegExpMatchArray
		const [, declaredBinding] = declaration.match(/export const (\w+):/) as RegExpMatchArray
		const [plugin] = runSetup().plugins

		const resolved = plugin.resolveId(declaredModule) as string

		expect(resolved).toBeTruthy()
		expect(plugin.load(resolved)).toContain(`default as ${declaredBinding}`)
	})

	test('reads the path a theme named instead', () => {
		const [plugin] = runSetup({ theme: './src/custom-theme.ts' }).plugins
		const resolved = plugin.resolveId('virtual:gophenberg/theme') as string

		expect(plugin.load(resolved)).toContain(`${starterRoot}/src/custom-theme.ts`)
	})

	test('refuses a theme path that resolves to nothing, naming it', () => {
		expect(() => runSetup(undefined, '/themes/vanished')).toThrow('/themes/vanished/src/theme.ts')
	})

	test('decodes a root the file URL had to encode', () => {
		const root = mkdtempSync(join(tmpdir(), 'maría pérez '))
		mkdirSync(join(root, 'src'))
		writeFileSync(join(root, 'src', 'theme.ts'), 'export default {}\n')
		try {
			const [plugin] = runSetup(undefined, root).plugins
			const resolved = plugin.resolveId('virtual:gophenberg/theme') as string

			expect(plugin.load(resolved)).toContain(`${root}/src/theme.ts`)
		} finally {
			rmSync(root, { recursive: true, force: true })
		}
	})

	test('leaves every other module alone', () => {
		const [plugin] = runSetup().plugins

		expect(plugin.resolveId('some/other/module')).toBeUndefined()
		expect(plugin.load('some/other/module')).toBeUndefined()
	})
})
