Feature: Remove Gophenberg from a Linux server
  sudo gophenberg server:uninstall removes what server:install set up and
  keeps the site's data. It reads its settings from
  /etc/gophenberg/server.env alone, never from the environment or the
  site's .env. It stops on a server with no install record, on a run with
  -yes while another server:install or server:uninstall is running, and
  at a Caddyfile the person changed that is a link while its import line
  has to come out. It lists what it removes and what it keeps, and
  without -yes it changes nothing. With
  -yes it stops and removes the site's service, takes Gophenberg's site
  out of Caddy, and removes the command on the path, the installer's own
  copy, the builds, the downloads and the links. It keeps the database and
  its role, the install root with the site's .env, media, themes and
  backups, server.env, the install record, the system user gophenberg and
  the packages. A Caddyfile still as server:install wrote it goes back to
  the one the caddy package wrote, and a Caddyfile the person changed
  stays and loses only its import line, so a later server:install asks
  for that line back and never writes over it. The install record keeps
  the sum of that Caddyfile before Caddy's files change, and Caddy
  restarts on it where server:install never ended its caddy step, else
  it reloads. With -delete-data it also drops the
  database and its role and removes the install root, server.env and the
  user, and it removes an install root only when the install record shows
  that server:install made it. Each folder the run empties goes too,
  /usr/local/lib/gophenberg, /usr/local/share/gophenberg and with
  -delete-data /etc/gophenberg, and the list names the folder in place of
  its last entry. A folder that holds anything else stays, with what it
  holds. The record keeps the domain, since the
  kept Caddy still holds port 80, so a later install for that domain
  skips the domain check. A unit systemd already let go of once the site
  stopped counts as cleared. Each piece already gone is skipped, so a
  second run finds nothing left to remove. The install record shows that
  the Caddyfile is going back before it does, so a run cut off part way
  is finished by the next one, which restarts or reloads Caddy where the
  cut-off run did not.

  Background:
    Given a fresh Ubuntu 24.04 server run by systemd 255

  Scenario: A plain user is refused
    Given server:install installed Gophenberg for example.com on the server
    And the operator is not root
    When the operator runs "server:uninstall -yes"
    Then the run stops with "server:uninstall must run as root, such as with sudo, so nothing was changed"
    And nothing on the server changed

  Scenario: A server with no install record has nothing to remove
    When the operator runs "server:uninstall -yes"
    Then the run stops with "this server has no install record at /var/lib/gophenberg-installer/install.json, so server:uninstall has nothing to remove"
    And nothing on the server changed

  Scenario: An installer folder another user can write is refused
    Given server:install installed Gophenberg for example.com on the server
    And another user can write /var/lib/gophenberg-installer
    When the operator runs "server:uninstall -yes"
    Then the run stops with "/var/lib/gophenberg-installer, which holds the install record, must be a real folder owned by root and writable by root alone, so nothing was changed"
    And nothing on the server changed

  Scenario: A run with -yes while another run is going is refused, and a run without -yes still lists
    Given server:install installed Gophenberg for example.com on the server
    And another server:install is running on the server
    When the operator runs "server:uninstall -yes"
    Then the run stops with "another server:install or server:uninstall is running on this server, so nothing was changed"
    And nothing on the server changed
    When the operator runs "server:uninstall"
    Then the run succeeds

  Scenario: A server whose Caddyfile and the caddy package's one are both gone is refused
    Given server:install installed Gophenberg for example.com on the server
    And the person deleted /etc/caddy/Caddyfile
    And the person deleted /etc/caddy/Caddyfile.before-gophenberg
    When the operator runs "server:uninstall -yes"
    Then the run stops with "server:uninstall needs /etc/caddy/Caddyfile to take Gophenberg's site out of Caddy, and it is not there, so nothing was changed"
    And nothing on the server changed

  Scenario: A Caddyfile the person changed that is a link to another file holding the import line is refused
    Given server:install installed Gophenberg for example.com on the server
    And the person added the site example.org to /etc/caddy/Caddyfile
    And the person moved /etc/caddy/Caddyfile to /etc/caddy/main.caddy and left a link to it in its place
    When the operator runs "server:uninstall -yes"
    Then the run stops with "/etc/caddy/Caddyfile is a link, and the line "import /etc/caddy/gophenberg.caddy" must come out of the file it leads to before server:uninstall runs again, so nothing was changed"
    And nothing on the server changed
    And /etc/caddy/Caddyfile is still a link to /etc/caddy/main.caddy
    And /etc/caddy/main.caddy still holds the line "import /etc/caddy/gophenberg.caddy"

  Scenario: Without -yes it lists what it removes and keeps and changes nothing
    Given server:install installed Gophenberg for example.com on the server
    When the operator runs "server:uninstall"
    Then the run succeeds
    And the run prints:
      """
      server:uninstall removes:
        gophenberg.service, stopped and disabled
        /etc/systemd/system/gophenberg.service
        /etc/systemd/system/gophenberg-rollback.service
        /etc/caddy/Caddyfile, putting back /etc/caddy/Caddyfile.before-gophenberg, the one the caddy package wrote
        /etc/systemd/system/caddy.service.d/gophenberg.conf
        /etc/caddy/gophenberg.caddy
        /usr/local/share/gophenberg
        /usr/local/bin/gophenberg
        /usr/local/lib/gophenberg
        /opt/gophenberg/versions
        /opt/gophenberg/downloads
        /opt/gophenberg/current
      server:uninstall keeps:
        the database gophenberg and its role gophenberg
        the install root /opt/gophenberg, with the site's .env, media, themes and backups
        /etc/gophenberg/server.env
        the system user gophenberg
        /var/lib/gophenberg-installer/install.json, so a later install picks up what is kept
        the packages server:install installed, which sudo apt-get purge postgresql caddy removes
        /etc/apt/keyrings/caddy.asc, /etc/apt/sources.list.d/caddy.sources and /etc/apt/preferences.d/gophenberg-caddy, which bring Caddy's updates from its own apt repository
      """
    And nothing on the server changed

  Scenario: With -yes it removes the service, Caddy's site, the command and the builds and keeps the data
    Given server:install installed Gophenberg for example.com on the server
    And the file /opt/gophenberg/media/2026/10/photo.jpg holds the line "a photo of Maria Perez's garden"
    When the operator runs "server:uninstall -yes"
    Then the run succeeds
    And the run ran these programs, the checks' own left out:
      """
      systemctl disable --now gophenberg.service
      systemctl reset-failed gophenberg.service gophenberg-rollback.service
      systemctl daemon-reload
      systemctl daemon-reload
      systemctl restart caddy.service
      """
    And the run ends with:
      """
      server:uninstall removed what it lists and kept the rest.
      """
    And the server holds nothing at these paths:
      """
      /etc/systemd/system/gophenberg.service
      /etc/systemd/system/gophenberg-rollback.service
      /etc/systemd/system/caddy.service.d/gophenberg.conf
      /etc/caddy/Caddyfile.before-gophenberg
      /etc/caddy/gophenberg.caddy
      /usr/local/share/gophenberg/maintenance
      /usr/local/share/gophenberg
      /usr/local/bin/gophenberg
      /usr/local/lib/gophenberg/gophenberg
      /usr/local/lib/gophenberg
      /opt/gophenberg/versions
      /opt/gophenberg/downloads
      /opt/gophenberg/current
      """
    And the server still holds these paths:
      """
      /opt/gophenberg/.env
      /opt/gophenberg/media/2026/10/photo.jpg
      /opt/gophenberg/themes
      /opt/gophenberg/backups
      /etc/gophenberg/server.env
      /var/lib/gophenberg-installer/install.json
      /etc/apt/keyrings/caddy.asc
      /etc/apt/sources.list.d/caddy.sources
      /etc/apt/preferences.d/gophenberg-caddy
      """
    And the server holds /etc/caddy/Caddyfile as the caddy package wrote it
    And the server still has the system user gophenberg
    And the install record marks only these steps ended: server-env, packages, user, database, admin
    And the install record owns only these pieces: postgresql, caddy, /opt/gophenberg
    And the install record holds no SHA-256 of a Caddyfile

  Scenario: Units systemd already let go of once the site stopped count as cleared, and the run goes on
    Given server:install installed Gophenberg for example.com on the server
    And systemd lets go of the site's units once the site stops
    When the operator runs "server:uninstall -yes"
    Then the run succeeds
    And the run ran these programs, the checks' own left out:
      """
      systemctl disable --now gophenberg.service
      systemctl reset-failed gophenberg.service gophenberg-rollback.service
      systemctl daemon-reload
      systemctl daemon-reload
      systemctl restart caddy.service
      """
    And the run ends with:
      """
      server:uninstall removed what it lists and kept the rest.
      """
    And the server holds nothing at these paths:
      """
      /etc/systemd/system/gophenberg.service
      /etc/systemd/system/gophenberg-rollback.service
      /etc/caddy/gophenberg.caddy
      /usr/local/bin/gophenberg
      /opt/gophenberg/versions
      """

  Scenario: It keeps a Caddyfile the person changed, with the drop-in, and takes out only the import line
    Given server:install installed Gophenberg for example.com on the server
    And the person added the site example.org to /etc/caddy/Caddyfile
    When the operator runs "server:uninstall -yes"
    Then the run succeeds
    And the run prints:
      """
      server:uninstall removes:
        gophenberg.service, stopped and disabled
        /etc/systemd/system/gophenberg.service
        /etc/systemd/system/gophenberg-rollback.service
        the line import /etc/caddy/gophenberg.caddy of /etc/caddy/Caddyfile
        /etc/caddy/gophenberg.caddy
      """
    And the run prints:
      """
      server:uninstall keeps:
        /etc/caddy/Caddyfile, which changed since server:install wrote it, so a reinstall asks for that line back
        /etc/systemd/system/caddy.service.d/gophenberg.conf, which gives Caddy the folder of the admin socket its Caddyfile names
        /etc/caddy/Caddyfile.before-gophenberg, the Caddyfile the caddy package wrote
        the database gophenberg and its role gophenberg
      """
    And /etc/caddy/Caddyfile still holds the person's site
    And /etc/caddy/Caddyfile lacks the line "import /etc/caddy/gophenberg.caddy"
    And the server still holds these paths:
      """
      /etc/systemd/system/caddy.service.d/gophenberg.conf
      /etc/caddy/Caddyfile.before-gophenberg
      """
    And the run ran "systemctl reload caddy.service"
    And the run never ran "systemctl restart caddy.service"
    And the install record still holds the SHA-256 of the Caddyfile server:install wrote
    And the install record marks the step caddy ended

  Scenario: A Caddyfile the person changed keeps its mode, owner and group once the import line comes out
    Given server:install installed Gophenberg for example.com on the server
    And the person added the site example.org to /etc/caddy/Caddyfile
    And the person gave /etc/caddy/Caddyfile the group caddy and mode 0640
    When the operator runs "server:uninstall -yes"
    Then the run succeeds
    And /etc/caddy/Caddyfile lacks the line "import /etc/caddy/gophenberg.caddy"
    And /etc/caddy/Caddyfile still belongs to root and the group caddy with mode 0640

  Scenario: A reinstall over a Caddyfile the person changed stops at the import line and never writes over it
    Given server:install installed Gophenberg for example.com on the server
    And the person added the site example.org to /etc/caddy/Caddyfile
    When the operator runs "server:uninstall -yes"
    And the operator runs "server:install -archive /var/lib/gophenberg-installer/release.tar.gz"
    Then the run stops with "/etc/caddy/Caddyfile lacks the line "import /etc/caddy/gophenberg.caddy" that an earlier run wrote, which has Caddy serve the site, and it must go back in before server:install runs again, so nothing was changed"
    And nothing on the server changed
    When the person puts the line "import /etc/caddy/gophenberg.caddy" back in /etc/caddy/Caddyfile
    And the operator runs "server:install -archive /var/lib/gophenberg-installer/release.tar.gz"
    Then the run succeeds
    And /etc/caddy/Caddyfile still holds the person's site
    And the run ran "systemctl reload caddy.service"
    And the run never ran "systemctl restart caddy.service"

  Scenario: With -delete-data it also drops the database, its role, the install root, server.env and the user
    Given server:install installed Gophenberg for example.com on the server
    When the operator runs "server:uninstall -delete-data -yes"
    Then the run succeeds
    And the run ran these programs, the checks' own left out:
      """
      systemctl disable --now gophenberg.service
      systemctl reset-failed gophenberg.service gophenberg-rollback.service
      systemctl daemon-reload
      systemctl daemon-reload
      systemctl restart caddy.service
      runuser -u postgres -- dropdb gophenberg
      runuser -u postgres -- dropuser gophenberg
      userdel gophenberg
      """
    And the run prints:
      """
        /opt/gophenberg/current
        the database gophenberg and its role gophenberg
        the install root /opt/gophenberg and everything in it
        /etc/gophenberg
        the system user gophenberg
      server:uninstall keeps:
        /var/lib/gophenberg-installer/install.json, holding the packages server:install installed, so a later install takes them as its own
        the packages server:install installed, which sudo apt-get purge postgresql caddy removes
      """
    And the server holds nothing at these paths:
      """
      /opt/gophenberg
      /etc/gophenberg/server.env
      /etc/gophenberg
      """
    And the server has no system user gophenberg
    And the install record marks no step
    And the install record owns only these pieces: postgresql, caddy

  Scenario: A folder holding a file the person put there stays, with its file
    Given server:install installed Gophenberg for example.com on the server
    And the file /usr/local/lib/gophenberg/notes.txt holds the line "Maria Perez's notes"
    And the file /usr/local/share/gophenberg/notes.txt holds the line "Maria Perez's notes"
    And the file /etc/gophenberg/notes.txt holds the line "Maria Perez's notes"
    When the operator runs "server:uninstall -delete-data -yes"
    Then the run succeeds
    And the run prints:
      """
        /etc/caddy/gophenberg.caddy
        /usr/local/share/gophenberg/maintenance
        /usr/local/bin/gophenberg
        /usr/local/lib/gophenberg/gophenberg
      """
    And the run prints:
      """
        the install root /opt/gophenberg and everything in it
        /etc/gophenberg/server.env
      """
    And the server holds nothing at these paths:
      """
      /usr/local/share/gophenberg/maintenance
      /usr/local/lib/gophenberg/gophenberg
      /etc/gophenberg/server.env
      """
    And the server still holds these paths:
      """
      /usr/local/lib/gophenberg/notes.txt
      /usr/local/share/gophenberg/notes.txt
      /etc/gophenberg/notes.txt
      """

  Scenario: On a server that keeps its own web server it leaves Caddy alone
    Given Caddy is installed on the server
    And the person's own Caddy serves example.com from /etc/caddy/Caddyfile
    And server:install installed Gophenberg for example.com with -web-server none on the server
    When the operator runs "server:uninstall -yes"
    Then the run succeeds
    And the run ran these programs, the checks' own left out:
      """
      systemctl disable --now gophenberg.service
      systemctl reset-failed gophenberg.service gophenberg-rollback.service
      systemctl daemon-reload
      """
    And /etc/caddy/Caddyfile is as the person wrote it
    And the run prints:
      """
        the packages server:install installed, which sudo apt-get purge postgresql removes
      server:uninstall removed what it lists and kept the rest.
      """

  Scenario: A second run with -yes finds nothing left to remove
    Given server:install installed Gophenberg for example.com on the server
    When the operator runs "server:uninstall -yes"
    And the operator runs "server:uninstall -yes"
    Then the run succeeds
    And the run prints:
      """
      server:uninstall finds nothing left to remove.
      server:uninstall keeps:
        the database gophenberg and its role gophenberg
      """
    And nothing on the server changed

  Scenario: A reinstall after server:uninstall reuses the kept database and asks nothing
    Given server:install installed Gophenberg for example.com on the server
    And the operator's terminal answers:
      """
      admin@example.com
      """
    When the operator runs "server:uninstall -yes"
    And the operator runs "server:install -archive /var/lib/gophenberg-installer/release.tar.gz"
    Then the run succeeds
    And the run asked nothing on the terminal
    And the run ran these programs, the checks' own left out:
      """
      dpkg-query -W -f ${db:Status-Abbrev}\n postgresql caddy
      runuser -u gophenberg -- /opt/gophenberg/current/gophenberg check
      systemctl daemon-reload
      systemctl enable gophenberg.service
      runuser -u caddy -- caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile
      systemctl restart caddy.service
      systemctl reset-failed gophenberg.service
      systemctl start gophenberg.service
      """
    And the run created no administrator
    And the current link names the build of this installer, owned by gophenberg
    And the install record holds the SHA-256 of the Caddyfile the run wrote

  Scenario: After server:uninstall, a reinstall with the web server line changed to none goes through and leaves Caddy to the person
    Given server:install installed Gophenberg for example.com on the server
    And the operator ran "server:uninstall -yes"
    And the web server line of the server settings was changed by hand to none
    When the operator runs "server:install -archive /var/lib/gophenberg-installer/release.tar.gz"
    Then the run succeeds
    And the run never ran "systemctl restart caddy.service"
    And the run never ran "systemctl reload caddy.service"
    And the server holds /etc/caddy/Caddyfile as the caddy package wrote it
    And the server holds nothing at /etc/caddy/gophenberg.caddy
    And the server holds nothing at /etc/systemd/system/caddy.service.d/gophenberg.conf

  Scenario: A GOPHENBERG_INSTALL_ROOT line planted in the site's .env never steers -delete-data
    Given server:install installed Gophenberg for example.com on the server
    And the site's .env holds the line "GOPHENBERG_INSTALL_ROOT=/srv/planted"
    And the file /srv/planted/notes.txt holds the line "Maria Perez's notes"
    When the operator runs "server:uninstall -delete-data -yes"
    Then the run succeeds
    And the server holds nothing at /opt/gophenberg
    And the server still holds these paths:
      """
      /srv/planted/notes.txt
      """

  Scenario: A GOPHENBERG_INSTALL_ROOT set in the environment never steers -delete-data
    Given server:install installed Gophenberg for example.com on the server
    And the environment sets GOPHENBERG_INSTALL_ROOT to /srv/planted
    And the file /srv/planted/notes.txt holds the line "Maria Perez's notes"
    When the operator runs "server:uninstall -delete-data -yes"
    Then the run succeeds
    And the server holds nothing at /opt/gophenberg
    And the server still holds these paths:
      """
      /srv/planted/notes.txt
      """

  Scenario: A second -delete-data after server.env is gone removes no folder
    Given the environment sets GOPHENBERG_INSTALL_ROOT to /srv/gophenberg
    And server:install installed Gophenberg for example.com on the server
    And the operator ran "server:uninstall -delete-data -yes"
    And the file /opt/gophenberg/notes.txt holds the line "Maria Perez's notes"
    When the operator runs "server:uninstall -delete-data -yes"
    Then the run succeeds
    And the server holds nothing at /srv/gophenberg
    And the run prints:
      """
      server:uninstall finds nothing left to remove.
      server:uninstall keeps:
        /opt/gophenberg, since the install record does not show that server:install made it
      """
    And the server still holds these paths:
      """
      /opt/gophenberg/notes.txt
      """

  Scenario: An install root the record owns that server.env no longer names is listed as kept and leaves the record
    Given server:install installed Gophenberg for example.com on the server
    And the server settings set GOPHENBERG_INSTALL_ROOT to /srv/gophenberg
    When the operator runs "server:uninstall -delete-data -yes"
    Then the run succeeds
    And the run prints:
      """
      server:uninstall keeps:
        /opt/gophenberg, an install root server:install made, kept since /etc/gophenberg/server.env does not name it
      """
    And the server still holds these paths:
      """
      /opt/gophenberg/.env
      """
    And the install record owns only these pieces: postgresql, caddy

  Scenario: A reinstall after -delete-data behind NAT skips the domain check while the kept Caddy holds port 80
    Given server:install installed Gophenberg for example.com on the server
    And the operator ran "server:uninstall -delete-data -yes"
    And something listens on TCP port 80
    And example.com points at 198.51.100.7, a public address a NAT in front of the server forwards to it
    When the operator runs "server:install -dry-run -domain example.com"
    Then the run succeeds

  Scenario: A first run cut off in the caddy step leaves the installer's own Caddyfile, which server:uninstall puts back as the package's
    Given a first run for example.com was cut off in the caddy step once it wrote the Caddyfile
    When the operator runs "server:uninstall -yes"
    Then the run succeeds
    And the run prints:
      """
        /etc/caddy/Caddyfile, putting back /etc/caddy/Caddyfile.before-gophenberg, the one the caddy package wrote
      """
    And the run ran "systemctl restart caddy.service"
    And the server holds /etc/caddy/Caddyfile as the caddy package wrote it
    And the server holds nothing at /etc/caddy/Caddyfile.before-gophenberg
    And the install record holds no SHA-256 of a Caddyfile

  Scenario: A Caddyfile the person changed after a first run cut off in the caddy step stays, and Caddy restarts on it
    Given a first run for example.com was cut off in the caddy step once it wrote the Caddyfile
    And the person added the site example.org to /etc/caddy/Caddyfile
    When the operator runs "server:uninstall -yes"
    Then the run succeeds
    And the run prints:
      """
      server:uninstall keeps:
        /etc/caddy/Caddyfile, which changed since server:install wrote it, so a reinstall asks for that line back
      """
    And /etc/caddy/Caddyfile still holds the person's site
    And /etc/caddy/Caddyfile lacks the line "import /etc/caddy/gophenberg.caddy"
    And the run ran "systemctl restart caddy.service"
    And the run never ran "systemctl reload caddy.service"
    And the install record still holds the SHA-256 of the Caddyfile server:install wrote
    And the install record marks the step caddy ended

  Scenario: A Caddyfile the person wrote in place of the one a cut-off first run wrote stays, and a reinstall asks for the import line
    Given a first run for example.com was cut off in the caddy step once it wrote the Caddyfile
    And the person's own Caddy serves example.com from /etc/caddy/Caddyfile
    When the operator runs "server:uninstall -yes"
    Then the run succeeds
    And the run prints:
      """
      server:uninstall keeps:
        /etc/caddy/Caddyfile, which changed since server:install wrote it, so a reinstall asks for the line import /etc/caddy/gophenberg.caddy in it
      """
    And /etc/caddy/Caddyfile is as the person wrote it
    And the run ran "systemctl restart caddy.service"
    And the install record still holds the SHA-256 of the Caddyfile server:install wrote
    When the operator runs "server:install -archive /var/lib/gophenberg-installer/release.tar.gz"
    Then the run stops with "/etc/caddy/Caddyfile lacks the line "import /etc/caddy/gophenberg.caddy" that an earlier run wrote, which has Caddy serve the site, and it must go back in before server:install runs again, so nothing was changed"
    And nothing on the server changed
    And /etc/caddy/Caddyfile is as the person wrote it

  Scenario: A reinstall after a cut-off first run never writes over the Caddyfile the person changed, even once server:uninstall stopped at Caddy
    Given a first run for example.com was cut off in the caddy step once it wrote the Caddyfile
    And the person added the site example.org to /etc/caddy/Caddyfile
    And server:uninstall -yes was cut off at its first "systemctl restart caddy.service"
    When the operator runs "server:install -archive /var/lib/gophenberg-installer/release.tar.gz"
    Then the run stops with "/etc/caddy/Caddyfile lacks the line "import /etc/caddy/gophenberg.caddy" that an earlier run wrote, which has Caddy serve the site, and it must go back in before server:install runs again, so nothing was changed"
    And nothing on the server changed
    And /etc/caddy/Caddyfile still holds the person's site

  Scenario: A run cut off once the Caddyfile went back is finished by the next run, and a reinstall writes the Caddyfile again
    Given server:install installed Gophenberg for example.com on the server
    And server:uninstall -yes was cut off at its second "systemctl daemon-reload"
    When the operator runs "server:uninstall -yes"
    Then the run succeeds
    And the run prints:
      """
      server:uninstall removes:
        Gophenberg's settings in Caddy, restarting Caddy on /etc/caddy/Caddyfile, the one the caddy package wrote
        /etc/caddy/gophenberg.caddy
      """
    And the run ran these programs, the checks' own left out:
      """
      systemctl daemon-reload
      systemctl restart caddy.service
      """
    And the server holds /etc/caddy/Caddyfile as the caddy package wrote it
    And the server holds nothing at /etc/systemd/system/caddy.service.d/gophenberg.conf
    And the install record marks only these steps ended: server-env, packages, user, database, admin
    And the install record holds no SHA-256 of a Caddyfile
    When the operator runs "server:install -archive /var/lib/gophenberg-installer/release.tar.gz"
    Then the run succeeds
    And the run ran "systemctl restart caddy.service"
    And the server holds /etc/caddy/Caddyfile.before-gophenberg as the caddy package wrote it
    And the install record holds the SHA-256 of the Caddyfile the run wrote

  Scenario: A run cut off before Caddy reloaded a Caddyfile the person changed has the next run reload it
    Given server:install installed Gophenberg for example.com on the server
    And the person added the site example.org to /etc/caddy/Caddyfile
    And server:uninstall -yes was cut off at its first "systemctl reload caddy.service"
    When the operator runs "server:uninstall -yes"
    Then the run succeeds
    And the run ran these programs, the checks' own left out:
      """
      systemctl reload caddy.service
      """
    And the server holds nothing at /etc/caddy/gophenberg.caddy
    And /etc/caddy/Caddyfile still holds the person's site
    And /etc/caddy/Caddyfile lacks the line "import /etc/caddy/gophenberg.caddy"
    And the install record still holds the SHA-256 of the Caddyfile server:install wrote
