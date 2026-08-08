// @ts-check
// SPDX-License-Identifier: Apache-2.0
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
	site: 'https://docs.gophenberg.org',
	integrations: [
		starlight({
			title: 'Gophenberg',
			description:
				'A plugin-first CMS. Go backend, React admin, Gutenberg editor.',
			social: [
				{ icon: 'github', label: 'GitHub', href: 'https://github.com/gopherium/gophenberg' },
			],
			editLink: {
				baseUrl: 'https://github.com/gopherium/gophenberg/edit/main/docs/',
			},
			sidebar: [
				{
					label: 'Start here',
					items: [
						{ slug: 'start/what-is-gophenberg' },
						{ slug: 'start/local-development' },
					],
				},
				{
					label: 'Using Gophenberg',
					items: [
						{ slug: 'guides/posts' },
						{ slug: 'guides/editor' },
						{ slug: 'guides/publishing' },
						{ slug: 'guides/users' },
					],
				},
				{
					label: 'Self-hosting',
					items: [
						{ slug: 'self-hosting/install' },
						{ slug: 'self-hosting/configuration' },
						{ slug: 'self-hosting/updates-and-backups' },
					],
				},
				{
					label: 'Themes',
					items: [
						{ slug: 'themes/how-theming-works' },
						{ slug: 'themes/writing-a-theme' },
						{ slug: 'themes/rendering-blocks' },
						{ slug: 'themes/installing-a-theme' },
					],
				},
				{
					label: 'Extending',
					items: [
						{ slug: 'extending/write-a-plugin' },
						{ slug: 'extending/the-plugin-sdk' },
					],
				},
				{
					label: 'Reference',
					items: [
						{ slug: 'reference/content-api' },
						{ slug: 'reference/rss-feed' },
						{ slug: 'reference/identification' },
					],
				},
				{
					label: 'Legal',
					items: [{ slug: 'legal/licensing' }],
				},
			],
		}),
	],
});
