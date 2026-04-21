import * as React from "react";

import { cn } from "@/lib/utils";

function Table({ className, ...props }: React.ComponentProps<"table">) {
  return (
    <div data-slot="table-container" className="relative w-full overflow-x-auto">
      <table
        data-slot="table"
        className={cn(
          "w-full caption-bottom border-separate border-spacing-0 text-sm",
          className,
        )}
        {...props}
      />
    </div>
  );
}

function TableHeader({ className, ...props }: React.ComponentProps<"thead">) {
  return <thead data-slot="table-header" className={cn(className)} {...props} />;
}

function TableBody({ className, ...props }: React.ComponentProps<"tbody">) {
  return <tbody data-slot="table-body" className={cn(className)} {...props} />;
}

function TableFooter({ className, ...props }: React.ComponentProps<"tfoot">) {
  return (
    <tfoot
      data-slot="table-footer"
      className={cn(
        "border-t border-border-subtle bg-subtle font-medium",
        className,
      )}
      {...props}
    />
  );
}

// TableRow is row-only — it doesn't own row height or hover state.
// Row height goes on cells via utility (`h-11`). Focus/hover state is
// project-specific (keyboard nav + 2px accent left bar) so we keep it
// out of the primitive — callers set `data-focused` and a className.
function TableRow({ className, ...props }: React.ComponentProps<"tr">) {
  return <tr data-slot="table-row" className={cn(className)} {...props} />;
}

// Table header cell — uppercase muted label per the Linear-grade look.
// `h-9` fixes the header strip height; callers override via className if
// a column needs a different alignment (e.g., right-align for numbers).
function TableHead({ className, ...props }: React.ComponentProps<"th">) {
  return (
    <th
      data-slot="table-head"
      className={cn(
        "h-9 whitespace-nowrap border-b border-border-subtle px-4 text-left align-middle text-[11px] font-medium uppercase tracking-[0.06em] text-fg-4",
        className,
      )}
      {...props}
    />
  );
}

// Table body cell — row height is fixed at 44px; padding matches header.
// Vertical padding is 0 so the 44px comes from the `h-11` the row sets;
// if the row forgets, the 44px comes from the cell's `h-11` fallback.
function TableCell({ className, ...props }: React.ComponentProps<"td">) {
  return (
    <td
      data-slot="table-cell"
      className={cn(
        "h-11 border-b border-border-subtle px-4 align-middle whitespace-nowrap text-[13px] text-fg-1",
        className,
      )}
      {...props}
    />
  );
}

function TableCaption({
  className,
  ...props
}: React.ComponentProps<"caption">) {
  return (
    <caption
      data-slot="table-caption"
      className={cn("mt-4 text-sm text-fg-3", className)}
      {...props}
    />
  );
}

export {
  Table,
  TableHeader,
  TableBody,
  TableFooter,
  TableHead,
  TableRow,
  TableCell,
  TableCaption,
};
