"use client";

import { withoutEmptyFieldErrors } from "@/helper/validation/empty-field-errors";
import { ValidateMediuxInfo } from "@/services/validation/mediux";

import React, { useCallback, useEffect, useRef, useState } from "react";

import { GetConnectionColor } from "@/components/settings-onboarding/ConfigSectionMediaServer";
import type { ConfigConnectionStatus } from "@/components/settings-onboarding/ConfigSectionSonarrRadarr";
import { CONNECTION_STATUS_COLORS_BG } from "@/components/settings-onboarding/ConfigSectionSonarrRadarr";
import { PopoverHelp } from "@/components/shared/popover-help";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";

import { cn } from "@/lib/cn";

import type { AppConfigMediux } from "@/types/config/config";

interface ConfigSectionMediuxProps {
  value: AppConfigMediux;
  editing: boolean;
  showActions?: boolean; // Show actions hidden while editing, as Settings is always editable
  hideEmptyFieldErrors?: boolean; // Don't turn the card red for empty fields during setup
  onConnectionVerifiedChange?: (verified: boolean, checking: boolean) => void; // Reports whether the connection has been checked and works
  configAlreadyLoaded: boolean;
  dirtyFields?: Partial<Record<keyof AppConfigMediux, boolean>>;
  onChange: <K extends keyof AppConfigMediux>(field: K, value: AppConfigMediux[K]) => void;
  errorsUpdate?: (errors: Partial<Record<keyof AppConfigMediux, string>>) => void;
}

const QUALITY_OPTIONS = ["optimized", "original"] as const;
const SIMULTANEOUS_DOWNLOAD_OPTIONS = [1, 2, 3, 4, 5];

export const ConfigSectionMediux: React.FC<ConfigSectionMediuxProps> = ({
  value,
  editing,
  showActions = false,
  hideEmptyFieldErrors = false,
  onConnectionVerifiedChange,
  configAlreadyLoaded,
  dirtyFields = {},
  onChange,
  errorsUpdate,
}) => {
  const runRemoteValidationOnceRef = useRef<boolean>(false);
  const prevErrorsRef = useRef<string>("");

  const [runningRemoteValidation, setRunningRemoteValidation] = useState(false);
  const [remoteTokenError, setRemoteTokenError] = useState<string | null>(null);
  const [testingToken, setTestingToken] = useState(false);
  const [connectionStatus, setConnectionStatus] = useState<ConfigConnectionStatus>({
    status: "unknown",
    color: GetConnectionColor("unknown"),
  });

  // Validation (local + remote)
  const errors = React.useMemo(() => {
    const errs: Partial<Record<keyof AppConfigMediux, string>> = {};
    if (!value.api_token.trim()) errs.api_token = "Token is required.";
    if (!value.download_quality.trim()) errs.download_quality = "Select a download quality.";
    else if (!QUALITY_OPTIONS.includes(value.download_quality as (typeof QUALITY_OPTIONS)[number]))
      errs.download_quality = "Invalid quality option.";
    // Merge remote error (overrides local message for Token if present)
    if (remoteTokenError) errs.api_token = remoteTokenError;
    return errs;
  }, [value.api_token, value.download_quality, remoteTokenError]);

  // Errors that turn the card red
  const displayErrors = React.useMemo(
    () =>
      hideEmptyFieldErrors ? withoutEmptyFieldErrors(errors, value as unknown as Record<string, unknown>) : errors,
    [errors, hideEmptyFieldErrors, value]
  );

  // Emit errors upward
  useEffect(() => {
    if (!errorsUpdate) return;
    const serialized = JSON.stringify(errors);
    if (serialized === prevErrorsRef.current) return;
    prevErrorsRef.current = serialized;
    errorsUpdate(errors);
  }, [errors, errorsUpdate]);

  // Reset remote error and connection status when token text changes
  useEffect(() => {
    setRemoteTokenError(null);
    setConnectionStatus({ status: "unknown", color: GetConnectionColor("unknown") });
  }, [value.api_token]);

  // Report whether the token has been checked and works
  useEffect(() => {
    onConnectionVerifiedChange?.(connectionStatus.status === "ok" && !runningRemoteValidation, runningRemoteValidation);
  }, [connectionStatus.status, runningRemoteValidation, onConnectionVerifiedChange]);

  const runRemoteValidation = useCallback(
    async (showToast = true) => {
      // Nothing to check without a token
      if (!value.api_token.trim()) {
        setConnectionStatus({ status: "unknown", color: GetConnectionColor("unknown") });
        return;
      }
      setRunningRemoteValidation(true);

      setTestingToken(true);
      const start = Date.now();
      const { valid, message } = await ValidateMediuxInfo(value, showToast);
      const elapsed = Date.now() - start;
      const minDelay = 400; // milliseconds

      if (elapsed < minDelay) {
        await new Promise((resolve) => setTimeout(resolve, minDelay - elapsed));
      }

      setTestingToken(false);

      if (valid) {
        setRemoteTokenError(null);
        setConnectionStatus({ status: "ok", color: GetConnectionColor("ok") });
      } else {
        setRemoteTokenError(message || "Token invalid");
        setConnectionStatus({ status: "error", color: GetConnectionColor("error") });
      }
      setRunningRemoteValidation(false);
    },
    [value, setRemoteTokenError, setTestingToken]
  );

  // If the config is already loaded, we can run validation
  useEffect(() => {
    if (runRemoteValidationOnceRef.current) return;
    if (configAlreadyLoaded) {
      runRemoteValidation(false);
      runRemoteValidationOnceRef.current = true;
    }
  }, [configAlreadyLoaded, runRemoteValidation]);

  return (
    <Card
      className={`gap-4 p-4 sm:p-5 ${Object.values(displayErrors).some(Boolean) ? "border-red-500" : "border-muted"}`}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h2 className={`text-xl font-semibold text-${connectionStatus.color}`}>MediUX</h2>
          {runningRemoteValidation ? (
            // Loading Spinner
            <span className="h-2 w-2 rounded-full border-2 border-transparent border-t-primary animate-spin" />
          ) : (
            <span
              className={`h-2 w-2 rounded-full ${CONNECTION_STATUS_COLORS_BG[connectionStatus.status]} animate-pulse`}
              title={`Connection status: ${connectionStatus.status}`}
            />
          )}
        </div>
        <Button
          variant="outline"
          size="sm"
          hidden={editing && !showActions}
          disabled={(editing && !showActions) || testingToken}
          onClick={() => runRemoteValidation()}
          className="cursor-pointer hover:text-primary"
        >
          {testingToken ? "Testing..." : "Test Token"}
        </Button>
      </div>
      {/* Token */}
      <div className={cn("space-y-1 border rounded-md p-3 transition")}>
        <div className="flex items-center justify-between">
          <Label>Token</Label>
          {editing && (
            <PopoverHelp ariaLabel="help-mediux-token">
              <p>MediUX API token. Paste the personal token generated from your MediUX account.</p>
            </PopoverHelp>
          )}
        </div>
        <Input
          disabled={!editing}
          placeholder="MediUX API token"
          value={value.api_token}
          onChange={(e) => onChange("api_token", e.target.value)}
          onBlur={() => {
            runRemoteValidation();
          }}
          className={cn(dirtyFields.api_token && "border-amber-500")}
        />
        {errors.api_token && <p className="text-xs text-red-500">{errors.api_token}</p>}
      </div>

      {/* Download Quality */}
      <div className="space-y-1 border rounded-md p-3 transition">
        <div className="flex items-center justify-between">
          <Label>Download Quality</Label>
          {editing && (
            <PopoverHelp ariaLabel="help-mediux-download-quality">
              <p className="mb-2">Select the desired quality for fetched downloads:</p>
              <ul className="space-y-2">
                <li>
                  <div className="flex items-center gap-3 rounded-md border border-muted bg-muted/50 px-3 py-2">
                    <p className="inline-flex items-center rounded bg-background px-2 py-1 font-mono">optimized</p>
                    <p>Balanced for size & performance.</p>
                  </div>
                </li>
                <li>
                  <div className="flex items-center gap-3 rounded-md border border-muted bg-muted/50 px-3 py-2">
                    <p className="inline-flex items-center rounded bg-background px-2 py-1 font-mono">original</p>
                    <p>Full source quality (largest size).</p>
                  </div>
                </li>
              </ul>
            </PopoverHelp>
          )}
        </div>
        <Select
          disabled={!editing}
          value={value.download_quality}
          onValueChange={(v) => onChange("download_quality", v as AppConfigMediux["download_quality"])}
        >
          <SelectTrigger
            id="mediux-download-quality-trigger"
            className={cn("w-full", dirtyFields.download_quality && "border-amber-500")}
          >
            <SelectValue placeholder="Select quality..." />
          </SelectTrigger>
          <SelectContent>
            {QUALITY_OPTIONS.map((q) => (
              <SelectItem key={q} value={q}>
                {q}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {errors.download_quality && <p className="text-xs text-red-500">{errors.download_quality}</p>}
      </div>

      {/* Simultaneous Downloads */}
      <div className="space-y-1 border rounded-md p-3 transition">
        <div className="flex items-center justify-between">
          <Label>Simultaneous Downloads</Label>
          {editing && (
            <PopoverHelp ariaLabel="help-mediux-simultaneous-downloads">
              <p>
                How many images each download gets at once, for the Download Queue, the Download Modal, Auto Download
                and every other download. The Download Queue also downloads this many sets at once. More is faster, as
                MediUX can take a few seconds for each image, but puts more load on MediUX.
              </p>
            </PopoverHelp>
          )}
        </div>
        <Select
          disabled={!editing}
          value={String(value.simultaneous_downloads || 3)}
          onValueChange={(v) => onChange("simultaneous_downloads", Number(v))}
        >
          <SelectTrigger
            id="mediux-simultaneous-downloads-trigger"
            className={cn("w-full", dirtyFields.simultaneous_downloads && "border-amber-500")}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {/* A value set in the config file is shown too */}
            {[...new Set([...SIMULTANEOUS_DOWNLOAD_OPTIONS, value.simultaneous_downloads || 3])].map((n) => (
              <SelectItem key={n} value={String(n)}>
                {n}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
    </Card>
  );
};
