# Deployment

The four-machine Ansible deployment lives in [`ansible/`](ansible/README.md).

It deploys the full RunCodes stack (frontend/backend, judge, PostgreSQL, Redis,
SeaweedFS) plus the complete observability stack with rootless Podman.

Start with [`ansible/README.md`](ansible/README.md). The topology and placement
rationale are in [`ansible/docs/ARCHITECTURE.md`](ansible/docs/ARCHITECTURE.md).
