### Added

- **`Agreement.AcceptCounter`** accepts a counter-offer: it sends
  `{"counter": ..., "original_offer": ...}` to
  `POST /admin/agreements/accept`, with the counter `Counter` returned and
  the offer it counters. Needs a privileged key.

### Fixed

- **A counter-offer could not be accepted.** `Accept` said it accepted an
  offer or a counter-offer, but it always sends `{"offer": ...}`, and the NA
  refuses a counter sent that way (422 `accept_rejected`). `Accept` now says
  it takes an offer only; use `AcceptCounter` for a counter-offer.
