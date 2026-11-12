# Local release preparation

Prepared 12 November 2026 as an independent, credential-free source project. No remote repository was created and nothing was pushed or published.

## Distribution boundary

Share this repository or its source archive, not its parent workspace. The project contains application source, synthetic test fixtures, read-only deployment manifests, a public IronCore in a Box guide, CI checks and license material.

Environment-specific connectors, device targets, passwords, operational captures, historical environment reports/screenshots, binaries and generated browser traces are not part of the Git/source distribution. A recoverable local backup of the earlier experimental implementation is outside this project and must not be included when sharing a workspace.

Only fixture mode and direct, explicitly configured read-only Kubernetes API access are supported by the executable. There is no equipment backend, SSH reader, general command endpoint, telemetry collector or bundled operational dataset. Reserved observation components report no available data.

## Content checks

The source tree is checked by scripts/check-public-content.py for personal paths, internal domains, credential-bearing URLs, private-key markers, suspicious local files and globally routed address defaults. Known environment identifiers and recorded credentials were additionally checked locally without adding those identifiers to the public project's scanner rules or reports. Third-party dependency names and integrity hashes were treated as public dependency metadata, not edited for incidental short-string matches.

Git contains only the clean initial source revision. A neutral contributor identity is used for local commit metadata, and no remote is configured. Distribution archives are generated from committed paths rather than the entire working directory; dependencies, credentials, generated outputs and private backups are excluded.

## Validation scope

Tests use synthetic fixtures, fake Kubernetes clients/local HTTP servers and an isolated browser profile. No operational environment is contacted. The IronCore in a Box integration was checked against public documentation; no cluster was created, modified or deployed for this preparation. Operator/CRD availability depends on the user's explicit local environment and is checked by the read-only helper.

Exact executed results are recorded in docs/validation-results.md. Review future changes and locally supplied inventory before publishing new screenshots or fixtures; the application's live read model can legitimately contain names and configuration that are sensitive in a deployment.
