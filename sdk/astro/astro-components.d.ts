// SPDX-License-Identifier: Apache-2.0

declare module '*.astro' {
	import type { AstroComponentFactory } from 'astro/runtime/server/index.js'

	const Component: AstroComponentFactory
	export default Component
}
