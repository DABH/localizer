// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import sitemap from '@astrojs/sitemap';

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
				{ tag: 'link', attrs: { rel: 'icon', href: '/favicon.ico', sizes: '16x16 32x32 48x48' } },
				{ tag: 'link', attrs: { rel: 'icon', href: '/favicon-32.png', type: 'image/png', sizes: '32x32' } },
				{ tag: 'link', attrs: { rel: 'apple-touch-icon', href: '/apple-touch-icon.png' } },
				// The social image, shared by every page; scripts/images.py regenerates it.
				{ tag: 'meta', attrs: { property: 'og:image', content: 'https://locale.dev/og.png' } },
				{ tag: 'meta', attrs: { property: 'og:image:width', content: '1200' } },
				{ tag: 'meta', attrs: { property: 'og:image:height', content: '630' } },
				{ tag: 'meta', attrs: { property: 'og:image:alt', content: 'Localizer: your CLI, in your users’ language' } },
				{ tag: 'meta', attrs: { name: 'twitter:image', content: 'https://locale.dev/og.png' } },
			],
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/DABH/localizer' }],
			editLink: { baseUrl: 'https://github.com/DABH/localizer/edit/main/site/' },
			customCss: ['./src/styles/custom.css'],
			components: { Footer: './src/components/Footer.astro' },
			sidebar: [
				{ label: 'Start here', items: ['getting-started', 'how-it-works', 'pricing', 'agents'] },
				{
					label: 'Guides',
					items: ['guides/integration', 'guides/kong', 'guides/urfave', 'guides/github-action', 'guides/github-app', 'guides/testing'],
				},
				{ label: 'Reference', items: ['reference/configuration', 'reference/catalogs', 'reference/runtime'] },
				{ label: 'More', items: ['security', 'faq'] },
			],
		}),
		// Starlight adds this integration itself unless the config has one; this one leaves the
		// post-checkout page out of the sitemap (it is also marked noindex).
		sitemap({ filter: (page) => !page.includes('/welcome/') }),
	],
});
