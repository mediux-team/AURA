"use client";

import {
  PreviewMainMediaServerSwitch,
  SwitchMainMediaServer,
  type SwitchMainPlan,
} from "@/services/config/main-media-server";
import { Loader2, TriangleAlert } from "lucide-react";
import { toast } from "sonner";

import React, { useEffect, useState } from "react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";

import { ClearAllStores } from "@/lib/stores/clear-all-stores";

import type { AppConfigMediaServer } from "@/types/config/config";

interface MakeMainMediaServerDialogProps {
  server: AppConfigMediaServer | null; // The media server to make the main one. The dialog is open while it is set.
  onClose: () => void;
}

// Shows what happens to the saved items, then makes the media server the main one
export const MakeMainMediaServerDialog: React.FC<MakeMainMediaServerDialogProps> = ({ server, onClose }) => {
  const [plan, setPlan] = useState<SwitchMainPlan | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [switching, setSwitching] = useState(false);
  const name = server?.name || server?.type || "this media server";

  // Check the saved items when the dialog opens
  useEffect(() => {
    if (!server?.id) return;
    let cancelled = false;
    setPlan(null);
    setError(null);
    void PreviewMainMediaServerSwitch(server.id).then((response) => {
      if (cancelled) return;
      if (response.status === "error" || !response.data) {
        setError(response.error?.message || "Could not check the saved items");
        return;
      }
      setPlan(response.data.plan);
    });
    return () => {
      cancelled = true;
    };
  }, [server?.id]);

  const handleSwitch = async () => {
    if (!server?.id) return;
    setSwitching(true);
    setError(null);
    const response = await SwitchMainMediaServer(server.id);
    if (response.status === "error" || !response.data?.status) {
      setSwitching(false);
      setError(response.error?.message || "Could not change the main media server");
      return;
    }
    // Every item is different on the new main media server, so load everything again
    toast.success(`${name} is now the main media server`);
    await ClearAllStores();
    window.location.reload();
  };

  const notFound = (plan?.items ?? []).filter((item) => !item.found);
  const total = plan?.items?.length ?? 0;
  const found = total - notFound.length;
  const libraryChanges = Object.entries(plan?.library_changes ?? {}).filter(([from, to]) => from !== to);

  return (
    <Dialog open={server !== null} onOpenChange={(open) => !open && !switching && onClose()}>
      <DialogContent className="max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Make {name} the main media server?</DialogTitle>
          <DialogDescription>
            The Home page will show {name}'s items, and only {name} will save images locally. Your saved sets move to
            the same items on {name}.
          </DialogDescription>
        </DialogHeader>

        {!plan && !error && (
          <p className="flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            Checking your saved items on {name}...
          </p>
        )}

        {plan && (
          <div className="space-y-3 text-sm">
            {plan.blocked && (
              <Alert className="border-amber-500 text-amber-500">
                <TriangleAlert className="h-4 w-4" />
                <AlertTitle>Can't switch right now</AlertTitle>
                <AlertDescription>{plan.blocked}</AlertDescription>
              </Alert>
            )}
            <ul className="list-disc space-y-1 pl-5">
              <li>
                {total === 0
                  ? "There are no saved items to move."
                  : found === total
                    ? `All ${total} saved ${total === 1 ? "item moves" : "items move"} to ${name}.`
                    : `${found} out of ${total} saved ${total === 1 ? "item moves" : "items move"} to ${name}.`}
              </li>
              {plan.merged > 0 && (
                <li>
                  {plan.merged} saved {plan.merged === 1 ? "item becomes" : "items become"} one item on {name}, e.g. two
                  editions of a movie. Their saved sets are kept.
                </li>
              )}
              {plan.not_found > 0 && (
                <li>
                  {plan.not_found} saved {plan.not_found === 1 ? "item isn't" : "items aren't"} on {name}. They are kept
                  and marked as not on the server.
                </li>
              )}
            </ul>

            {notFound.length > 0 && (
              <div className="rounded-md border p-3">
                <p className="mb-1 font-medium">Not on {name}</p>
                <ul className="space-y-0.5 text-muted-foreground">
                  {notFound.slice(0, 10).map((item) => (
                    <li key={`${item.tmdb_id}-${item.library_title}-${item.edition}`}>
                      {item.title} ({item.year}) - {item.library_title}
                    </li>
                  ))}
                  {notFound.length > 10 && <li>and {notFound.length - 10} more</li>}
                </ul>
              </div>
            )}

            {libraryChanges.length > 0 && (
              <div className="rounded-md border p-3">
                <p className="mb-1 font-medium">Libraries</p>
                <ul className="space-y-0.5 text-muted-foreground">
                  {libraryChanges.map(([from, to]) => (
                    <li key={from}>
                      {from} → {to}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}

        {error && <p className="text-sm text-red-500">{error}</p>}

        <DialogFooter>
          <Button variant="ghost" onClick={onClose} disabled={switching} className="cursor-pointer">
            Cancel
          </Button>
          <Button onClick={handleSwitch} disabled={!plan || !!plan.blocked || switching} className="cursor-pointer">
            {switching && <Loader2 className="h-4 w-4 animate-spin" />}
            {switching ? "Switching..." : "Make Main"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};
