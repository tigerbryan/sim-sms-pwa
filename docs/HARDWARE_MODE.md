# Hardware mode

Hardware mode is optional, explicit, and SMS-only. It is designed for one QMI
control node, not a general modem-management shell.

The inherited advanced container tooling is not the default native installer. Its reviewed preflight design is:

1. Detects /dev/cdc-wdm* dynamically using sysfs and driver information.
2. Requires --device or a local choice when more than one candidate exists.
3. Checks the selected node with fuser and lsof when available; a busy node
   stops the install. The script never kills a process, stops an unknown
   service, or takes over another container.
4. Reads the current device major/minor at runtime. No hard-coded device number,
   full device major, wildcard, or broad /dev mapping is used.
5. Saves the exact existing ACL for the selected node, grants UID 65532 only
   rw access to that one node, verifies it, and stores a local restoration
   record. revoke-device-acl.sh and uninstall restore the saved ACL only after
   matching the same path and VID:PID.
6. Runs a non-root open/fstat/close permission probe in a temporary container.
   The probe sends no QMI request.
7. Starts the receive-only QMI backend. Its allocation order is DMS, UIM, NAS,
   read-only WDS status, and WMS. It has no data-session start capability.

The runtime receives one bind-mounted control node plus one exact
c major:minor rw cgroup rule. It runs as UID/GID 65532, with all capabilities
dropped, no added capability, a read-only root filesystem, no Docker socket,
no AT serial port, and bridge networking.

If any preflight, probe, Compose rendering, runtime inspect, or health check
fails, the newly created QMI Web container is stopped. The installer does not
reset USB mode, modify APN/profile state, change routes, or send/delete SMS.

Real SMS-only validation has passed on a Quectel-compatible QMI setup using
qmi_wwan and cdc-wdm. This is a compatibility data point, not a promise of
support for every device or carrier.
