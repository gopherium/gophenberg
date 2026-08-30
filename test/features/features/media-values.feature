Feature: Serving media field values
  A published item's media fields serve the file itself, its address, its
  words and its renditions, so a theme renders an image without a login.
  A file that is gone drops out rather than breaking the page.

  Background:
    Given a running Gophenberg with the default content types
    And a signed in administrator
    And the group "Extras" placed on "post"

  Scenario: A cover serves the file it points at
    Given the media field "cover" in "Extras"
    And the administrator uploads a 640 by 480 pixel JPEG named "sunrise.jpg"
    And the published post "Hello world"
    And the administrator saves the image named "sunrise" into "cover" of "Hello world"
    When a visitor resolves "/hello-world"
    Then the served field "cover" is one object addressing "sunrise.jpg"
    And the served field "cover" carries the size 640 by 480

  Scenario: The words on a file go public without the librarian's note
    Given the media field "cover" in "Extras"
    And the administrator uploads a 640 by 480 pixel JPEG named "sunrise.jpg"
    And the published post "Hello world"
    And the administrator saves the image named "sunrise" into "cover" of "Hello world"
    And the administrator describes the image "sunrise"
    When a visitor resolves "/hello-world"
    Then the served field "cover" carries the saved title, alt text and caption
    And the served field "cover" carries no description

  Scenario: A gallery keeps its stored order
    Given the many media field "gallery" in "Extras"
    And the administrator uploads a 640 by 480 pixel JPEG named "beach.jpg"
    And the administrator uploads a 640 by 480 pixel JPEG named "cliff.jpg"
    And the published post "Hello world"
    And the administrator saves the images named "cliff, beach" into "gallery" of "Hello world"
    When a visitor resolves "/hello-world"
    Then the served field "gallery" lists the addresses "cliff.jpg, beach.jpg" in that order

  Scenario: A file that is gone drops out of a gallery
    Given the many media field "gallery" in "Extras"
    And the administrator uploads a 640 by 480 pixel JPEG named "beach.jpg"
    And the administrator uploads a 640 by 480 pixel JPEG named "cliff.jpg"
    And the published post "Hello world"
    And the administrator saves the images named "cliff, beach" into "gallery" of "Hello world"
    And the administrator deletes the image named "cliff"
    When a visitor resolves "/hello-world"
    Then the served field "gallery" lists the addresses "beach.jpg" in that order

  Scenario: A cover whose file is gone leaves the field out
    Given the media field "cover" in "Extras"
    And the administrator uploads a 640 by 480 pixel JPEG named "sunrise.jpg"
    And the published post "Hello world"
    And the administrator saves the image named "sunrise" into "cover" of "Hello world"
    And the administrator deletes the image named "sunrise"
    When a visitor resolves "/hello-world"
    Then the served fields carry no "cover"

  Scenario: A term's own item serves its media
    Given the type "category" labeled "Category" and "Categories" under "categories" serving term pages
    And the group "Category art" placed on "category"
    And the media field "portrait" in "Category art"
    And the administrator uploads a 640 by 480 pixel JPEG named "banner.jpg"
    And the published category "News"
    And the administrator saves the image named "banner" into "portrait" of "News"
    When a visitor resolves "/categories/news"
    Then the served field "portrait" is one object addressing "banner.jpg"

  Scenario: Editing the words on a file changes the item's validator
    Given the media field "cover" in "Extras"
    And the administrator uploads a 640 by 480 pixel JPEG named "sunrise.jpg"
    And the published post "Hello world"
    And the administrator saves the image named "sunrise" into "cover" of "Hello world"
    And a visitor remembers the validator of "/hello-world"
    When the administrator describes the image "sunrise"
    Then resolving "/hello-world" answers a changed validator

  Scenario: An animated GIF serves its address and no renditions
    Given the media field "cover" in "Extras"
    And the administrator uploads an animated GIF named "wave.gif"
    And the published post "Hello world"
    And the administrator saves the image named "wave" into "cover" of "Hello world"
    When a visitor resolves "/hello-world"
    Then the served field "cover" is one object addressing "wave.gif"
    And the served field "cover" carries no renditions
