"use client";

import { subscribeToAppStatus } from "@/services/config/status-stream";
import { Info, LoaderIcon } from "lucide-react";
import { toast } from "sonner";

import React, { useEffect, useState } from "react";

import { useRouter } from "next/navigation";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { H2 } from "@/components/ui/typography";

import { cn } from "@/lib/cn";
import { useOnboardingStore } from "@/lib/stores/global-store-onboarding";

const boolBadge = (val: boolean | undefined, invert = false) => {
  const normalized = val === true;
  const isGood = invert ? !normalized : normalized;

  return (
    <Badge className={cn("ml-2", isGood ? "bg-green-500 text-white" : "bg-red-500 text-white")}>
      {normalized ? "Yes" : "No"}
    </Badge>
  );
};

const STEP_BADGE_STYLES: Record<string, string> = {
  "Initializing Application": "bg-amber-500 text-white",
  "Bootstrapping Application": "bg-sky-500 text-white",
  "Setting UMask for File Permissions": "bg-cyan-500 text-white",
  "Loading Configuration": "bg-blue-500 text-white",
  "Validating Configuration": "bg-violet-500 text-white",
  "Performing Pre-Flight Checks": "bg-purple-500 text-white",
  "Validating Media Server Connection": "bg-fuchsia-500 text-white",
  "Waiting for Media Server to Start": "bg-amber-500 text-black",
  "Retrieving Media Server Admin User": "bg-pink-500 text-white",
  "Validating MediUX Token": "bg-rose-500 text-white",
  "Waiting for MediUX": "bg-amber-500 text-black",
  "Warming Up Application": "bg-indigo-500 text-white",
  "Preloading MediUX Users into Cache": "bg-teal-500 text-white",
  "Preloading MediUX Items with Sets into Cache": "bg-emerald-500 text-white",
  "Initializing Database": "bg-lime-500 text-black",
  "Running Database Migrations": "bg-yellow-500 text-black",
  "Loading Media Server Data from the Database": "bg-amber-600 text-white",
  "Preloading Media Server Data into Cache": "bg-orange-500 text-white",
  "Optimizing Database": "bg-red-500 text-white",
  "Starting Background Jobs": "bg-slate-600 text-white",
  "Checking MediUX Site Link Availability": "bg-zinc-600 text-white",
  "App Fully Loaded": "bg-green-600 text-white",
};

const getStepBadgeClass = (step?: string) => {
  if (!step) return "bg-gray-500 text-white";
  return STEP_BADGE_STYLES[step] ?? "bg-gray-500 text-white";
};

// The steps aura goes through while it starts, in order
const STEP_ORDER = [
  "Initializing Application",
  "Bootstrapping Application",
  "Setting UMask for File Permissions",
  "Initializing Database",
  "Loading Configuration",
  "Validating Configuration",
  "Performing Pre-Flight Checks",
  "Validating Media Server Connection",
  "Waiting for Media Server to Start",
  "Retrieving Media Server Admin User",
  "Validating MediUX Token",
  "Waiting for MediUX",
  "Warming Up Application",
  "Preloading MediUX Users into Cache",
  "Preloading MediUX Items with Sets into Cache",
  "Running Database Migrations",
  "Loading Media Server Data from the Database",
  "Preloading Media Server Data into Cache",
  "Optimizing Database",
  "Starting Background Jobs",
  "Checking MediUX Site Link Availability",
  "App Fully Loaded",
];

// How far through the steps aura is
const stepProgress = (step?: string) => {
  const index = step ? STEP_ORDER.indexOf(step) : -1;
  if (index < 0) return 0;
  return Math.round((index / (STEP_ORDER.length - 1)) * 100);
};

// A label and its value, the value wraps under the label on narrow screens
const StatusRow = ({ label, children, last = false }: { label: string; children: React.ReactNode; last?: boolean }) => (
  <div
    className={cn("flex flex-wrap items-center justify-between gap-x-4 gap-y-1 px-3 py-3 sm:px-4", !last && "border-b")}
  >
    <span className="font-medium text-muted-foreground">{label}</span>
    <span className="min-w-0 max-w-full text-right">{children}</span>
  </div>
);

const AppStatusPage: React.FC = () => {
  const router = useRouter();

  const status = useOnboardingStore((state) => state.status);
  const hasHydrated = useOnboardingStore((state) => state.hasHydrated);
  const setStatus = useOnboardingStore((state) => state.setStatus);
  const fetchStatus = useOnboardingStore((state) => state.fetchStatus);
  const [connected, setConnected] = useState(false);

  // The backend sends the status as soon as each step starts
  useEffect(() => subscribeToAppStatus(setStatus, setConnected), [setStatus]);

  const progress = status?.app_fully_loaded ? 100 : stepProgress(status?.app_loading_step);

  useEffect(() => {
    if (!hasHydrated || !status) return;

    const isReady = status.app_fully_loaded && !status.needs_setup && status.config_valid && status.config_loaded;
    if (isReady) {
      toast.dismiss("app-loading-toast");
      router.replace("/");
    }
  }, [hasHydrated, status, router]);

  return (
    <div className="min-h-screen bg-gradient-to-b from-background to-muted/30 flex items-center justify-center p-3 sm:p-4">
      <div className="w-full max-w-4xl rounded-2xl border bg-card/80 backdrop-blur p-4 sm:p-6 shadow-xl">
        <div className="mb-6">
          <H2 className="text-2xl sm:text-3xl font-bold tracking-tight">App Status</H2>
          <div className="mt-1 flex flex-wrap items-center justify-between gap-x-3 gap-y-2">
            <p className="flex items-center gap-2 text-sm text-muted-foreground">
              {progress < 100 && <LoaderIcon className="animate-spin" size={16} />}
              aura is starting. This page updates as each step finishes.
            </p>
            <Badge
              variant="outline"
              className={cn("shrink-0 gap-1.5", connected ? "text-green-500" : "text-muted-foreground")}
            >
              <span className={cn("h-2 w-2 rounded-full", connected ? "bg-green-500" : "bg-muted-foreground")} />
              {connected ? "Live" : "Reconnecting"}
            </Badge>
          </div>
        </div>

        <div className="mb-6">
          <div className="h-2 rounded-full bg-muted overflow-hidden">
            <div
              className="h-full bg-primary transition-[width] duration-500 ease-out"
              style={{ width: `${progress}%` }}
            />
          </div>
        </div>

        {/* Silo rate limits its API, so loading the media server data takes longer */}
        {hasHydrated &&
          status?.current_setup?.media_server?.type === "Silo" &&
          status?.app_loading_step === "Preloading Media Server Data into Cache" && (
            <div className="mb-6 flex items-start gap-3 rounded-lg border border-sky-500/30 bg-sky-500/10 p-4 text-sm">
              <Info className="mt-0.5 h-5 w-5 shrink-0 text-sky-500" />
              <div>
                <p className="font-medium text-foreground">Silo may take a little longer to load</p>
                <p className="mt-1 text-muted-foreground">
                  Silo limits how many API requests aura can make, so loading your libraries can take a few minutes for
                  larger libraries. Setting the API key's rate tier to{" "}
                  <span className="font-medium text-foreground">elevated</span> in Silo will speed this up.
                </p>
              </div>
            </div>
          )}

        {!hasHydrated ? (
          <div className="text-sm text-muted-foreground">Loading status...</div>
        ) : (
          <div className="w-full rounded-lg text-sm border overflow-hidden">
            <StatusRow label="Media Server Name">{status?.media_server_name || "N/A"}</StatusRow>
            <StatusRow label="App Version">{status?.app_version || "N/A"}</StatusRow>
            <StatusRow label="App Fully Loaded">{boolBadge(status?.app_fully_loaded)}</StatusRow>
            <StatusRow label="Config Loaded">{boolBadge(status?.config_loaded)}</StatusRow>
            <StatusRow label="Config Valid">{boolBadge(status?.config_valid)}</StatusRow>
            <StatusRow label="Needs Setup">{boolBadge(status?.needs_setup, true)}</StatusRow>
            <StatusRow label="Current Step" last>
              <Badge className={cn("h-auto whitespace-normal text-left", getStepBadgeClass(status?.app_loading_step))}>
                {status?.app_loading_step || "N/A"}
              </Badge>
            </StatusRow>

            <div className="p-3 sm:p-4 border-t bg-muted/20 flex justify-end">
              <Button variant="outline" onClick={() => void fetchStatus()} className="cursor-pointer">
                Refresh Status
              </Button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default AppStatusPage;
