# Validation — standalone source preparation

12 November 2026. All validation uses synthetic resources and localhost. No equipment or live cluster was contacted or modified.

| Check | Result |
|---|---|
| Go build with -trimpath | Passed |
| Go race tests across all packages | Passed |
| go vet | Passed |
| Frontend unit/theme tests | 20 tests passed across four files |
| TypeScript/Vite production build | Passed |
| Frontend formatting | Passed |
| Public-content source check | Passed |
| Public IronCore in a Box reference | Repository URL and public README verified |
| Direct Kubernetes reader | Fake-client/local HTTP regression tests passed |
| Live IronCore in a Box environment | Not launched or modified; integration helper implemented for an explicit existing context |
| Docker image build | Not rerun during this preparation; build context/manifests reviewed and single-service Compose supplied |

The browser workflow checks synthetic inventory, actual React Flow selection, Cell configuration, port summaries, association evidence, search, role filters, persisted Dark/Light themes, disabled observations and rejected write methods. Browser traces and test output are local ignored artifacts; any included screenshot is fixture-only. Final E2E: **passed**, one test / under 40 seconds using an isolated headless Chrome profile. Early runs exposed test-locator ambiguity and the original 30-second workflow budget; locators were corrected and the bounded budget increased to 60 seconds. No application failure was masked.

[Fixture-only screenshot](screenshots/fixture-overview.png)


## README demo verification

Reverified on 12 November 2026: Go race tests, go vet, native build, 20 frontend tests, TypeScript/Vite build and formatting checks passed. The final fixture-only browser workflow passed (39.7 seconds), and the opt-in README tour passed (31.6 seconds); both tests passed together. Both ran in isolated headless browser contexts; neither contacted a live environment.

The README GIF uses eight actual UI screenshots captured through validated topology selection, Cell/port/evidence inspection, search, role filtering and theme changes. Captions are added outside the UI. A static overview and regeneration instructions accompany it. Visual review discovered clipped topology cards when opening the inspector. Canvas-size and node-set changes now trigger a queued viewport fit, with browser bounding-box assertions covering the regression. The final rendered frames were reviewed: all cards fit, and only synthetic resource data appears. The GIF contains 24 frames at 1280 × 850 pixels and is approximately 2.7 MB; every frame decodes successfully. Live IronCore in a Box integration and a new Docker build were not exercised by this verification.
