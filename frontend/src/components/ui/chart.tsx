"use client";

import * as RechartsPrimitive from "recharts";

import * as React from "react";

import { cn } from "@/lib/cn";

// The label and color of each series in a chart, by its data key
export type ChartConfig = Record<string, { label?: React.ReactNode; color?: string }>;

const ChartContext = React.createContext<{ config: ChartConfig } | null>(null);

function useChart() {
  const context = React.useContext(ChartContext);
  if (!context) {
    throw new Error("useChart must be used within a <ChartContainer />");
  }
  return context;
}

// ChartContainer sizes a chart to its parent, and sets each series' color as --color-<key>
export function ChartContainer({
  id,
  className,
  children,
  config,
  ...props
}: React.ComponentProps<"div"> & {
  config: ChartConfig;
  children: React.ComponentProps<typeof RechartsPrimitive.ResponsiveContainer>["children"];
}) {
  const uniqueId = React.useId();
  const chartId = `chart-${id || uniqueId.replace(/:/g, "")}`;

  return (
    <ChartContext.Provider value={{ config }}>
      <div
        data-slot="chart"
        data-chart={chartId}
        className={cn(
          "flex aspect-video justify-center text-xs",
          "[&_.recharts-cartesian-axis-tick_text]:fill-muted-foreground",
          "[&_.recharts-cartesian-grid_line[stroke='#ccc']]:stroke-border/50",
          "[&_.recharts-curve.recharts-tooltip-cursor]:stroke-border",
          "[&_.recharts-rectangle.recharts-tooltip-cursor]:fill-muted",
          "[&_.recharts-polar-grid_[stroke='#ccc']]:stroke-border",
          "[&_.recharts-layer]:outline-hidden [&_.recharts-surface]:outline-hidden [&_.recharts-sector]:outline-hidden",
          className
        )}
        {...props}
      >
        <ChartStyle id={chartId} config={config} />
        <RechartsPrimitive.ResponsiveContainer>{children}</RechartsPrimitive.ResponsiveContainer>
      </div>
    </ChartContext.Provider>
  );
}

function ChartStyle({ id, config }: { id: string; config: ChartConfig }) {
  const colors = Object.entries(config).filter(([, item]) => item.color);
  if (colors.length === 0) return null;
  return (
    <style
      dangerouslySetInnerHTML={{
        __html: `[data-chart=${id}] {\n${colors.map(([key, item]) => `  --color-${key}: ${item.color};`).join("\n")}\n}`,
      }}
    />
  );
}

export const ChartTooltip = RechartsPrimitive.Tooltip;

type ChartPayloadItem = {
  dataKey?: string | number | ((obj: unknown) => unknown);
  name?: string | number;
  value?: number | string | readonly (number | string)[];
  color?: string;
  payload?: Record<string, unknown>;
};

// ChartTooltipContent is the tooltip of a chart, with each series' label from the config
export function ChartTooltipContent({
  active,
  payload,
  label,
  labelFormatter,
  valueFormatter,
  hideLabel = false,
  className,
}: {
  active?: boolean;
  payload?: readonly ChartPayloadItem[];
  label?: string | number;
  labelFormatter?: (label: string) => React.ReactNode;
  valueFormatter?: (value: number, key: string) => React.ReactNode;
  hideLabel?: boolean;
  className?: string;
}) {
  const { config } = useChart();
  if (!active || !payload || payload.length === 0) return null;

  return (
    <div
      className={cn(
        "grid min-w-32 items-start gap-1.5 rounded-lg border border-border/50 bg-background px-2.5 py-1.5 text-xs shadow-xl",
        className
      )}
    >
      {!hideLabel && label !== undefined && (
        <div className="font-medium">{labelFormatter ? labelFormatter(String(label)) : label}</div>
      )}
      <div className="grid gap-1.5">
        {payload.map((item, index) => {
          const key = String(item.dataKey ?? item.name ?? index);
          const itemConfig = config[key];
          const value = typeof item.value === "number" ? item.value : Number(item.value ?? 0);
          return (
            <div key={key} className="flex w-full items-center gap-2">
              <span
                className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
                style={{ backgroundColor: item.color ?? `var(--color-${key})` }}
              />
              <span className="flex-1 text-muted-foreground">{itemConfig?.label ?? item.name}</span>
              <span className="font-mono font-medium tabular-nums text-foreground">
                {valueFormatter ? valueFormatter(value, key) : value.toLocaleString()}
              </span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export const ChartLegend = RechartsPrimitive.Legend;

// ChartLegendContent is the legend of a chart, with each series' label from the config
export function ChartLegendContent({
  payload,
  className,
}: {
  payload?: readonly { dataKey?: string | number | ((obj: unknown) => unknown); value?: string; color?: string }[];
  className?: string;
}) {
  const { config } = useChart();
  if (!payload || payload.length === 0) return null;

  return (
    <div className={cn("flex flex-wrap items-center justify-center gap-x-4 gap-y-1 pt-3", className)}>
      {payload.map((item) => {
        const key = String(item.dataKey ?? item.value);
        return (
          <div key={key} className="flex items-center gap-1.5">
            <span
              className="h-2.5 w-2.5 shrink-0 rounded-[2px]"
              style={{ backgroundColor: item.color ?? `var(--color-${key})` }}
            />
            <span className="text-muted-foreground">{config[key]?.label ?? item.value}</span>
          </div>
        );
      })}
    </div>
  );
}
