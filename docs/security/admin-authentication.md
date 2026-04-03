# Admin Authentication

You will learn how admin authentication works, why it defaults to fail-closed, and how to configure it safely.

## Required mode (default)

By default, admin authentication is enabled. The daemon expects HTTP Basic Auth credentials from environment variables:

```bash
export UPDATE_IPSETS_ADMIN_USER=admin
export UPDATE_IPSETS_ADMIN_PASSWORD=your-secret-password
```

Access the admin UI and API with those credentials:

```bash
curl -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18888/api/v1/admin/status
```

Browser access opens a standard Basic Auth prompt.

### Fail-closed behavior

If the credentials are missing or empty, admin access is denied entirely. The daemon does not fall back to open access. This prevents accidental exposure from:

- Forgetting to set the environment variables
- Setting only one of the two variables
- Setting an empty password

## Disabled mode

Disabling authentication requires **two** explicit flags:

```bash
--admin-auth-mode=disabled --allow-unauthenticated-admin
```

Both flags are required. Setting only `--admin-auth-mode=disabled` without `--allow-unauthenticated-admin` still blocks admin access.

### Why two flags?

A single typo in the configuration should not expose the admin surface. If `--admin-auth-mode=disabled` were sufficient on its own, a misspelled flag value or an unintended config deploy could open the admin UI to the world.

The second flag acts as an explicit acknowledgment: you understand that the admin surface is unprotected.

## Common mistake

Setting `--admin-auth-mode=disabled` without `--allow-unauthenticated-admin`:

```bash
# This does NOT open admin access
update-ipsets daemon --admin-auth-mode=disabled

# Admin returns 401 because the second flag is missing
```

You see 401 errors because the daemon treats this as a misconfiguration, not as an intentional choice.

## Bind address is not a security boundary

Running the admin listener on `127.0.0.1` (loopback only) does not make unauthenticated admin safe. The daemon does not infer safety from the bind address.

Reasons loopback is not sufficient:

- Other processes on the same host can reach it
- SSRF vulnerabilities in co-hosted applications can reach it
- DNS rebinding attacks can redirect browser requests to loopback

Use authentication even on loopback, unless you are in a controlled development environment.

## Configuring in systemd

Set credentials in a systemd drop-in:

```ini
# /etc/systemd/system/update-ipsets.service.d/credentials.conf
[Service]
Environment="UPDATE_IPSETS_ADMIN_USER=admin"
Environment="UPDATE_IPSETS_ADMIN_PASSWORD=your-secret-password"
```

Avoid putting credentials in the `ExecStart=` line — they would be visible in the process list.

After editing:

```bash
sudo systemctl daemon-reload
sudo systemctl restart update-ipsets
```

## Verifying auth works

Test with correct credentials:

```bash
curl -u "$UPDATE_IPSETS_ADMIN_USER:$UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/status
```

Should return JSON status.

Test with wrong credentials:

```bash
curl -u "$UPDATE_IPSETS_ADMIN_USER:$WRONG_UPDATE_IPSETS_ADMIN_PASSWORD" http://localhost:18889/api/v1/admin/status
```

Should return 401.
