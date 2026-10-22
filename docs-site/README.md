# RunCodes documentation site

This directory contains the RunCodes documentation website, built with
[VitePress](https://vitepress.dev/) and deployed to GitHub Pages.

**Live site:** <https://reisaraujo-miguel.github.io/runcodes/>

## Local development

Requires Node.js 20 or 22.

```bash
npm ci
npm run dev      # http://localhost:5173/runcodes/
npm run build    # static output in docs/.vitepress/dist
npm run preview  # preview the production build
```

To preview at the server root instead of the GitHub Pages sub-path:

```bash
DOCS_BASE=/ npm run dev
```

## Editing

- Content lives in `docs/`.
- Navigation and sidebars are defined in `docs/.vitepress/config.mts`.
- Branding and layout tweaks are in `docs/.vitepress/theme/styles/custom.css`.
- The logo is `docs/public/runcodes-icon.svg`.
- Diagrams use fenced `mermaid` blocks.
- Use relative `.md` links between pages; link to repository files with full
  GitHub URLs.

When adding a page, add it to the sidebar array for its section in the config
and link it from at least one other page.

## Deployment

`.github/workflows/docs.yml` builds the site and deploys it with the official
GitHub Pages actions. The workflow runs on pushes to `main` that touch this
directory and on manual dispatch; pull requests build without deploying.

The site is a project page served from `/runcodes/`, so `DOCS_BASE` is set to
`/runcodes/` in CI. For a custom domain, set `DOCS_BASE=/` and configure the
domain in the repository's Pages settings.

### One-time setup

In **Settings → Pages**, set **Source** to **GitHub Actions**. Then push to
`main` or run the **Docs** workflow manually.

## Content map

See `docs/reference/source-map.md` for the mapping from the repository's source
documents to pages on this site.
