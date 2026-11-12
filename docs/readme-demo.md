# README demo

The README animation is generated from actual fixture-mode browser interactions. It shows the logical topology, switch inspector, Cell configuration, local Interfaces, association evidence, search/role filters and light/dark themes. Captions are added outside the UI to explain each step. All displayed resource data is synthetic; operational observation collection is disabled.

The Playwright configuration starts its own loopback-only fixture server and never reuses a running server. Capture checks the API mode, number of example switches and visible fixture badge before saving frames. No live environment or credentials are involved.

## Regenerate

Build the frontend and backend, install the documentation-only browser/Pillow dependencies, then capture and render:

~~~sh
npm --prefix web run build
go build -trimpath -o bin/fabric-explorer ./cmd/fabric-explorer
cd web
npx playwright install chromium
CAPTURE_README_DEMO=1 npm run test:e2e
cd ..
python3 -m pip install Pillow
python3 scripts/render-readme-demo.py
python3 scripts/check-public-content.py
~~~

An optional PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH can point to an installed browser; Playwright still creates a new isolated profile. Screenshot frames, browser traces and the contact sheet stay in ignored web/test-results. Only docs/images/fabric-explorer-demo.gif and docs/images/fabric-explorer-overview.png are public README assets. Review their rendered content before distributing them.

The overview image is linked from the GIF as a static alternative. This is an annotated UI tour, not a recording of device telemetry or physical-link discovery.
