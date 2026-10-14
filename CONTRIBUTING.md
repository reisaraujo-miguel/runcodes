# Contributing to **RunCodes**

Thank you for considering contributing to our project! Here's how you can help:

## Reporting Bugs or Suggesting Enhancements

- Please use the [GitHub Issues](https://github.com/reisaraujo-miguel/runcodes/issues) page to report bugs or suggest new features.

## Making Code Changes

- Fork the repository.
- Make your changes in your fork.
- Submit a pull request through GitHub.

Try to make your pull request descritive, indicating the changes you made and the reason for them. If the pull request is
related to an issue, please reference it in the description.

## Development setup

### Frontend

The frontend is a [Bun](https://bun.sh) project:

```sh
cd frontend
bun install
```

Installing also enables the repository's Git hooks (`.githooks/`). The
`pre-commit` hook runs `bun run check-svg` and refuses the commit when a source
SVG is not optimised; fix it by running `bun run optimize-svg` and staging the
result. To enable the hooks without installing dependencies, run:

```sh
git config core.hooksPath .githooks
```

## Questions?

- Feel free to ask in the comments of the relevant issue or pull request.

Thank you for your interest in **RunCodes**! Your contributions make this project better for everyone.
