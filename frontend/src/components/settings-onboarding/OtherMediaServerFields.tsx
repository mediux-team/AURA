"use client";

import React from "react";

import { ConfigSectionMediaServer } from "@/components/settings-onboarding/ConfigSectionMediaServer";
import { PopoverHelp } from "@/components/shared/popover-help";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";

import { cn } from "@/lib/cn";

import type { AppConfigMediaServer, AppConfigMediaServerLibrary } from "@/types/config/config";

// Media servers that haven't been saved yet have a key starting with this, which isn't sent as their ID
export const NEW_SERVER_PREFIX = "new-";

// Select value for a library that doesn't match any main server library
const NO_MATCH = "__no_match__";

export const newMediaServer = (): AppConfigMediaServer => ({
  name: "",
  enabled: true,
  type: "Plex",
  url: "",
  api_token: "",
  libraries: [],
});

interface OtherMediaServerFieldsProps {
  serverKey: string;
  server: AppConfigMediaServer;
  mainLibraries: AppConfigMediaServerLibrary[];
  isNew: boolean; // Not saved yet: empty fields don't turn the card red, and the connection isn't checked on load
  highlighted?: boolean; // Show the border as changed
  dirtyFields?: Partial<Record<keyof AppConfigMediaServer, boolean>>;
  actions: React.ReactNode; // Buttons next to Enabled
  footer?: React.ReactNode;
  onChange: <K extends keyof AppConfigMediaServer>(field: K, value: AppConfigMediaServer[K]) => void;
  errorsUpdate?: (errors: Partial<Record<keyof AppConfigMediaServer, string>>) => void;
  onConnectionVerifiedChange?: (verified: boolean, checking: boolean) => void;
}

// The settings of a media server that isn't the main one, used in Settings and during setup
export const OtherMediaServerFields: React.FC<OtherMediaServerFieldsProps> = ({
  serverKey,
  server,
  mainLibraries,
  isNew,
  highlighted = false,
  dirtyFields = {},
  actions,
  footer,
  onChange,
  errorsUpdate,
  onConnectionVerifiedChange,
}) => {
  // The main server library each library matches. Libraries with the same title match unless set otherwise.
  const mainLibraryTitles = mainLibraries.map((library) => library.title);
  const matchFor = (title: string) => {
    const match = server.library_map?.[title];
    if (match === undefined) return mainLibraryTitles.includes(title) ? title : NO_MATCH;
    return match && mainLibraryTitles.includes(match) ? match : NO_MATCH;
  };
  const setMatch = (title: string, value: string) => {
    onChange("library_map", { ...(server.library_map ?? {}), [title]: value === NO_MATCH ? "" : value });
  };

  return (
    <div className={cn("space-y-3 rounded-md border p-3", highlighted ? "border-amber-500" : "border-muted")}>
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <Switch
            id={`media-server-enabled-${serverKey}`}
            checked={server.enabled ?? true}
            onCheckedChange={(enabled) => onChange("enabled", enabled)}
          />
          <Label htmlFor={`media-server-enabled-${serverKey}`}>Enabled</Label>
          <PopoverHelp ariaLabel="help-media-server-enabled">
            <p>Turn off to stop applying images to this media server, without removing it.</p>
          </PopoverHelp>
        </div>
        <div className="flex items-center gap-2">{actions}</div>
      </div>

      <ConfigSectionMediaServer
        value={server}
        editing={true}
        showActions={true}
        configAlreadyLoaded={!isNew}
        hideEmptyFieldErrors={isNew}
        title={server.name || server.type || "Media Server"}
        isMainServer={false}
        showName={true}
        dirtyFields={dirtyFields}
        onChange={onChange}
        errorsUpdate={errorsUpdate}
        onConnectionVerifiedChange={onConnectionVerifiedChange}
      />

      {/* Library Mapping */}
      {server.libraries.length > 0 && (
        <div
          className={cn(
            "space-y-2 rounded-md border p-3",
            dirtyFields.library_map ? "border-amber-500" : "border-muted"
          )}
        >
          <div className="flex items-center justify-between">
            <Label>Library Mapping</Label>
            <PopoverHelp ariaLabel="help-media-server-library-mapping">
              <p className="mb-2">Which main media server library each of these libraries matches.</p>
              <p className="text-muted-foreground">
                Images applied to an item in a main server library are also applied to the same item here. Libraries
                with the same name match automatically.
              </p>
            </PopoverHelp>
          </div>
          {server.libraries.map((library) => (
            <div key={library.title} className="flex items-center justify-between gap-3">
              <span className="truncate text-sm">{library.title}</span>
              <Select value={matchFor(library.title)} onValueChange={(value) => setMatch(library.title, value)}>
                <SelectTrigger className="w-48 cursor-pointer">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {mainLibraryTitles.map((title) => (
                    <SelectItem key={title} value={title} className="cursor-pointer">
                      {title}
                    </SelectItem>
                  ))}
                  <SelectItem value={NO_MATCH} className="cursor-pointer">
                    Don't match
                  </SelectItem>
                </SelectContent>
              </Select>
            </div>
          ))}
        </div>
      )}

      {footer}
    </div>
  );
};
