# Contributing

Use a topic branch and pull request. Keep receive-only behavior, existing session/CSRF protections, push subscription identity and encrypted state intact. Do not add modem writes, SMS send/delete, APN/data dialing, routing/NAT or automatic firmware flashing.

Before submitting: `make build`, `go test -mod=vendor -buildvcs=false ./...`, `go vet -mod=vendor -buildvcs=false ./...`, frontend tests, Python ops tests, `scripts/verify-security.sh`, and `python3 scripts/check-public-tree.py`. Use fictional UI data only. Dependency upgrades must retain LICENSE notices and update the lockfile/vendor inventory deliberately.

Hardware support requires evidence for the exact host, modem variant, driver, firmware and carrier, with identifiers redacted. A mock pass or one QMI query is not an end-to-end SMS test. Never attach a real SMS database or raw production log to an issue.
