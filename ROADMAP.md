# Roadmap

Ideas that are intentionally **not** implemented yet. They are recorded here so
the design can be reviewed before any enforcement code is written.

## Friction-based limits (future)

Today a limit only raises a notification after the threshold is crossed. A future
version could turn limits into actual friction:

- **Application limits.** When a limited app is launched, route the user to a
  built-in DeepFocus page instead of letting the app open normally. The page
  shows today's usage, the limit, and a short prompt that encourages the user to
  stop or postpone.
- **Website limits.** Match the active browser URL against the same site classes
  used by experimental domain grouping (for example all `bilibili.com` pages and
  subdomains count as one class). A limited class opens a DeepFocus interstitial
  page rather than the site.
- **Deliberate escape hatch.** Enforcement must never trap the user. After
  repeated, explicit confirmation ("I understand this breaks my limit"), the app
  or site can be opened anyway.
- **Higher-level record.** Every bypass should be written as a distinct,
  higher-severity event: which limit, which app/site class, when, and how many
  confirmations were needed. The dashboard can then show "limits broken" next to
  "limits respected" instead of hiding the exception.

### Open design questions

- Browser URL visibility needs an extension or accessibility/UI Automation
  integration; reading only the window title is not reliable for every browser.
- Enforcement needs a watchdog that survives the GUI being closed, while still
  allowing a documented, user-approved bypass.
- Interstitial pages must not become a single point of failure: if the helper is
  unavailable, the user keeps access and the event is logged as "unprotected".
- The bypass flow must be resistant to accidental double-clicks, not to the user
  themselves. This is a self-management tool, not a parental-control product.
