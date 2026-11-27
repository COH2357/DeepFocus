# DeepFocus Guard extension

This Manifest V3 extension gives DeepFocus real URL-level control in Edge and Chrome.

## Install

1. Install DeepFocus (the installer copies this folder to `%APPDATA%\focusd\extension` and registers `com.deepfocus.host`).
2. Open `edge://extensions` or `chrome://extensions`.
3. Enable **Developer mode**.
4. Choose **Load unpacked** and select `%APPDATA%\focusd\extension`.
5. Keep the extension enabled. DeepFocus shows it as connected in Settings → Strong web supervision.

The native host manifest is registered by `deepfocus_native.exe --install`; the extension ID is fixed by the `key` field in `manifest.json`.

## Behaviour

- Every top-level `http(s)` navigation asks the native host first.
- Strong mode: sites outside the search/homepage allowlist require a reason and an allow duration.
- Normal mode: existing per-site limits are enforced at navigation time.
- An allowance applies to the whole site class and all its tabs/subdomains until it expires.
- When the native host is unavailable, the extension shows a local “service not running” pause page instead of silently allowing the site.

Firefox is not packaged yet; the native host manifest is registered for Chrome and Edge.
