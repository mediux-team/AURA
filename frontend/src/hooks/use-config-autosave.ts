"use client";

import { configChanges, configValuesEqual, splitChanges } from "@/helper/config/config-changes";
import { configSectionLabel, configSettingLabel } from "@/helper/config/settings-search";
import { UpdateAppConfigSection } from "@/services/config/update-section";
import type { AppConfigSection } from "@/services/config/update-section";
import { toast } from "sonner";

import { useCallback, useEffect, useRef, useState } from "react";

import { useOnboardingStore } from "@/lib/stores/global-store-onboarding";

import type { AppConfig } from "@/types/config/config";

// Settings save automatically shortly after the last change.
// Connection settings like the media server URL and token only save with "Test & Save".
export const CONFIG_GROUP_KEYS: Partial<Record<AppConfigSection, readonly string[]>> = {
  media_server: ["type", "url", "api_token"],
  mediux: ["api_token"],
  auth: ["enabled", "password_enabled", "password", "oidc"],
  sonarr_radarr: ["applications"],
  notifications: ["providers"],
};

// Settings that are never saved from here. aura gets the User ID from the media server
const CONFIG_IGNORED_KEYS: Partial<Record<AppConfigSection, readonly string[]>> = {
  media_server: ["user_id"],
};

// sectionChanges returns the changed settings in a section, without the ignored settings
const sectionChanges = (section: AppConfigSection, saved: unknown, draft: unknown) => {
  const changes = configChanges(saved, draft);
  for (const key of CONFIG_IGNORED_KEYS[section] ?? []) delete changes[key];
  return changes;
};

const AUTOSAVE_DELAY_MS = 800;
const SAVED_MESSAGE_MS = 2500;
const UNDO_TOAST_MS = 6000;

// changedPaths returns the path of each changed setting
const changedPaths = (changes: Record<string, unknown>, prefix = ""): string[] =>
  Object.entries(changes).flatMap(([key, value]) =>
    typeof value === "object" && value !== null && !Array.isArray(value)
      ? changedPaths(value as Record<string, unknown>, `${prefix}${key}.`)
      : [`${prefix}${key}`]
  );

const valueAtPath = (value: unknown, path: string): unknown =>
  path.split(".").reduce<unknown>((current, key) => (current as Record<string, unknown> | undefined)?.[key], value);

// Values of settings that look secret are never shown in messages
const isSecretPath = (path: string) => /token|password|secret|key|webhook/i.test(path);

// savedMessage describes a saved change, e.g. "Level changed to DEBUG" or "Cache Images turned on"
const savedMessage = (
  section: AppConfigSection,
  changes: Record<string, unknown>,
  labelFor?: (section: AppConfigSection, path: string) => string | undefined
): string => {
  const paths = changedPaths(changes);
  if (paths.length !== 1) return `${configSectionLabel(section)} settings saved`;

  const path = paths[0];
  const label =
    labelFor?.(section, path) ??
    configSettingLabel(section, path) ??
    path
      .split(".")
      .pop()!
      .replaceAll("_", " ")
      .replace(/^./, (first) => first.toUpperCase());
  const value = valueAtPath(changes, path);
  if (typeof value === "boolean") return `${label} turned ${value ? "on" : "off"}`;
  if (!isSecretPath(path) && (typeof value === "string" || typeof value === "number") && String(value).length <= 40) {
    return String(value) === "" ? `${label} cleared` : `${label} changed to ${value}`;
  }
  return `${label} changed`;
};

export type ConfigSaveStatus = "idle" | "saving" | "saved" | "error";

export interface ConfigSectionSaveState {
  status: ConfigSaveStatus;
  error?: string;
}

interface UseConfigAutosaveOptions {
  sections: readonly AppConfigSection[];
  savedConfig: AppConfig;
  draftConfig: AppConfig;
  setSavedConfig: React.Dispatch<React.SetStateAction<AppConfig>>;
  setDraftConfig: React.Dispatch<React.SetStateAction<AppConfig>>;
  validationErrors: Partial<Record<AppConfigSection, Record<string, string>>>;
  settingLabel?: (section: AppConfigSection, path: string) => string | undefined; // Names settings that aren't on the Settings page
}

// Whether a validation error is for one of the group keys
const isGroupError = (errorKey: string, groupKeys: readonly string[]) => {
  const field = errorKey.split(/[.[]/)[0].toLowerCase();
  return groupKeys.some((key) => key.replaceAll("_", "").toLowerCase() === field.replaceAll("_", ""));
};

export function useConfigAutosave({
  sections,
  savedConfig,
  draftConfig,
  setSavedConfig,
  setDraftConfig,
  validationErrors,
  settingLabel,
}: UseConfigAutosaveOptions) {
  const [saveState, setSaveState] = useState<Partial<Record<AppConfigSection, ConfigSectionSaveState>>>({});

  // Refs so saves always use the latest config
  const savedRef = useRef(savedConfig);
  const draftRef = useRef(draftConfig);
  savedRef.current = savedConfig;
  draftRef.current = draftConfig;

  const timers = useRef<Partial<Record<AppConfigSection, ReturnType<typeof setTimeout>>>>({});
  const scheduledChanges = useRef<Partial<Record<AppConfigSection, string>>>({});
  const failedChanges = useRef<Partial<Record<AppConfigSection, string>>>({});
  const savedMessageTimers = useRef<Partial<Record<AppConfigSection, ReturnType<typeof setTimeout>>>>({});
  const inFlight = useRef<Partial<Record<AppConfigSection, boolean>>>({});
  const queued = useRef<Partial<Record<AppConfigSection, { includeGroup: boolean }>>>({});

  const setSectionState = (section: AppConfigSection, state: ConfigSectionSaveState) =>
    setSaveState((prev) => ({ ...prev, [section]: state }));

  const getChanges = useCallback((section: AppConfigSection) => {
    const changes = sectionChanges(section, savedRef.current[section], draftRef.current[section]);
    return splitChanges(changes, CONFIG_GROUP_KEYS[section] ?? []);
  }, []);

  // Save the changes in a section. Connection changes are only included when includeGroup is true.
  // options.changes saves those changes instead, used for undo. options.silent skips the undo toast.
  const saveSection = useCallback(
    async (
      section: AppConfigSection,
      includeGroup: boolean,
      options?: { changes?: Record<string, unknown>; silent?: boolean }
    ): Promise<boolean> => {
      if (inFlight.current[section]) {
        // Save again once the current save finishes
        queued.current[section] = { includeGroup: includeGroup || !!queued.current[section]?.includeGroup };
        return false;
      }

      const sentDraft = draftRef.current[section];
      const previousSaved = savedRef.current[section];
      const { autoChanges, groupChanges } = getChanges(section);
      const changes = options?.changes ?? (includeGroup ? { ...autoChanges, ...groupChanges } : autoChanges);
      if (Object.keys(changes).length === 0) return true;

      const changesKey = JSON.stringify(changes);
      inFlight.current[section] = true;
      setSectionState(section, { status: "saving" });
      const response = await UpdateAppConfigSection(section, changes);
      inFlight.current[section] = false;

      let success = false;
      if (response.status === "error" || !response.data) {
        // Keep the changes, so they can be fixed and saved again
        failedChanges.current[section] = changesKey;
        setSectionState(section, { status: "error", error: response.error?.message || "Failed to save" });
      } else {
        success = true;
        delete failedChanges.current[section];
        const status = response.data.status;
        const serverSection = status.current_setup[section];

        // The saved values come back from the server with secrets masked
        setSavedConfig((prev) => ({ ...prev, [section]: serverSection }));
        // Show the saved values, unless the settings changed while saving. Unsaved connection changes are kept
        setDraftConfig((prev) => {
          if (!configValuesEqual(prev[section], sentDraft)) return prev;
          const next = { ...(serverSection as object) } as Record<string, unknown>;
          if (!includeGroup) {
            for (const key of Object.keys(groupChanges)) {
              next[key] = (prev[section] as unknown as Record<string, unknown>)[key];
            }
          }
          return { ...prev, [section]: next };
        });
        // Keep the rest of the app in sync
        useOnboardingStore.getState().setStatus(status);

        setSectionState(section, { status: "saved" });

        // Offer to undo automatic saves, but not connection changes
        if (!includeGroup && !options?.silent) {
          toast.success(savedMessage(section, changes, settingLabel), {
            id: `config-saved-${section}`,
            duration: UNDO_TOAST_MS,
            action: {
              label: "Undo",
              onClick: () => {
                // Save the previous values of the changed settings
                const undoChanges = Object.fromEntries(
                  Object.keys(changes).map((key) => [key, (previousSaved as unknown as Record<string, unknown>)[key]])
                );
                void saveSection(section, false, { changes: undoChanges, silent: true }).then((undone) => {
                  if (undone) toast.success("Change undone", { id: `config-saved-${section}` });
                });
              },
            },
          });
        }
        clearTimeout(savedMessageTimers.current[section]);
        savedMessageTimers.current[section] = setTimeout(() => {
          setSaveState((prev) =>
            prev[section]?.status === "saved" ? { ...prev, [section]: { status: "idle" } } : prev
          );
        }, SAVED_MESSAGE_MS);
      }

      const next = queued.current[section];
      if (next) {
        delete queued.current[section];
        void saveSection(section, next.includeGroup);
      }
      return success;
    },
    [getChanges, setDraftConfig, setSavedConfig, settingLabel]
  );

  // Schedule an automatic save for each section with changes
  useEffect(() => {
    for (const section of sections) {
      const { autoChanges } = getChanges(section);
      const changesKey = Object.keys(autoChanges).length > 0 ? JSON.stringify(autoChanges) : "";

      // Already scheduled for these changes
      if (scheduledChanges.current[section] === changesKey) continue;
      scheduledChanges.current[section] = changesKey;
      clearTimeout(timers.current[section]);
      delete timers.current[section];

      if (!changesKey) continue;
      // Don't retry failed changes until they change again or Retry is clicked
      if (failedChanges.current[section] === changesKey) continue;
      // Don't save while the section has validation errors, other than connection settings
      const groupKeys = CONFIG_GROUP_KEYS[section] ?? [];
      const errors = Object.keys(validationErrors[section] ?? {}).filter((key) => !isGroupError(key, groupKeys));
      if (errors.length > 0) continue;

      timers.current[section] = setTimeout(() => {
        delete timers.current[section];
        scheduledChanges.current[section] = "";
        void saveSection(section, false);
      }, AUTOSAVE_DELAY_MS);
    }
  }, [draftConfig, savedConfig, validationErrors, sections, getChanges, saveSection]);

  // Save any scheduled changes right away when leaving the page
  useEffect(() => {
    const pendingTimers = timers.current;
    const flush = () => {
      for (const section of Object.keys(pendingTimers) as AppConfigSection[]) {
        clearTimeout(pendingTimers[section]);
        delete pendingTimers[section];
        void saveSection(section, false);
      }
    };
    const handleBeforeUnload = (event: BeforeUnloadEvent) => {
      flush();
      // Warn about connection changes that have not been saved with "Test & Save"
      const hasUnsavedGroupChanges = sections.some(
        (section) => Object.keys(getChanges(section).groupChanges).length > 0
      );
      if (hasUnsavedGroupChanges) {
        event.preventDefault();
      }
    };
    window.addEventListener("beforeunload", handleBeforeUnload);
    return () => {
      window.removeEventListener("beforeunload", handleBeforeUnload);
      flush();
    };
  }, [sections, getChanges, saveSection]);

  // Whether a section has connection changes waiting for "Test & Save"
  const hasGroupChanges = useCallback(
    (section: AppConfigSection) =>
      Object.keys(
        splitChanges(
          sectionChanges(section, savedConfig[section], draftConfig[section]),
          CONFIG_GROUP_KEYS[section] ?? []
        ).groupChanges
      ).length > 0,
    [savedConfig, draftConfig]
  );

  // Whether a section has changes that can't save because of validation errors
  const isBlockedByErrors = useCallback(
    (section: AppConfigSection) => {
      const groupKeys = CONFIG_GROUP_KEYS[section] ?? [];
      const { autoChanges } = splitChanges(
        sectionChanges(section, savedConfig[section], draftConfig[section]),
        groupKeys
      );
      const errors = Object.keys(validationErrors[section] ?? {}).filter((key) => !isGroupError(key, groupKeys));
      return Object.keys(autoChanges).length > 0 && errors.length > 0;
    },
    [savedConfig, draftConfig, validationErrors]
  );

  // Save all changes in a section, including connection changes. Used by Test & Save
  const saveGroup = useCallback((section: AppConfigSection) => saveSection(section, true), [saveSection]);

  // Retry a failed save
  const retry = useCallback(
    (section: AppConfigSection) => {
      delete failedChanges.current[section];
      return saveSection(section, hasGroupChanges(section));
    },
    [saveSection, hasGroupChanges]
  );

  // Undo connection changes that have not been saved
  const discardGroup = useCallback(
    (section: AppConfigSection) => {
      const groupKeys = CONFIG_GROUP_KEYS[section] ?? [];
      setDraftConfig((prev) => {
        const next = { ...(prev[section] as object) } as Record<string, unknown>;
        for (const key of groupKeys) {
          next[key] = (savedRef.current[section] as unknown as Record<string, unknown>)[key];
        }
        return { ...prev, [section]: next };
      });
      setSaveState((prev) => ({ ...prev, [section]: { status: "idle" } }));
    },
    [setDraftConfig]
  );

  return { saveState, hasGroupChanges, isBlockedByErrors, saveGroup, retry, discardGroup };
}
