// SPDX-License-Identifier: Apache-2.0

import node from '@astrojs/node'
import type { AstroIntegration } from 'astro'

import { kitName } from './kit.ts'
import { buildProfile, profileComplaint } from './profile.ts'

/** The module the injected routes read the active theme through. */
export const themeModuleId = 'virtual:gophenberg/theme'

/** The path a theme lives at when it names none. */
const defaultThemePath = './src/theme.ts'

/** The addresses the kit serves, and the files it serves them from. */
const injectedRoutes = [
	{ pattern: '/', entrypoint: 'archive.astro' },
	{ pattern: '/[type]/page/[page]', entrypoint: 'archive.astro' },
	{ pattern: '/[type]/[slug]', entrypoint: 'post.astro' },
	{ pattern: '/404', entrypoint: 'not-found.astro' },
	{ pattern: '/_gophenberg/health', entrypoint: 'health.ts' },
]

/** How a theme names itself to the integration. */
export interface GophenbergOptions {
	theme?: string
}

/**
 * Returns the integration wiring a theme, the injected routes, and the build profile into a project.
 * @param options - The path the theme lives at.
 * @returns The integration an Astro config lists.
 */
export function gophenberg(options: GophenbergOptions = {}): AstroIntegration {
	const themePath = options.theme ?? defaultThemePath
	return {
		name: kitName,
		hooks: {
			'astro:config:setup': ({ config, updateConfig, injectRoute }) => {
				const profile = buildProfile()
				updateConfig({
					...profile,
					integrations: [node({ mode: 'standalone' })],
					vite: { ...profile.vite, plugins: [themePlugin(config.root, themePath)] },
				})
				for (const route of injectedRoutes) {
					injectRoute({
						pattern: route.pattern,
						entrypoint: `@gophenberg/astro/routes/${route.entrypoint}`,
						prerender: false,
					})
				}
			},
			'astro:config:done': ({ config }) => {
				const complaint = profileComplaint(config)
				if (complaint) {
					throw new Error(complaint)
				}
			},
		},
	}
}

/**
 * Returns the plugin serving the active theme to the injected routes.
 * @param root - The project root the theme path is relative to.
 * @param themePath - Where the theme lives.
 * @returns The vite plugin.
 */
function themePlugin(root: URL, themePath: string) {
	const resolvedId = `\0${themeModuleId}`
	const themeFile = new URL(themePath, root).pathname
	return {
		name: 'gophenberg:theme',
		resolveId: (id: string) => (id === themeModuleId ? resolvedId : undefined),
		load: (id: string) =>
			id === resolvedId ? `export { default as theme } from ${JSON.stringify(themeFile)}` : undefined,
	}
}
