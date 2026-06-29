import type { VariantProps } from "class-variance-authority"
import { cva } from "class-variance-authority"

export { default as Badge } from "./Badge.vue"

export const badgeVariants = cva(
  "inline-flex items-center justify-center rounded-full border px-2 py-0.5 text-xs font-medium w-fit whitespace-nowrap shrink-0 [&>svg]:size-3 gap-1 [&>svg]:pointer-events-none focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-3 aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive transition-[color,box-shadow] overflow-hidden",
  {
    variants: {
      variant: {
        default:
          "border-transparent bg-primary text-primary-foreground [a&]:hover:bg-primary/90",
        secondary:
          "border-primary/15 bg-primary/10 text-primary [a&]:hover:bg-primary/15",
        destructive:
         "border-transparent bg-destructive text-white [a&]:hover:bg-destructive/90 focus-visible:ring-destructive/20 dark:focus-visible:ring-destructive/40 dark:bg-destructive/60",
        outline:
          "text-foreground [a&]:hover:bg-accent [a&]:hover:text-accent-foreground",
        identity:
          "border-transparent bg-foreground text-background [a&]:hover:bg-foreground/90",
        verified:
          "border-success/20 bg-success/10 text-success [a&]:hover:bg-success/15",
        trust:
          "border-primary/20 bg-primary/10 text-primary [a&]:hover:bg-primary/15",
        capability:
          "border-info/20 bg-info/10 text-info [a&]:hover:bg-info/15",
        model:
          "border-signal/20 bg-signal-soft text-foreground [a&]:hover:bg-accent",
        status:
          "border-primary/15 bg-secondary text-secondary-foreground [a&]:hover:bg-secondary/90",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
)
export type BadgeVariants = VariantProps<typeof badgeVariants>
