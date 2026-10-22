import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

const repo = 'https://github.com/reisaraujo-miguel/runcodes'

// GitHub Pages serves project sites from https://<owner>.github.io/<repo>/.
// The workflow sets DOCS_BASE=/runcodes/; override it for a custom domain or
// when previewing the site at the server root.
const base = process.env.DOCS_BASE || '/runcodes/'

const guideSidebar = [
  {
    text: 'Introduction',
    items: [
      { text: 'What is RunCodes?', link: '/guide/overview' },
      { text: 'Architecture', link: '/guide/architecture' },
      { text: 'Getting started', link: '/guide/getting-started' },
      { text: 'Using RunCodes', link: '/guide/using-runcodes' }
    ]
  },
  {
    text: 'Platform guide',
    items: [
      { text: 'Submissions & live judging', link: '/guide/submissions' },
      { text: 'Classes, roles & access', link: '/guide/classes-and-roles' },
      { text: 'Accounts & settings', link: '/guide/accounts-and-settings' },
      { text: 'Frontend application', link: '/guide/frontend' }
    ]
  }
]

const servicesSidebar = [
  {
    text: 'Services',
    items: [
      { text: 'Backend API', link: '/services/backend' },
      { text: 'Judge', link: '/services/judge' },
      { text: 'Runner images & monitor', link: '/services/runners-and-monitor' },
      { text: 'Database', link: '/services/database' }
    ]
  }
]

const apiSidebar = [
  {
    text: 'HTTP API',
    items: [
      { text: 'Overview & authentication', link: '/api/' },
      { text: 'Users & accounts', link: '/api/auth' },
      { text: 'Classes & exercises', link: '/api/classes' },
      { text: 'Submissions & events', link: '/api/submissions' },
      { text: 'Administration', link: '/api/admin' }
    ]
  },
  {
    text: 'Internal contracts',
    items: [{ text: 'Judge API', link: '/api/judge' }]
  }
]

const deploymentSidebar = [
  {
    text: 'Deployment',
    items: [
      { text: 'Choose a deployment', link: '/deployment/' },
      { text: 'Docker Compose', link: '/deployment/compose' },
      { text: 'Ansible: setup', link: '/deployment/ansible-setup' },
      { text: 'Ansible: operations', link: '/deployment/ansible-operations' },
      { text: 'Security & scaling', link: '/deployment/security' }
    ]
  }
]

const observabilitySidebar = [
  {
    text: 'Observability',
    items: [
      { text: 'Overview', link: '/observability/' },
      { text: 'Local stack', link: '/observability/local' },
      { text: 'Four-machine deployment', link: '/observability/production' },
      { text: 'Runbook', link: '/observability/runbook' }
    ]
  }
]

const referenceSidebar = [
  {
    text: 'Reference',
    items: [
      { text: 'Configuration', link: '/reference/configuration' },
      { text: 'Ports & storage', link: '/reference/ports-and-storage' },
      { text: 'Languages & images', link: '/reference/languages' },
      { text: 'Glossary', link: '/reference/glossary' }
    ]
  },
  {
    text: 'Project',
    items: [
      { text: 'CI/CD & releases', link: '/ci-cd' },
      { text: 'Contributing', link: '/contributing' },
      { text: 'Documentation site', link: '/documentation' },
      { text: 'Documentation coverage', link: '/reference/source-map' }
    ]
  }
]

export default withMermaid(
  defineConfig({
    lang: 'en-US',
    title: 'RunCodes Docs',
    titleTemplate: ':title | RunCodes',
    description:
      'Documentation for RunCodes, an open-source platform for programming exercises with isolated containers and live judging.',

    base,
    cleanUrls: false,
    lastUpdated: true,
    metaChunk: true,

    // The docs intentionally link to services that only exist on the reader's
    // machine (localhost:3000, localhost:8080, ...). Those are valid getting
    // started targets, not missing site pages.
    ignoreDeadLinks: [/^https?:\/\/localhost(:\d+)?(\/.*)?$/],

    head: [
      ['link', { rel: 'icon', type: 'image/svg+xml', href: `${base}runcodes-icon.svg` }],
      ['meta', { name: 'theme-color', content: '#723bf7' }],
      ['meta', { property: 'og:type', content: 'website' }],
      ['meta', { property: 'og:site_name', content: 'RunCodes Docs' }],
      [
        'meta',
        {
          property: 'og:title',
          content: 'RunCodes Documentation'
        }
      ],
      [
        'meta',
        {
          property: 'og:description',
          content:
            'Architecture, API, deployment and operations documentation for the RunCodes programming-education platform.'
        }
      ],
      ['meta', { property: 'og:image', content: `${base}runcodes-icon.svg` }],
      ['meta', { name: 'twitter:card', content: 'summary' }]
    ],

    sitemap: {
      hostname: 'https://reisaraujo-miguel.github.io/runcodes/'
    },

    markdown: {
      lineNumbers: true,
      theme: {
        light: 'github-light',
        dark: 'github-dark'
      }
    },

    mermaid: {
      theme: 'neutral',
      flowchart: { curve: 'basis' }
    },

    themeConfig: {
      logo: '/runcodes-icon.svg',
      siteTitle: 'RunCodes Docs',

      nav: [
        { text: 'Guide', link: '/guide/overview', activeMatch: '/guide/' },
        { text: 'Services', link: '/services/backend', activeMatch: '/services/' },
        { text: 'API', link: '/api/', activeMatch: '/api/' },
        { text: 'Deployment', link: '/deployment/', activeMatch: '/deployment/' },
        {
          text: 'Observability',
          link: '/observability/',
          activeMatch: '/observability/'
        },
        {
          text: 'Reference',
          activeMatch: '/(reference|ci-cd|contributing|documentation)/',
          items: [
            { text: 'Configuration', link: '/reference/configuration' },
            { text: 'Ports & storage', link: '/reference/ports-and-storage' },
            { text: 'Languages & images', link: '/reference/languages' },
            { text: 'Glossary', link: '/reference/glossary' },
            { text: 'CI/CD & releases', link: '/ci-cd' },
            { text: 'Contributing', link: '/contributing' },
            { text: 'Documentation site', link: '/documentation' }
          ]
        }
      ],

      sidebar: {
        '/guide/': guideSidebar,
        '/services/': servicesSidebar,
        '/api/': apiSidebar,
        '/deployment/': deploymentSidebar,
        '/observability/': observabilitySidebar,
        '/reference/': referenceSidebar,
        '/ci-cd': referenceSidebar,
        '/contributing': referenceSidebar,
        '/documentation': referenceSidebar
      },

      socialLinks: [{ icon: 'github', link: repo }],

      search: {
        provider: 'local',
        options: {
          translations: {
            button: {
              buttonText: 'Search docs',
              buttonAriaLabel: 'Search docs'
            },
            modal: {
              displayDetails: 'Display detailed list',
              resetButtonTitle: 'Reset search',
              backButtonTitle: 'Close search',
              noResultsText: 'No results for',
              footer: {
                selectText: 'to select',
                navigateText: 'to navigate',
                closeText: 'to close'
              }
            }
          }
        }
      },

      editLink: {
        pattern: `${repo}/edit/main/docs-site/docs/:path`,
        text: 'Edit this page on GitHub'
      },

      lastUpdated: {
        text: 'Last updated',
        formatOptions: { dateStyle: 'medium', timeStyle: 'short' }
      },

      outline: {
        level: [2, 3],
        label: 'On this page'
      },

      docFooter: {
        prev: 'Previous page',
        next: 'Next page'
      },

      footer: {
        message:
          'Released under the GNU Affero General Public License v3.0. Built with VitePress.',
        copyright: 'Copyright © RunCodes contributors'
      },

      returnToTopLabel: 'Return to top',
      sidebarMenuLabel: 'Menu',
      darkModeSwitchLabel: 'Theme',
      lightModeSwitchTitle: 'Switch to light theme',
      darkModeSwitchTitle: 'Switch to dark theme',
      externalLinkIcon: true
    }
  })
)
