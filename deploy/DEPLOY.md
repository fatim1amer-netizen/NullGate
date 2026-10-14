# NullGate WAF — Cloud Deployment Guide

This guide walks through deploying NullGate WAF to a cloud VPS (DigitalOcean, AWS, Hetzner, Vultr, Linode) with automatic HTTPS via Let's Encrypt.

---

## Prerequisites

- A cloud VPS running Ubuntu 22.04+ or Debian 11+ (1 GB RAM minimum, 2 GB recommended)
- A domain name with DNS pointing to your server's IP
- SSH access to your server
- Port 80 and 443 open in your firewall

---

## Quick Start (5 minutes)

### 1. Point Your Domain

Add an **A record** for your domain pointing to your server's public IP:

```
Type: A
Name: @ (or your subdomain)
Value: YOUR_SERVER_IP
TTL: 300
```

Wait 2-5 minutes for DNS propagation. Verify with:

```bash
dig +short yourdomain.com
```

### 2. Install NullGate

SSH into your server and run:

```bash
# Install Go 1.23+
wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf go1.23.0.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc

# Clone and build NullGate
git clone https://github.com/YOUR_USERNAME/NullGate.git
cd NullGate
go build -o nullgate cmd/nullgate/main.go

# Create config directory
mkdir -p /etc/nullgate
cp configs/config.yaml /etc/nullgate/config.yaml
```

### 3. Configure for Your Backend

Edit `/etc/nullgate/config.yaml`:

```yaml
server:
  listen_addr: ":8080"          # Internal listen (behind auto-TLS)
  target_backend: "http://127.0.0.1:3000"  # Your app
  auto_tls: true
  auto_tls_domains:
    - "yourdomain.com"
    - "www.yourdomain.com"
  auto_tls_cache_dir: "/var/lib/nullgate/autocert"
  auto_tls_email: "admin@yourdomain.com"

admin:
  enabled: true
  listen_addr: "127.0.0.1:9091"
  username: "admin"
  password_hash: ""  # Generate below
  read_only: false
  session_ttl_min: 720  # 12 hours
```

Generate a secure admin password hash:

```bash
./nullgate --gen-secret  # Copy this value
# Then hash it (example):
echo -n "your-strong-password" | sha256sum
# Copy the hex output and prefix with "sha256:" in config:
# password_hash: "sha256:abc123..."
```

### 4. Create Systemd Service

Create `/etc/systemd/system/nullgate.service`:

```ini
[Unit]
Description=NullGate WAF
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=root
WorkingDirectory=/root/NullGate
ExecStart=/root/NullGate/nullgate --config /etc/nullgate/config.yaml
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/nullgate

[Install]
WantedBy=multi-user.target
```

### 5. Launch

```bash
# Create cache directory
sudo mkdir -p /var/lib/nullgate/autocert
sudo chmod 700 /var/lib/nullgate/autocert

# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable nullgate
sudo systemctl start nullgate

# Check status
sudo systemctl status nullgate
sudo journalctl -u nullgate -f
```

Within 30 seconds, Let's Encrypt will provision your certificate. Visit `https://yourdomain.com` — your site is now protected.

---

## Docker Deployment (Alternative)

### Using Docker Compose

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  nullgate:
    build: .
    container_name: nullgate
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./configs/config.yaml:/etc/nullgate/config.yaml:ro
      - autocert-cache:/var/lib/nullgate/autocert
    environment:
      - NULLGATE_CONFIG=/etc/nullgate/config.yaml
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE
    security_opt:
      - no-new-privileges:true

volumes:
  autocert-cache:
    driver: local
```

Create `Dockerfile`:

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o nullgate -ldflags="-w -s" cmd/nullgate/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/nullgate /app/nullgate
RUN mkdir -p /var/lib/nullgate/autocert && chmod 700 /var/lib/nullgate/autocert
EXPOSE 80 443
ENTRYPOINT ["/app/nullgate"]
CMD ["--config", "/etc/nullgate/config.yaml"]
```

Launch:

```bash
docker-compose up -d
docker-compose logs -f nullgate
```

---

## Admin Dashboard Access

The admin dashboard is **localhost-only** by default for security. To access it:

### Option 1: SSH Tunnel (Recommended)

From your local machine:

```bash
ssh -L 9091:127.0.0.1:9091 user@yourserver.com
```

Then open `http://localhost:9091` in your browser.

### Option 2: Expose with Caution

**⚠ Only do this with strong authentication enabled.**

Edit config:

```yaml
admin:
  enabled: true
  listen_addr: "0.0.0.0:9091"  # ⚠ Exposed to network
  require_localhost: false
  password_hash: "sha256:..."  # Must be set
```

Then access `https://yourdomain.com:9091`.

---

## Platform-Specific Notes

### DigitalOcean

- Droplets come with a firewall; ensure ports 80 and 443 are open.
- Use **1 GB RAM minimum** ($6/month tier).
- Attach a Volume for `/var/lib/nullgate` if you need persistent storage across rebuilds.

### AWS EC2

- Security Group: allow inbound TCP 80, 443, 22.
- Instance type: `t3.micro` or `t3.small`.
- Attach an Elastic IP so your IP doesn't change.
- IAM role: none required (NullGate is stateless).

### Hetzner

- CPX11 or CX21 instance types work well.
- Hetzner firewall: allow 80, 443, 22.
- Very cost-effective for EU traffic.

### Vultr / Linode

- Standard $5-6/month instances are sufficient.
- Configure firewall via their control panel or `ufw`:

```bash
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

---

## Monitoring and Logs

### View Live Logs

```bash
sudo journalctl -u nullgate -f
```

### Prometheus Metrics

NullGate exposes `/metrics` on the public listener. Scrape it with Prometheus:

```yaml
scrape_configs:
  - job_name: 'nullgate'
    static_configs:
      - targets: ['yourserver.com:443']
    scheme: https
```

### Health Check

```bash
curl http://127.0.0.1:9091/healthz
```

---

## Certificate Renewal

Let's Encrypt certificates auto-renew. NullGate checks every 24 hours and renews when <30 days remain. No manual action needed.

To force re-provisioning (e.g., after adding a domain):

```bash
sudo rm -rf /var/lib/nullgate/autocert/*
sudo systemctl restart nullgate
```

---

## Troubleshooting

### Certificate Provisioning Fails

**Symptom:** `acme: authorization error` in logs.

**Fix:**
1. Verify DNS points to your server: `dig +short yourdomain.com`
2. Ensure port 80 is reachable: `curl -I http://yourdomain.com`
3. Check firewall: `sudo ufw status`
4. Let's Encrypt rate limit: 5 failures per hour per domain. Wait 1 hour and retry.

### Port 80/443 Already in Use

**Symptom:** `bind: address already in use`

**Fix:**
```bash
sudo lsof -i :80
sudo lsof -i :443
# Stop conflicting service (nginx, apache, etc.)
sudo systemctl stop nginx
sudo systemctl disable nginx
```

### High Memory Usage

**Symptom:** Server becomes unresponsive.

**Fix:**
- Lower `decoding.max_payload_size` in config (default 10 MB).
- Lower `decoding.max_request_size` (default 32 MB).
- Add swap:

```bash
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
echo '/swapfile none swap sw 0 0' | sudo tee -a /etc/fstab
```

### Dashboard Won't Load

**Symptom:** Connection refused on `:9091`.

**Fix:**
1. Verify service is running: `sudo systemctl status nullgate`
2. Check SSH tunnel is active.
3. Ensure `admin.enabled: true` in config.

---

## Security Hardening (Production)

### 1. Run as Non-Root User

```bash
sudo useradd -r -s /bin/false nullgate
sudo chown -R nullgate:nullgate /var/lib/nullgate
```

Update systemd service:

```ini
[Service]
User=nullgate
Group=nullgate
```

### 2. Enable Fail2Ban for SSH

```bash
sudo apt install fail2ban
sudo systemctl enable fail2ban
```

### 3. Firewall Rules

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
```

### 4. Rotate Admin Password

Generate new hash monthly:

```bash
echo -n "new-password" | sha256sum
# Update config and restart
sudo systemctl restart nullgate
```

---

## Scaling and Performance

### Single Server Limits

- **1 GB RAM:** ~500 req/s
- **2 GB RAM:** ~2,000 req/s
- **4 GB RAM:** ~5,000 req/s

### Horizontal Scaling

Deploy multiple NullGate instances behind a load balancer (HAProxy, AWS ALB, DigitalOcean LB):

```
                     ┌─> NullGate 1 ─> Backend
Internet -> LB ─────┼─> NullGate 2 ─> Backend
                     └─> NullGate 3 ─> Backend
```

Each instance runs independently; no shared state.

---

## Updating NullGate

```bash
cd ~/NullGate
git pull origin main
go build -o nullgate cmd/nullgate/main.go
sudo systemctl restart nullgate
```

---

## Support

- GitHub Issues: https://github.com/YOUR_USERNAME/NullGate/issues
- Self-Test: `./nullgate --selftest`
- Config Validation: `./nullgate --show-config`

---

**Deploy with confidence. Your site is now protected by NullGate WAF with automatic HTTPS.**
