# Security model

QMI Web is constrained by both source-level API design and deployment policy.

## Application boundary

The modem abstraction exposes only identification, SIM/status/signal reads,
read-only packet-service status, inbound SMS listing/reading, and WMS event
subscription. There are no application methods for starting data sessions,
modifying APN/profile state, routing, DHCP, DNS, NAT, AT command execution,
SMS send/delete, SIM writes, modem resets, or USB mode changes.

## Runtime boundary

The container is non-root (65532:65532), read-only, and uses CapDrop=ALL with
no added capability. It has a small tmpfs, bridge network, no privileged mode,
no Docker socket, no broad device map, and no serial port.

No-device mode has no QMI node at all. Hardware mode carries one exact
bind-mounted cdc-wdm node and an exact current cgroup character-device rule.
The installer inspects the running container after creation and stops it if the
user, privilege, capability, mount, device-rule, or network policy differs.

## Data boundary

SQLite and the encryption key stay in the installation's persistent data
directory. They are ignored by Git and excluded from source and release assets.
Backups are local, permission-restricted, explicitly restored, and include a
consistent SQLite snapshot plus configuration and key material when present.
