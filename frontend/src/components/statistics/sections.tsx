"use client";

import { Crown, Server } from "lucide-react";

import type { ReactNode } from "react";

import { formatNumber, formatPercent } from "@/components/statistics/format";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";

import { cn } from "@/lib/cn";

import type { ItemCounts, ItemRef, MediaServerStats } from "@/types/statistics";

// StatTile is one number with a label, and an optional line under it
export function StatTile({ label, value, detail }: { label: string; value: ReactNode; detail?: ReactNode }) {
  return (
    <div className="rounded-lg border bg-card p-3 sm:p-4">
      <div className="text-xs text-muted-foreground">{label}</div>
      <div className="mt-1 text-xl font-semibold tabular-nums sm:text-2xl">{value}</div>
      {detail && <div className="mt-0.5 text-xs text-muted-foreground">{detail}</div>}
    </div>
  );
}

// Section is a card with a title and a description
export function Section({
  title,
  description,
  action,
  children,
  className,
}: {
  title: string;
  description?: ReactNode;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <Card className={cn("gap-4", className)}>
      <CardHeader className="px-4 sm:px-6">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="space-y-1">
            <CardTitle>{title}</CardTitle>
            {description && <CardDescription>{description}</CardDescription>}
          </div>
          {action}
        </div>
      </CardHeader>
      <CardContent className="space-y-4 px-4 sm:px-6">{children}</CardContent>
    </Card>
  );
}

// countsText is the movies and shows, with the seasons and episodes when shown.
// A movie library only has movies, and a show library only shows.
export function countsText(counts: ItemCounts, showSeasonsEpisodes: boolean, libraryType?: string) {
  const parts: string[] = [];
  if (libraryType !== "show") parts.push(`${formatNumber(counts.movies)} movies`);
  if (libraryType !== "movie") {
    parts.push(`${formatNumber(counts.shows)} shows`);
    if (showSeasonsEpisodes) {
      parts.push(`${formatNumber(counts.seasons)} seasons`, `${formatNumber(counts.episodes)} episodes`);
    }
  }
  return parts.join(" · ");
}

// CoverageRow is a label with how many out of a total, as a bar
export function CoverageRow({ label, value, total }: { label: string; value: number; total: number }) {
  return (
    <div className="space-y-1.5">
      <div className="flex items-baseline justify-between gap-2 text-sm">
        <span className="truncate">{label}</span>
        <span className="shrink-0 tabular-nums text-muted-foreground">
          {formatNumber(value)} / {formatNumber(total)} · {formatPercent(value, total)}
        </span>
      </div>
      <Progress value={total > 0 ? (value / total) * 100 : 0} className="h-2" />
    </div>
  );
}

// BarRow is a label with a value, and a bar of the value out of the largest value
export function BarRow({
  label,
  value,
  max,
  detail,
}: {
  label: ReactNode;
  value: number;
  max: number;
  detail?: string;
}) {
  return (
    <div className="space-y-1">
      <div className="flex items-baseline justify-between gap-2 text-sm">
        <span className="min-w-0 truncate">{label}</span>
        <span className="shrink-0 tabular-nums text-muted-foreground">{detail ?? formatNumber(value)}</span>
      </div>
      <div className="h-1.5 rounded-full bg-muted">
        <div
          className="h-1.5 rounded-full bg-[var(--primary-dynamic)]"
          style={{ width: `${max > 0 ? Math.max((value / max) * 100, 2) : 0}%` }}
        />
      </div>
    </div>
  );
}

function ItemList({ items, total }: { items: ItemRef[]; total: number }) {
  if (items.length === 0) return <p className="text-sm text-muted-foreground">None</p>;
  return (
    <div className="space-y-1">
      <ul className="max-h-64 space-y-1 overflow-y-auto pr-1 text-sm">
        {items.map((item) => (
          <li key={`${item.library}-${item.type}-${item.tmdb_id}`} className="flex justify-between gap-2">
            <span className="min-w-0 truncate">
              {item.title}
              {item.year > 0 && <span className="text-muted-foreground"> ({item.year})</span>}
            </span>
            <span className="shrink-0 text-xs text-muted-foreground">{item.library}</span>
          </li>
        ))}
      </ul>
      {total > items.length && (
        <p className="text-xs text-muted-foreground">
          Showing the first {formatNumber(items.length)} of {formatNumber(total)}
        </p>
      )}
    </div>
  );
}

// MediaServerCard is one media server's items, and how it compares to the main one
export function MediaServerCard({
  server,
  mainName,
  showSeasonsEpisodes,
}: {
  server: MediaServerStats;
  mainName: string;
  showSeasonsEpisodes: boolean;
}) {
  const notLoaded = !server.main && server.counts.items === 0;
  return (
    <div className="space-y-3 rounded-lg border p-3 sm:p-4">
      <div className="flex flex-wrap items-center gap-2">
        <Server className="h-4 w-4 text-muted-foreground" />
        <span className="font-medium">{server.name}</span>
        <Badge variant="outline">{server.type}</Badge>
        {server.main && (
          <Badge>
            <Crown className="h-3 w-3" />
            Main
          </Badge>
        )}
      </div>
      <div className="text-sm text-muted-foreground">
        {formatNumber(server.counts.items)} items · {countsText(server.counts, showSeasonsEpisodes)}
      </div>

      {notLoaded && (
        <p className="text-sm text-muted-foreground">aura hasn&apos;t loaded this media server&apos;s libraries yet.</p>
      )}

      {!server.main && !notLoaded && (
        <>
          <div className="grid grid-cols-3 gap-2 text-center">
            <div className="rounded-md bg-muted/50 p-2">
              <div className="text-lg font-semibold tabular-nums">{formatNumber(server.matched)}</div>
              <div className="text-xs text-muted-foreground">In both</div>
            </div>
            <div className="rounded-md bg-muted/50 p-2">
              <div className="text-lg font-semibold tabular-nums">{formatNumber(server.only_in_main_count)}</div>
              <div className="text-xs text-muted-foreground">Only in {mainName}</div>
            </div>
            <div className="rounded-md bg-muted/50 p-2">
              <div className="text-lg font-semibold tabular-nums">{formatNumber(server.only_in_server_count)}</div>
              <div className="text-xs text-muted-foreground">Only in {server.name}</div>
            </div>
          </div>
          <Accordion type="multiple" className="w-full">
            <AccordionItem value="only-in-main">
              <AccordionTrigger className="py-2 text-sm">Only in {mainName}</AccordionTrigger>
              <AccordionContent>
                <ItemList items={server.only_in_main} total={server.only_in_main_count} />
              </AccordionContent>
            </AccordionItem>
            <AccordionItem value="only-in-server" className="border-b-0">
              <AccordionTrigger className="py-2 text-sm">Only in {server.name}</AccordionTrigger>
              <AccordionContent>
                <ItemList items={server.only_in_server} total={server.only_in_server_count} />
              </AccordionContent>
            </AccordionItem>
          </Accordion>
          {(server.unmapped_libraries?.length ?? 0) > 0 && (
            <p className="text-xs text-muted-foreground">
              Not compared, as they don&apos;t match a {mainName} library: {server.unmapped_libraries?.join(", ")}
            </p>
          )}
        </>
      )}
    </div>
  );
}
