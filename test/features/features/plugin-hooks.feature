Feature: Plugins add tags to the head of the site's pages through a hook
  A hook is a named moment where the site lets plugins join in, the way
  WordPress plugins add to the page head through wp_head. Each plugin lists
  in its plugin.json the hooks it implements, under the key hooks, and the
  start checks that list against the plugin's methods. The site builds each
  hook once over the plugins that started, so a page calls only the plugins
  that implement it. The first hook, public.head-tags, lets a plugin add meta
  and link tags to the head of every page the built-in site draws. The site
  checks each tag and writes it itself. A plugin that fails, panics or runs
  out of time leaves the page whole, and the log names the plugin.

  Background:
    Given the settings point at an empty database
    And the compiled plugins "events" and "notes"

  @wip
  Scenario Outline: A plugin's head tag shows on the <page>
    Given "notes" adds a head tag naming the page
    When the site serves
    Then the head of "<address>" holds the meta tag "notes-page" saying "<says>" once

    Examples:
      | page           | address  | says              |
      | home listing   | /        | archive /         |
      | not found page | /missing | notfound /missing |

  @wip
  Scenario: The head tags of two plugins follow the order the plugins are compiled in
    Given "events" adds a head tag marking itself
    And "notes" adds a head tag marking itself
    When the site serves
    Then the head of "/" holds the mark of "events" before the mark of "notes"

  @wip
  Scenario: A page links no feed when no compiled plugin adds it
    When the site serves
    Then the head of "/" links no feed

  @wip
  Scenario: A plugin whose head tags fail leaves the page whole
    Given "events" adds a head tag marking itself
    And the head tags of "notes" fail with "no head today"
    When the site serves
    Then the head of "/" holds the mark of "events"
    And the site logged that the hook "public.head-tags" of "notes" failed with "no head today"

  @wip
  Scenario: A plugin whose head tags panic leaves the page whole
    Given "events" adds a head tag marking itself
    And the head tags of "notes" panic with "boom"
    When the site serves
    Then the head of "/" holds the mark of "events"
    And the site logged that the hook "public.head-tags" of "notes" panicked with "boom", with its stack

  @wip
  Scenario Outline: A head tag outside the rules is dropped and the plugin's other tags stay
    Given "notes" adds <tag> beside a head tag marking itself
    When the site serves
    Then the head of "/" holds the mark of "notes"
    And the head of "/" holds no tag carrying "<left out>"
    And the site logged that the hook "public.head-tags" of "notes" failed with "<error>"

    Examples:
      | tag                                    | left out    | error                                                                      |
      | a script tag                           | script      | dropped the head tag script: only meta and link are kept                   |
      | a link to "javascript:alert(1)"        | javascript: | dropped the head tag link: its href must be an http, https or site address |
      | a meta tag whose attribute is "on top" | on top      | dropped the head tag meta: its attribute "on top" is not a lowercase name  |
      | a meta tag whose attribute is "2col"   | 2col        | dropped the head tag meta: its attribute "2col" is not a lowercase name    |

  @wip
  Scenario: A plugin still adding head tags when the hook's time runs out leaves the page whole
    Given GOPHENBERG_PLUGIN_HOOK_BUDGET is "1ns"
    And the head tags of "notes" wait until the hook's time runs out
    When the site serves
    Then the page "/" answers 200
    And the site logged that the hook "public.head-tags" of "notes" ran out of its time

  @wip
  Scenario: A plugin paused at start adds no head tag
    Given "notes" adds a head tag marking itself
    And the Start of "notes" fails with "no route table"
    When the site serves
    Then the head of "/" holds no tag carrying "notes-mark"
