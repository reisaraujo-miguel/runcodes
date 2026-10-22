# Deployment

The four-machine Ansible deployment lives in [`ansible/`](ansible/README.md). It
deploys the frontend/backend, judge, PostgreSQL, Redis, SeaweedFS and the full
observability stack with **rootless Podman**.

**Full documentation:**

- [Deployment overview](https://reisaraujo-miguel.github.io/runcodes/deployment/)
- [Ansible setup](https://reisaraujo-miguel.github.io/runcodes/deployment/ansible-setup.html)
- [Ansible operations](https://reisaraujo-miguel.github.io/runcodes/deployment/ansible-operations.html)
- [Security & scaling](https://reisaraujo-miguel.github.io/runcodes/deployment/security.html)
- [Four-machine observability](https://reisaraujo-miguel.github.io/runcodes/observability/production.html)

For a single host, use [Docker Compose](https://reisaraujo-miguel.github.io/runcodes/deployment/compose.html).
