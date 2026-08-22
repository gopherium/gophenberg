---
title: Install with Docker Compose
description: From an empty server to a running Gophenberg with a login.
---

This page takes you from an empty server to a running Gophenberg.
You need Docker with Compose, and a reverse proxy in front holding
your HTTPS certificate, such as Caddy, Traefik, or nginx.

:::caution[HTTPS is not optional]
The admin login cookie is marked secure, so browsers refuse to
store it over plain HTTP. Without HTTPS in front, nobody can log
in. The one exception is `localhost`, which browsers trust.
:::

## 1. The compose file

Create a directory on your server with this `compose.yaml`:

```yaml
services:
  db:
    image: postgres:18
    environment:
      POSTGRES_DB: gophenberg
      POSTGRES_PASSWORD: change-me
    volumes:
      - db-data:/var/lib/postgresql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 2s
      timeout: 2s
      retries: 15

  gophenberg:
    image: ghcr.io/gopherium/gophenberg:%VERSION%
    restart: unless-stopped
    environment:
      GOPHENBERG_DATABASE_URL: postgres://postgres:change-me@db:5432/gophenberg?sslmode=disable
      GOPHENBERG_SITE_TITLE: My Site
      GOPHENBERG_TRUSTED_PROXIES: 172.16.0.0/12
    volumes:
      - themes:/themes
      - media:/media
    ports:
      - "127.0.0.1:8081:8081"
    depends_on:
      db:
        condition: service_healthy

volumes:
  db-data:
  themes:
  media:
```

Three values to change:

- **The password**, in both places it appears.
- **The image tag.** Pin the newest version from the
  [tags page](https://github.com/gopherium/gophenberg/tags), never
  `latest`.
- **`GOPHENBERG_TRUSTED_PROXIES`**, the network your proxy
  connects from, in CIDR notation. The value above fits a proxy in
  Docker and a proxy on the host, which reaches the container
  through the same Docker bridge.

The healthcheck and the `condition` keep Gophenberg from starting
before the database is ready on first boot. The `themes` volume is
where themes you upload in the admin are kept, so it has to stay
writable.

## 2. Start it and create your login

```sh
docker compose up -d
docker compose run --rm -T gophenberg \
  createadmin -email admin@example.com -name "Maria Perez" -rank admin
```

Migrations run at startup, so there is no setup step.
`createadmin` waits for you to type the password, keeping it out
of your shell history. The `-rank` flag says what the account may do,
and `admin` is the rank that can reach everything.

Upgrading a site whose accounts were made before ranks existed needs
one extra command, `grantrank`, which
[Users and signing in](/guides/users/#giving-a-rank-to-accounts-that-hold-none)
covers. Until it runs, those accounts hold no rank and can do
nothing.

## 3. Point your proxy at it

Forward your domain to `127.0.0.1:8081`. With Caddy:

```text
example.com {
    reverse_proxy 127.0.0.1:8081
}
```

## 4. Check it works

- Your domain shows the public site.
- `curl -sI https://example.com | grep -i x-generator` answers
  with `Gophenberg %FEATURE_VERSION%`. The `-i` matters, HTTP/2
  lowercases header names.
- `/admin/` loads and your login works.

From here: [configuration](/self-hosting/configuration/) lists
every setting, and
[installing a theme](/themes/installing-a-theme/) changes how the
site looks.
