Feature: Install Gophenberg with one command
  curl -fsSL https://updates.gophenberg.org/install.sh | sudo sh -s --
  --domain example.com installs Gophenberg on a Linux server. install.sh
  is a POSIX sh script that names one release and the SHA-256 of each of
  its two Linux archives. It checks that the copy arrived whole, that root
  runs it, that the copy names a release, that no argument names an
  archive, that the server is an x86_64 or aarch64 machine with curl,
  OpenSSL 3 and tar, and that /var/lib/gophenberg-installer is a real
  folder root alone may write, made with mode 0700 when it is missing.
  Then it downloads the archive and its .sig over HTTPS into a fresh
  download folder inside it, checks the archive against its SHA-256 and
  against the release key or the standby, and runs the archive's
  gophenberg as server:install with that archive and every argument as
  given, from / and with standard input from /dev/null. It ends with
  server:install's exit code and removes the download folder when it
  ends, after Ctrl-C too. A check that fails stops the run with a line
  that says why, and nothing was installed.

  Background:
    Given install.sh names Gophenberg 0.20.0 and the checksum of each Linux archive
    And the download address serves the linux/amd64 archive the release key signed
    And the server is an x86_64 machine

  Scenario: install.sh hands the checked archive to server:install with the arguments as given
    When the operator runs install.sh as root with these arguments:
      | --domain     |
      | example.com  |
      | --admin-name |
      | Maria Perez  |
      | -dry-run     |
    Then server:install ran from /, from a download folder inside /var/lib/gophenberg-installer, with these arguments:
      | server:install                                         |
      | -archive                                               |
      | <download folder>/gophenberg_0.20.0_linux_amd64.tar.gz |
      | --domain                                               |
      | example.com                                            |
      | --admin-name                                           |
      | Maria Perez                                            |
      | -dry-run                                               |
    And server:install read its standard input from /dev/null
    And install.sh printed:
      """
      Downloading Gophenberg 0.20.0 for linux/amd64
      The archive matches its checksum and a Gophenberg release key signed it
      """
    And install.sh ended with exit code 0
    And no download folder is left in /var/lib/gophenberg-installer
    And install.sh made nothing outside /var/lib/gophenberg-installer

  Scenario: An archive the standby key signed is taken
    Given the download address serves the linux/amd64 archive the standby key signed
    When the operator runs install.sh as root with "--domain example.com"
    Then server:install ran on the archive gophenberg_0.20.0_linux_amd64.tar.gz
    And install.sh ended with exit code 0
    And no download folder is left in /var/lib/gophenberg-installer

  Scenario: An aarch64 server gets the linux/arm64 archive
    Given the server is an aarch64 machine
    And the download address serves the linux/arm64 archive the release key signed
    When the operator runs install.sh as root with "--domain example.com"
    Then server:install ran on the archive gophenberg_0.20.0_linux_arm64.tar.gz
    And install.sh printed:
      """
      Downloading Gophenberg 0.20.0 for linux/arm64
      The archive matches its checksum and a Gophenberg release key signed it
      """
    And install.sh ended with exit code 0

  Scenario: install.sh downloads over HTTPS alone and gives up on a download that stalls for 60 seconds
    When the operator runs install.sh as root with "--domain example.com"
    Then curl downloaded each file with these arguments:
      | -fsSL | --proto | =https | --proto-redir | =https | --tlsv1.2 | --speed-limit | 1 | --speed-time | 60 | -o | <download folder>/gophenberg_0.20.0_linux_amd64.tar.gz     | https://github.com/gopherium/gophenberg/releases/download/v0.20.0/gophenberg_0.20.0_linux_amd64.tar.gz     |
      | -fsSL | --proto | =https | --proto-redir | =https | --tlsv1.2 | --speed-limit | 1 | --speed-time | 60 | -o | <download folder>/gophenberg_0.20.0_linux_amd64.tar.gz.sig | https://github.com/gopherium/gophenberg/releases/download/v0.20.0/gophenberg_0.20.0_linux_amd64.tar.gz.sig |

  Scenario: A copy of install.sh cut off right after the word main runs nothing
    Given the copy of install.sh ends right after the word main of its last line
    When the operator runs install.sh as root with "-dry-run"
    Then install.sh stops with "this copy of install.sh was cut off, so nothing was installed"
    And nothing was downloaded
    And server:install never ran

  Scenario: A run as a plain user is refused
    When the operator runs install.sh as a plain user with "--domain example.com"
    Then install.sh stops with "run it as root, such as with sudo sh -c 'curl -fsSL https://updates.gophenberg.org/install.sh | sh -s -- --domain example.com'"
    And nothing was downloaded
    And server:install never ran

  Scenario: A copy of install.sh that names no release is refused
    Given install.sh still names no release
    When the operator runs install.sh as root with "--domain example.com"
    Then install.sh stops with "this copy of install.sh names no release, so run it from https://updates.gophenberg.org/install.sh"
    And nothing was downloaded
    And server:install never ran

  Scenario Outline: install.sh refuses an archive given on its command line
    When the operator runs install.sh as root with "<arguments>"
    Then install.sh stops with "install.sh passes the archive it checked itself, so leave out -archive"
    And nothing was downloaded
    And server:install never ran

    Examples:
      | arguments                                                              |
      | --domain example.com -archive /var/lib/gophenberg-installer/a.tar.gz   |
      | --domain example.com --archive /var/lib/gophenberg-installer/a.tar.gz  |
      | --domain example.com -archive=/var/lib/gophenberg-installer/a.tar.gz   |
      | --domain example.com --archive=/var/lib/gophenberg-installer/a.tar.gz  |

  Scenario: A machine Gophenberg ships no archive for is refused
    Given the server is a riscv64 machine
    When the operator runs install.sh as root with "--domain example.com"
    Then install.sh stops with "Gophenberg ships for x86_64 and aarch64 servers, and this server is riscv64"
    And nothing was downloaded
    And server:install never ran

  Scenario Outline: A server without <tool> is refused, naming the package to add
    Given the server has no <tool>
    When the operator runs install.sh as root with "--domain example.com"
    Then install.sh stops with "install.sh needs <tool>, so add it with sudo apt-get install <tool>"
    And nothing was downloaded
    And server:install never ran

    Examples:
      | tool    |
      | curl    |
      | openssl |
      | tar     |

  Scenario Outline: OpenSSL older than 3 is refused
    Given openssl version prints "<line>"
    When the operator runs install.sh as root with "--domain example.com"
    Then install.sh stops with "checking the download needs OpenSSL 3 or newer, and this server has <found>"
    And nothing was downloaded
    And server:install never ran

    Examples:
      | line                        | found          |
      | OpenSSL 1.1.1w  11 Sep 2023 | OpenSSL 1.1.1w |
      | LibreSSL 3.8.2              | LibreSSL 3.8.2 |

  Scenario: A missing installer folder is made for root alone
    Given /var/lib/gophenberg-installer does not exist yet
    When the operator runs install.sh as root with "--domain example.com"
    Then /var/lib/gophenberg-installer is a folder with mode 0700
    And server:install ran on the archive gophenberg_0.20.0_linux_amd64.tar.gz
    And install.sh ended with exit code 0
    And no download folder is left in /var/lib/gophenberg-installer

  Scenario Outline: An installer folder that is not root's alone is refused
    Given /var/lib/gophenberg-installer is <what>
    When the operator runs install.sh as root with "--domain example.com"
    Then install.sh stops with "/var/lib/gophenberg-installer, which holds the install record, must be a real folder owned by root and writable by root alone, so nothing was installed"
    And nothing was downloaded
    And server:install never ran
    And no download folder is left in /var/lib/gophenberg-installer

    Examples:
      | what                       |
      | a link to another folder   |
      | a folder another user owns |
      | a folder with mode 0770    |
      | a folder with mode 0707    |
      | a plain file               |

  Scenario Outline: A download that cannot reach GitHub names the address and IPv4
    Given curl cannot reach the download address and exits with code <code>
    When the operator runs install.sh as root with "--domain example.com"
    Then install.sh stops with "cannot download https://github.com/gopherium/gophenberg/releases/download/v0.20.0/gophenberg_0.20.0_linux_amd64.tar.gz, so nothing was installed. GitHub's downloads need IPv4, which a server with IPv6 alone cannot reach" after curl's own error
    And server:install never ran
    And no download folder is left in /var/lib/gophenberg-installer

    Examples:
      | code |
      | 6    |
      | 7    |

  Scenario Outline: A file the release does not hold names its address alone
    Given the download address serves no <file>
    When the operator runs install.sh as root with "--domain example.com"
    Then install.sh stops with "cannot download https://github.com/gopherium/gophenberg/releases/download/v0.20.0/<file>, so nothing was installed" after curl's own error
    And server:install never ran
    And no download folder is left in /var/lib/gophenberg-installer

    Examples:
      | file                                     |
      | gophenberg_0.20.0_linux_amd64.tar.gz     |
      | gophenberg_0.20.0_linux_amd64.tar.gz.sig |

  Scenario: An archive with another checksum is refused
    Given the download address serves another linux/amd64 archive the release key signed
    When the operator runs install.sh as root with "--domain example.com"
    Then install.sh stops with "the archive is not the one this install.sh names, so nothing was installed"
    And server:install never ran
    And no download folder is left in /var/lib/gophenberg-installer

  Scenario: An archive neither release key signed is refused
    Given the download address serves the linux/amd64 archive signed by a key that is neither release key
    When the operator runs install.sh as root with "--domain example.com"
    Then install.sh stops with "the archive's signature matches neither Gophenberg release key, so nothing was installed"
    And server:install never ran
    And no download folder is left in /var/lib/gophenberg-installer

  Scenario: An empty signature is refused
    Given the archive's .sig is empty
    When the operator runs install.sh as root with "--domain example.com"
    Then install.sh stops with "the archive's signature matches neither Gophenberg release key, so nothing was installed"
    And server:install never ran
    And no download folder is left in /var/lib/gophenberg-installer

  Scenario: A signature that is not base64 is refused
    Given the archive's .sig holds the text "%%% this is not a signature %%%"
    When the operator runs install.sh as root with "--domain example.com"
    Then install.sh stops with "the archive's signature matches neither Gophenberg release key, so nothing was installed"
    And server:install never ran
    And no download folder is left in /var/lib/gophenberg-installer

  Scenario: An archive without the gophenberg binary is refused
    Given the download address serves a linux/amd64 archive without the gophenberg binary, which install.sh names and the release key signed
    When the operator runs install.sh as root with "--domain example.com"
    Then install.sh stops with "cannot unpack gophenberg from gophenberg_0.20.0_linux_amd64.tar.gz, so nothing was installed" after tar's own error
    And server:install never ran
    And no download folder is left in /var/lib/gophenberg-installer

  Scenario: A server:install that fails ends install.sh with its exit code
    Given server:install exits with code 3
    When the operator runs install.sh as root with "--domain example.com"
    Then server:install ran on the archive gophenberg_0.20.0_linux_amd64.tar.gz
    And install.sh ended with exit code 3
    And no download folder is left in /var/lib/gophenberg-installer

  Scenario: Ctrl-C while server:install runs leaves no download folder
    When the operator runs install.sh as root with "--domain example.com" and presses Ctrl-C while server:install runs
    Then install.sh ended with exit code 130
    And no download folder is left in /var/lib/gophenberg-installer
