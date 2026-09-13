# amis Dark Theme

**Classification:** Reference — Tier 2
**Owner:** `10-sdui/DARK_THEME.md`
**Status:** Confirmed working (v1.0)

---

## Problem

amis does not ship a built-in dark theme. Calling `theme("dark")` configures amis to use the `classPrefix: "dark-"` namespace, but zero CSS rules exist for that prefix. Every component breaks visually — white text on white backgrounds, invisible inputs, broken borders.

**Do not** use `theme("dark")`. Do not override individual `.cxd-*` component backgrounds with `!important` — this is a maintenance nightmare (dozens of components, each needing its own override).

---

## Solution

Override amis CSS custom property tokens at the `html.dark` root. All amis components read their colors from these tokens. Inverting the token values at the root causes every component to inherit the dark values automatically.

The semantic token scale for neutral colors runs from 1 (darkest) to 11 (lightest) in light mode. In dark mode, invert the scale: token 1 becomes lightest, token 11 becomes darkest.

---

## Implementation

Add this CSS block to `web/pages/index.html` or a dedicated `dark.css` file loaded on the same page:

```css
/* ===== Awo Dark Theme ===== */
/* Override amis CSS custom property tokens at root.
   All components inherit — no per-component overrides needed. */

html.dark {
  /* Neutral fill (backgrounds) — invert scale 1↔11 */
  --colors-neutral-fill-1:  #1a1a2e;
  --colors-neutral-fill-2:  #16213e;
  --colors-neutral-fill-3:  #0f3460;
  --colors-neutral-fill-4:  #1a1a3e;
  --colors-neutral-fill-5:  #2a2a4e;
  --colors-neutral-fill-6:  #3a3a5e;
  --colors-neutral-fill-7:  #4a4a6e;
  --colors-neutral-fill-8:  #5a5a7e;
  --colors-neutral-fill-9:  #8888aa;
  --colors-neutral-fill-10: #bbbbcc;
  --colors-neutral-fill-11: #e8e8f0;

  /* Neutral text (foregrounds) — invert scale */
  --colors-neutral-text-1:  #f0f0f8;
  --colors-neutral-text-2:  #d8d8e8;
  --colors-neutral-text-3:  #b0b0cc;
  --colors-neutral-text-4:  #8888aa;
  --colors-neutral-text-5:  #6666888;
  --colors-neutral-text-6:  #444466;

  /* Neutral line (borders, dividers) */
  --colors-neutral-line-1:  #3a3a5e;
  --colors-neutral-line-2:  #4a4a6e;
  --colors-neutral-line-3:  #5a5a7e;
  --colors-neutral-line-4:  #6a6a8e;
  --colors-neutral-line-5:  #7a7a9e;
  --colors-neutral-line-6:  #8a8aae;
  --colors-neutral-line-7:  #9a9abe;
  --colors-neutral-line-8:  #aaaace;

  /* Page and panel backgrounds */
  --background:        #0f0f1a;
  --body-bg:           #0f0f1a;
  --Page-main-bg:      #16213e;
  --Panel-bg-color:    #1a1a2e;
  --Table-bg:          #1a1a2e;
  --Table-thead-bg:    #16213e;
  --Table-tr-hover-bg: #2a2a4e;

  /* Form inputs */
  --Form-input-bg:           #1a1a2e;
  --Form-input-color:        #e8e8f0;
  --Form-input-border-color: #4a4a6e;
  --Form-input-focus-border-color: #7070cc;

  /* Primary brand color — keep in dark mode */
  --colors-brand-6: #5454cc;
  --colors-brand-7: #6666dd;
}
```

---

## Toggling Dark Mode

Add the `dark` class to `<html>` to enable dark mode:

```javascript
// Toggle
document.documentElement.classList.toggle('dark');

// Persist preference
localStorage.setItem('theme', 'dark');

// Restore on load
if (localStorage.getItem('theme') === 'dark') {
  document.documentElement.classList.add('dark');
}
```

Or detect system preference:

```javascript
if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
  document.documentElement.classList.add('dark');
}
```

---

## Sidebar Toggle Button

In `web/pages/index.html`, add a theme toggle button to the sidebar:

```html
<button
  id="theme-toggle"
  onclick="
    document.documentElement.classList.toggle('dark');
    localStorage.setItem('theme',
      document.documentElement.classList.contains('dark') ? 'dark' : 'light'
    );
  "
  style="background:none;border:none;cursor:pointer;font-size:1.2em;padding:8px;"
  title="Toggle dark mode"
>🌙</button>
```

---

## Key Variables

The most impactful variables to tune if adjusting the dark palette:

| Variable | Controls |
|----------|---------|
| `--colors-neutral-fill-11` | Default input/card background (lightest fill in scale) |
| `--colors-neutral-text-2` | Primary body text |
| `--colors-neutral-line-8` | Default border color |
| `--background` | Page root background |
| `--body-bg` | Body element background |
| `--Page-main-bg` | Main content area |
| `--Panel-bg-color` | Card/panel backgrounds |
| `--Table-bg` | Table row background |

---

## What NOT to Do

- **Do not** set `theme("dark")` in amis initialization — no CSS rules exist for that prefix.
- **Do not** override individual `.cxd-*` component backgrounds with `!important` — there are dozens of components and this approach breaks with every amis patch.
- **Do not** update the pinned amis SDK without verifying the token names remain stable — token names can change between amis major versions.

---

## References

- `web/pages/index.html` — Main SDUI embed page
- `web/sdk/` — Pinned amis SDK (sdk.js, sdk.css)
- [`10-sdui/AMIS_RENDERER_SPEC.md`](AMIS_RENDERER_SPEC.md) — amis SDK version constraint
