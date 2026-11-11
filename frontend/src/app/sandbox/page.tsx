"use client";

import { formatExactDateTime } from "@/helper/format-date-last-updates";
import { formatImageTypeLabel } from "@/helper/format-image-type";
import {
  type SandboxChange,
  type SandboxRecheckResult,
  type SandboxState,
  addSandboxFailure,
  clearSandboxFailures,
  findSavedItems,
  getSandboxState,
  recheckSandboxItem,
  setSandboxDelay,
  simulateSandboxChange,
} from "@/services/dev/sandbox";
import { toast } from "sonner";

import { useEffect, useState } from "react";

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

import { IS_DEV_BUILD } from "@/lib/build-info";

import type { DBSavedItem } from "@/types/database/db-poster-set";

// The image types a failure can be for
const FAILURE_TYPES = [
  { value: "poster", label: "Posters" },
  { value: "backdrop", label: "Backdrops" },
  { value: "season_poster", label: "Season posters" },
  { value: "special_season_poster", label: "Special season posters" },
  { value: "titlecard", label: "Title cards" },
];

// itemKey is the item as the sandbox finds it
const itemKey = (item: DBSavedItem) => ({
  tmdb_id: item.media_item.tmdb_id,
  library_title: item.media_item.library_title,
  edition: item.media_item.edition ?? "",
});

const itemName = (item: DBSavedItem) =>
  `${item.media_item.title} (${item.media_item.library_title}${item.media_item.edition ? ` · ${item.media_item.edition}` : ""})`;

// The test sandbox, only in dev builds
export default function SandboxPage() {
  const [query, setQuery] = useState("");
  const [results, setResults] = useState<DBSavedItem[]>([]);
  const [item, setItem] = useState<DBSavedItem | null>(null);
  const [state, setState] = useState<SandboxState | null>(null);
  const [delay, setDelay] = useState("0");
  const [failureTypes, setFailureTypes] = useState<string[]>([]);
  const [failureCount, setFailureCount] = useState("1");
  const [failureReason, setFailureReason] = useState("");
  const [recheck, setRecheck] = useState<SandboxRecheckResult | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!IS_DEV_BUILD) return;
    getSandboxState().then((response) => {
      if (response.status === "success" && response.data) {
        setState(response.data);
        setDelay(String(response.data.delay_seconds));
      }
    });
  }, []);

  if (!IS_DEV_BUILD) {
    return <p className="p-6 text-muted-foreground">The test sandbox is only in dev builds.</p>;
  }

  const search = async () => {
    const response = await findSavedItems({ title: query });
    if (response.status === "error") return toast.error(response.error?.message || "Search failed");
    setResults(response.data?.items ?? []);
  };

  // reload gets the item's saved data again, after a change
  const reload = async (current: DBSavedItem) => {
    const key = itemKey(current);
    const response = await findSavedItems({ tmdbID: key.tmdb_id, library: key.library_title });
    const latest = response.data?.items.find((saved) => (saved.media_item.edition ?? "") === key.edition);
    setItem(latest ?? null);
  };

  const simulate = async (change: Omit<SandboxChange, "tmdb_id" | "library_title" | "edition">, label: string) => {
    if (!item) return;
    setBusy(true);
    const response = await simulateSandboxChange({ ...itemKey(item), ...change });
    setBusy(false);
    if (response.status === "error") return toast.error(response.error?.message || "The change failed");
    toast.success(`${label}. Run the Auto Download check to see what it does.`);
    await reload(item);
  };

  const runRecheck = async () => {
    if (!item) return;
    setBusy(true);
    const response = await recheckSandboxItem(itemKey(item));
    setBusy(false);
    if (response.status === "error") return toast.error(response.error?.message || "The check failed");
    setRecheck(response.data ?? null);
    await reload(item);
  };

  const saveDelay = async () => {
    const seconds = Number(delay);
    const response = await setSandboxDelay(Number.isFinite(seconds) ? seconds : 0);
    if (response.status === "error") return toast.error(response.error?.message || "Saving the wait failed");
    setState(response.data ?? null);
    toast.success(seconds > 0 ? `Each image download waits ${seconds} seconds` : "Image downloads don't wait");
  };

  const addFailure = async () => {
    if (!item) return;
    const response = await addSandboxFailure({
      ...itemKey(item),
      title: item.media_item.title,
      types: failureTypes,
      remaining: Math.max(0, Number(failureCount) || 0),
      reason: failureReason,
    });
    if (response.status === "error") return toast.error(response.error?.message || "Adding the failure failed");
    setState(response.data ?? null);
    toast.success(`Downloads of ${item.media_item.title} will fail`);
  };

  const clearFailures = async () => {
    const response = await clearSandboxFailures();
    if (response.status === "error") return toast.error(response.error?.message || "Clearing the failures failed");
    setState(response.data ?? null);
  };

  // In order, as the database doesn't keep it
  const seasons = [...(item?.media_item.series?.seasons ?? [])]
    .sort((a, b) => a.season_number - b.season_number)
    .map((season) => ({
      ...season,
      episodes: [...season.episodes].sort((a, b) => a.episode_number - b.episode_number),
    }));

  return (
    <div className="container mx-auto space-y-4 p-4">
      <div>
        <h1 className="text-2xl font-bold">Test Sandbox</h1>
        <p className="text-sm text-muted-foreground">
          Make changes here to Saved Items, and see how the Auto Download check reacts. You can also make downloads
          fail, and set a wait before each image download.
        </p>
      </div>

      {/* Downloads, for every item */}
      <Card>
        <CardHeader>
          <CardTitle>Downloads</CardTitle>
          <CardDescription>A wait before each image downloads, and the downloads made to fail.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex flex-wrap items-end gap-2">
            <div className="space-y-1">
              <Label>Wait before each image (seconds)</Label>
              <Input className="w-32" type="number" min={0} value={delay} onChange={(e) => setDelay(e.target.value)} />
            </div>
            <Button variant="outline" onClick={saveDelay}>
              Save
            </Button>
          </div>

          <div className="space-y-2">
            <Label>Downloads made to fail</Label>
            {(state?.failures ?? []).length === 0 ? (
              <p className="text-sm text-muted-foreground">None</p>
            ) : (
              <ul className="space-y-1 text-sm">
                {state?.failures.map((rule, index) => (
                  <li key={index}>
                    {rule.title} ({rule.library_title}): {rule.types.length > 0 ? rule.types.join(", ") : "every image"}
                    {", "}
                    {rule.remaining > 0 ? `${rule.remaining} more` : "until cleared"} · {rule.reason}
                  </li>
                ))}
              </ul>
            )}
            {(state?.failures ?? []).length > 0 && (
              <Button variant="outline" size="sm" onClick={clearFailures}>
                Clear all
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Item */}
      <Card>
        <CardHeader>
          <CardTitle>Saved item</CardTitle>
          <CardDescription>Pick a saved item to change, then run the Auto Download check.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <form
            className="flex gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              search();
            }}
          >
            <Input placeholder="Title" value={query} onChange={(e) => setQuery(e.target.value)} />
            <Button type="submit" variant="outline">
              Search
            </Button>
          </form>
          {results.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {results.map((result) => (
                <Button
                  key={`${result.media_item.tmdb_id}-${result.media_item.library_title}-${result.media_item.edition}`}
                  variant={item && itemName(item) === itemName(result) ? "default" : "outline"}
                  size="sm"
                  onClick={() => {
                    setItem(result);
                    setRecheck(null);
                  }}
                >
                  {itemName(result)}
                </Button>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {item && (
        <>
          {/* Auto Download check */}
          <Card>
            <CardHeader>
              <CardTitle>{itemName(item)}</CardTitle>
              <CardDescription>
                Runs the Auto Download check for this item now. Changed images are added to the Download Queue.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <Button onClick={runRecheck} disabled={busy}>
                Run the Auto Download check
              </Button>
              {recheck && (
                <div className="space-y-1 text-sm">
                  <p>
                    <Badge variant="outline">{recheck.overall_result}</Badge> {recheck.overall_message}
                  </p>
                  <ul className="list-disc pl-5 text-muted-foreground">
                    {(recheck.sets ?? []).map((set) => (
                      <li key={set.id}>
                        {set.title}: {set.result}, {set.reason}
                      </li>
                    ))}
                  </ul>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Make downloads fail */}
          <Card>
            <CardHeader>
              <CardTitle>Make downloads fail</CardTitle>
              <CardDescription>
                The item&apos;s next downloads fail, of every image type unless some are picked.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              <div className="flex flex-wrap gap-4">
                {FAILURE_TYPES.map((type) => (
                  <label key={type.value} className="flex items-center gap-2 text-sm">
                    <Checkbox
                      checked={failureTypes.includes(type.value)}
                      onCheckedChange={(checked) =>
                        setFailureTypes((prev) =>
                          checked ? [...prev, type.value] : prev.filter((value) => value !== type.value)
                        )
                      }
                    />
                    {type.label}
                  </label>
                ))}
              </div>
              <div className="flex flex-wrap items-end gap-2">
                <div className="space-y-1">
                  <Label>How many fail (0 until cleared)</Label>
                  <Input
                    className="w-32"
                    type="number"
                    min={0}
                    value={failureCount}
                    onChange={(e) => setFailureCount(e.target.value)}
                  />
                </div>
                <div className="min-w-48 flex-1 space-y-1">
                  <Label>Reason</Label>
                  <Input
                    placeholder="Sandbox: this download was made to fail"
                    value={failureReason}
                    onChange={(e) => setFailureReason(e.target.value)}
                  />
                </div>
                <Button variant="outline" onClick={addFailure}>
                  Make them fail
                </Button>
              </div>
            </CardContent>
          </Card>

          {/* Simulate changes */}
          <Card>
            <CardHeader>
              <CardTitle>Simulate changes</CardTitle>
              <CardDescription>
                Changes what aura has saved for the item, as if it changed since the last download.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Accordion type="multiple">
                {item.media_item.type === "movie" && (
                  <AccordionItem value="movie">
                    <AccordionTrigger>Movie file</AccordionTrigger>
                    <AccordionContent className="space-y-2">
                      <p className="break-all text-xs text-muted-foreground">
                        {item.media_item.movie?.file?.path || "No file saved"}
                      </p>
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={busy}
                        onClick={() => simulate({ change: "movie_file_changed" }, "The movie's file changed")}
                      >
                        File changed
                      </Button>
                    </AccordionContent>
                  </AccordionItem>
                )}

                {seasons.map((season) => (
                  <AccordionItem key={season.season_number} value={`season-${season.season_number}`}>
                    <AccordionTrigger>
                      {season.season_number === 0 ? "Specials" : `Season ${season.season_number}`} ·{" "}
                      {season.episodes.length} episodes
                    </AccordionTrigger>
                    <AccordionContent className="space-y-2">
                      <Button
                        size="sm"
                        variant="outline"
                        disabled={busy}
                        onClick={() =>
                          simulate(
                            { change: "season_added", season_number: season.season_number },
                            `Season ${season.season_number} was added`
                          )
                        }
                      >
                        Season added
                      </Button>
                      <ul className="grid gap-1 text-sm sm:grid-cols-2 xl:grid-cols-3">
                        {season.episodes.map((episode) => (
                          <li key={episode.episode_number} className="flex items-center justify-between gap-2">
                            <span className="truncate">Episode {episode.episode_number}</span>
                            <span className="flex shrink-0 gap-1">
                              <Button
                                size="sm"
                                variant="ghost"
                                disabled={busy}
                                onClick={() =>
                                  simulate(
                                    {
                                      change: "episode_added",
                                      season_number: season.season_number,
                                      episode_number: episode.episode_number,
                                    },
                                    `S${season.season_number}E${episode.episode_number} was added`
                                  )
                                }
                              >
                                Added
                              </Button>
                              <Button
                                size="sm"
                                variant="ghost"
                                disabled={busy}
                                onClick={() =>
                                  simulate(
                                    {
                                      change: "episode_file_changed",
                                      season_number: season.season_number,
                                      episode_number: episode.episode_number,
                                    },
                                    `S${season.season_number}E${episode.episode_number}'s file changed`
                                  )
                                }
                              >
                                File changed
                              </Button>
                            </span>
                          </li>
                        ))}
                      </ul>
                    </AccordionContent>
                  </AccordionItem>
                ))}

                {item.poster_sets.map((set) => (
                  <AccordionItem key={set.id} value={`set-${set.id}`}>
                    <AccordionTrigger>
                      Set {set.title} · {set.images?.length ?? 0} saved images
                    </AccordionTrigger>
                    <AccordionContent>
                      <ul className="grid gap-1 text-sm sm:grid-cols-2 xl:grid-cols-3">
                        {(set.images ?? []).map((image) => (
                          <li key={`${image.type}-${image.id}`} className="flex items-center justify-between gap-2">
                            <span className="min-w-0 truncate">
                              {formatImageTypeLabel({
                                image_type: image.type,
                                season_number: image.season_number,
                                episode_number: image.episode_number,
                              }) || image.type}
                              <span className="block text-xs text-muted-foreground">
                                {formatExactDateTime(image.modified)}
                              </span>
                            </span>
                            <span className="flex shrink-0 gap-1">
                              <Button
                                size="sm"
                                variant="ghost"
                                disabled={busy}
                                onClick={() =>
                                  simulate(
                                    { change: "image_updated", image_id: image.id, image_type: image.type },
                                    "The image was updated on MediUX"
                                  )
                                }
                              >
                                Updated
                              </Button>
                              <Button
                                size="sm"
                                variant="ghost"
                                disabled={busy}
                                onClick={() =>
                                  simulate(
                                    { change: "image_new", image_id: image.id, image_type: image.type },
                                    "The image was added to the set"
                                  )
                                }
                              >
                                New
                              </Button>
                            </span>
                          </li>
                        ))}
                      </ul>
                    </AccordionContent>
                  </AccordionItem>
                ))}
              </Accordion>
            </CardContent>
          </Card>
        </>
      )}
    </div>
  );
}
