// SPDX-License-Identifier: Apache-2.0

import { gophenberg } from '@gophenberg/astro/config'
import { defineConfig } from 'astro/config'

export default defineConfig({
	integrations: [gophenberg()],
})
