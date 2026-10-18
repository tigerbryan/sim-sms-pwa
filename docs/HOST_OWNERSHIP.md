# Exclusive QMI host ownership

QMI Web hardware mode is receive-only SMS software, not a general modem
manager. The selected `/dev/cdc-wdmX` must have one owner for the entire
deployment and soak. A host process that only asks for read-only WDS status
still opens the same QMI control channel and is a conflict.

## Inspect first

```bash
sudo ./scripts/host/qmi-claim.sh status --device /dev/cdc-wdmX
sudo ./scripts/host/qmi-claim.sh observe --device /dev/cdc-wdmX --duration 300 \
  --log /var/log/qmi-web/ownership-preflight.log
```

The watcher uses `/proc`, `pgrep`, and `fuser`/`lsof` only. It never runs
`qmicli`, `uqmi`, `qmi-network`, `mbimcli`, or an AT command. It records the
PID, parent chain, executable, working directory, cgroup, and command line of
foreign processes without storing modem identifiers or SMS content.

## Explicit migration

The generic helper does not guess a service name or stop a product called
VoHive. Select only the units proven by host audit to open the target node:

```bash
sudo ./scripts/host/qmi-claim.sh isolate \
  --device /dev/cdc-wdmX \
  --state /var/lib/qmi-web/pre-qmiweb-cutover-state.json \
  --container <legacy-container> \
  --unit <conflicting-timer> \
  --unit <conflicting-service> \
  --modemmanager
```

`isolate` stops the explicitly named units, adds runtime-only systemd masks,
stops the explicitly named legacy container, and performs a short foreign
process/device-free check. It does not modify source files, disable unrelated
services, grant capabilities, or broaden the device mapping. Preserve the
state file even if a later gate fails.

After QMI Web is stopped and the device is released:

```bash
sudo ./scripts/host/qmi-release.sh \
  /var/lib/qmi-web/pre-qmiweb-cutover-state.json
```

This removes only the runtime masks recorded by `qmi-claim.sh` and restores the
captured enabled/active state and legacy container state. It is intentionally
not a blanket `enable --now` operation.

## Production gate

The pre-cutover observation and the complete soak must both report:

```text
FOREIGN_QMI_EXEC_COUNT=0
FOREIGN_MODEMMANAGER_COUNT=0
FOREIGN_TARGET_DEVICE_OPEN_COUNT=0
```

Any foreign QMI execution is an immediate production failure. Do not whitelist
`qmicli`, extend the soak, or claim success from a healthy HTTP endpoint.
