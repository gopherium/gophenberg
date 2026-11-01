Feature: The site listens where its host expects
  A small hosting plan starts a site through its own app slot. The slot
  names a PORT or talks to a private socket, every account on the server
  can reach a loopback port, and the host stops a program that sits idle.
  A site listens on GOPHENBERG_SOCKET, else on GOPHENBERG_ADDR, else on
  every interface at PORT, else on localhost:8081. Only the site's own
  user can open the socket, so the program in front of it is the one peer
  the site believes over it. That program names the visitor in the header
  GOPHENBERG_CLIENT_ADDR_HEADER names, and the scheme in
  X-Forwarded-Proto, on every request. The site believes nothing else
  forwarded over the socket. A theme and the first start of an update
  reach the site over TCP, so a site on a socket runs neither yet. With
  GOPHENBERG_IDLE_EXIT set, a site with nothing open for that long exits
  with code 0, and the host starts it again at the next visit.

  Scenario: A site on a socket answers over it
    Given the site listens on a socket with the visitor's address in "X-Client-Addr"
    When the site serves
    Then only the site's own user can open the socket
    And a request over the socket from the visitor "203.0.113.7" is answered

  Scenario: A request over the socket that names no visitor is refused
    Given the site listens on a socket with the visitor's address in "X-Client-Addr"
    When the site serves
    Then a request over the socket that names no visitor answers 400

  Scenario: The socket wins over the address
    Given the site listens on a socket with the visitor's address in "X-Client-Addr"
    And the settings name a free address
    When the site serves
    Then a request over the socket from the visitor "203.0.113.7" is answered
    And nothing answers on the address

  Scenario: The address wins over PORT
    Given the settings name a free address
    And the host names a free PORT
    When the site serves
    Then the site answers on the address
    And nothing answers on the PORT

  Scenario: A site with no address listens on the PORT its host names
    Given the host names a free PORT
    When the site serves
    Then the site answers on the PORT

  Scenario: A write over the socket that names its scheme passes the origin check
    Given the site listens on a socket with the visitor's address in "X-Client-Addr"
    When the site serves
    And an older browser on "https://example.com" signs in over the socket, naming the scheme "https"
    Then the write is not refused as cross-origin

  Scenario: A write over the socket that names no scheme is refused
    Given the site listens on a socket with the visitor's address in "X-Client-Addr"
    When the site serves
    And an older browser on "https://example.com" signs in over the socket, naming no scheme
    Then the write is refused as cross-origin

  Scenario: A forwarded host over the socket is not believed
    Given the site listens on a socket with the visitor's address in "X-Client-Addr"
    When the site serves
    And an older browser on "https://forged.example" signs in over the socket, naming the forwarded host "forged.example"
    Then the write is refused as cross-origin

  Scenario: A site with nothing open exits by itself
    Given the settings name a free address
    And the setting "GOPHENBERG_IDLE_EXIT" holds "300ms"
    When the site starts
    Then the site exits with code 0 by itself

  Scenario: A site with no idle exit keeps serving
    Given the settings name a free address
    When the site serves
    Then the site answers on the address
    And the site is still serving

  Scenario Outline: Settings that cannot serve stop check
    Given the settings hold <settings>
    When the operator runs "check"
    Then the command fails, saying "<error>"

    Examples:
      | settings                                                                                                  | error                                                                                                                |
      | "GOPHENBERG_SOCKET=/run/site.sock"                                                                        | GOPHENBERG_CLIENT_ADDR_HEADER is required to serve on a socket                                                       |
      | "GOPHENBERG_SOCKET=/run/site.sock, GOPHENBERG_CLIENT_ADDR_HEADER=X-Client-Addr, GOPHENBERG_THEMES_DIR=/x" | GOPHENBERG_SOCKET: a theme reaches the site over TCP, so leave GOPHENBERG_THEMES_DIR empty                           |
      | "GOPHENBERG_SOCKET=/run/site.sock, GOPHENBERG_CLIENT_ADDR_HEADER=X-Client-Addr, GOPHENBERG_INSTALL_ROOT=/x" | GOPHENBERG_SOCKET: the first start of an update checks the site over TCP, so leave GOPHENBERG_INSTALL_ROOT empty |
      | "GOPHENBERG_IDLE_EXIT=soon"                                                                               | GOPHENBERG_IDLE_EXIT: must be a duration like 30s                                                                    |
      | "PORT=70000"                                                                                              | PORT: must stand at or below 65535                                                                                   |
