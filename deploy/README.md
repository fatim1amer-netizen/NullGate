# deploy/ — production operational extras

This directory holds deployment scaffolding for running NullGate in production.

| File | Purpose |
|------|---------|
| `nullgate.service` | systemd unit with a hardened sandbox (NoNewPrivileges, ProtectSystem=strict, private /tmp). |

## systemd quick start

```bash
# 1. Lay the binary and config down
sudo mkdir -p /opt/nullgate /var/log/nullgate
sudo cp nullgate /opt/nullgate/
sudo cp configs/config.yaml /opt/nullgate/

# 2. Create the unprivileged service user
sudo useradd --system --home /opt/nullgate --shell /usr/sbin/nologin nullgate
sudo chown -R nullgate:nullgate /opt/nullgate /var/log/nullgate

# 3. Install and start
sudo cp deploy/nullgate.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now nullgate

# 4. Verify
sudo systemctl status nullgate
journalctl -u nullgate -f
```

## Operational tips

- Set `logging.output: file` and point `file_path` under `/var/log/nullgate` (the
  only writable path in the unit).
- Put the admin dashboard on the loopback socket with a `password_hash`, or omit
  it entirely (`admin.enabled: false`) for pure edge deployments.
- Enable TLS at the WAF, or terminate at an upstream proxy and list that proxy in
  `trusted_proxies` so client-IP resolution stays spoof-proof.
