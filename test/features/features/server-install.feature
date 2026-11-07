Feature: Install Gophenberg on a Linux server
  sudo gophenberg server:install sets up Gophenberg on a fresh server
  that runs Ubuntu 24.04 or newer, or Debian 12 or newer, and notes a
  system Gophenberg's CI does not test yet. Before it changes anything it
  checks that root runs it and that its installer folder is a real folder
  root alone may write, then reads its settings, reads its install record
  and checks the server: the system, systemd and the sandbox of the site's
  service, memory, disk, the ports, any PostgreSQL or Caddy it did not
  install, the folders above the install root, and that the domain points
  at this server. A check that fails stops the run with a line that says
  why and that nothing was changed. A later run skips the ports and the
  pieces the install record says an earlier run installed. With -dry-run
  the run then prints each step it would take and the full text of every
  file it would write, and still changes nothing. A line the person added
  to the site's .env prints by its key alone, its value hidden, and the
  file keeps that value. Without -dry-run it then opens the release
  archive -archive names and checks that its release.json names this
  installer's version and this server's platform. While the first
  administrator is still to be made, it asks on the terminal for what the
  flags leave out: the email, the name and the password, typed twice and
  never shown. A typed answer the account rules refuse is asked again.
  Then it takes the steps its dry run prints, in order, and writes each
  file its dry run prints. The install record marks each step started
  before it and ended after it, so a run cut off part way picks up where
  it stopped, and a step an earlier run ended never opens again. A later
  run leaves the running build alone, asks nothing, keeps the person's
  Caddyfile, reloads Caddy and restarts the site. A site that never
  answers stops the run, and an HTTPS check that gets no answer only warns
  and checks again on the next run. The checks also stop at a gophenberg
  command on the path that the installer did not write, at a Caddyfile
  that lost a line an earlier run wrote, and at a current link that names
  no build. Root never takes a setting from the site's .env, since the
  site's own user can write it. It reads only the five stop graces from
  it, as plain numbers, for the time systemd gives the site to stop, and
  on a server that keeps its own web server the upload cap. With
  -web-server none the server keeps a web server of its own: the run
  leaves out Caddy, the ports Caddy answers on and the domain check, and
  its last step prints the lines that web server needs to send the domain
  to the site. A later run never moves a server from one web server to
  the other.

  Background:
    Given a fresh Ubuntu 24.04 server run by systemd 255

  Scenario: A plain user is refused
    Given the operator is not root
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "server:install must run as root, such as with sudo, so nothing was changed"
    And nothing on the server changed

  Scenario: A plain user is refused on a server an earlier run set up
    Given an earlier run installed Gophenberg for example.com on the server
    And only root may read the server settings and the install record
    And the operator is not root
    When the operator runs "server:install -dry-run"
    Then the run stops with "server:install must run as root, such as with sudo, so nothing was changed"
    And nothing on the server changed

  Scenario Outline: A system the installer does not set up is refused
    Given the server runs <system>
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "this server runs <named>, and server:install sets up Ubuntu 24.04 or newer and Debian 12 or newer only, while Docker Compose runs Gophenberg on other systems, so nothing was changed"
    And nothing on the server changed

    Examples:
      | system       | named                            |
      | Ubuntu 22.04 | Ubuntu 22.04.5 LTS               |
      | Fedora 41    | Fedora Linux 41 (Server Edition) |

  Scenario Outline: A release Gophenberg's CI tests is set up with no note
    Given the server runs <system>
    When the operator runs "server:install -dry-run -domain localhost -admin-email admin@example.com"
    Then the run succeeds
    And the run notes nothing
    And nothing on the server changed

    Examples:
      | system       |
      | Ubuntu 24.04 |
      | Ubuntu 26.04 |

  Scenario Outline: A newer Ubuntu or Debian is set up with a note that CI does not test it yet
    Given the server runs <system>
    When the operator runs "server:install -dry-run -domain example.com -admin-email admin@example.com"
    Then the run succeeds
    And the run notes "<named> is not yet tested in Gophenberg's CI, which tests Ubuntu 24.04 and Ubuntu 26.04"
    And nothing on the server changed

    Examples:
      | system       | named                       |
      | Ubuntu 26.10 | Ubuntu 26.10                |
      | Debian 14    | Debian GNU/Linux 14 (forky) |

  Scenario: A systemd older than 251 is refused
    Given the server runs systemd 250
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "this server runs systemd 250, and the site's units need systemd 251 or newer, so nothing was changed"
    And nothing on the server changed

  Scenario: A server where systemd cannot set up the sandbox of the site's service is refused
    Given systemd cannot set up a sandbox on the server, a container systemd-detect-virt reports as lxc
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "systemd cannot set up the sandbox the site's service runs in on this server, as on a container plan without nesting, and systemd-detect-virt reports lxc, so nothing was changed"
    And nothing on the server changed

  Scenario: A server with too little memory is refused
    Given the server has 512 MB of memory
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "this server has 512 MB of memory, less than the 900 MB GOPHENBERG_INSTALL_MEMORY_FLOOR_MB asks, so nothing was changed"
    And nothing on the server changed

  Scenario: A server where something listens on port 80 is refused
    Given something listens on TCP port 80
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "something already listens on TCP port 80, which Caddy answers on, so nothing was changed"
    And nothing on the server changed

  Scenario: A site address on a port Caddy will answer on is refused
    Given the environment sets GOPHENBERG_ADDR to 127.0.0.1:443
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "127.0.0.1:443, the GOPHENBERG_ADDR the site listens on, takes TCP port 443, which Caddy answers on, and GOPHENBERG_ADDR must name another port, in the environment or in its line in /etc/gophenberg/server.env, so nothing was changed"
    And the checks listened on no port
    And nothing on the server changed

  Scenario: A server that keeps its own web server skips the web server checks
    Given something listens on TCP port 80
    And Caddy is installed on the server
    And example.com has no A or AAAA record
    When the operator runs "server:install -dry-run -domain example.com -web-server none"
    Then the run succeeds
    And the run notes nothing
    And the checks listened only on 127.0.0.1:8081 and TCP port 5432
    And the checks looked up no domain
    And nothing on the server changed

  Scenario: A later run that asks for the other web server is refused
    Given an earlier run installed Gophenberg for example.com on the server
    When the operator runs "server:install -dry-run -web-server none"
    Then the run stops with "GOPHENBERG_INSTALL_WEB_SERVER: this server was set up with caddy, and a later run cannot move it to none, so run sudo gophenberg server:uninstall first, then change the line in /etc/gophenberg/server.env"
    And nothing on the server changed

  Scenario: A later run whose server settings were changed by hand to the other web server is refused
    Given an earlier run installed Gophenberg for example.com on the server
    And the web server line of the server settings was changed by hand to none
    When the operator runs "server:install -dry-run"
    Then the run stops with "GOPHENBERG_INSTALL_WEB_SERVER: this server was set up with caddy, and a later run cannot move it to none, so run sudo gophenberg server:uninstall first, then change the line in /etc/gophenberg/server.env"
    And nothing on the server changed

  Scenario: A later run that asks for another install root is refused
    Given an earlier run installed Gophenberg for example.com on the server
    And the environment sets GOPHENBERG_INSTALL_ROOT to /srv/gophenberg
    When the operator runs "server:install"
    Then the run stops with "GOPHENBERG_INSTALL_ROOT: this server was set up with /opt/gophenberg, and a later run cannot move it to /srv/gophenberg, so run sudo gophenberg server:uninstall first, then change the line in /etc/gophenberg/server.env"
    And nothing on the server changed

  Scenario: A domain with no address is refused
    Given example.com has no A or AAAA record
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "cannot find an A or AAAA record of example.com, which must point at this server: lookup example.com: no such host, so nothing was changed"
    And the run notes "the hosting provider's firewall must let TCP 80, TCP 443 and UDP 443 in for HTTPS to work, and this check cannot see it"
    And nothing on the server changed

  Scenario: A domain that points at another server is refused
    Given example.com points at 198.51.100.7, where another server answers every page with 404
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "example.com points at 198.51.100.7, which did not answer the check as this server would (it answered 404 Not Found), likely because example.com points at another server or because the network does not route a request from this server to its own public address back to it, and -skip-domain-check skips this check for a server behind a proxy it cannot cross, so nothing was changed"
    And the run notes "the hosting provider's firewall must let TCP 80, TCP 443 and UDP 443 in for HTTPS to work, and this check cannot see it"
    And nothing on the server changed

  Scenario: A domain that resolves to a loopback address is refused
    Given example.com resolves to 127.0.1.1, as a line in /etc/hosts makes it
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "example.com resolves to 127.0.1.1, an address that never leaves this server, likely from a line in /etc/hosts, and Let's Encrypt asks for the domain from outside, which needs its public A or AAAA record to point at this server, and -skip-domain-check skips this check once it does, so nothing was changed"
    And the run notes "the hosting provider's firewall must let TCP 80, TCP 443 and UDP 443 in for HTTPS to work, and this check cannot see it"
    And the checks fetched no page
    And nothing on the server changed

  Scenario: A later run for a new domain behind NAT is refused while the Caddy an earlier run installed holds port 80
    Given an earlier run installed Gophenberg for example.com on the server
    And www.example.com points at 198.51.100.7, a public address a NAT in front of the server forwards to it
    When the operator runs "server:install -dry-run -domain www.example.com"
    Then the run stops with "the Caddy this installer set up holds TCP port 80, so the check that www.example.com points at this server cannot run, and -skip-domain-check skips it once the domain points here, so nothing was changed"
    And the run notes "the hosting provider's firewall must let TCP 80, TCP 443 and UDP 443 in for HTTPS to work, and this check cannot see it"
    And the checks fetched no page
    And nothing on the server changed

  Scenario: A domain that is not a host name is refused before the server is read
    When the operator runs "server:install -dry-run -domain Example.com"
    Then the run stops with "-domain: must be localhost or a host name such as example.com: labels of 1 to 63 lower-case letters, digits and hyphens joined by dots, no hyphen at either end of a label, a letter in the last label, 253 characters at most, got "Example.com""
    And nothing on the server changed

  Scenario: An archive outside the installer's folder is refused before the server is read
    When the operator runs "server:install -dry-run -domain example.com -archive /tmp/gophenberg_0.20.0_linux_amd64.tar.gz"
    Then the run stops with "-archive: must be a file inside /var/lib/gophenberg-installer, the folder install.sh downloads it into, got "/tmp/gophenberg_0.20.0_linux_amd64.tar.gz""
    And nothing on the server changed

  Scenario: A domain whose last label holds no letter is refused before the server is read
    When the operator runs "server:install -dry-run -domain 203.0.113.7"
    Then the run stops with "-domain: must be localhost or a host name such as example.com: labels of 1 to 63 lower-case letters, digits and hyphens joined by dots, no hyphen at either end of a label, a letter in the last label, 253 characters at most, got "203.0.113.7""
    And nothing on the server changed

  Scenario: A server that already runs PostgreSQL is refused
    Given PostgreSQL is installed on the server
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "this server already has PostgreSQL at /etc/postgresql, and server:install sets up a fresh server only, so nothing was changed"
    And nothing on the server changed

  Scenario: An install root under a folder another user can write is refused
    Given another user can write /opt
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "/opt, a parent of the install root /opt/gophenberg, must be owned by root and writable by root alone, so nothing was changed"
    And nothing on the server changed

  Scenario: An installer folder another user can write is refused
    Given another user can write /var/lib/gophenberg-installer
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "/var/lib/gophenberg-installer, which holds the install record, must be a real folder owned by root and writable by root alone, so nothing was changed"
    And nothing on the server changed

  Scenario: A plain file where the installer folder belongs is refused before the run reads the install record
    Given a plain file sits at /var/lib/gophenberg-installer
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "/var/lib/gophenberg-installer, which holds the install record, must be a real folder owned by root and writable by root alone, so nothing was changed"
    And nothing on the server changed
    When the operator runs "server:install -domain example.com"
    Then the run stops with "/var/lib/gophenberg-installer, which holds the install record, must be a real folder owned by root and writable by root alone, so nothing was changed"
    And nothing on the server changed

  Scenario: An install root that would hand the site's user a folder root trusts is refused
    Given the environment sets GOPHENBERG_INSTALL_ROOT to /usr/local
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "the install root /usr/local would hand /usr/local/lib/gophenberg, whose files root runs or reads, to the site's user, and GOPHENBERG_INSTALL_ROOT must name another folder, so nothing was changed"
    And nothing on the server changed

  Scenario: An installer older than the one that set up the server is refused
    Given the install record names the installer "1000.0.0"
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops because this installer is older than the 1000.0.0 installer
    And nothing on the server changed

  Scenario: A run while another run is going is refused, and a dry run still goes through
    Given an earlier run installed Gophenberg for example.com on the server
    And another server:install is running on the server
    When the operator runs "server:install"
    Then the run stops with "another server:install or server:uninstall is running on this server, so nothing was changed"
    And nothing on the server changed
    When the operator runs "server:install -dry-run"
    Then the run succeeds

  Scenario: A later run skips the ports and the pieces an earlier run installed
    Given an earlier run installed Gophenberg for example.com on the server
    When the operator runs "server:install -dry-run"
    Then the run succeeds
    And the checks listened on no port
    And nothing on the server changed
    And the dry run lists these steps:
      """
      Step 1: start the install record /var/lib/gophenberg-installer/install.json and write the server settings
      Step 2: install PostgreSQL and Caddy with apt-get, Caddy from its own apt repository
      Step 3: make the system user gophenberg, which runs the site and signs in to PostgreSQL
      Step 4: make the install root /opt/gophenberg and its folders, owned by gophenberg
      Step 5: make the database role gophenberg and its database gophenberg as the postgres user
      Step 6: keep the build /opt/gophenberg/current names, since updates go through update:apply
      Step 7: keep a copy of this installer at /usr/local/lib/gophenberg/gophenberg, the one root runs for server:install and server:uninstall
      Step 8: write the site's settings, then run check as gophenberg in /opt/gophenberg
      Step 9: write the systemd units and Caddy's drop-in, then reload systemd and enable gophenberg.service
      Step 10: write the gophenberg command on the path
      Step 11: write Caddy's files, check them with caddy validate and reload Caddy
      Step 12: restart gophenberg.service and wait until /admin/, /api/locale and / answer on 127.0.0.1:8081
      Step 13: check that https://example.com/admin/ answers within 2m0s
      Step 14: print the summary and end the install record
      """

  Scenario: A GOPHENBERG_INSTALL_ROOT line planted in the site's .env is never read
    Given an earlier run installed Gophenberg for example.com on the server
    And the site's .env holds the line "GOPHENBERG_INSTALL_ROOT=/etc"
    And the site's .env holds the line "GOPHENBERG_SITE_TITLE=Maria Perez's site"
    And the site's .env holds the line "GOPHENBERG_SHUTDOWN_GRACE=4m"
    When the operator runs "server:install -dry-run"
    Then the run succeeds
    And nothing on the server changed
    And the dry run writes "/opt/gophenberg/.env", owned by gophenberg with mode 0600:
      """
      # Written by gophenberg server:install.
      GOPHENBERG_DATABASE_URL=postgres:///gophenberg?host=/var/run/postgresql&user=gophenberg
      GOPHENBERG_SUPERUSER_POLICY=refuse
      GOPHENBERG_INSTALL_ROOT=/opt/gophenberg
      GOPHENBERG_ADDR=127.0.0.1:8081
      GOPHENBERG_PUBLIC_URL=https://example.com
      GOPHENBERG_TRUSTED_PROXIES=127.0.0.1/32,::1/128
      GOPHENBERG_THEMES_DIR=/opt/gophenberg/themes
      GOPHENBERG_MEDIA_DIR=/opt/gophenberg/media
      GOPHENBERG_DATABASE_WAIT=60s
      GOPHENBERG_SHUTDOWN_GRACE=(kept as it is)
      GOPHENBERG_SITE_TITLE=(kept as it is)
      """
    And the site's unit holds the line "WorkingDirectory=/opt/gophenberg"
    And the site's unit holds the line "ReadWritePaths=/opt/gophenberg"
    And the site's unit holds the line "TimeoutStopSec=260s"

  Scenario: A dry run hides the value of a password line the person added to the site's .env, and the run keeps it
    Given an earlier run installed Gophenberg for example.com on the server
    And the site's .env holds the line "GOPHENBERG_SMTP_PASSWORD=Maria's mail password"
    When the operator runs "server:install -dry-run"
    Then the run succeeds
    And nothing the run printed holds "Maria's mail password"
    And the dry run writes "/opt/gophenberg/.env", owned by gophenberg with mode 0600:
      """
      # Written by gophenberg server:install.
      GOPHENBERG_DATABASE_URL=postgres:///gophenberg?host=/var/run/postgresql&user=gophenberg
      GOPHENBERG_SUPERUSER_POLICY=refuse
      GOPHENBERG_INSTALL_ROOT=/opt/gophenberg
      GOPHENBERG_ADDR=127.0.0.1:8081
      GOPHENBERG_PUBLIC_URL=https://example.com
      GOPHENBERG_TRUSTED_PROXIES=127.0.0.1/32,::1/128
      GOPHENBERG_THEMES_DIR=/opt/gophenberg/themes
      GOPHENBERG_MEDIA_DIR=/opt/gophenberg/media
      GOPHENBERG_DATABASE_WAIT=60s
      GOPHENBERG_SMTP_PASSWORD=(kept as it is)
      """
    When the operator runs "server:install"
    Then the run succeeds
    And nothing the run printed holds "Maria's mail password"
    And the server holds every file the dry run printed, with its owner and mode

  Scenario: A line of the site's .env that holds a terminal control sequence is refused
    Given an earlier run installed Gophenberg for example.com on the server
    And the site's .env holds the line "GOPHENBERG_SITE_TITLE=Maria<ESC>[2JPerez", <ESC> standing for the escape character
    When the operator runs "server:install -dry-run"
    Then the run stops with "installer: /opt/gophenberg/.env: "GOPHENBERG_SITE_TITLE" holds a control character or a byte that is not UTF-8, which no setting needs"
    And the run prints nothing
    And nothing on the server changed

  Scenario Outline: The site's unit restarts directly only where systemd knows how
    Given the server runs <system> with systemd <systemd>
    When the operator runs "server:install -dry-run -domain example.com -admin-email admin@example.com"
    Then the run succeeds
    And the site's unit <holds> the line "RestartMode=direct"

    Examples:
      | system       | systemd | holds |
      | Ubuntu 24.04 | 255     | holds |
      | Debian 12    | 252     | lacks |

  Scenario: A dry run prints every step and the full text of every file for a real domain
    Given unattended-upgrades is installed on the server
    When the operator runs "server:install -dry-run -domain example.com -admin-email admin@example.com"
    Then the run succeeds
    And the run notes "the hosting provider's firewall must let TCP 80, TCP 443 and UDP 443 in for HTTPS to work, and this check cannot see it"
    And nothing on the server changed
    And the dry run lists these steps:
      """
      Step 1: start the install record /var/lib/gophenberg-installer/install.json and write the server settings
      Step 2: install PostgreSQL and Caddy with apt-get, Caddy from its own apt repository
      Step 3: make the system user gophenberg, which runs the site and signs in to PostgreSQL
      Step 4: make the install root /opt/gophenberg and its folders, owned by gophenberg
      Step 5: make the database role gophenberg and its database gophenberg as the postgres user
      Step 6: unpack the release archive into /opt/gophenberg/versions and point /opt/gophenberg/current at it
      Step 7: keep a copy of this installer at /usr/local/lib/gophenberg/gophenberg, the one root runs for server:install and server:uninstall
      Step 8: write the site's settings, then run check as gophenberg in /opt/gophenberg
      Step 9: write the systemd units and Caddy's drop-in, then reload systemd and enable gophenberg.service
      Step 10: write the gophenberg command on the path
      Step 11: write Caddy's files, keep the package's Caddyfile as /etc/caddy/Caddyfile.before-gophenberg, check them with caddy validate and restart Caddy
      Step 12: start gophenberg.service and wait until /admin/, /api/locale and / answer on 127.0.0.1:8081
      Step 13: create the first administrator with account:create-admin as gophenberg
      Step 14: check that https://example.com/admin/ answers within 2m0s
      Step 15: print the summary and end the install record
      """
    And the dry run writes "/etc/gophenberg/server.env", owned by root with mode 0600:
      """
      # Written by gophenberg server:install.
      GOPHENBERG_INSTALL_ROOT=/opt/gophenberg
      GOPHENBERG_PUBLIC_URL=https://example.com
      GOPHENBERG_ADDR=127.0.0.1:8081
      GOPHENBERG_INSTALL_WEB_SERVER=caddy
      GOPHENBERG_INSTALL_CERTIFICATE_EMAIL=admin@example.com
      GOPHENBERG_SERVICE_RESTART_DELAY=5s
      GOPHENBERG_SERVICE_START_LIMIT_INTERVAL=300s
      GOPHENBERG_SERVICE_START_LIMIT_BURST=10
      GOPHENBERG_SERVICE_STOP_MARGIN=5s
      GOPHENBERG_MAINTENANCE_RETRY_AFTER=600s
      GOPHENBERG_PROXY_RETRY_DURATION=10s
      """
    And the dry run writes "/etc/apt/keyrings/caddy.asc", owned by root with mode 0644, holding Caddy's repository key
    And the dry run writes "/etc/apt/sources.list.d/caddy.sources", owned by root with mode 0644:
      """
      # Written by gophenberg server:install.
      Types: deb
      URIs: https://dl.cloudsmith.io/public/caddy/stable/deb/debian
      Suites: any-version
      Components: main
      Signed-By: /etc/apt/keyrings/caddy.asc
      """
    And the dry run writes "/etc/apt/preferences.d/gophenberg-caddy", owned by root with mode 0644:
      """
      Explanation: Written by gophenberg server:install.
      Package: caddy
      Pin: origin dl.cloudsmith.io
      Pin-Priority: 500

      Package: *
      Pin: origin dl.cloudsmith.io
      Pin-Priority: -1
      """
    And the dry run writes "/etc/apt/apt.conf.d/52gophenberg-caddy", owned by root with mode 0644:
      """
      # Written by gophenberg server:install.
      Unattended-Upgrade::Origins-Pattern {
      	"origin=cloudsmith/caddy/stable,site=dl.cloudsmith.io";
      };
      """
    And the dry run writes "/opt/gophenberg/.env", owned by gophenberg with mode 0600:
      """
      # Written by gophenberg server:install.
      GOPHENBERG_DATABASE_URL=postgres:///gophenberg?host=/var/run/postgresql&user=gophenberg
      GOPHENBERG_SUPERUSER_POLICY=refuse
      GOPHENBERG_INSTALL_ROOT=/opt/gophenberg
      GOPHENBERG_ADDR=127.0.0.1:8081
      GOPHENBERG_PUBLIC_URL=https://example.com
      GOPHENBERG_TRUSTED_PROXIES=127.0.0.1/32,::1/128
      GOPHENBERG_THEMES_DIR=/opt/gophenberg/themes
      GOPHENBERG_MEDIA_DIR=/opt/gophenberg/media
      GOPHENBERG_DATABASE_WAIT=60s
      """
    And the dry run writes "/etc/systemd/system/gophenberg.service", owned by root with mode 0644:
      """
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
      """
    And the dry run writes "/etc/systemd/system/gophenberg-rollback.service", owned by root with mode 0644:
      """
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
      """
    And the dry run writes "/etc/systemd/system/caddy.service.d/gophenberg.conf", owned by root with mode 0644:
      """
      # Written by gophenberg server:install.
      [Service]
      RuntimeDirectory=caddy
      RuntimeDirectoryMode=0700
      """
    And the dry run writes "/usr/local/bin/gophenberg", owned by root with mode 0755:
      """
      #!/bin/sh
      # Written by gophenberg server:install.
      if [ "$(id -u)" -ne 0 ]; then
      	echo "gophenberg: run it with sudo, such as sudo gophenberg update:status" >&2
      	exit 1
      fi
      case "${1-}" in
      server:install | server:uninstall)
      	cd / || exit 1
      	exec /usr/local/lib/gophenberg/gophenberg "$@"
      	;;
      esac
      cd /opt/gophenberg || exit 1
      /usr/sbin/runuser --pty -u gophenberg -- /opt/gophenberg/current/gophenberg "$@"
      code=$?
      if [ "$code" -ne 75 ]; then
      	exit "$code"
      fi
      /usr/bin/systemctl reset-failed gophenberg.service
      /usr/bin/systemctl restart gophenberg.service || exit 1
      echo "gophenberg: restarted the site to start the new build"
      """
    And the dry run writes "/usr/local/share/gophenberg/maintenance/index.html", owned by root with mode 0644:
      """
      <!DOCTYPE html>
      <html lang="en">
      <head>
      	<meta charset="utf-8">
      	<meta name="viewport" content="width=device-width, initial-scale=1">
      	<title>Briefly unavailable</title>
      	<style>
      		body {
      			margin: 0;
      			padding: 48px 16px;
      			background: #fcfcfc;
      			color: #1e1e1e;
      			font-family: system-ui, sans-serif;
      		}
      		main {
      			box-sizing: border-box;
      			max-width: 700px;
      			margin: 0 auto;
      			padding: 24px 32px;
      			border: 1px solid #dbdbdb;
      			border-radius: 8px;
      			background: #fff;
      		}
      		p {
      			margin: 0;
      			font-size: 16px;
      			line-height: 1.5;
      		}
      	</style>
      </head>
      <body>
      	<main>
      		<p>This site is briefly unavailable. Check back in a minute.</p>
      	</main>
      </body>
      </html>
      """
    And the dry run writes "/etc/caddy/gophenberg.caddy", owned by root with mode 0644:
      """
      # Written by gophenberg server:install.
      example.com {
      	reverse_proxy 127.0.0.1:8081 {
      		lb_try_duration 10s
      	}
      	handle_errors 502 503 {
      		root * /usr/local/share/gophenberg/maintenance
      		rewrite * /index.html
      		header Retry-After 600
      		file_server {
      			status 503
      		}
      	}
      }
      """
    And the dry run writes "/etc/caddy/Caddyfile", owned by root with mode 0644:
      """
      # Written by gophenberg server:install on its first run.
      {
      	email admin@example.com
      	admin unix//run/caddy/admin.sock
      }

      import /etc/caddy/gophenberg.caddy
      """
    And the dry run writes no other file

  Scenario: A dry run for localhost names no certificate email
    Given unattended-upgrades is installed on the server
    When the operator runs "server:install -dry-run -domain localhost -admin-email admin@example.com"
    Then the run succeeds
    And the run notes nothing
    And nothing on the server changed
    And the dry run writes "/etc/gophenberg/server.env", owned by root with mode 0600:
      """
      # Written by gophenberg server:install.
      GOPHENBERG_INSTALL_ROOT=/opt/gophenberg
      GOPHENBERG_PUBLIC_URL=https://localhost
      GOPHENBERG_ADDR=127.0.0.1:8081
      GOPHENBERG_INSTALL_WEB_SERVER=caddy
      GOPHENBERG_INSTALL_CERTIFICATE_EMAIL=admin@example.com
      GOPHENBERG_SERVICE_RESTART_DELAY=5s
      GOPHENBERG_SERVICE_START_LIMIT_INTERVAL=300s
      GOPHENBERG_SERVICE_START_LIMIT_BURST=10
      GOPHENBERG_SERVICE_STOP_MARGIN=5s
      GOPHENBERG_MAINTENANCE_RETRY_AFTER=600s
      GOPHENBERG_PROXY_RETRY_DURATION=10s
      """
    And the dry run writes "/opt/gophenberg/.env", owned by gophenberg with mode 0600:
      """
      # Written by gophenberg server:install.
      GOPHENBERG_DATABASE_URL=postgres:///gophenberg?host=/var/run/postgresql&user=gophenberg
      GOPHENBERG_SUPERUSER_POLICY=refuse
      GOPHENBERG_INSTALL_ROOT=/opt/gophenberg
      GOPHENBERG_ADDR=127.0.0.1:8081
      GOPHENBERG_PUBLIC_URL=https://localhost
      GOPHENBERG_TRUSTED_PROXIES=127.0.0.1/32,::1/128
      GOPHENBERG_THEMES_DIR=/opt/gophenberg/themes
      GOPHENBERG_MEDIA_DIR=/opt/gophenberg/media
      GOPHENBERG_DATABASE_WAIT=60s
      """
    And the dry run writes "/etc/caddy/gophenberg.caddy", owned by root with mode 0644:
      """
      # Written by gophenberg server:install.
      localhost {
      	reverse_proxy 127.0.0.1:8081 {
      		lb_try_duration 10s
      	}
      	handle_errors 502 503 {
      		root * /usr/local/share/gophenberg/maintenance
      		rewrite * /index.html
      		header Retry-After 600
      		file_server {
      			status 503
      		}
      	}
      }
      """
    And the dry run writes "/etc/caddy/Caddyfile", owned by root with mode 0644:
      """
      # Written by gophenberg server:install on its first run.
      {
      	admin unix//run/caddy/admin.sock
      }

      import /etc/caddy/gophenberg.caddy
      """
    And the dry run writes every other file as it does for example.com

  Scenario: -dry-run with -web-server none prints the lines for the person's web server
    When the operator runs "server:install -dry-run -domain example.com -admin-email admin@example.com -web-server none"
    Then the run succeeds
    And the run notes nothing
    And nothing on the server changed
    And the dry run lists these steps:
      """
      Step 1: start the install record /var/lib/gophenberg-installer/install.json and write the server settings
      Step 2: install PostgreSQL with apt-get
      Step 3: make the system user gophenberg, which runs the site and signs in to PostgreSQL
      Step 4: make the install root /opt/gophenberg and its folders, owned by gophenberg
      Step 5: make the database role gophenberg and its database gophenberg as the postgres user
      Step 6: unpack the release archive into /opt/gophenberg/versions and point /opt/gophenberg/current at it
      Step 7: keep a copy of this installer at /usr/local/lib/gophenberg/gophenberg, the one root runs for server:install and server:uninstall
      Step 8: write the site's settings, then run check as gophenberg in /opt/gophenberg
      Step 9: write the systemd units, then reload systemd and enable gophenberg.service
      Step 10: write the gophenberg command on the path
      Step 11: start gophenberg.service and wait until /admin/, /api/locale and / answer on 127.0.0.1:8081
      Step 12: create the first administrator with account:create-admin as gophenberg
      Step 13: print the summary and end the install record
      Step 14: print the lines for nginx, Caddy, Traefik and Apache that send example.com to the site at 127.0.0.1:8081
      """
    And the dry run writes "/etc/gophenberg/server.env", owned by root with mode 0600:
      """
      # Written by gophenberg server:install.
      GOPHENBERG_INSTALL_ROOT=/opt/gophenberg
      GOPHENBERG_PUBLIC_URL=https://example.com
      GOPHENBERG_ADDR=127.0.0.1:8081
      GOPHENBERG_INSTALL_WEB_SERVER=none
      GOPHENBERG_INSTALL_CERTIFICATE_EMAIL=admin@example.com
      GOPHENBERG_SERVICE_RESTART_DELAY=5s
      GOPHENBERG_SERVICE_START_LIMIT_INTERVAL=300s
      GOPHENBERG_SERVICE_START_LIMIT_BURST=10
      GOPHENBERG_SERVICE_STOP_MARGIN=5s
      GOPHENBERG_MAINTENANCE_RETRY_AFTER=600s
      GOPHENBERG_PROXY_RETRY_DURATION=10s
      """
    And the dry run writes only these files:
      """
      /etc/gophenberg/server.env
      /opt/gophenberg/.env
      /etc/systemd/system/gophenberg.service
      /etc/systemd/system/gophenberg-rollback.service
      /usr/local/bin/gophenberg
      """
    And the dry run prints for the person's web server:
      """
      Your own web server provides HTTPS for example.com and must run on this server, since the
      site answers on 127.0.0.1:8081 alone. During the restart after an update it shows its own
      error page, unless it can hold requests until the site answers again. A web server in a
      container needs this server's own network, such as network_mode: host, since a
      container's loopback address is its own. The lines below send example.com to the site with
      the visitor's host, client address and scheme, and a body limit as large as the largest
      upload the site takes: a file of the 128 MB GOPHENBERG_MEDIA_UPLOAD_CAP_MB allows or a
      theme of 64 MB, each with 64 KB of form framing around it.

      nginx, in the server block that serves example.com over HTTPS:

      client_max_body_size 131136k;

      location / {
      	proxy_pass http://127.0.0.1:8081;
      	proxy_set_header Host $host;
      	proxy_set_header X-Forwarded-Host $host;
      	proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
      	proxy_set_header X-Forwarded-Proto $scheme;
      }

      Caddy, in its Caddyfile, where reverse_proxy sends the visitor's host, client address
      and scheme on its own, and Caddy sets no upload limit:

      example.com {
      	reverse_proxy 127.0.0.1:8081
      }

      Traefik, in a file its file provider reads, where websecure and letsencrypt stand for
      your HTTPS entry point and certificate resolver, and Traefik sends the visitor's host,
      client address and scheme on its own and sets no size limit:

      http:
        routers:
          gophenberg:
            rule: Host(`example.com`)
            entryPoints:
              - websecure
            tls:
              certResolver: letsencrypt
            service: gophenberg
        services:
          gophenberg:
            loadBalancer:
              servers:
                - url: http://127.0.0.1:8081

      Traefik 3 cuts a request it has not read in full within the entry point's
      transport.respondingTimeouts.readTimeout, 60s by default. For large uploads, raise it in
      Traefik's install configuration to at least GOPHENBERG_UPLOAD_TIMEOUT, 5m0s unless the
      site's .env sets it, or to 0.

      Apache, in the virtual host that serves example.com over HTTPS, with its proxy_http and
      headers modules enabled:

      LimitRequestBody 134283264
      ProxyAddHeaders Off
      ProxyPreserveHost On
      RequestHeader set X-Forwarded-Host expr=%{HTTP_HOST}
      RequestHeader set X-Forwarded-For expr=%{REMOTE_ADDR}
      RequestHeader set X-Forwarded-Proto expr=%{REQUEST_SCHEME}
      ProxyPass / http://127.0.0.1:8081/
      ProxyPassReverse / http://127.0.0.1:8081/
      """

  Scenario: A dry run asks no question and opens no archive
    Given the operator's terminal answers:
      """
      admin@example.com
      """
    When the operator runs "server:install -dry-run -domain example.com -archive /var/lib/gophenberg-installer/missing.tar.gz"
    Then the run succeeds
    And the run asked nothing on the terminal
    And nothing on the server changed

  Scenario: A first run without -archive is refused before anything changes
    Given the operator's terminal answers:
      """
      admin@example.com
      """
    When the operator runs "server:install -domain example.com"
    Then the run stops with "-archive is required while /opt/gophenberg/current names no build, such as the archive install.sh downloads into /var/lib/gophenberg-installer, so nothing was changed"
    And the run asked nothing on the terminal
    And nothing on the server changed

  Scenario: An archive of another version than this installer is refused
    Given a release archive of Gophenberg 0.0.1 is at /var/lib/gophenberg-installer/release.tar.gz
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz"
    Then the run stops because the archive /var/lib/gophenberg-installer/release.tar.gz holds Gophenberg 0.0.1
    And the run asked nothing on the terminal
    And nothing on the server changed

  Scenario: An archive built for another platform is refused
    Given a release archive of this installer built for windows/arm64 is at /var/lib/gophenberg-installer/release.tar.gz
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz"
    Then the run stops because the archive /var/lib/gophenberg-installer/release.tar.gz is built for windows/arm64
    And the run asked nothing on the terminal
    And nothing on the server changed

  Scenario: A password file that is not an absolute path is refused before the server is read
    When the operator runs "server:install -domain example.com -admin-password-file admin-password"
    Then the run stops with "-admin-password-file: must be an absolute path, got "admin-password""
    And the checks listened on no port
    And nothing on the server changed

  Scenario: A password file whose first line ends in a carriage return is refused
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the file /root/admin-password holds the line "correct horse battery staple" ending in a carriage return
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com -admin-name Maria -admin-password-file /root/admin-password"
    Then the run stops with "-admin-password-file: the first line of /root/admin-password ends in a carriage return, as a file saved with Windows line endings does, and a password never holds one, so nothing was changed"
    And the run asked nothing on the terminal
    And nothing on the server changed

  Scenario: A password shorter than 12 characters is refused before anything changes
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the file /root/admin-password holds the line "too short"
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com -admin-name Maria -admin-password-file /root/admin-password"
    Then the run stops with "-admin-password-file: password shorter than 12 characters, so nothing was changed"
    And the run asked nothing on the terminal
    And nothing on the server changed

  Scenario: A first run takes each step's programs in order and creates the first administrator from the flags
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the file /root/admin-password holds the line "correct horse battery staple"
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com -admin-name Maria -admin-password-file /root/admin-password"
    Then the run succeeds
    And the run asked nothing on the terminal
    And the run ran these programs, the checks' own left out:
      """
      dpkg-query -W -f ${db:Status-Abbrev}\n postgresql caddy
      apt-get -o DPkg::Lock::Timeout=300 update
      apt-get -o DPkg::Lock::Timeout=300 install -y --no-install-recommends postgresql caddy
      useradd --system --user-group --home-dir /opt/gophenberg --no-create-home --shell /usr/sbin/nologin gophenberg
      runuser -u postgres -- createuser --no-superuser --no-createdb --no-createrole gophenberg
      runuser -u postgres -- createdb --owner=gophenberg --encoding=UTF8 --template=template0 --locale-provider=icu --icu-locale=und gophenberg
      runuser -u gophenberg -- /opt/gophenberg/current/gophenberg check
      systemctl daemon-reload
      systemctl enable gophenberg.service
      runuser -u caddy -- caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
      systemctl restart caddy.service
      systemctl reset-failed gophenberg.service
      systemctl start gophenberg.service
      runuser -u gophenberg -- /opt/gophenberg/current/gophenberg account:create-admin -email admin@example.com -name Maria -role admin
      """
    And the run ran "runuser -u gophenberg -- /opt/gophenberg/current/gophenberg check" in /opt/gophenberg
    And the run ran "runuser -u gophenberg -- /opt/gophenberg/current/gophenberg account:create-admin -email admin@example.com -name Maria -role admin" in /opt/gophenberg, the line "correct horse battery staple" on its standard input
    And the run ran "runuser -u caddy -- caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile" with HOME=/var/lib/caddy in its environment
    And the run checked https://example.com/admin/ trusting the system's root certificates
    And the server holds /etc/caddy/Caddyfile.before-gophenberg as the caddy package wrote it
    And the install record holds the SHA-256 of the Caddyfile the run wrote
    And the install record marks every step it took ended
    And the run ends with:
      """
      Step 15: print the summary and end the install record

      Gophenberg %VERSION% runs at https://example.com. Sign in at https://example.com/admin/ as admin@example.com.
      The hosting provider's firewall must let TCP 80, TCP 443 and UDP 443 in for visitors to reach the site.
      Commands to know:
        sudo gophenberg update:status     shows the build that runs and the update on offer
        sudo gophenberg update:apply      installs the update on offer
        sudo journalctl -u gophenberg     shows what the site logged
        sudo gophenberg server:uninstall  removes Gophenberg and keeps its data
      """

  Scenario: A first run asks on the terminal for the first administrator the flags leave out, the password unseen and twice
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the operator's terminal answers:
      """
      Maria Perez
      correct horse battery staple
      correct horse battery staple
      """
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com"
    Then the run succeeds
    And the terminal showed:
      """
      Name of the first administrator [admin]:
      Password: (unseen)
      Password again: (unseen)
      """
    And the run ran "runuser -u gophenberg -- /opt/gophenberg/current/gophenberg account:create-admin -email admin@example.com -name Maria Perez -role admin" in /opt/gophenberg, the line "correct horse battery staple" on its standard input

  Scenario: An empty name takes the part of the email before @
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the file /root/admin-password holds the line "correct horse battery staple"
    And the operator's terminal answers:
      """
      maria.perez@example.com

      """
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz -admin-password-file /root/admin-password"
    Then the run succeeds
    And the terminal showed:
      """
      Email of the first administrator:
      Name of the first administrator [maria.perez]:
      """
    And the run ran "runuser -u gophenberg -- /opt/gophenberg/current/gophenberg account:create-admin -email maria.perez@example.com -name maria.perez -role admin" in /opt/gophenberg, the line "correct horse battery staple" on its standard input

  Scenario: A first run for localhost checks HTTPS against Caddy's own root certificate
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the file /root/admin-password holds the line "correct horse battery staple"
    When the operator runs "server:install -domain localhost -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com -admin-name Maria -admin-password-file /root/admin-password"
    Then the run succeeds
    And the run notes nothing
    And the run checked https://localhost/admin/ trusting Caddy's own root certificate

  Scenario: A first run with -web-server none installs everything but Caddy and prints the lines for the person's web server
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the file /root/admin-password holds the line "correct horse battery staple"
    When the operator runs "server:install -dry-run -domain example.com -admin-email admin@example.com -web-server none"
    And the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com -admin-name Maria -admin-password-file /root/admin-password -web-server none"
    Then the run succeeds
    And the run notes nothing
    And the server holds every file the dry run printed, with its owner and mode
    And the run ran these programs, the checks' own left out:
      """
      dpkg-query -W -f ${db:Status-Abbrev}\n postgresql
      apt-get -o DPkg::Lock::Timeout=300 update
      apt-get -o DPkg::Lock::Timeout=300 install -y --no-install-recommends postgresql
      useradd --system --user-group --home-dir /opt/gophenberg --no-create-home --shell /usr/sbin/nologin gophenberg
      runuser -u postgres -- createuser --no-superuser --no-createdb --no-createrole gophenberg
      runuser -u postgres -- createdb --owner=gophenberg --encoding=UTF8 --template=template0 --locale-provider=icu --icu-locale=und gophenberg
      runuser -u gophenberg -- /opt/gophenberg/current/gophenberg check
      systemctl daemon-reload
      systemctl enable gophenberg.service
      systemctl reset-failed gophenberg.service
      systemctl start gophenberg.service
      runuser -u gophenberg -- /opt/gophenberg/current/gophenberg account:create-admin -email admin@example.com -name Maria -role admin
      """
    And the server holds nothing at /etc/caddy
    And the run checked no page over HTTPS
    And the run prints:
      """
      Step 13: print the summary and end the install record

      Gophenberg %VERSION% answers on 127.0.0.1:8081, the GOPHENBERG_ADDR the site listens on.
      Once your own web server sends example.com there, sign in at https://example.com/admin/ as admin@example.com.
      Commands to know:
        sudo gophenberg update:status     shows the build that runs and the update on offer
        sudo gophenberg update:apply      installs the update on offer
        sudo journalctl -u gophenberg     shows what the site logged
        sudo gophenberg server:uninstall  removes Gophenberg and keeps its data
      Step 14: print the lines for nginx, Caddy, Traefik and Apache that send example.com to the site at 127.0.0.1:8081
      """
    And the run ends with the lines for the person's web server that the dry run printed

  Scenario: Two typed passwords that differ are asked again
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the operator's terminal answers:
      """
      admin@example.com
      Maria Perez
      correct horse battery staple
      correct horse battery stable
      correct horse battery staple
      correct horse battery staple
      """
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz"
    Then the run succeeds
    And the terminal showed:
      """
      Email of the first administrator:
      Name of the first administrator [admin]:
      Password: (unseen)
      Password again: (unseen)
      the two passwords differ, so type them again
      Password: (unseen)
      Password again: (unseen)
      """
    And the run ran "runuser -u gophenberg -- /opt/gophenberg/current/gophenberg account:create-admin -email admin@example.com -name Maria Perez -role admin" in /opt/gophenberg, the line "correct horse battery staple" on its standard input

  Scenario: A typed answer the account rules refuse is asked again
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the operator's terminal answers:
      """
      maria
      admin@example.com

      too short
      correct horse battery staple
      correct horse battery staple
      """
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz"
    Then the run succeeds
    And the terminal showed:
      """
      Email of the first administrator:
      invalid email, so type it again
      Email of the first administrator:
      Name of the first administrator [admin]:
      Password: (unseen)
      password shorter than 12 characters, so type it again
      Password: (unseen)
      Password again: (unseen)
      """
    And the run ran "runuser -u gophenberg -- /opt/gophenberg/current/gophenberg account:create-admin -email admin@example.com -name admin -role admin" in /opt/gophenberg, the line "correct horse battery staple" on its standard input

  Scenario: A typed email that cannot become the certificate email is asked again
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the operator's terminal answers:
      """
      maria{perez}@example.com
      admin@example.com

      correct horse battery staple
      correct horse battery staple
      """
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz"
    Then the run succeeds
    And the terminal showed:
      """
      Email of the first administrator:
      the email also becomes GOPHENBERG_INSTALL_CERTIFICATE_EMAIL, which must be one plain email address such as admin@example.com, got "maria{perez}@example.com", so type it again
      Email of the first administrator:
      Name of the first administrator [admin]:
      Password: (unseen)
      Password again: (unseen)
      """
    And the server holds /etc/caddy/Caddyfile with the certificate email admin@example.com

  Scenario: A run that must ask for the first administrator without a terminal is refused, naming the flags
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com"
    Then the run stops with "this run has no terminal to ask the first administrator's name and password on, so give -admin-name and -admin-password-file: open /dev/tty: no such device or address, so nothing was changed"
    And nothing on the server changed

  Scenario: A terminal that ends before every answer is given stops the run
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the operator's terminal answers:
      """
      admin@example.com
      """
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz"
    Then the run stops with "asking for the first administrator's name ended: the terminal gave no more input, so nothing was changed"
    And nothing on the server changed

  Scenario: A later run leaves the running build alone, asks nothing, restarts the site and reloads Caddy
    Given an earlier run installed Gophenberg for example.com on the server
    And the operator's terminal answers:
      """
      admin@example.com
      """
    When the operator runs "server:install"
    Then the run succeeds
    And the run asked nothing on the terminal
    And the run ran these programs, the checks' own left out:
      """
      dpkg-query -W -f ${db:Status-Abbrev}\n postgresql caddy
      runuser -u gophenberg -- /opt/gophenberg/current/gophenberg check
      systemctl daemon-reload
      systemctl enable gophenberg.service
      runuser -u caddy -- caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
      systemctl reload caddy.service
      systemctl reset-failed gophenberg.service
      systemctl restart gophenberg.service
      """
    And the current link still names the earlier build 0.19.0
    And /etc/caddy/Caddyfile is as the earlier run left it
    And the run ends with:
      """
      Step 14: print the summary and end the install record

      Gophenberg 0.19.0 runs at https://example.com. Sign in at https://example.com/admin/.
      The hosting provider's firewall must let TCP 80, TCP 443 and UDP 443 in for visitors to reach the site.
      Commands to know:
        sudo gophenberg update:status     shows the build that runs and the update on offer
        sudo gophenberg update:apply      installs the update on offer
        sudo journalctl -u gophenberg     shows what the site logged
        sudo gophenberg server:uninstall  removes Gophenberg and keeps its data
      """

  Scenario: A gophenberg command on the path that the installer did not write is refused
    Given another package put a gophenberg command at /usr/local/bin/gophenberg
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run stops with "this server already has a /usr/local/bin/gophenberg that server:install did not write, and server:install puts the gophenberg command there, so nothing was changed"
    And nothing on the server changed

  Scenario Outline: A later run whose Caddyfile lost a line an earlier run wrote is refused
    Given an earlier run installed Gophenberg for example.com on the server
    And the line "<line>" was taken out of /etc/caddy/Caddyfile
    When the operator runs "server:install -dry-run"
    Then the run stops with "/etc/caddy/Caddyfile lacks the line "<line>" that an earlier run wrote, <why>, and it must go back in before server:install runs again, so nothing was changed"
    And nothing on the server changed

    Examples:
      | line                               | why                                                                               |
      | import /etc/caddy/gophenberg.caddy | which has Caddy serve the site                                                    |
      | admin unix//run/caddy/admin.sock   | which keeps Caddy's admin address off the loopback port any local user can reach |

  Scenario Outline: A current link that names no build is refused
    Given an earlier run installed Gophenberg for example.com on the server
    And the current link of the install root <names>
    When the operator runs "server:install -dry-run"
    Then the run stops with "cannot tell which build /opt/gophenberg/current names: <why>, so nothing was changed"
    And nothing on the server changed

    Examples:
      | names                                                  | why                                                                                      |
      | names /opt/gophenberg/versions/0.19.0 by its full path | updater: the current link names /opt/gophenberg/versions/0.19.0, not a build in versions |
      | names versions/0.19.0, a build folder deleted by hand  | /opt/gophenberg/versions/0.19.0/gophenberg is not there                                  |
      | names versions/0.19.0, whose program nobody may run    | /opt/gophenberg/versions/0.19.0/gophenberg is not executable                             |

  Scenario: A first run installs Gophenberg and writes every file its dry run prints, with its owner and mode
    Given unattended-upgrades is installed on the server
    And the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the file /root/admin-password holds the line "correct horse battery staple"
    When the operator runs "server:install -dry-run -domain example.com -admin-email admin@example.com"
    And the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com -admin-name Maria -admin-password-file /root/admin-password"
    Then the run succeeds
    And the server holds every file the dry run printed, with its owner and mode
    And the server holds these folders:
      | folder                        | owner      | mode |
      | /var/lib/gophenberg-installer | root       | 0700 |
      | /etc/gophenberg               | root       | 0755 |
      | /opt/gophenberg               | gophenberg | 0750 |
      | /opt/gophenberg/versions      | gophenberg | 0755 |
      | /opt/gophenberg/downloads     | gophenberg | 0755 |
      | /opt/gophenberg/state         | gophenberg | 0700 |
      | /opt/gophenberg/backups       | gophenberg | 0700 |
      | /opt/gophenberg/media         | gophenberg | 0750 |
      | /opt/gophenberg/themes        | gophenberg | 0750 |
    And the server holds the install record, owned by root with mode 0600
    And the server holds the installer's own copy, owned by root with mode 0755
    And the build folder of this installer holds only the files of its archive, owned by gophenberg
    And the current link names the build of this installer, owned by gophenberg

  Scenario: A run cut off while it installed the packages runs dpkg --configure -a first
    Given a first run for example.com was cut off while apt-get installed PostgreSQL
    And the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the file /root/admin-password holds the line "correct horse battery staple"
    When the operator runs "server:install -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com -admin-name Maria -admin-password-file /root/admin-password"
    Then the run succeeds
    And the run ran "dpkg --configure -a" before "apt-get -o DPkg::Lock::Timeout=300 update"
    And the run ran "apt-get -o DPkg::Lock::Timeout=300 install -y --no-install-recommends postgresql caddy" after "apt-get -o DPkg::Lock::Timeout=300 update"
    And the install record marks the step packages ended

  Scenario: A run cut off after createuser takes the role that already exists as made
    Given a first run for example.com was cut off after createuser made the database role gophenberg
    And the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the file /root/admin-password holds the line "correct horse battery staple"
    When the operator runs "server:install -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com -admin-name Maria -admin-password-file /root/admin-password"
    Then the run succeeds
    And the run ran "runuser -u postgres -- createdb --owner=gophenberg --encoding=UTF8 --template=template0 --locale-provider=icu --icu-locale=und gophenberg" after "runuser -u postgres -- createuser --no-superuser --no-createdb --no-createrole gophenberg"
    And the install record marks the step database ended

  Scenario: A run cut off after the unpack removes the leftover build folder
    Given a first run for example.com was cut off after it unpacked part of the build of this installer
    And the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the file /root/admin-password holds the line "correct horse battery staple"
    When the operator runs "server:install -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com -admin-name Maria -admin-password-file /root/admin-password"
    Then the run succeeds
    And the build folder of this installer holds only the files of its archive, owned by gophenberg
    And the current link names the build of this installer, owned by gophenberg
    And the install record marks the step build ended

  Scenario: A run cut off after account:create-admin asks nothing on the next run
    Given an earlier run for example.com was cut off after account:create-admin made the first administrator admin@example.com
    And the operator's terminal answers:
      """
      admin@example.com
      """
    When the operator runs "server:install"
    Then the run succeeds
    And the run asked nothing on the terminal
    And the run ran "runuser -u gophenberg -- /opt/gophenberg/current/gophenberg account:list -json" in /opt/gophenberg
    And the run ran "runuser -u gophenberg -- /opt/gophenberg/current/gophenberg account:list -json" before "systemctl daemon-reload"
    And the run created no administrator
    And the install record marks the step admin ended
    And the install record marks the step summary ended

  Scenario: An administrator email an account under another role holds stops the run, and the next run asks again
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the file /root/admin-password holds the line "correct horse battery staple"
    And an account a person made by hand holds admin@example.com under the role editor
    And the operator's terminal answers:
      """
      maria.perez@example.com
      Maria Perez
      correct horse battery staple
      correct horse battery staple
      """
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com -admin-name Maria -admin-password-file /root/admin-password"
    Then the run stops with "admin@example.com already belongs to an account that is not an administrator, so run sudo gophenberg server:install again and give the first administrator another email"
    And the install record marks the step admin started
    When the operator runs "server:install"
    Then the run succeeds
    And the terminal showed:
      """
      Email of the first administrator:
      Name of the first administrator [maria.perez]:
      Password: (unseen)
      Password again: (unseen)
      """
    And the run ran "runuser -u gophenberg -- /opt/gophenberg/current/gophenberg account:create-admin -email maria.perez@example.com -name Maria Perez -role admin" in /opt/gophenberg, the line "correct horse battery staple" on its standard input
    And the install record marks the step admin ended

  Scenario: An administrator email an account under another role holds stops the run while another email is under admin
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the file /root/admin-password holds the line "correct horse battery staple"
    And an account a person made by hand holds owner@example.com under the role admin
    And an account a person made by hand holds admin@example.com under the role editor
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com -admin-name Maria -admin-password-file /root/admin-password"
    Then the run stops with "admin@example.com already belongs to an account that is not an administrator, so run sudo gophenberg server:install again and give the first administrator another email"
    And the install record marks the step admin started

  Scenario: A later run keeps the person's .env lines and writes a changed restart delay from server.env
    Given an earlier run installed Gophenberg for example.com on the server
    And the site's .env holds the line "GOPHENBERG_SITE_TITLE=Maria Perez's site"
    And the server settings set GOPHENBERG_SERVICE_RESTART_DELAY to 3s
    When the operator runs "server:install"
    Then the run succeeds
    And the server holds "/opt/gophenberg/.env", owned by gophenberg with mode 0600:
      """
      # Written by gophenberg server:install.
      GOPHENBERG_DATABASE_URL=postgres:///gophenberg?host=/var/run/postgresql&user=gophenberg
      GOPHENBERG_SUPERUSER_POLICY=refuse
      GOPHENBERG_INSTALL_ROOT=/opt/gophenberg
      GOPHENBERG_ADDR=127.0.0.1:8081
      GOPHENBERG_PUBLIC_URL=https://example.com
      GOPHENBERG_TRUSTED_PROXIES=127.0.0.1/32,::1/128
      GOPHENBERG_THEMES_DIR=/opt/gophenberg/themes
      GOPHENBERG_MEDIA_DIR=/opt/gophenberg/media
      GOPHENBERG_DATABASE_WAIT=60s
      GOPHENBERG_SITE_TITLE="Maria Perez's site"
      """
    And the site's unit on the server holds the line "RestartSec=3s"
    And the run ran "systemctl daemon-reload" before "systemctl restart gophenberg.service"

  Scenario: A run whose HTTPS check gets no answer warns, ends with exit 0 and checks again on the next run
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the file /root/admin-password holds the line "correct horse battery staple"
    And the hosting provider's firewall keeps TCP port 443 closed
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com -admin-name Maria -admin-password-file /root/admin-password"
    Then the run succeeds
    And the run notes "warning: https://example.com/admin/ did not answer 200 within 2m0s, the time GOPHENBERG_INSTALL_HTTPS_WAIT allows, as at the last try it could not be reached: dial tcp 203.0.113.7:443: i/o timeout. The site runs, so check that the hosting provider's firewall lets TCP 80, TCP 443 and UDP 443 in, then read sudo journalctl -u caddy, and the next sudo gophenberg server:install checks again"
    And the install record marks the step https started
    And the install record marks the step summary ended
    When the hosting provider's firewall lets TCP port 443 in
    And the operator runs "server:install"
    Then the run succeeds
    And the run notes nothing
    And the run checked https://example.com/admin/ trusting the system's root certificates
    And the install record marks the step https ended

  Scenario: A run whose site never answers stops, names the journal and leaves the start step open
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the file /root/admin-password holds the line "correct horse battery staple"
    And the site never answers on 127.0.0.1:8081
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com -admin-name Maria -admin-password-file /root/admin-password"
    Then the run stops with "gophenberg.service did not pass its start check within 1m0s, the time GOPHENBERG_UPDATE_VERIFY_TIMEOUT allows, as at the last try it could not ask for /admin/: dial tcp 127.0.0.1:8081: connect: connection refused, so read why with sudo journalctl -u gophenberg"
    And the install record marks the step start started
    And the run created no administrator

  Scenario: A run from the installer's own copy leaves the copy alone
    Given an earlier run installed Gophenberg for example.com on the server
    And the operator runs the installer's own copy
    When the operator runs "server:install"
    Then the run succeeds
    And the installer's own copy is the file the run ran, left as it was

  Scenario: A later run whose caddy validate fails keeps the person's Caddyfile on the next run
    Given an earlier run installed Gophenberg for example.com on the server
    And the person added a site to /etc/caddy/Caddyfile whose directive respnd caddy validate does not know
    When the operator runs "server:install"
    Then the run stops with "caddy validate refused Caddy's files, so Caddy runs on with the settings it has: installer: run caddy: exit status 1", followed by what the program printed
    And /etc/caddy/Caddyfile still holds the person's site
    And the install record marks the step caddy ended
    When the person fixes the typo in /etc/caddy/Caddyfile
    And the operator runs "server:install"
    Then the run succeeds
    And /etc/caddy/Caddyfile still holds the person's site
    And the run ran "systemctl reload caddy.service"
    And the run never ran "systemctl restart caddy.service"

  Scenario: A first run whose Caddy restart fails keeps the site the person adds to the Caddyfile on the next run
    Given the release archive of this installer is at /var/lib/gophenberg-installer/release.tar.gz
    And the file /root/admin-password holds the line "correct horse battery staple"
    And Caddy fails to restart on the server
    When the operator runs "server:install -domain example.com -archive /var/lib/gophenberg-installer/release.tar.gz -admin-email admin@example.com -admin-name Maria -admin-password-file /root/admin-password"
    Then the run stops with "installer: run systemctl: exit status 1", followed by what the program printed
    And the install record marks the step caddy started
    When Caddy restarts again on the server
    And the person adds the site example.org to /etc/caddy/Caddyfile
    And the operator runs "server:install -admin-email admin@example.com -admin-name Maria -admin-password-file /root/admin-password"
    Then the run succeeds
    And the run prints:
      """
      Step 11: write Caddy's files, check them with caddy validate and restart Caddy
      """
    And /etc/caddy/Caddyfile still holds the person's site
    And the run ran "systemctl restart caddy.service"
    And the run never ran "systemctl reload caddy.service"
    And the install record marks the step caddy ended
