#!/usr/bin/env python3
"""Create first-install credentials without overwriting an existing environment."""
import os
from pathlib import Path
import secrets


def main():
    destination = Path(__file__).resolve().parent / ".env"
    values = {
        "APP_PORT": "6666",
        "BIND_HOST": "0.0.0.0",
        "SUB2API_IMAGE": "sub2api-online:local",
        "ADMIN_EMAIL": "admin@sub2api.local",
        "ADMIN_PASSWORD": secrets.token_urlsafe(24),
        "POSTGRES_PASSWORD": secrets.token_hex(24),
        "REDIS_PASSWORD": secrets.token_hex(24),
        "JWT_SECRET": secrets.token_hex(32),
        "TOTP_ENCRYPTION_KEY": secrets.token_hex(32),
    }
    try:
        descriptor = os.open(destination, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    except FileExistsError:
        print("Existing .env preserved.")
        return
    with os.fdopen(descriptor, "w") as output:
        output.write("".join(f"{key}={value}\n" for key, value in values.items()))
    print("Created .env with unique credentials (mode 0600).")


if __name__ == "__main__":
    main()
