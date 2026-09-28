// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// Published at https://locale.dev by .github/workflows/site.yml.
export default defineConfig({
	site: 'https://locale.dev',
	trailingSlash: 'always',
	integrations: [
		starlight({
			title: 'Localizer',
			description:
				'Ship your CLI in your users’ language: AI translations kept in sync through pull requests and shipped inside your binary or package. Go and Python.',
			logo: { src: './src/assets/logo.svg' },
			head: [
				{ tag: 'link', attrs: { rel: 'icon', href: '/favicon-32.png', type: 'image/png', sizes: '32x32' } },
				{ tag: 'link', attrs: { rel: 'apple-touch-icon', href: '/apple-touch-icon.png' } },
			],
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/DABH/localizer' }],
			editLink: { baseUrl: 'https://github.com/DABH/localizer/edit/main/site/' },
			customCss: ['./src/styles/custom.css'],
			components: { Footer: './src/components/Footer.astro' },
			sidebar: [
				{ label: 'Start here', items: ['getting-started', 'how-it-works', 'agents'] },
				{
					label: 'Guides',
					items: ['guides/integration', 'guides/github-action', 'guides/github-app', 'guides/testing'],
				},
				{ label: 'Reference', items: ['reference/configuration', 'reference/catalogs', 'reference/runtime'] },
				{ label: 'More', items: ['security', 'faq'] },
			],
		}),
	],
});
