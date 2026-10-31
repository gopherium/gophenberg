Feature: Uploading media files
  A signed in administrator builds the site's media library by
  uploading files. Every accepted image is stored ready to embed,
  with upright pixels and the image sizes the editor offers.

  Background:
    Given a running Gophenberg with an empty media directory
    And a signed in administrator

  Scenario: Uploading a photo derives the image sizes the editor offers
    When the administrator uploads a 2400 by 1600 pixel JPEG named "harbor.jpg"
    Then the library lists one image named "harbor"
    And the image offers the sizes "thumbnail", "medium", "large" and "full"
    And every size fits its bounding box

  Scenario: A sideways phone photo is stored upright
    When the administrator uploads a JPEG carrying orientation tag 6
    Then the stored image is taller than it is wide
    And the stored image carries no orientation tag

  Scenario: An oversized photo gains a scaled display copy
    When the administrator uploads a 3000 by 2000 pixel JPEG named "cliff.jpg"
    Then the full size is at most 2560 pixels wide
    And the original upload is kept on disk

  @wip
  Scenario: A sideways phone photo keeps its upload and gains an upright copy
    When the administrator uploads a JPEG carrying orientation tag 6
    Then the image is taller than it is wide
    And the full size is stored as "sideways-rotated.jpg"
    And the full size measures 20 by 40 pixels
    And the full size carries no orientation tag
    And the original upload is kept on disk

  @wip
  Scenario: A sideways photo past 2560 pixels is turned while it is scaled
    When the administrator uploads a 3000 by 2000 pixel JPEG carrying orientation tag 6 named "cliff.jpg"
    Then the image is taller than it is wide
    And the full size is stored as "cliff-scaled.jpg"
    And the full size measures 1707 by 2560 pixels
    And the large size measures 683 by 1024 pixels
    And the medium size measures 200 by 300 pixels
    And the thumbnail size measures 150 by 150 pixels
    And the full size carries no orientation tag
    And the original upload is kept on disk

  @wip
  Scenario: A thin panorama keeps its thumbnail though its scaled copy is thinner
    When the administrator uploads a 3000 by 160 pixel JPEG named "ridge.jpg"
    Then the image offers the sizes "thumbnail", "medium", "large" and "full"
    And the full size measures 2560 by 137 pixels
    And the thumbnail size measures 150 by 150 pixels

  Scenario: An animated GIF keeps its animation
    When the administrator uploads an animated GIF named "loader.gif"
    Then the library lists one image named "loader"
    And the image offers no sizes
    And the stored file is byte for byte the upload

  Scenario: A plain file is stored without image sizes
    When the administrator uploads a PDF named "manual.pdf"
    Then the library lists one file named "manual"
    And the stored file is byte for byte the upload

  Scenario: Two uploads may carry the same name
    When the administrator uploads a valid JPEG named "team.jpg" twice
    Then the library lists two images named "team"
    And their stored files differ

  Scenario Outline: An upload that cannot be trusted is refused whole
    When the administrator uploads a file that <flaw>
    Then the upload is refused explaining <reason>
    And the library holds nothing
    And the media directory holds no trace of the upload

    Examples:
      | flaw                                        | reason                       |
      | carries a type outside the allowed set      | the file type is not allowed |
      | is an SVG document                          | the file type is not allowed |
      | names a JPEG but carries executable content | the content does not match   |
      | exceeds the upload size cap                 | the file is too large        |
      | decodes to more pixels than the budget      | the image is too large       |
      | is a corrupt image                          | the image cannot be read     |

  Scenario Outline: A refused upload names what the administrator can change
    When the administrator uploads a file that <flaw>
    Then the request is refused with the code "<code>"
    And the error names "<value>" under "<detail>"

    Examples:
      | flaw                                         | code                        | detail    | value      |
      | carries a type outside the allowed set       | file_type_not_allowed       | extension | txt        |
      | is an SVG document                           | file_type_not_allowed       | extension | svg        |
      | names a JPEG but carries executable content  | file_content_mismatch       | extension | jpg        |
      | names a JPEG but carries executable content  | file_content_mismatch       | detected  | text/plain |
      | decodes to more pixels than the budget       | image_pixel_budget_exceeded | width     | 20000      |
      | decodes to more pixels than the budget       | image_pixel_budget_exceeded | height    | 20000      |
      | decodes to more pixels than the budget       | image_pixel_budget_exceeded | max       | 80000000   |
      | is an animation with a frame past the budget | image_frame_too_large       | max       | 80000000   |

  Scenario: A file named without an extension is refused without naming one
    When the administrator uploads a file that carries no extension
    Then the request is refused with the code "file_type_not_allowed"
    And the error carries no data

  Scenario: Uploading without a session is refused
    Given no administrator is signed in
    When a visitor posts a file to the media endpoint
    Then the request is refused as unauthenticated
