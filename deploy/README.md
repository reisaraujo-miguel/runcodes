# Deployment

Two deployment paths live in this directory:

- [`compose/`](compose/README.md) — a single host using the published GHCR
  images. Compose pulls the frontend, backend, judge and database images and
  never builds from source. It runs on Podman by default or Docker Engine with
  Compose v2; the judge pulls the runner images through a rootless Podman
  socket.
- [`ansible/`](ansible/README.md) — four machines with **rootless Podman**,
  published images and the full observability stack.

The repository root also has a `docker-compose.yml` that **builds from source**;
it is the development and demo stack, not the deployment path. It runs with
Podman Compose (default) or Docker Compose v2.

**Full documentation:**

- [Deployment overview](https://reisaraujo-miguel.github.io/runcodes/deployment/)
- [Image-based Compose](https://reisaraujo-miguel.github.io/runcodes/deployment/image-compose.html)
- [Ansible setup](https://reisaraujo-miguel.github.io/runcodes/deployment/ansible-setup.html)
- [Ansible operations](https://reisaraujo-miguel.github.io/runcodes/deployment/ansible-operations.html)
- [Security & scaling](https://reisaraujo-miguel.github.io/runcodes/deployment/security.html)
- [Four-machine observability](https://reisaraujo-miguel.github.io/runcodes/observability/production.html)
