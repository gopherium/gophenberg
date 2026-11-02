---
title: Install with Docker Compose
description: From an empty server to a running Gophenberg with a login.
---

To install on a Linux server with one command instead, see
[Install on a Linux server](/self-hosting/server-install/).

This page takes you from an empty server to a running Gophenberg.
You need Docker with Compose, and a reverse proxy in front holding
your HTTPS certificate, such as Caddy, Traefik, or nginx.

:::caution[HTTPS is not optional]
The admin login cookie is marked secure, so browsers refuse to
store it over plain HTTP. Without HTTPS in front, nobody can log
in. The one exception is `localhost`, which browsers trust.
:::

:::note[PostgreSQL, not SQLite]
This page runs the site on PostgreSQL. Gophenberg can check and
migrate a SQLite file, but it cannot serve a site from one yet, see
[a SQLite database](/self-hosting/configuration/#a-sqlite-database).
:::

## 1. The compose file

Create a directory on your server with this `compose.yaml`:

```yaml
services:
  db:
    image: postgres:18
    environment:
      POSTGRES_DB: gophenberg
      POSTGRES_PASSWORD: 'change-me'
    volumes:
      - db-data:/var/lib/postgresql
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -h 127.0.0.1"]
      interval: 2s
      timeout: 2s
      retries: 15

  database-owner:
    image: ghcr.io/gopherium/gophenberg:%VERSION%
    command: ["database:create-owner", "-yes"]
    environment:
      GOPHENBERG_DATABASE_URL: postgres://gophenberg:change-me-too@db:5432/gophenberg?sslmode=disable
      GOPHENBERG_DATABASE_SUPERUSER_URL: postgres://postgres:change-me@db:5432/gophenberg?sslmode=disable
    depends_on:
      db:
        condition: service_healthy

  gophenberg:
    image: ghcr.io/gopherium/gophenberg:%VERSION%
    restart: unless-stopped
    stop_grace_period: 30s
    environment:
      GOPHENBERG_DATABASE_URL: postgres://gophenberg:change-me-too@db:5432/gophenberg?sslmode=disable
      GOPHENBERG_SITE_TITLE: My Site
      GOPHENBERG_TRUSTED_PROXIES: 172.16.0.0/12
      GOPHENBERG_PUBLIC_URL: https://example.com
    volumes:
      - themes:/themes
      - media:/media
    ports:
      - "127.0.0.1:8081:8081"
    depends_on:
      database-owner:
        condition: service_completed_successfully

volumes:
  db-data:
  themes:
  media:
```

Four values to change:

- **The two passwords.** `change-me` is the database superuser's, in
  both places it appears. `change-me-too` is the site's own, also in
  both places. Keep the `POSTGRES_PASSWORD` value in single quotes,
  so a password such as `0012345678` is not read as a number. A long
  password of only letters and digits is simplest, because it needs
  nothing more. Otherwise write a `$` as `$$` and a `'` as `''` in
  `POSTGRES_PASSWORD`. In the two addresses write every sign that is
  not a letter or a digit as its `%` code, such as `%40` for `@`,
  `%23` for `#`, `%3F` for `?`, `%2F` for `/`, `%25` for `%`, `%24`
  for `$` and `%20` for a space, see
  [the PostgreSQL address](/self-hosting/configuration/#the-postgresql-address).
- **The image tag**, in both places. Pin the newest version from the
  [tags page](https://github.com/gopherium/gophenberg/tags), never
  `latest`.
- **`GOPHENBERG_TRUSTED_PROXIES`**, the network your proxy
  connects from, in CIDR notation. The value above fits a proxy in
  Docker and a proxy on the host, which reaches the container
  through the same Docker bridge.
- **`GOPHENBERG_PUBLIC_URL`**, your own domain as people type it.
  Every write sent to another address is refused, see
  [the public address](/self-hosting/configuration/#the-public-address).

The official PostgreSQL image only offers a superuser, and a site must
not log in as one, because a superuser can run programs on the database
server. So the `database-owner` service runs once on every start. It
creates the role the site logs in as and gives it the database, then it
stops, and Gophenberg starts after it. Its second address is the only
place the site's setup needs the superuser, and running it again
changes nothing. See
[`database:create-owner`](/self-hosting/commands/#databasecreate-owner).

The healthcheck and the `condition` lines keep each service from
starting before the one it needs is ready. The `themes` volume is
where themes you upload in the admin are kept, so it has to stay
writable. The `stop_grace_period` gives a stop time to finish before
Docker kills the server, see
[stopping the server](/self-hosting/configuration/#stopping-the-server).

## 2. Start it and create your login

```sh
docker compose up -d
docker compose run --rm -T gophenberg \
  account:create-admin -email admin@example.com -name "Maria Perez" -role admin
```

Migrations run at startup, so there is no setup step.
`account:create-admin` prints `Password:` and reads the password
from its input, keeping it out of your shell history. Type it and
press Enter, or pipe it in, for example from a file only you can
read by adding `< password.txt` at the end of the command. The
`-role` flag says what the account may do, and takes `admin`,
`editor`, `author`, `member` or a role a plugin declares. Give the first
account `admin`, the role that can reach everything. Only a role
carrying `manage_users` can manage the other accounts, and `admin`
always carries it.

The image runs `serve` when it is given no command, which is what
`docker compose up` does. Any other command runs the way the one
above does, in a container of its own that `--rm` removes when the
command ends. For example, this lists every command:

```sh
docker compose run --rm -T gophenberg list
```

If your setup passes the image its own arguments, such as a
`command:` line in the compose file, name `serve` first. Run with
no command at all, Gophenberg only lists its commands and stops.
Every command and its flags are on
[the commands page](/self-hosting/commands/).

That is everything a new site needs. Upgrading a site that ran an
earlier version needs two manual steps first, renaming the role
column and giving a role to the accounts that hold none, both
covered by
[Users and signing in](/guides/users/#upgrading-a-site-that-ran-an-earlier-version).
Skipping the first one leaves a site that starts without complaint
and then refuses every login.

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

## Check a download

Every release also publishes the binary itself, packed for Linux on
`amd64` and `arm64` and for macOS on Apple chips, such as
`gophenberg_%VERSION%_linux_amd64.tar.gz`. An archive holds the
`gophenberg` binary with the admin inside, `release.json`, which
`gophenberg about -json` also prints, and the license files. Beside
each archive the release carries a `.sig` file with its signature, and
`SHA256SUMS` lists the SHA-256 of every archive.

Before you run a binary you downloaded, check who signed it. A release
is signed by one of two keys, the release key or the standby, a second
key that signs releases while the release key is being replaced. A file
signed by either one is genuine. Save the release key as
`release.pub.pem`:

```text
-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEANLMkB0RToFiEfIMw2VdG5Ml636MuXEwm4C+hGbbd5Mw=
-----END PUBLIC KEY-----
```

And save the standby as `standby.pub.pem`:

```text
-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAWrchu9WREyEOuKPjK7hoKSyEClGejpYjcaISqSJ1CPM=
-----END PUBLIC KEY-----
```

Then check the archive with OpenSSL alone. It needs OpenSSL 3, and the
last line prints `Signature Verified Successfully`. If it prints
`Signature Verification Failure` instead, run the last line again with
`standby.pub.pem` in place of `release.pub.pem`. A release the standby
signed passes that way:

```sh
printf '%s==' "$(tr '_-' '/+' < gophenberg_%VERSION%_linux_amd64.tar.gz.sig | tr -d '\n')" | openssl base64 -d -A > sig.bin
openssl dgst -sha384 -binary gophenberg_%VERSION%_linux_amd64.tar.gz > digest.bin
openssl pkeyutl -verify -pubin -inkey release.pub.pem -rawin -in digest.bin -sigfile sig.bin
```

Three other checks reach the same answer another way:

- `sha256sum -c SHA256SUMS --ignore-missing` compares each archive you
  downloaded with the list. Check `SHA256SUMS.sig` first, the same way
  as an archive, so you know the list itself came from the release.
- From a checkout of the repository,
  `go run ./cmd/releasetool verify -public z6MkhzygKXb6Xx7PB1NZsPhuNamiHiinZcaBijQZKMFYgWc3 gophenberg_%VERSION%_linux_amd64.tar.gz`
  checks the signature in one step. The long `z6Mk` text is the same
  release key, written as a Multikey. The standby written the same way
  is `z6MkkZNhcEbvKKuSsj3Ywz39iXYwCtxm5AVwxEzqjHJw35vJ`.
- `gh attestation verify gophenberg_%VERSION%_linux_amd64.tar.gz --repo gopherium/gophenberg --signer-workflow gopherium/gophenberg/.github/workflows/release.yml --source-ref refs/tags/v%VERSION%`
  checks that the release workflow built the file from the release tag.
