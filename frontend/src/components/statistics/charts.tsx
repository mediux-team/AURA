"use client";

import { Bar, BarChart, CartesianGrid, Line, LineChart, XAxis, YAxis } from "recharts";

import { type StatisticsPeriod, formatBytes, formatNumber, formatPeriodDate } from "@/components/statistics/format";
import {
  type ChartConfig,
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart";

import type { DownloadPoint, Snapshot } from "@/types/statistics";

// The charts are short on phones and taller on larger screens
const CHART_CLASS = "aspect-auto h-52 w-full sm:h-64";

const axisProps = { tickLine: false, axisLine: false, tickMargin: 8, minTickGap: 24 } as const;

export type DownloadsMetric = "sets" | "images" | "failed" | "bytes";

const downloadsConfig: Record<DownloadsMetric, ChartConfig> = {
  sets: {
    sets: { label: "Sets downloaded", color: "var(--chart-1)" },
    updates: { label: "Sets updated", color: "var(--chart-2)" },
  },
  images: { images: { label: "Images", color: "var(--chart-1)" } },
  failed: { images_failed: { label: "Images failed", color: "var(--destructive)" } },
  bytes: { bytes: { label: "Downloaded", color: "var(--chart-3)" } },
};

// DownloadsChart is the downloads of each day, week or month
export function DownloadsChart({
  timeline,
  period,
  metric,
}: {
  timeline: DownloadPoint[];
  period: StatisticsPeriod;
  metric: DownloadsMetric;
}) {
  const config = downloadsConfig[metric];
  const keys = Object.keys(config);
  const formatValue = (value: number) => (metric === "bytes" ? formatBytes(value) : formatNumber(value));

  return (
    <ChartContainer config={config} className={CHART_CLASS}>
      <BarChart data={timeline} margin={{ left: 0, right: 8 }}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="date" {...axisProps} tickFormatter={(date: string) => formatPeriodDate(date, period)} />
        <YAxis {...axisProps} width={48} allowDecimals={false} tickFormatter={formatValue} />
        <ChartTooltip
          cursor={false}
          content={
            <ChartTooltipContent
              labelFormatter={(date) => formatPeriodDate(date, period, true)}
              valueFormatter={(value) => formatValue(value)}
            />
          }
        />
        {keys.length > 1 && <ChartLegend content={<ChartLegendContent />} />}
        {keys.map((key, index) => (
          <Bar
            key={key}
            dataKey={key}
            stackId="downloads"
            fill={`var(--color-${key})`}
            radius={index === keys.length - 1 ? [4, 4, 0, 0] : 0}
          />
        ))}
      </BarChart>
    </ChartContainer>
  );
}

const growthConfig = {
  movies: { label: "Movies", color: "var(--chart-1)" },
  shows: { label: "Shows", color: "var(--chart-2)" },
} satisfies ChartConfig;

// niceStep is the round step of 1, 2 or 5 times a power of 10 closest to a third of the range
const niceStep = (range: number) => {
  const rough = range / 3;
  const power = Math.pow(10, Math.floor(Math.log10(rough)));
  const step = [1, 2, 5, 10]
    .map((n) => n * power)
    .reduce((best, n) => (Math.abs(n - rough) < Math.abs(best - rough) ? n : best));
  return Math.max(1, Math.round(step));
};

// axisTicks is ticks a round step apart from the low end, ending exactly at the high end
const axisTicks = (low: number, high: number) => {
  if (high <= low) return [low];
  const step = niceStep(high - low);
  const ticks: number[] = [];
  // Leave out a tick that's too close to the high end
  for (let tick = low; tick < high - step / 2; tick += step) ticks.push(tick);
  return [...ticks, high];
};

// GrowthChart is how many movies and shows the libraries had, by when they were added.
// Movies and shows each have their own axis, from the first day of the chart to today.
export function GrowthChart({
  growth,
  period,
}: {
  growth: { date: string; movies: number; shows: number }[];
  period: StatisticsPeriod;
}) {
  const first = growth[0];
  const last = growth[growth.length - 1];
  // A series without any items isn't shown
  const series = (["movies", "shows"] as const).filter((key) => last && last[key] > 0);

  return (
    <ChartContainer config={growthConfig} className={CHART_CLASS}>
      <LineChart data={growth} margin={{ left: 0, right: 0 }}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="date" {...axisProps} tickFormatter={(date: string) => formatPeriodDate(date, period)} />
        {series.map((key, index) => {
          const low = first[key];
          const high = last[key];
          return (
            <YAxis
              key={key}
              yAxisId={key}
              orientation={index === 0 ? "left" : "right"}
              {...axisProps}
              width={48}
              domain={high > low ? [low, high] : [Math.max(low - 1, 0), high + 1]}
              ticks={axisTicks(low, high)}
              minTickGap={0}
              tick={{ fill: `var(--color-${key})` }}
              tickFormatter={formatNumber}
            />
          );
        })}
        <ChartTooltip
          content={<ChartTooltipContent labelFormatter={(date) => formatPeriodDate(date, period, true)} />}
        />
        <ChartLegend content={<ChartLegendContent />} />
        {series.map((key) => (
          <Line
            key={key}
            yAxisId={key}
            dataKey={key}
            type="monotone"
            stroke={`var(--color-${key})`}
            strokeWidth={2}
            dot={false}
          />
        ))}
      </LineChart>
    </ChartContainer>
  );
}

const historyConfig = {
  items: { label: "Media items", color: "var(--chart-2)" },
  items_with_sets: { label: "With saved sets", color: "var(--chart-1)" },
} satisfies ChartConfig;

// HistoryChart is the items, and the items with saved sets, recorded each day
export function HistoryChart({ history }: { history: Snapshot[] }) {
  const data = history.map((snapshot) => ({
    date: snapshot.date,
    items: snapshot.counts.items,
    items_with_sets: snapshot.items_with_sets,
  }));

  return (
    <ChartContainer config={historyConfig} className={CHART_CLASS}>
      <LineChart data={data} margin={{ left: 0, right: 8 }}>
        <CartesianGrid vertical={false} />
        <XAxis dataKey="date" {...axisProps} tickFormatter={(date: string) => formatPeriodDate(date, "day")} />
        <YAxis {...axisProps} width={48} allowDecimals={false} tickFormatter={formatNumber} />
        <ChartTooltip
          content={<ChartTooltipContent labelFormatter={(date) => formatPeriodDate(date, "day", true)} />}
        />
        <ChartLegend content={<ChartLegendContent />} />
        {(["items", "items_with_sets"] as const).map((key) => (
          <Line key={key} dataKey={key} type="monotone" stroke={`var(--color-${key})`} strokeWidth={2} dot={false} />
        ))}
      </LineChart>
    </ChartContainer>
  );
}
