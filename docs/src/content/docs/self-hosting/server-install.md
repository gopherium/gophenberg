---
title: Install on a Linux server
description: One command installs Gophenberg with PostgreSQL, Caddy and HTTPS on a fresh server.
---

This page installs Gophenberg on a Linux server with one command. It
sets up PostgreSQL, Caddy for HTTPS, and a systemd service that runs
the site and takes back an update that fails. To run Gophenberg in
containers instead, see
[install with Docker Compose](/self-hosting/install/).

## What you need

This section lists what the server needs before you start.

- A fresh server running Ubuntu 24.04 or newer, or Debian 12 or newer.
  Gophenberg's CI tests Ubuntu 24.04 and Ubuntu 26.04, and on any other
  release the run prints that the CI does not test it yet. On an older
  release or another system,
  [install with Docker Compose](/self-hosting/install/) instead.
- An `x86_64` or `aarch64` processor.
- At least 1 GB of memory and 1 GB of free disk.
- A domain whose A or AAAA record points at the server.
- TCP 80, TCP 443 and UDP 443 open in the hosting provider's firewall.
- IPv4. The release downloads from GitHub, which a server with IPv6
  alone cannot reach.

Fresh means no PostgreSQL, Caddy or other web server yet. A server
that already runs a web server for other sites can keep it, see
[with your own web server](#with-your-own-web-server). Every check is
listed under [the checks](/self-hosting/commands/#the-checks).

## Install

This section runs the install. Run this line on the server, with your
own domain in place of `example.com`:

```sh
curl -fsSL https://updates.gophenberg.org/install.sh | sudo sh -s -- --domain example.com
```

`curl` fetches `install.sh`, and `sudo sh` runs it as root. Everything
after `--` goes to `install.sh`, which passes it on to
[`server:install`](/self-hosting/commands/#serverinstall), where every
flag is listed. A flag takes one dash or two, so `-domain` works too.

`install.sh` checks the server and the release, then hands over to
`server:install`. That asks on the terminal for the first
administrator's email, name and password, see
[the first administrator](/self-hosting/commands/#the-first-administrator).
It ends with a summary that names the site's address and the commands
to know, see [taking the steps](/self-hosting/commands/#taking-the-steps).

To see what it would do first, add `--dry-run`. A dry run downloads and
checks the release too, runs every check, then prints each step and
every file it would write. It changes nothing on the server but
`/var/lib/gophenberg-installer`, a folder it makes when it is missing.

A script has no terminal to answer on, so give it the three answers as
flags. The password file holds the password on its first line, and
its path must be absolute:

```sh
curl -fsSL https://updates.gophenberg.org/install.sh | sudo sh -s -- --domain example.com \
  --admin-email admin@example.com --admin-name "Maria Perez" --admin-password-file /root/admin-password
```

`sudo` leaves out your own environment, so give a setting on the first
run through `env`.
[The server settings](/self-hosting/configuration/#the-server-settings)
lists each one:

```sh
curl -fsSL https://updates.gophenberg.org/install.sh | sudo env GOPHENBERG_SERVICE_RESTART_DELAY=3s sh -s -- --domain example.com
```

## What install.sh does

This section lists what `install.sh` checks before it hands over, in
order. The script at `https://updates.gophenberg.org/install.sh` names
one release, the newest stable one, and the SHA-256 of its two Linux
archives. Each stable release writes these into the script and signs
it.

1. It arrived whole. All its work runs inside one function that the
   last line calls, so a download cut off part way runs nothing. A copy
   cut off between the name of that function and its arguments stops
   with a line of its own.
2. It runs as root.
3. It names a release. The copy in the repository names none.
4. You left out `-archive`, since it passes the archive it checked
   itself.
5. The processor is `x86_64` or `aarch64`.
6. `curl`, `openssl` and `tar` are there. It also looks in `/usr/sbin`
   and `/sbin`, which `su` without a dash leaves out of `PATH`.
7. OpenSSL is version 3 or newer, which the signature check needs.
8. `/var/lib/gophenberg-installer` is a real folder, not a link, owned
   by root and writable by root alone. It makes the folder with mode
   0700 when it is missing. `server:install` keeps its install record,
   `install.json`, in the same folder.
9. It makes a download folder in it, named `download.` and ten random
   characters. It downloads the archive and its `.sig` there from
   `https://github.com/gopherium/gophenberg/releases/download/`, over
   HTTPS alone, redirects included, with TLS 1.2 or newer. It gives up
   on a download that sends nothing for 60 seconds.
10. The archive's SHA-256 is the one the script names for this
    processor. So an older signed archive put up under a newer name
    stops too, which a signature alone would let through.
11. The release key or the standby signed the archive. These are the
    keys of [check a download](/self-hosting/install/#check-a-download),
    and it checks them with OpenSSL alone.

Along the way it prints two lines of its own:

```text
Downloading Gophenberg %VERSION% for linux/amd64
The archive matches its checksum and a Gophenberg release key signed it
```

Then it unpacks only `gophenberg` from the archive into the download
folder and moves to `/`, where no `.env` file is near. It runs that
binary's `server:install` with `-archive` and your flags, and standard
input from `/dev/null`. The questions use the terminal itself, so they
still reach you.

`install.sh` ends with the exit code of `server:install`. A stop ends
it with code 1, a hangup with 129, Ctrl-C with 130 and `SIGTERM` with
143, and each removes the download folder.

It checks only what it needs to download and run the binary.
`server:install` checks the rest, such as the system, memory, ports and
the domain, in Go, where tests reach every case.

## Check install.sh before you run it

This section checks the script before root runs it. Download the
script and its signature:

```sh
curl -fsSLO https://updates.gophenberg.org/install.sh
curl -fsSLO https://updates.gophenberg.org/install.sh.sig
```

Check them as [check a download](/self-hosting/install/#check-a-download)
shows, with the same two keys and `install.sh` in place of the
archive's name:

```sh
printf '%s==' "$(tr '_-' '/+' < install.sh.sig | tr -d '\n')" | openssl base64 -d -A > sig.bin
openssl dgst -sha384 -binary install.sh > digest.bin
openssl pkeyutl -verify -pubin -inkey release.pub.pem -rawin -in digest.bin -sigfile sig.bin
```

Right after a release, the update site may serve the new
`install.sh.sig` beside the old `install.sh`, or the other way round,
for up to ten minutes. When the check fails then, wait a few minutes
and download both files again.

Once the last line prints `Signature Verified Successfully`, read the
script if you like, then run it:

```sh
sudo sh install.sh --domain example.com
```

## What server:install does

This section sums up the install itself. `server:install` checks the
server first and changes nothing until every check passes. Then it
installs PostgreSQL and Caddy, makes the system user `gophenberg` and
the install root `/opt/gophenberg`, and unpacks the build there. It
writes the site's settings, two systemd units, the `gophenberg`
command and Caddy's site. It starts the site, creates the first
administrator and checks that HTTPS answers.

As it goes, the run prints each step, and under it each program the
step runs. Installing the packages can take a few minutes, and on a
fresh server `apt-get` may first wait for the automatic updates to
finish. So when a program runs longer than 30 seconds, the run prints
a line every 30 seconds with the time so far and the last line the
program printed:

```text
Step 2: install PostgreSQL and Caddy with apt-get, Caddy from its own apt repository
  dpkg-query -W postgresql caddy
  apt-get update
  apt-get install postgresql caddy
  still running apt-get install postgresql caddy after 30s: Waiting for cache lock: Could not get lock /var/lib/dpkg/lock-frontend. It is held by process 1234 (unattended-upgr)...
  still running apt-get install postgresql caddy after 1m0s: Setting up postgresql-16 (16.10-0ubuntu0.24.04.1) ...
```

A program's own output shows in full only when it fails.
While the run waits for the site to start or for HTTPS to answer, it
prints the same kind of line, such as
`still waiting for https://example.com/admin/ after 30s`, with why the
last try failed.
`GOPHENBERG_INSTALL_PROGRESS_INTERVAL` changes the 30 seconds, see
[the server settings](/self-hosting/configuration/#the-server-settings).

[What a dry run prints](/self-hosting/commands/#what-a-dry-run-prints)
lists every step and every file with its owner and mode. Each step is
marked in the install record,
`/var/lib/gophenberg-installer/install.json`. So a run cut off part
way picks up where it stopped when you run the same line again, see
[the install record and later runs](/self-hosting/commands/#the-install-record-and-later-runs).

## Change a setting

This section changes a setting after the install. The server's
settings live in `/etc/gophenberg/server.env`. Change a line there,
then run the install again:

```sh
sudo gophenberg server:install
```

It needs no flag and asks nothing. It writes the units, the site's
`.env` and Caddy's site again, then restarts the site.
[The server.env file](/self-hosting/configuration/#the-serverenv-file)
shows the file, and
[the site's .env on a server install](/self-hosting/configuration/#the-sites-env-on-a-server-install)
says which lines of `/opt/gophenberg/.env` are yours to change.

## What root runs and reads

This section explains why root trusts nothing in the install root. The
site runs as the user `gophenberg`, which owns `/opt/gophenberg`, since
updates write there. So a hole in the site, or in a theme's Node.js
process, could change any file there. Root follows five rules, so that
such a change never runs as root:

1. Root never runs a file from the install root. `install.sh` runs the
   binary it checked, from its own download folder. Later, the
   `gophenberg` command runs `server:install` and `server:uninstall`
   from the installer's own copy, `/usr/local/lib/gophenberg/gophenberg`,
   which root owns. Every other command runs as `gophenberg`.
2. Root never takes its settings from a `.env` file. They come from
   `/etc/gophenberg/server.env` and the install record, which only root
   can read. It reads the site's `.env` only for the few values
   [the site's .env on a server install](/self-hosting/configuration/#the-sites-env-on-a-server-install)
   names.
3. Every program root starts gets an environment of its own with a
   fixed `PATH`, so nothing from your shell, such as `LD_PRELOAD`,
   reaches `apt-get` or `systemctl`.
4. Root changes the install root only through one handle that refuses
   any link leading out of it. So a link the site plants cannot turn a
   write into one under `/etc`.
5. Root checks what it unpacks. `install.sh` checks the archive's
   checksum and signature. `server:install` takes an archive only from
   `/var/lib/gophenberg-installer`, and its `release.json` must name the
   binary's own version and this server's platform.

## The gophenberg command

This section explains the command the install puts on the path,
`/usr/local/bin/gophenberg`. Run it with `sudo`. Without it, it stops:

```text
gophenberg: run it with sudo, such as sudo gophenberg update:status
```

It runs `server:install` and `server:uninstall` as root with the
installer's own copy, and every other command as the user `gophenberg`
in `/opt/gophenberg`, see
[the install record and later runs](/self-hosting/commands/#the-install-record-and-later-runs).

`update:apply` and `update:rollback` end with exit code 75 once they
switch builds, which asks for a restart. The `gophenberg` command then
restarts the site and prints:

```text
gophenberg: restarted the site to start the new build
```

Before it restarts, it clears the site's failed state, so restarts you
made earlier never count against [the start limit](#the-start-limit).

## The logs

This section says where to read what happened. The site, its rollback
unit and Caddy each log to the systemd journal:

```sh
sudo journalctl -u gophenberg
sudo journalctl -u gophenberg-rollback
sudo journalctl -u caddy
```

Add `-f` to follow new lines as they come, or `-b` for this boot alone.
`sudo systemctl status gophenberg` shows whether the site runs, with
its last lines.

## The site's unit

This section explains `/etc/systemd/system/gophenberg.service`, the
unit that runs the site. On Ubuntu 24.04 with the default settings it
reads:

```ini
# Written by gophenberg server:install.
[Unit]
Description=Gophenberg
Documentation=https://docs.gophenberg.org/self-hosting/server-install/
After=network-online.target postgresql.service
Wants=network-online.target
StartLimitIntervalSec=300
StartLimitBurst=10
OnFailure=gophenberg-rollback.service

[Service]
Type=exec
User=gophenberg
Group=gophenberg
WorkingDirectory=/opt/gophenberg
ExecStart=/opt/gophenberg/current/gophenberg serve
Restart=on-failure
RestartMode=direct
RestartSec=5s
SuccessExitStatus=75
RestartForceExitStatus=75
RestartPreventExitStatus=76
TimeoutStopSec=30s
KillMode=control-group
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/opt/gophenberg
PrivateTmp=yes

[Install]
WantedBy=multi-user.target
```

- `Description` and `Documentation` are what
  `systemctl status gophenberg` shows, with this page.
- `After` starts the site after the network and PostgreSQL at boot.
  PostgreSQL may still not answer then, so the site waits for it, see
  [a database that starts late](#a-database-that-starts-late).
- `Wants` makes systemd wait for the network at all. There is no
  `Wants=postgresql.service`, so a PostgreSQL you stopped by hand stays
  stopped when the site starts.
- `StartLimitIntervalSec` and `StartLimitBurst` come from
  `GOPHENBERG_SERVICE_START_LIMIT_INTERVAL` and
  `GOPHENBERG_SERVICE_START_LIMIT_BURST`, see
  [the start limit](#the-start-limit).
- `OnFailure` starts [the rollback unit](#the-rollback-unit) once the
  site failed.
- `Type=exec` counts a start as failed when systemd cannot run the
  binary at all, such as a `current` link to a missing build.
- `User` and `Group` run the site as `gophenberg`, which signs in to
  PostgreSQL by its user name, with no password.
- `WorkingDirectory` is the install root, where the site reads its
  `.env`.
- `ExecStart` always runs the build `current` names, so an update only
  moves that link.
- `Restart=on-failure` starts the site again after a crash or an error
  exit, but not after a clean stop.
- `RestartMode=direct` restarts the site without marking it failed in
  between, so the rollback unit runs only once restarts stop. The
  install writes it only on systemd 254 or newer, which knows it.
- `RestartSec` is how long systemd waits before a restart, from
  `GOPHENBERG_SERVICE_RESTART_DELAY`.
- `SuccessExitStatus=75` counts exit code 75, the restart an update
  asks for, as a clean exit, so it never starts the rollback unit.
- `RestartForceExitStatus=75` still restarts the site after 75, which
  starts the new build.
- `RestartPreventExitStatus=76` restarts nothing after exit code 76, a
  new build that failed its start check, so the rollback unit takes
  over.
- `TimeoutStopSec` is how long a stop may take before systemd kills
  the site. It is the five graces of
  [stopping the server](/self-hosting/configuration/#stopping-the-server),
  read from the site's `.env`, plus `GOPHENBERG_SERVICE_STOP_MARGIN`.
- `KillMode=control-group` also stops a theme's Node.js process when a
  stop runs out of time.
- `NoNewPrivileges`, `ProtectSystem=strict` and `ReadWritePaths` let
  the site write under the install root alone, and nothing it runs
  gains rights.
- `PrivateTmp=yes` gives the site a temporary folder of its own, which
  large uploads need under `ProtectSystem=strict`.
- `WantedBy=multi-user.target` starts the site at boot.

## The rollback unit

This section explains `/etc/systemd/system/gophenberg-rollback.service`,
the unit that takes back an update whose new build fails to start. It
reads:

```ini
# Written by gophenberg server:install.
[Unit]
Description=Roll Gophenberg back after a failed update
Documentation=https://docs.gophenberg.org/self-hosting/updates-and-backups/#a-version-that-fails-its-check
ConditionPathExists=/opt/gophenberg/previous
StartLimitIntervalSec=0

[Service]
Type=oneshot
User=gophenberg
Group=gophenberg
WorkingDirectory=/opt/gophenberg
ExecCondition=+/usr/bin/systemctl is-failed --quiet gophenberg.service
ExecStart=/opt/gophenberg/previous/gophenberg update:rollback -after-failure
ExecStartPost=+/usr/bin/systemctl reset-failed gophenberg.service
ExecStartPost=+/usr/bin/systemctl start gophenberg.service
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/opt/gophenberg
PrivateTmp=yes
```

- `Documentation` names the page on a version that fails its check.
- `ConditionPathExists` runs it only once an update left a `previous`
  build to go back to. A fresh install has none.
- `StartLimitIntervalSec=0` gives it no start limit, so systemd never
  refuses the start that rolls back.
- `Type=oneshot` runs it once, to its end.
- `ExecCondition` goes on only when the site failed for good, not while
  systemd still restarts it. Without `RestartMode=direct`, systemd may
  start this unit after every failed start, and this line skips those.
- `ExecStart` runs `update:rollback -after-failure` with the previous
  build. It takes the update back for any result but success, see
  [update:rollback](/self-hosting/commands/#updaterollback).
- The two `ExecStartPost` lines run only when it took the update back.
  The `+` gives them root's rights, so they clear the site's failed
  state and start it again.
- `User`, `Group`, `WorkingDirectory` and the last four lines are as in
  the site's unit. There is no `[Install]` section, since only
  `OnFailure` starts it.

## The start limit

This section explains when systemd stops restarting the site. systemd
counts the site's starts. Once `StartLimitBurst` starts, 10, fall
within `StartLimitIntervalSec`, 300 seconds, it stops. The site has
failed for good, and the rollback unit runs.

A build that crashes at once restarts every `RestartSec`, 5 seconds,
so ten starts take about 50 seconds and it rolls back fast. A start
that waits the whole `GOPHENBERG_DATABASE_WAIT`, 60 seconds, takes
about 65 seconds with the restart delay. So at most five such starts
fit in 300 seconds. A database that is down at boot never hits the
limit, and the site comes back on its own once PostgreSQL answers.

When you change these settings, keep this rule. The database wait plus
the restart delay, times one less than the burst, must stay above the
interval. With the defaults, 65 seconds times 9 is 585 seconds, above
300.

## A database that starts late

At boot, systemd may start the site before PostgreSQL answers. The
site then waits for the database, see
[a database that starts late](/self-hosting/configuration/#a-database-that-starts-late).
Right after an update, each wait costs the new build one of its tries,
see
[a database that starts late after an update](/self-hosting/updates-and-backups/#a-database-that-starts-late-after-an-update).

## Who can reach the site's port

This section explains who can reach the site without Caddy. The site
listens on `127.0.0.1:8081`, from `GOPHENBERG_ADDR`, and Caddy
forwards visitors there. Nobody outside the server reaches that port.
Any account on the server itself can, though.

The site believes the `X-Forwarded-For` header from loopback, see
[trusted proxies](/self-hosting/configuration/#trusted-proxies). So an
account on the server can pose as any visitor, such as to get around
the login rate limiter. On a fresh server the only accounts are yours
and system users such as `postgres` and `caddy`. Give nobody else an
account on the server.

Caddy's own admin endpoint listens on `/run/caddy/admin.sock`, a
socket that only Caddy and root can open, so no other account can
change Caddy's settings.

## Fixes to the units and to Caddy

This section says how fixes reach what the install set up.
`update:apply` updates the build but never the units or Caddy's site,
since it runs as `gophenberg`. `sudo gophenberg server:install` writes
them again, but with the installer's own copy, which an update leaves
as it is. So a newer release's fixes to them reach the server only when
you run the install line again:

```sh
curl -fsSL https://updates.gophenberg.org/install.sh | sudo sh
```

It needs no flag, since `server.env` keeps the settings. It writes the
units and Caddy's site again, replaces the installer's own copy, and
keeps the build `current` names.

Caddy and PostgreSQL get their updates from apt, see
[keeping the server up to date](#keeping-the-server-up-to-date).

## Keeping the server up to date

This section says what keeps the rest of the server up to date, and
what you do by hand. Gophenberg updates itself from the Updates
screen, see
[updating a server install](/self-hosting/updates-and-backups/#updating-a-server-install).
PostgreSQL, Caddy and the system come from apt, the system's package
tool.

### What updates itself

Ubuntu servers and most Debian cloud images come with
`unattended-upgrades` switched on, and the check below shows whether
yours does. Once a day it installs, without asking:

- The system's security fixes, PostgreSQL's among them. Debian also
  installs its point releases. Other PostgreSQL updates wait for
  `sudo apt update` and then `sudo apt upgrade`.
- Every new Caddy release, from Caddy's own repository. The install
  adds that repository in `/etc/apt/apt.conf.d/52gophenberg-caddy`,
  but only when `unattended-upgrades` was installed before the install
  line ran. If you install it later, run the install line again.

An update restarts what it updates. PostgreSQL stops for a few
seconds, so a page asked for then may fail. Caddy restarts too, so
HTTPS stops for a moment.

To check that it is on, read this file. Both of its lines must hold
`"1"`:

```sh
cat /etc/apt/apt.conf.d/20auto-upgrades
```

If the file is missing or a line holds `"0"`, turn it on, answering
yes to its question, then run the install line again so it adds
Caddy's repository:

```sh
sudo apt install unattended-upgrades
sudo dpkg-reconfigure -plow unattended-upgrades
```

`/var/log/unattended-upgrades/unattended-upgrades.log` lists what it
installed. `apt-cache policy caddy` shows the Caddy you have. Read the
line with `dl.cloudsmith.io`, which is Caddy's own repository. The
older Caddy of Ubuntu's own archive shows too, and never wins.

### Restarting the server

Some fixes, such as a new kernel, only take effect after a restart,
and `unattended-upgrades` never restarts the server by itself. When
one is due, the file `/var/run/reboot-required` exists, and on Ubuntu
signing in also prints `*** System restart required ***`. On Debian,
`unattended-upgrades` writes that file when it installs a new kernel,
so it shows only while that package is installed. To check:

```sh
ls /var/run/reboot-required
```

`No such file or directory` means no restart is due. When one is,
restart at a quiet time:

```sh
sudo reboot
```

The site, PostgreSQL and Caddy start again on their own.

### A new Ubuntu or Debian release

Ubuntu 24.04 gets security fixes until 2029, so moving to a new
release is never urgent. When you move, such as from Ubuntu 24.04 to
26.04, three things need your hands: a backup first, PostgreSQL's new
major version, and Caddy's repository.

1. Stop the site, then take a backup of the database:

   ```sh
   sudo systemctl stop gophenberg
   sudo -u gophenberg pg_dump -Fc \
     -f /opt/gophenberg/backups/before-release-upgrade.dump gophenberg
   ```

   Use the install root you chose in place of `/opt/gophenberg`. An
   update never removes a backup with this name, and
   [restoring on a server install](/self-hosting/updates-and-backups/#restoring-on-a-server-install)
   puts one back.

2. Upgrade the system, on Ubuntu with:

   ```sh
   sudo do-release-upgrade
   ```

   It asks two questions about what the install set up. It lists
   `caddy` among packages from outside Ubuntu and asks whether to go
   on. Answer yes. It asks whether to upgrade the PostgreSQL cluster
   16/main to 18. Answer no, and move it in step 3. It also turns
   Caddy's repository off, by adding `Enabled: no` to
   `/etc/apt/sources.list.d/caddy.sources`. At the end it asks to
   restart the server. Answer yes, and the site starts again with it.

3. Move PostgreSQL to the new major. Ubuntu 26.04 ships PostgreSQL 18,
   and 16 gets no more fixes there, so do this soon. First see which
   cluster holds the site's data:

   ```sh
   pg_lsclusters
   ```

   The cluster online on port 5432 is the one the site uses. Never
   remove a cluster that is online on 5432.

   - If `16 main` is online on 5432, move it:

     ```sh
     sudo systemctl stop gophenberg
     sudo pg_upgradecluster 16 main
     pg_lsclusters
     ```

     `pg_upgradecluster` copies the data into a new 18 cluster, which
     takes port 5432, and leaves the old one stopped on another port.
     It needs free disk for a second copy of the database. Stop the
     site before it, never PostgreSQL. Keep its default way of
     copying, since `--method=upgrade` and `--method=link` can leave
     indexes that need rebuilding after a new system release. The
     second `pg_lsclusters` shows `18 main` online on 5432 and
     `16 main` down.

     If the first `pg_lsclusters` also listed an `18 main` on another
     port, `pg_upgradecluster` stops, since that name is taken. While
     `16 main` is the one on 5432, that `18 main` holds none of the
     site's data, and `sudo pg_dropcluster --stop 18 main` removes it.

   - If `18 main` is already online on 5432 and `16 main` is down,
     the upgrade moved the data when its question was answered yes.
     Skip the move.

   Then start the site and check it:

   ```sh
   sudo systemctl start gophenberg
   ```

   Once the site works, remove the old cluster and its packages:

   ```sh
   sudo pg_dropcluster 16 main
   sudo apt purge postgresql-16 postgresql-client-16
   ```

4. Turn Caddy's repository back on with the install line, and let apt
   read it again:

   ```sh
   curl -fsSL https://updates.gophenberg.org/install.sh | sudo sh
   sudo apt update
   ```

   The install line writes Caddy's source again without
   `Enabled: no`, and writes the units for the new release. Use the
   install line rather than `sudo gophenberg server:install`, whose
   copy of the installer may be older than the first release that
   knows the new system.

On Debian, take step 1, then follow Debian's release notes to upgrade
the system. When apt asks whether to upgrade the PostgreSQL cluster
15/main to 17, answer no. Then take steps 3 and 4, with 15 and 17 in
place of 16 and 18.

## With your own web server

A server that already runs nginx, Caddy, Traefik or Apache for other
sites can keep it. Add `--web-server none`:

```sh
curl -fsSL https://updates.gophenberg.org/install.sh | sudo sh -s -- --domain example.com --web-server none
```

The install then sets up no Caddy and leaves ports 80 and 443 to your
web server. That web server must run on the same server and provide
HTTPS. The run ends with the lines nginx, Caddy, Traefik and Apache
need to send the domain to the site, see
[with your own web server](/self-hosting/commands/#with-your-own-web-server).
To move a server the install set Caddy up on, see
[moving to your own web server](/self-hosting/commands/#moving-to-your-own-web-server).

## Uninstall

This section removes Gophenberg from the server.
`sudo gophenberg server:uninstall` lists what it would remove and what
it keeps, and `-yes` removes it. It keeps the database, `server.env`,
and the install root with the site's `.env`, media, themes and
backups. So the install line run again brings the site back, see
[server:uninstall](/self-hosting/commands/#serveruninstall) and
[installing again](/self-hosting/commands/#installing-again).

## When install.sh stops

This section lists each line `install.sh` stops with and what to do.
Each one goes to standard error and ends `install.sh` with exit code 1.
Nothing was installed, and no download folder is left.

- **The copy was cut off.** The download of `install.sh` itself
  stopped part way. Run the line again.

  ```text
  install.sh: this copy of install.sh was cut off, so nothing was installed
  ```

- **Not root.** Run it with `sudo`, as the line shows.

  ```text
  install.sh: run it as root, such as with curl -fsSL https://updates.gophenberg.org/install.sh | sudo sh -s -- --domain example.com
  ```

- **No release named.** This is the copy in the repository, not the
  published one. Run the published one.

  ```text
  install.sh: this copy of install.sh names no release, so run it from https://updates.gophenberg.org/install.sh
  ```

- **An archive given.** Leave out `-archive`. To install from an
  archive of your own, run `server:install` by hand, see
  [server:install](/self-hosting/commands/#serverinstall).

  ```text
  install.sh: install.sh passes the archive it checked itself, so leave out -archive
  ```

- **Another processor.** Gophenberg ships no build for it.

  ```text
  install.sh: Gophenberg ships for x86_64 and aarch64 servers, and this server is riscv64
  ```

- **A missing tool.** Run the `apt-get` line it names, then the install
  line again.

  ```text
  install.sh: install.sh needs openssl, so add it with sudo apt-get install openssl
  ```

- **An old OpenSSL.** The system is older than the ones Gophenberg
  installs on. Ubuntu 24.04, Debian 12 and their newer releases ship
  OpenSSL 3 or newer.

  ```text
  install.sh: checking the download needs OpenSSL 3 or newer, and this server has OpenSSL 1.1.1w
  ```

- **The installer folder.** `/var/lib/gophenberg-installer` is a link,
  is not root's, or another user can write it. Find out who changed it
  before you change it back.

  ```text
  install.sh: /var/lib/gophenberg-installer, which holds the install record, must be a real folder owned by root and writable by root alone, so nothing was installed
  ```

- **A failed download.** `curl` prints its own error on the line above,
  such as `Operation too slow` for a download that stalled. Check the
  server's network and run the line again. When `curl` could
  not resolve or reach GitHub, the line adds that GitHub needs IPv4.

  ```text
  install.sh: cannot download https://github.com/gopherium/gophenberg/releases/download/v%VERSION%/gophenberg_%VERSION%_linux_amd64.tar.gz, so nothing was installed
  install.sh: cannot download https://github.com/gopherium/gophenberg/releases/download/v%VERSION%/gophenberg_%VERSION%_linux_amd64.tar.gz, so nothing was installed. GitHub's downloads need IPv4, which a server with IPv6 alone cannot reach
  ```

- **A changed archive.** Something changed the archive or its
  signature on the way. Run the line again, and when it stops the same
  way, install nothing.

  ```text
  install.sh: the archive is not the one this install.sh names, so nothing was installed
  install.sh: the archive's signature matches neither Gophenberg release key, so nothing was installed
  ```

- **No room to unpack.** `tar` prints its own error on the line above,
  most often a full disk. Free some space and run the line again.

  ```text
  install.sh: cannot unpack gophenberg from gophenberg_%VERSION%_linux_amd64.tar.gz, so nothing was installed
  ```

Once `install.sh` hands over, a stop line starts with `gophenberg:`
instead, see [the checks](/self-hosting/commands/#the-checks).

A run killed before it could clean up, such as by a power cut, leaves
its download folder behind. Every `download.` folder in
`/var/lib/gophenberg-installer` is safe to delete once no install
runs:

```sh
sudo sh -c 'rm -rf /var/lib/gophenberg-installer/download.*'
```

Keep `install.json` beside them. It is the install record, which every
later install and `server:uninstall` read.
