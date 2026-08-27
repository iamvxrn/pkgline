import { defineConfig } from 'vitepress'

export default defineConfig({
  title: "pkgline",
  description: "A single Go CLI for installing developer tools, orchestrating builds, and managing packages.",
  appearance: 'dark',
  head: [['link', { rel: 'icon', href: '/favicon.svg' }]],
  themeConfig: {
    logo: '/logo.svg',
    nav: [
      { text: 'Home', link: '/' },
      { text: 'GitHub', link: 'https://github.com/iamvxrn/pkgline' }
    ],
    search: {
      provider: 'local'
    },
    sidebar: [
      {
        text: 'START',
        items: [
          { text: 'Overview', link: '/overview' },
          { text: 'Install', link: '/install' },
          { text: 'Quickstart', link: '/quickstart' },
          { text: 'CLI Reference', link: '/cli' },
          { text: 'Changelog', link: '/changelog' },
        ]
      },
      {
        text: 'FEATURES',
        items: [
          { text: 'Manifest', link: '/manifest' },
          { text: 'Multi-Forge', link: '/multi-forge' },
          { text: 'Rollbacks', link: '/rollbacks' },
        ]
      }
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/iamvxrn/pkgline' }
    ]
  }
})
