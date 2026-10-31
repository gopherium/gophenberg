Feature: The pixel budget follows the server's setting
  A picture takes far more memory once decoded than its file takes, so
  the media library refuses a picture, or one frame of a GIF, holding
  more pixels than the budget. The budget is
  GOPHENBERG_MEDIA_MAX_MEGAPIXELS, 80 megapixels unless the server
  names another, and the error names it so the administrator knows how
  far to shrink the picture.

  Background:
    Given the pixel budget is 2 megapixels
    And a running Gophenberg with an empty media directory
    And a signed in administrator

  Scenario: A photo exactly at the budget is stored
    When the administrator uploads a 2000 by 1000 pixel JPEG named "field.jpg"
    Then the library lists one image named "field"

  Scenario Outline: A picture past the budget names the budget it broke
    When the administrator uploads a <upload>
    Then the request is refused with the code "<code>"
    And the error names "<value>" under "<detail>"

    Examples:
      | upload                                                                          | code                        | detail | value   |
      | 2000 by 1001 pixel JPEG named "wide.jpg"                                        | image_pixel_budget_exceeded | width  | 2000    |
      | 2000 by 1001 pixel JPEG named "wide.jpg"                                        | image_pixel_budget_exceeded | height | 1001    |
      | 2000 by 1001 pixel JPEG named "wide.jpg"                                        | image_pixel_budget_exceeded | max    | 2000000 |
      | GIF named "big.gif" holding a 1500 by 1500 pixel frame on a 1 by 1 pixel canvas | image_frame_too_large       | max    | 2000000 |
