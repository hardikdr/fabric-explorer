# Contributing

Run make test, npm --prefix web run test:e2e and make build before proposing changes. Use synthetic fixtures and an isolated browser profile; tests must not modify clusters or contact equipment. Keep all application code and build context in this repository.

Do not commit credentials, captured operational inventory, private hostnames, deployment-specific addresses, personal paths or screenshots from a real environment. Review the content of generated artifacts as well as source files. Run scripts/check-public-content.py before creating a distribution archive.

Maintain clear desired/API/observed-state semantics and honest missing/stale diagnostics. Do not infer physical links from logical Clos layout or readiness from Cell phase. Public Kubernetes API schemas remain unchanged by this application.
