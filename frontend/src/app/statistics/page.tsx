"use client";

import { GetStatistics } from "@/services/statistics/get";
import { Crown, RefreshCcw } from "lucide-react";

import { useEffect, useState } from "react";

import Link from "next/link";

import { ErrorMessage } from "@/components/shared/error-message";
import Loader from "@/components/shared/loader";
import { DownloadsChart, type DownloadsMetric, GrowthChart, HistoryChart } from "@/components/statistics/charts";
import { formatBytes, formatNumber, formatPercent, imageTypeLabel, sourceLabel } from "@/components/statistics/format";
import { BarRow, CoverageRow, MediaServerCard, Section, StatTile, countsText } from "@/components/statistics/sections";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";

import { STATISTICS_DEFAULT_DAYS, useStatisticsPageStore } from "@/lib/stores/page-store-statistics";

import type { APIResponse } from "@/types/api/api-response";
import type { Statistics } from "@/types/statistics";

const METRIC_OPTIONS: { value: DownloadsMetric; label: string }[] = [
  { value: "sets", label: "Sets" },
  { value: "images", label: "Images" },
  { value: "failed", label: "Failed" },
  { value: "bytes", label: "Size" },
];

export default function StatisticsPage() {
  const { days, setDays, showSeasonsEpisodes, setShowSeasonsEpisodes, hasHydrated } = useStatisticsPageStore();
  const [stats, setStats] = useState<Statistics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<APIResponse<Statistics> | null>(null);
  const [metric, setMetric] = useState<DownloadsMetric>("sets");
  const [reload, setReload] = useState(0);

  // The days typed. A number of 1 or more is used once typing stops, anything else becomes the default when leaving the box.
  const [daysText, setDaysText] = useState(String(days));
  useEffect(() => setDaysText(String(days)), [days]);
  useEffect(() => {
    const typed = Number(daysText);
    if (!Number.isInteger(typed) || typed < 1 || typed === days) return;
    const timer = setTimeout(() => setDays(typed), 600);
    return () => clearTimeout(timer);
  }, [daysText, days, setDays]);
  const applyDaysText = () => {
    const typed = Number(daysText);
    const next = daysText.trim() !== "" && Number.isInteger(typed) && typed >= 1 ? typed : STATISTICS_DEFAULT_DAYS;
    setDaysText(String(next));
    setDays(next);
  };

  useEffect(() => {
    // Wait for the saved days, so the statistics aren't loaded twice
    if (!hasHydrated) return;
    let cancelled = false;
    const fetchStatistics = async () => {
      setLoading(true);
      const response = await GetStatistics(days);
      if (cancelled) return;
      if (response.status === "error" || !response.data) {
        setError(response);
      } else {
        setStats(response.data);
        setError(null);
      }
      setLoading(false);
    };
    void fetchStatistics();
    return () => {
      cancelled = true;
    };
  }, [days, hasHydrated, reload]);

  const rangeText = days === 1 ? "today" : `the last ${formatNumber(days)} days`;

  return (
    <div className="mx-auto w-full max-w-6xl space-y-4 px-4 py-4 sm:px-6">
      {/* Header */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold">Statistics</h1>
          <p className="text-sm text-muted-foreground">Your libraries, media servers and downloads.</p>
        </div>
        <div className="flex items-center gap-2">
          <Label htmlFor="statistics-days" className="text-sm text-muted-foreground">
            Last
          </Label>
          <Input
            id="statistics-days"
            type="number"
            inputMode="numeric"
            min={1}
            step={1}
            value={daysText}
            onChange={(e) => setDaysText(e.target.value)}
            onBlur={applyDaysText}
            onKeyDown={(e) => {
              if (e.key === "Enter") e.currentTarget.blur();
            }}
            className="h-8 w-20 text-right tabular-nums"
            aria-label="How many days back the graphs go"
          />
          <span className="text-sm text-muted-foreground">{days === 1 ? "day" : "days"}</span>
          <Button
            variant="outline"
            size="icon"
            className="h-8 w-8 shrink-0"
            onClick={() => setReload((n) => n + 1)}
            disabled={loading}
            aria-label="Refresh the statistics"
          >
            <RefreshCcw className={loading ? "h-4 w-4 animate-spin" : "h-4 w-4"} />
          </Button>
        </div>
      </div>

      {error && <ErrorMessage error={error} />}
      {!stats && loading && <Loader />}

      {stats && (
        <>
          {/* Overview */}
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <StatTile
              label="Media items"
              value={formatNumber(stats.library.totals.items)}
              detail={countsText(stats.library.totals, false)}
            />
            <StatTile
              label="With saved sets"
              value={formatPercent(stats.sets_applied.items_with_sets, stats.sets_applied.items)}
              detail={`${formatNumber(stats.sets_applied.items_with_sets)} of ${formatNumber(stats.sets_applied.items)} items`}
            />
            <StatTile
              label="Sets downloaded"
              value={formatNumber(stats.downloads.in_range.sets)}
              detail={`${formatNumber(stats.downloads.all_time.sets)} all time`}
            />
            <StatTile
              label="Images downloaded"
              value={formatNumber(stats.downloads.in_range.images)}
              detail={`${formatNumber(stats.downloads.all_time.images)} all time`}
            />
          </div>

          <div className="grid gap-4 lg:grid-cols-2">
            {/* Library */}
            <Section
              title="Library"
              description="Your main media server's libraries, and how they grew by when items were added."
              action={
                <div className="flex items-center gap-2">
                  <Switch
                    id="show-seasons-episodes"
                    checked={showSeasonsEpisodes}
                    onCheckedChange={setShowSeasonsEpisodes}
                  />
                  <Label htmlFor="show-seasons-episodes" className="text-xs">
                    Seasons & episodes
                  </Label>
                </div>
              }
            >
              {stats.library.totals.items === 0 ? (
                <p className="text-sm text-muted-foreground">The libraries are still loading.</p>
              ) : (
                <>
                  <div className="space-y-2">
                    {(stats.library.libraries ?? []).map((library) => (
                      <div
                        key={library.title}
                        className="flex flex-wrap items-baseline justify-between gap-x-3 text-sm"
                      >
                        <span className="font-medium">{library.title}</span>
                        <span className="text-muted-foreground">
                          {countsText(library.counts, showSeasonsEpisodes, library.type)}
                        </span>
                      </div>
                    ))}
                    {showSeasonsEpisodes && (
                      <p className="text-xs text-muted-foreground">Total: {countsText(stats.library.totals, true)}</p>
                    )}
                  </div>
                  {(stats.library.growth?.length ?? 0) > 1 && (
                    <GrowthChart growth={stats.library.growth ?? []} period={stats.period} />
                  )}
                </>
              )}
            </Section>

            {/* Sets applied */}
            <Section
              title="Sets applied"
              description="Items with at least one saved set, and the image types selected."
            >
              <CoverageRow
                label="All items"
                value={stats.sets_applied.items_with_sets}
                total={stats.sets_applied.items}
              />
              {(stats.sets_applied.libraries ?? []).map((library) => (
                <CoverageRow
                  key={library.title}
                  label={library.title}
                  value={library.items_with_sets}
                  total={library.items}
                />
              ))}
              <div className="space-y-3 border-t pt-3">
                {(stats.sets_applied.image_types ?? []).map((imageType) => (
                  <CoverageRow
                    key={imageType.type}
                    label={imageTypeLabel(imageType.type)}
                    value={imageType.items}
                    total={imageType.total}
                  />
                ))}
              </div>
              <p className="text-xs text-muted-foreground">
                {formatNumber(stats.sets_applied.saved_sets)} saved sets in total.
              </p>
            </Section>

            {/* Media servers */}
            <Section
              title="Media servers"
              description="Each media server's items, and what's only in one of them."
              className="lg:col-span-2"
            >
              <div className="grid gap-3 md:grid-cols-2">
                {stats.media_servers.map((server) => (
                  <MediaServerCard
                    key={server.id}
                    server={server}
                    mainName={stats.media_servers.find((s) => s.main)?.name ?? "the main media server"}
                    showSeasonsEpisodes={showSeasonsEpisodes}
                  />
                ))}
              </div>
            </Section>

            {/* Downloads */}
            <Section
              title="Downloads"
              description={`Sets and images aura downloaded in ${rangeText}.`}
              className="lg:col-span-2"
              action={
                <ToggleGroup
                  type="single"
                  variant="outline"
                  size="sm"
                  value={metric}
                  onValueChange={(value) => value && setMetric(value as DownloadsMetric)}
                  aria-label="What the graph shows"
                >
                  {METRIC_OPTIONS.map((option) => (
                    <ToggleGroupItem key={option.value} value={option.value} className="px-3">
                      {option.label}
                    </ToggleGroupItem>
                  ))}
                </ToggleGroup>
              }
            >
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
                <StatTile label="Sets" value={formatNumber(stats.downloads.in_range.sets)} />
                <StatTile label="Images" value={formatNumber(stats.downloads.in_range.images)} />
                <StatTile
                  label="Downloaded again"
                  value={formatNumber(stats.downloads.in_range.redownloads)}
                  detail="The same set for an item"
                />
                <StatTile
                  label="Updated"
                  value={formatNumber(stats.downloads.in_range.updates)}
                  detail={`${formatNumber(stats.downloads.in_range.updated_images)} images`}
                />
                <StatTile
                  label="Applied again"
                  value={formatNumber(stats.downloads.in_range.reapplies)}
                  detail="After a refresh or upgrade"
                />
                <StatTile label="Failed" value={formatNumber(stats.downloads.in_range.images_failed)} detail="Images" />
              </div>
              <DownloadsChart timeline={stats.downloads.timeline} period={stats.period} metric={metric} />
              {stats.downloads.image_types.length > 0 && (
                <div className="space-y-2">
                  <h3 className="text-sm font-medium">Images by type</h3>
                  {stats.downloads.image_types.map((imageType) => (
                    <BarRow
                      key={imageType.name}
                      label={imageTypeLabel(imageType.name)}
                      value={imageType.images}
                      max={stats.downloads.image_types[0].images}
                    />
                  ))}
                </div>
              )}
              <p className="text-xs text-muted-foreground">
                All time: {formatNumber(stats.downloads.all_time.sets)} sets and{" "}
                {formatNumber(stats.downloads.all_time.images)} images.
                {stats.downloads.all_time.estimated_sets > 0 &&
                  ` ${formatNumber(stats.downloads.all_time.estimated_sets)} sets were counted from your saved sets, dated by when they were last downloaded.`}
              </p>
            </Section>

            {/* Creators */}
            <Section title="Creators" description={`The creators of the sets downloaded in ${rangeText}.`}>
              {stats.creators.top.length === 0 ? (
                <p className="text-sm text-muted-foreground">No sets were downloaded in {rangeText}.</p>
              ) : (
                <>
                  {stats.creators.favorite_count > 0 && (
                    <CoverageRow
                      label="From favorite creators"
                      value={stats.creators.favorite_sets}
                      total={stats.creators.favorite_sets + stats.creators.other_sets}
                    />
                  )}
                  <div className="space-y-2">
                    {stats.creators.top.map((creator) => (
                      <BarRow
                        key={creator.name}
                        label={
                          <Link href={`/user/${encodeURIComponent(creator.name)}`} className="hover:underline">
                            {creator.favorite && <Crown className="mr-1 inline h-3 w-3 text-yellow-500" />}
                            {creator.name}
                          </Link>
                        }
                        value={creator.sets}
                        max={stats.creators.top[0].sets}
                        detail={`${formatNumber(creator.sets)} sets · ${formatNumber(creator.images)} images`}
                      />
                    ))}
                  </div>
                </>
              )}
            </Section>

            {/* Sources and failures */}
            <Section title="Sources & failures" description={`Where downloads came from in ${rangeText}.`}>
              {stats.sources.length === 0 ? (
                <p className="text-sm text-muted-foreground">Nothing was downloaded in {rangeText}.</p>
              ) : (
                <div className="space-y-2">
                  {stats.sources.map((source) => (
                    <BarRow
                      key={source.name}
                      label={sourceLabel(source.name)}
                      value={source.images}
                      max={Math.max(...stats.sources.map((s) => s.images))}
                      detail={`${formatNumber(source.sets)} sets · ${formatNumber(source.images)} images`}
                    />
                  ))}
                </div>
              )}
              <div className="grid grid-cols-3 gap-2 border-t pt-3">
                <StatTile label="Failed" value={formatNumber(stats.failures.images_failed)} />
                <StatTile label="Failure rate" value={`${(stats.failures.failure_rate * 100).toFixed(1)}%`} />
                <StatTile
                  label="Still failing"
                  value={
                    <Link href="/download-queue" className="hover:underline">
                      {formatNumber(stats.failures.open_failures)}
                    </Link>
                  }
                />
              </div>
              {stats.failures.top_reasons.length > 0 && (
                <div className="space-y-1">
                  <h3 className="text-sm font-medium">Most common reasons</h3>
                  {stats.failures.top_reasons.map((reason) => (
                    <div key={reason.name} className="flex justify-between gap-2 text-sm">
                      <span className="min-w-0 break-words text-muted-foreground">{reason.name}</span>
                      <Badge variant="outline" className="shrink-0">
                        {formatNumber(reason.count)}
                      </Badge>
                    </div>
                  ))}
                </div>
              )}
            </Section>

            {/* Data and collections */}
            <Section title="Data & collections" description="The size of the images, and your collections.">
              <div className="grid grid-cols-2 gap-3">
                <StatTile
                  label="Downloaded"
                  value={formatBytes(stats.data.bytes_in_range)}
                  detail={`${formatBytes(stats.data.bytes_all_time)} all time`}
                />
                <StatTile
                  label="Image cache"
                  value={formatBytes(stats.data.image_cache_bytes)}
                  detail={`${formatNumber(stats.data.image_cache_files)} files`}
                />
                <StatTile
                  label="Collections"
                  value={formatNumber(stats.collections.collections)}
                  detail={`${formatNumber(stats.collections.with_images)} with their own images`}
                />
                <StatTile
                  label="Collection sets saved"
                  value={formatNumber(stats.collections.saved_collection_sets)}
                  detail={`${formatNumber(stats.collections.collection_images)} collection images`}
                />
              </div>
              <p className="text-xs text-muted-foreground">
                Sizes are counted for downloads recorded since aura started keeping statistics.
              </p>
            </Section>

            {/* History */}
            <Section
              title="Over time"
              description="Your media items, and how many have saved sets, recorded each day."
              className="lg:col-span-2"
            >
              {stats.history.length > 1 ? (
                <HistoryChart history={stats.history} />
              ) : (
                <p className="text-sm text-muted-foreground">
                  aura records these numbers once a day, so this graph fills in from today.
                </p>
              )}
            </Section>
          </div>

          <p className="text-center text-xs text-muted-foreground">
            Updated {new Date(stats.generated_at).toLocaleString()}
          </p>
        </>
      )}
    </div>
  );
}
