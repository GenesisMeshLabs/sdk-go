### Fixed

- An integer longer than 20 digits is refused as `integer_out_of_range`
  before it is parsed, as the reference does. A response or record holding
  an integer of millions of digits took seconds to refuse.
