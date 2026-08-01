# Functional Motion Guidelines

Date: 2026-07-31  
Executor: Codex

## Scenario: Transaction And State Feedback Motion

### 1. Scope / Trigger

- Trigger: frontend work that animates transaction dialogs, keyed list updates,
  publish or order results, status changes, steppers, or event timelines.
- Motion exists to clarify state and spatial continuity. Decorative loops,
  route-wide transitions, parallax, and promotional animation are outside this
  contract.

### 2. Signatures

```ts
import type { AutoAnimateOptions } from '@formkit/auto-animate'

export const functionalMotion = {
  duration: 180,
  easing: 'ease-out',
} satisfies Partial<AutoAnimateOptions>
```

```vue
<div v-auto-animate="functionalMotion">
  <Card v-for="item in rows" :key="item.id" />
</div>

<SoftTable animate-rows :columns="columns">
  <tr v-for="item in rows" :key="item.id" />
</SoftTable>
```

### 3. Contracts

- Use CSS transitions or Vue `Transition` for simple visual state changes.
- Use `@formkit/auto-animate` only for insertion, removal, reorder, or
  state-driven layout changes among immediate keyed children.
- Import and reuse `functionalMotion` from `@/lib/motion`; do not define
  page-local AutoAnimate durations or easing.
- `SoftTable` animation is opt-in through `animateRows`. Shared tables remain
  static unless their owner explicitly enables row motion.
- Preserve stable business identifiers as keys. Motion must not alter sorting,
  pagination, query state, row navigation, or keyboard behavior.
- Compose transaction overlays from the existing shadcn-vue `Dialog` and
  `Checkbox` primitives. `tw-animate-css` supplies their existing state-class
  animations; do not add custom overlay implementations.
- Show Sonner success feedback immediately after a successful mutation and
  before awaited cache refresh or navigation. Do not add artificial delays or
  a second success surface.
- Status detail pages animate only the affected status, action, stepper, or
  timeline region. Do not attach AutoAnimate to a route root or full page.
- Keep AutoAnimate's user-motion preference enabled. Global CSS must collapse
  non-essential animation and transition duration under
  `prefers-reduced-motion: reduce` while leaving the resulting state visible.

### 4. Validation And Error Matrix

| Condition | Expected behavior |
| --- | --- |
| A keyed row is inserted, removed, filtered, or reordered | The local list parent animates; row identity and final order remain unchanged. |
| `animateRows` is omitted from `SoftTable` | The table renders one ordinary `TableBody` without AutoAnimate. |
| A transaction dialog opens | Reka/shadcn focus management, Escape dismissal, overlay, and enter/exit classes remain active. |
| A mutation succeeds before a slow invalidation | Success toast appears immediately; invalidation and navigation still complete. |
| An order or application status changes | Only local status/action/timeline regions animate; unrelated page content does not replay. |
| Reduced motion is requested | Non-essential movement completes effectively immediately; state text and actions remain understandable. |
| AutoAnimate is unavailable during SSR | Static SSR output remains usable and client enhancement activates after hydration. |

### 5. Good / Base / Bad Cases

- Good: a buyer order filter removes keyed cards from a local AutoAnimate
  parent and the settled layout has no absolute outgoing children.
- Good: a status badge uses the status as its key, while the step indicator
  uses the shared CSS transition class.
- Base: a static settings table does not enable `animateRows`.
- Bad: attaching AutoAnimate to the application shell, a whole route, or a
  container whose children have index keys.
- Bad: defining another motion dependency, a page-local duration, or a custom
  modal overlay for the same transaction workflow.
- Bad: delaying navigation with `setTimeout` so a success animation can finish.

### 6. Tests Required

- Source contract tests assert dependency/module wiring, shared timing,
  reduced-motion CSS, official dialog composition, keyed call sites, opt-in
  table behavior, and success-before-refresh ordering.
- Run full frontend Vitest, Nuxt typecheck, and a real-backend-mode Nuxt build.
- Browser acceptance covers 1440x900 and 390x844: dialog focus and fit, no
  horizontal overflow, list removal settlement, status transitions, and
  immediate success feedback.
- Build warnings from third-party plugins must be reported; do not suppress
  them in project code.

### 7. Wrong Vs Correct

#### Wrong

```vue
<main v-auto-animate="{ duration: 600 }">
  <div v-for="(item, index) in rows" :key="index">{{ item.title }}</div>
</main>
```

#### Correct

```vue
<div v-auto-animate="functionalMotion">
  <Card v-for="item in rows" :key="item.id">{{ item.title }}</Card>
</div>
```

#### Wrong

```ts
await queryClient.invalidateQueries({ queryKey: ['orders'] })
toast.success('订单已创建。')
```

#### Correct

```ts
toast.success('订单已创建。')
await queryClient.invalidateQueries({ queryKey: ['orders'] })
```
