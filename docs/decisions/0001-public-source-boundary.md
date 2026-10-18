# 0001: Publish a clean receive-only source snapshot

## Status
Accepted — 2026-10-18.

## Context
The application originated from an MIT upstream plus a private deployment. Copying deployment history, operational notes or built binaries could expose users' SMS and infrastructure. Users need a reproducible public setup, not a replica of the maintainer's household.

## Decision
Create an isolated repository with fresh Git history, retained MIT/upstream/vendor attribution and synthetic examples. Keep Go module/binary/service names stable for compatibility. Publish generic hardware, deployment, PWA and agent guides. The public installer uses an explicitly selected local build and modem node, loopback bootstrap and no automatic remote root download.

Keep receive-only QMI and existing authentication/encrypted push state. Browser asset updates are distinct from OS-owned installed metadata; do not claim forced icon replacement or native iOS SMS integration.

## Consequences
No private Git history is available here; consult upstream for its authorship history. The runtime deployment must generate its own credentials/keys and supply its own HTTPS/access control. Hardware/carrier and lock-screen delivery require the user's end-to-end acceptance tests. Optional health checks must be adapted to the actual reverse proxy.
