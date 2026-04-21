import {
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  OctagonXIcon,
  TriangleAlertIcon,
} from "lucide-react";
import { Toaster as Sonner, type ToasterProps } from "sonner";

// Gospa's theme is dark-first via `data-theme="dark"` on <html>; we
// don't ship a runtime theme switcher, so hardcode "dark" here and
// point Sonner at our own tokens instead of shadcn's stock
// --popover* variables.
const Toaster = ({ ...props }: ToasterProps) => (
  <Sonner
    theme="dark"
    className="toaster group"
    icons={{
      success: <CircleCheckIcon className="size-4" />,
      info: <InfoIcon className="size-4" />,
      warning: <TriangleAlertIcon className="size-4" />,
      error: <OctagonXIcon className="size-4" />,
      loading: <Loader2Icon className="size-4 animate-spin" />,
    }}
    toastOptions={{
      classNames: {
        toast:
          "bg-subtle! text-fg-1! border-border-default! shadow-lg! rounded-lg!",
        description: "text-fg-3!",
        actionButton: "bg-fg-1! text-app!",
        cancelButton: "bg-transparent! text-fg-2!",
        success: "text-success!",
        error: "text-danger!",
        warning: "text-warning!",
        info: "text-accent-hover!",
      },
    }}
    {...props}
  />
);

export { Toaster };
