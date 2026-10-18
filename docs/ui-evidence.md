# UI requirement evidence

The shared Google Workspace styled frontend is used by the published schedule, CRUD forms, dashboard, reports, and download controls. Automated evidence comes from `frontend/tests/design.spec.js`, `business.js`, `workflows.js`, and `downloads.js`, executed with Chromium and actual Keycloak identities against isolated CI services. Authentication screenshots, traces, and videos remain disabled. Only fixed guest fixtures produce desktop/phone previews; CI retains them for seven days.

| Requirement | Implementation and proof |
| --- | --- |
| U01: at least two breakpoints | CSS uses 640/1024/1280px; public checks exercise 320, 375, 390, 639, 640, 768, 1023, 1024, and 1440px in both themes. CRUD/report journeys check 320/768/1440px. |
| U02: responsive images | `img { max-width: 100%; height: auto }`; visible campus SVG illustration accompanies empty schedule results. Empty-state and narrow-layout tests exercise it. |
| U03: distinct Header/Content/Footer | Header has brand/menu/theme/account controls; Content has heading/account/filters/results/actions; Footer has project/schedule/repository information. Their computed background colors are asserted distinct. |
| U04: varied data-entry controls | API-backed forms use text/search, date/time, number, checkbox, select/linked selectors, and textarea controls. All eight CRUD form journeys create, edit, validate, and delete related fixtures. |
| U05: animation | Menu, dialogs, cards, and controls have brief transitions/appearance animation. Reduced-motion checks assert animations are disabled while interactions still work. |
| U06: responsive menu | Persistent desktop navigation becomes an expandable menu below 1024px. Tests verify boundaries, keyboard activation, Escape dismissal, and returned menu focus. |
| U07: vector icons | Shared decorative SVG Icon component covers navigation, actions, calendar, account, and downloads, with accessible names on the owning controls. |
| U08: non-standard font | Roboto is requested through Google Fonts and configured in the shared theme, with Arial/system fallback when offline. Light/dark public previews retain readable text; external-font availability depends on network access. |
| U09: informational modal | Native schedule-information dialog, request conflict/decision dialog, clear confirmation, audit details, and private-backup notice. Tests assert title/initial focus, keyboard containment, Escape/cancellation, restored focus, and error recovery. |
| U10: stable alignment | Grid/flex layout, rounded containers, and labeled horizontal scroll regions for tables/calendar. Layout tests reject document overflow across supported widths and long errors. |
| U11: visible/accessibly operable items | Visible focus, large primary controls, contrast checks in both themes, labels/captions/status/alerts, reduced motion, keyboard-only dialog/menu checks, and 200% desktop zoom equivalent using a 720px CSS layout viewport. Wide results remain in labeled keyboard-focusable scroll regions. |
| U12: consistent forms | Shared field labels/help/required markers/errors, submit/cancel placement, retained input on failure, bounded linked selectors, immutable reviewed records, and repeated-submit prevention. Real browser tests verify duplicate errors, request conflict recovery, deletion focus, backup failure/retry, and role-specific controls. |

These are targeted functional and accessibility checks, not a claim of full WCAG conformance. Firefox/Safari, physical mobile devices, actual browser zoom controls, and manual screen-reader testing remain unverified. The zoom check models the CSS viewport of a 1440px desktop screen at 200%; it does not automate the browser's zoom menu.
