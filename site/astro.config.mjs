// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// Deployed to GitHub Pages by .github/workflows/pages.yml. For a custom domain, set `site` to it and
// remove `base`.
export default defineConfig({
	site: 'https://dabh.github.io',
	base: '/localizer',
	trailingSlash: 'always',
	integrations: [
		starlight({
			title: 'Localizer',
			description:
				'Ship your Go CLI in your users’ language: AI translations kept in sync through pull requests and compiled into your binary.',
			logo: { src: './src/assets/logo.svg' },
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/DABH/localizer' }],
			editLink: { baseUrl: 'https://github.com/DABH/localizer/edit/main/site/' },
			customCss: ['./src/styles/custom.css'],
			components: { Footer: './src/components/Footer.astro' },
			sidebar: [
				{ label: 'Start here', items: ['getting-started', 'how-it-works'] },
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
