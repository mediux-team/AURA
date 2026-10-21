"use client";

import { configDirtyFields } from "@/helper/config/config-changes";
import { ReturnErrorMessage } from "@/services/api-error-return";
import { ExportAppConfig, ImportAppConfig } from "@/services/config/export-import";
import { GetAppConfigStatus } from "@/services/config/status";
import { GetNotificationTemplateVariables } from "@/services/config/template-variables";
import type { AppConfigSection } from "@/services/config/update-section";
import { DownloadDebugBundle } from "@/services/logs/debug-bundle";
import {
  Bell,
  Bug,
  Clapperboard,
  DatabaseBackup,
  Download,
  ImageIcon,
  Loader2,
  ScrollText,
  Server,
  ShieldCheck,
  Tags,
  TriangleAlert,
  Upload,
  UserCog,
  Wrench,
} from "lucide-react";
import { toast } from "sonner";

import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";

import { useRouter, useSearchParams } from "next/navigation";

import { ConfigSaveStatus } from "@/components/settings-onboarding/ConfigSaveStatus";
import { ConfigSectionAuth } from "@/components/settings-onboarding/ConfigSectionAuth";
import { ConfigSectionBackups } from "@/components/settings-onboarding/ConfigSectionBackups";
import { ConfigSectionImages, type ImageNameOptions } from "@/components/settings-onboarding/ConfigSectionImages";
import { ConfigSectionLabelsAndTags } from "@/components/settings-onboarding/ConfigSectionLabelsAndTags";
import { ConfigSectionLogging } from "@/components/settings-onboarding/ConfigSectionLogging";
import { ConfigSectionMediaServer } from "@/components/settings-onboarding/ConfigSectionMediaServer";
import { ConfigSectionMediux } from "@/components/settings-onboarding/ConfigSectionMediux";
import {
  ConfigSectionNotifications,
  hasSiloMediaServer,
} from "@/components/settings-onboarding/ConfigSectionNotifications";
import { ConfigSectionOtherMediaServers } from "@/components/settings-onboarding/ConfigSectionOtherMediaServers";
import { ConfigSectionSonarrRadarr } from "@/components/settings-onboarding/ConfigSectionSonarrRadarr";
import { UserPreferencesCard } from "@/components/settings-onboarding/UserPreferences";
import { ConfirmDestructiveDialogActionButton } from "@/components/shared/dialog-destructive-action";
import { ErrorMessage } from "@/components/shared/error-message";
import Loader from "@/components/shared/loader";
import { MediuxLogoIcon } from "@/components/shared/mediux-logo-icon";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectSeparator,
  SelectTrigger,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";

import { cn } from "@/lib/cn";
import { ClearAllStores } from "@/lib/stores/clear-all-stores";
import { useOnboardingStore } from "@/lib/stores/global-store-onboarding";
import { useUserPreferencesStore } from "@/lib/stores/global-user-preferences";

import { useConfigAutosave } from "@/hooks/use-config-autosave";

import type { APIResponse } from "@/types/api/api-response";
import type { AppConfig, NotificationTemplateVariablesCatalog } from "@/types/config/config";
import { defaultAppConfig } from "@/types/config/config-default-app";
import type { AppStatusResponse } from "@/types/config/response-status";

interface SettingsSection {
  id: string;
  label: string;
  description: string;
  icon: React.ComponentType<{ className?: string }>;
  group: string;
  configSection?: AppConfigSection; // Config section the settings are saved to
}

const SETTINGS_SECTIONS: SettingsSection[] = [
  {
    id: "mediux",
    label: "MediUX",
    description: "Your MediUX token and the quality of downloaded images.",
    icon: MediuxLogoIcon,
    group: "Connections",
    configSection: "mediux",
  },
  {
    id: "media-server",
    label: "Media Servers",
    description: "The main media server, and any other media servers aura applies images to.",
    icon: Server,
    group: "Connections",
    configSection: "media_server",
  },
  {
    id: "sonarr-radarr",
    label: "Sonarr & Radarr",
    description: "Sonarr and Radarr instances, used for tags.",
    icon: Clapperboard,
    group: "Connections",
    configSection: "sonarr_radarr",
  },
  {
    id: "authentication",
    label: "Authentication",
    description: "Password and Single Sign-On login, and the API key for integrations.",
    icon: ShieldCheck,
    group: "General",
    configSection: "auth",
  },
  {
    id: "images",
    label: "Images",
    description: "Image caching, and saving images next to your media.",
    icon: ImageIcon,
    group: "General",
    configSection: "images",
  },
  {
    id: "labels-and-tags",
    label: "Labels & Tags",
    description: "Labels and tags added to or removed from items when images are applied.",
    icon: Tags,
    group: "General",
    configSection: "labels_and_tags",
  },
  {
    id: "notifications",
    label: "Notifications",
    description: "Where aura sends notifications, and what they say.",
    icon: Bell,
    group: "General",
    configSection: "notifications",
  },
  {
    id: "logging",
    label: "Logging",
    description: "How much aura logs, and the log files.",
    icon: ScrollText,
    group: "General",
    configSection: "logging",
  },
  {
    id: "user-preferences",
    label: "User Preferences",
    description: "Download defaults and display options. These are saved to aura, so they apply on every device.",
    icon: UserCog,
    group: "Personal",
  },
  {
    id: "backups",
    label: "Backups",
    description: "Backups of the database, with your settings and saved sets.",
    icon: DatabaseBackup,
    group: "System",
    configSection: "backups",
  },
  {
    id: "advanced",
    label: "Advanced",
    description: "Debug mode and the app cache in this browser.",
    icon: Wrench,
    group: "System",
  },
];

// Config sections that are objects of settings, not the media server list
type AppConfigObjectSection = Exclude<keyof AppConfig, "media_servers" | "main_media_server">;

// Config sections that save automatically. Defined here so it stays stable
const AUTOSAVE_SECTIONS = SETTINGS_SECTIONS.flatMap((section) =>
  section.configSection ? [section.configSection] : []
);

// Older links to the User Preferences section use #preferences-section
const sectionFromURL = (sectionParam: string | null): string | null => {
  if (typeof window !== "undefined" && window.location.hash === "#preferences-section") return "user-preferences";
  return sectionParam;
};

// highlightSetting scrolls to a setting by its label and briefly highlights it.
// Retries a few times, as the section may still be rendering.
const highlightSetting = (container: HTMLElement | null, label: string, attempt = 0) => {
  if (!container) return;
  const match = Array.from(container.querySelectorAll("label, h2, h3")).find(
    (element) => element.textContent?.trim().toLowerCase() === label.trim().toLowerCase()
  );
  if (!match) {
    // Open collapsed sections like Advanced and look again
    if (attempt === 2) {
      container
        .querySelectorAll<HTMLElement>('[data-slot="accordion-trigger"][data-state="closed"]')
        .forEach((trigger) => trigger.click());
    }
    if (attempt < 10) setTimeout(() => highlightSetting(container, label, attempt + 1), 150);
    return;
  }
  // Highlight the setting's box, or the label if it isn't in one
  const target = (match.closest(".border, .rounded-md") as HTMLElement | null) ?? (match as HTMLElement);
  target.scrollIntoView({ behavior: "smooth", block: "center" });
  const classes = ["ring-2", "ring-primary", "ring-offset-2", "ring-offset-background", "transition-shadow"];
  target.classList.add(...classes);
  setTimeout(() => target.classList.remove(...classes), 2500);
};

// Dirty field shapes for each settings card, built by configDirtyFields
const dirtyFieldsAs = <T,>(dirty: unknown): T => dirty as T;

type ValidationErrors = Partial<Record<AppConfigSection, Record<string, string>>>;

function SettingsPageContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const isMounted = useRef(false);
  const contentRef = useRef<HTMLDivElement>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<APIResponse<unknown> | null>(null);

  // savedConfig is what's saved in aura; draftConfig includes unsaved changes
  const [savedConfig, setSavedConfig] = useState<AppConfig>(() => defaultAppConfig());
  const [draftConfig, setDraftConfig] = useState<AppConfig>(() => defaultAppConfig());
  const [notificationTemplateVariables, setNotificationTemplateVariables] =
    useState<NotificationTemplateVariablesCatalog | null>(null);
  const [imageNameOptions, setImageNameOptions] = useState<ImageNameOptions | null>(null);
  const [validationErrors, setValidationErrors] = useState<ValidationErrors>({});
  const [apiKeyConfigured, setApiKeyConfigured] = useState(false);
  const [authDisabledByEnv, setAuthDisabledByEnv] = useState(false);

  // Export / Import
  const [exporting, setExporting] = useState(false);
  const [creatingBundle, setCreatingBundle] = useState(false);
  const [exportDialogOpen, setExportDialogOpen] = useState(false);
  const [importFile, setImportFile] = useState<{ name: string; text: string } | null>(null);
  const [importing, setImporting] = useState(false);
  const [importError, setImportError] = useState<string | null>(null);
  const importInputRef = useRef<HTMLInputElement>(null);

  const [activeSectionId, setActiveSectionId] = useState(SETTINGS_SECTIONS[0].id);
  const [groupSaving, setGroupSaving] = useState(false);

  // State - Debug Mode
  const [debugEnabled, setDebugEnabled] = useState(false);

  useEffect(() => {
    const savedMode = localStorage.getItem("debugMode") === "true";
    setDebugEnabled(savedMode);
  }, []);

  const toggleDebugMode = (checked: boolean) => {
    setDebugEnabled(checked);
    localStorage.setItem("debugMode", checked.toString());
  };

  const fetchAndSetConfig = async () => {
    try {
      setLoading(true);
      const response = await GetAppConfigStatus();
      const templateVariableResponse = await GetNotificationTemplateVariables();
      if (response.status === "error") {
        setError(response);
        setSavedConfig(defaultAppConfig());
        setDraftConfig(defaultAppConfig());
        return;
      }

      if (templateVariableResponse.status === "success" && templateVariableResponse.data?.variables) {
        setNotificationTemplateVariables(templateVariableResponse.data.variables);
        setImageNameOptions({
          variables: templateVariableResponse.data.image_name_variables ?? [],
          defaults: templateVariableResponse.data.image_name_defaults,
        });
      }

      const cfg = response.data?.status.current_setup ?? defaultAppConfig();
      setSavedConfig(cfg);
      setDraftConfig(cfg);
      setApiKeyConfigured(response.data?.status.api_key_configured ?? false);
      setAuthDisabledByEnv(response.data?.status.auth_disabled_by_env ?? false);
      setError(null);
    } catch (error) {
      setError(ReturnErrorMessage<AppConfig>(error));
      setSavedConfig(defaultAppConfig());
      setDraftConfig(defaultAppConfig());
    } finally {
      setLoading(false);
    }
  };

  // Fetch configuration data on mount
  useEffect(() => {
    if (isMounted.current) return;
    isMounted.current = true;
    fetchAndSetConfig();
  }, []);

  // Open the section from the URL and highlight the setting
  const sectionParam = searchParams.get("section");
  const settingParam = searchParams.get("setting");
  useEffect(() => {
    const sectionId = sectionFromURL(sectionParam);
    if (sectionId && SETTINGS_SECTIONS.some((section) => section.id === sectionId)) {
      setActiveSectionId(sectionId);
    }
  }, [sectionParam]);
  useEffect(() => {
    if (loading || !settingParam) return;
    highlightSetting(contentRef.current, settingParam);
  }, [loading, sectionParam, settingParam]);

  // User Preferences are saved by their own store
  const preferencesSaveStatus = useUserPreferencesStore((state) => state.saveStatus);
  const preferencesSaveError = useUserPreferencesStore((state) => state.saveError);
  const retryPreferencesSave = useUserPreferencesStore((state) => state.retrySave);

  const { saveState, hasGroupChanges, isBlockedByErrors, saveGroup, retry, discardGroup } = useConfigAutosave({
    sections: AUTOSAVE_SECTIONS,
    savedConfig,
    draftConfig,
    setSavedConfig,
    setDraftConfig,
    validationErrors,
  });

  // The other media servers are saved on their own, so only the media server list is updated
  const applyMediaServersStatus = useCallback((status: AppStatusResponse) => {
    const mediaServers = {
      media_servers: status.current_setup.media_servers,
      main_media_server: status.current_setup.main_media_server,
    };
    setSavedConfig((prev) => ({ ...prev, ...mediaServers }));
    setDraftConfig((prev) => ({ ...prev, ...mediaServers }));
    useOnboardingStore.getState().setStatus(status);
  }, []);

  // Changed fields are highlighted in the cards until they are saved
  const dirty = useMemo(
    () => configDirtyFields(savedConfig, draftConfig) as Partial<Record<keyof AppConfig, unknown>>,
    [savedConfig, draftConfig]
  );

  const updateConfigField = useCallback(
    <S extends AppConfigObjectSection, F extends keyof AppConfig[S]>(section: S, field: F, value: AppConfig[S][F]) => {
      setDraftConfig((prev) => ({ ...prev, [section]: { ...prev[section], [field]: value } }));
    },
    []
  );

  const updateImagesField = useCallback(
    <G extends keyof AppConfig["images"], F extends keyof AppConfig["images"][G]>(
      group: G,
      field: F,
      value: AppConfig["images"][G][F]
    ) => {
      setDraftConfig((prev) => ({
        ...prev,
        images: { ...prev.images, [group]: { ...prev.images[group], [field]: value } },
      }));
    },
    []
  );

  const updateSectionErrors = useCallback(
    (section: AppConfigSection, errs: Record<string, string> | Partial<Record<string, string>>) => {
      setValidationErrors((prev) => {
        const cleaned = Object.fromEntries(Object.entries(errs ?? {}).filter(([, message]) => !!message)) as Record<
          string,
          string
        >;
        if (Object.keys(cleaned).length === 0) {
          if (!prev[section]) return prev;
          const { [section]: _, ...rest } = prev;
          return rest;
        }
        return { ...prev, [section]: cleaned };
      });
    },
    []
  );

  // Labels & Tags only apply to Plex, or with Sonarr/Radarr
  const labelsAndTagsAvailable =
    draftConfig.media_server.type === "Plex" ||
    (Array.isArray(draftConfig.sonarr_radarr.applications) && draftConfig.sonarr_radarr.applications.length > 0);
  const visibleSections = SETTINGS_SECTIONS.filter(
    (section) => section.id !== "labels-and-tags" || labelsAndTagsAvailable
  );
  const activeSection = visibleSections.find((section) => section.id === activeSectionId) ?? visibleSections[0];
  const activeConfigSection = activeSection.configSection;

  const selectSection = (sectionId: string) => {
    setActiveSectionId(sectionId);
    window.history.replaceState(null, "", `?section=${sectionId}`);
  };

  // Show a dot on sections that need attention
  const sectionIndicator = (section: SettingsSection): "error" | "unsaved" | null => {
    const configSection = section.configSection;
    if (!configSection) return null;
    if (saveState[configSection]?.status === "error" || validationErrors[configSection]) return "error";
    if (hasGroupChanges(configSection) || isBlockedByErrors(configSection)) return "unsaved";
    return null;
  };

  const handleExport = async (redacted: boolean) => {
    setExporting(true);
    const result = await ExportAppConfig(redacted);
    setExporting(false);
    setExportDialogOpen(false);
    if (!result.ok) {
      toast.error(`Failed to export configuration: ${result.error}`);
      return;
    }
    const url = URL.createObjectURL(result.blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = result.filename;
    link.click();
    URL.revokeObjectURL(url);
    toast.success(redacted ? "Redacted configuration exported" : "Configuration exported");
  };

  const handleDebugBundle = async () => {
    setCreatingBundle(true);
    const result = await DownloadDebugBundle();
    setCreatingBundle(false);
    if (!result.ok) {
      toast.error(`Failed to download the debug bundle: ${result.error}`);
      return;
    }
    toast.success("Debug bundle downloaded");
  };

  const handleImportFileChosen = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = ""; // Allow choosing the same file again
    if (!file) return;
    setImportError(null);
    setImportFile({ name: file.name, text: await file.text() });
  };

  const handleImportConfirm = async () => {
    if (!importFile) return;
    setImporting(true);
    setImportError(null);
    const response = await ImportAppConfig(importFile.text);
    setImporting(false);
    if (response.status === "error") {
      setImportError(response.error?.message || "Failed to import configuration");
      return;
    }
    setImportFile(null);
    if (response.data?.status) {
      useOnboardingStore.getState().setStatus(response.data.status);
    }
    toast.success(
      response.status === "warn" ? "The file matches your current configuration" : "Configuration imported"
    );
    await fetchAndSetConfig();
  };

  const handleTestAndSave = async (section: AppConfigSection, label: string) => {
    setGroupSaving(true);
    const success = await saveGroup(section);
    setGroupSaving(false);
    if (success) {
      toast.success(`${label} settings saved`);
    } else {
      toast.error(`Failed to save ${label} settings`);
    }
  };

  const groupChangesPending = activeConfigSection ? hasGroupChanges(activeConfigSection) : false;
  const activeSectionHasErrors = activeConfigSection ? !!validationErrors[activeConfigSection] : false;

  const renderSectionContent = () => {
    switch (activeSection.id) {
      case "mediux":
        return (
          <ConfigSectionMediux
            value={draftConfig.mediux}
            editing={true}
            showActions={true}
            configAlreadyLoaded={true}
            dirtyFields={dirtyFieldsAs(dirty.mediux)}
            onChange={(field, value) => updateConfigField("mediux", field, value)}
            errorsUpdate={(errs) => updateSectionErrors("mediux", errs as Record<string, string>)}
          />
        );
      case "media-server":
        return (
          <>
            <ConfigSectionMediaServer
              value={draftConfig.media_server}
              editing={true}
              showActions={true}
              configAlreadyLoaded={true}
              title={draftConfig.media_server.name || draftConfig.media_server.type || "Media Server"}
              badge="Main"
              showName={true}
              dirtyFields={dirtyFieldsAs(dirty.media_server)}
              onChange={(field, value) => updateConfigField("media_server", field, value)}
              errorsUpdate={(errs) => updateSectionErrors("media_server", errs as Record<string, string>)}
            />
            <ConfigSectionOtherMediaServers
              servers={savedConfig.media_servers ?? []}
              mainMediaServer={savedConfig.main_media_server ?? ""}
              mainLibraries={savedConfig.media_server.libraries ?? []}
              onSaved={applyMediaServersStatus}
            />
          </>
        );
      case "sonarr-radarr":
        return (
          <ConfigSectionSonarrRadarr
            value={draftConfig.sonarr_radarr}
            editing={true}
            dirtyFields={dirtyFieldsAs(dirty.sonarr_radarr)}
            onChange={(field, val) => updateConfigField("sonarr_radarr", field, val)}
            errorsUpdate={(errs) => updateSectionErrors("sonarr_radarr", errs as Record<string, string>)}
            configAlreadyLoaded={true}
            libraries={draftConfig.media_server.libraries || []}
          />
        );
      case "authentication":
        return (
          <>
            {authDisabledByEnv && (
              <Alert className="mb-4 border-amber-500 text-amber-500">
                <TriangleAlert className="h-4 w-4" />
                <AlertTitle>Authentication is turned off by AURA_DISABLE_AUTH</AlertTitle>
                <AlertDescription>
                  Anyone who can reach aura can use it, even if authentication is enabled below. Once you have access
                  again (e.g. after setting a new password), remove the AURA_DISABLE_AUTH environment variable and
                  restart aura.
                </AlertDescription>
              </Alert>
            )}
            <ConfigSectionAuth
              value={draftConfig.auth}
              editing={true}
              dirtyFields={dirtyFieldsAs(dirty.auth)}
              onChange={(field, value) => updateConfigField("auth", field, value)}
              errorsUpdate={(errs) => updateSectionErrors("auth", errs as Record<string, string>)}
              apiKeyConfigured={apiKeyConfigured}
            />
          </>
        );
      case "images":
        return (
          <ConfigSectionImages
            value={draftConfig.images}
            editing={true}
            showActions={true}
            dirtyFields={dirtyFieldsAs(dirty.images)}
            onChange={updateImagesField}
            errorsUpdate={(errs) => updateSectionErrors("images", errs as Record<string, string>)}
            mediaServerType={draftConfig.media_server.type}
            imageNameOptions={imageNameOptions}
          />
        );
      case "labels-and-tags":
        return (
          <ConfigSectionLabelsAndTags
            value={draftConfig.labels_and_tags}
            editing={true}
            dirtyFields={dirtyFieldsAs(dirty.labels_and_tags)}
            mediaServerType={draftConfig.media_server.type}
            srOptions={Array.from(
              new Set((draftConfig.sonarr_radarr.applications ?? []).map((app) => app.type).filter((type) => !!type))
            )}
            onChange={(field, val) => updateConfigField("labels_and_tags", field, val)}
            errorsUpdate={(errs) => updateSectionErrors("labels_and_tags", errs as Record<string, string>)}
          />
        );
      case "notifications":
        return (
          <ConfigSectionNotifications
            value={draftConfig.notifications}
            editing={true}
            dirtyFields={dirtyFieldsAs(dirty.notifications)}
            onChange={(field, val) => updateConfigField("notifications", field, val)}
            errorsUpdate={(errs) => updateSectionErrors("notifications", errs as Record<string, string>)}
            // Checking a provider sends a test notification, so only check when Test is clicked
            configAlreadyLoaded={false}
            templateVariablesCatalog={notificationTemplateVariables}
            showSiloPlugin={hasSiloMediaServer(draftConfig)}
          />
        );
      case "logging":
        return (
          <ConfigSectionLogging
            value={draftConfig.logging}
            editing={true}
            showActions={true}
            dirtyFields={dirtyFieldsAs(dirty.logging)}
            onChange={(field, value) => updateConfigField("logging", field, value)}
            errorsUpdate={(errs) => updateSectionErrors("logging", errs as Record<string, string>)}
          />
        );
      case "backups":
        return (
          <ConfigSectionBackups
            value={draftConfig.backups}
            dirtyFields={dirtyFieldsAs(dirty.backups)}
            onChange={(field, value) => updateConfigField("backups", field, value)}
            errorsUpdate={(errs) => updateSectionErrors("backups", errs as Record<string, string>)}
          />
        );
      case "user-preferences":
        return <UserPreferencesCard />;
      case "advanced":
        return (
          <Card className="gap-4 border-muted p-4 sm:p-5">
            <div className="flex flex-col gap-3 border rounded-md p-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <Label>Export Configuration</Label>
                <p className="text-sm text-muted-foreground">
                  Download your configuration as a YAML file. Export the full file to back it up or move it to another
                  install, or a redacted one to share when you need help.
                </p>
              </div>
              <Button
                variant="outline"
                onClick={() => setExportDialogOpen(true)}
                disabled={exporting}
                className="cursor-pointer shrink-0"
              >
                {exporting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
                Export
              </Button>
            </div>

            <div className="flex flex-col gap-3 border rounded-md p-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <Label>Import Configuration</Label>
                <p className="text-sm text-muted-foreground">
                  Replace your configuration with an exported file (or a config.yaml). The connections are checked
                  before anything is saved.
                </p>
              </div>
              <input
                ref={importInputRef}
                type="file"
                accept=".yaml,.yml,application/x-yaml,text/yaml"
                className="hidden"
                onChange={handleImportFileChosen}
              />
              <Button
                variant="outline"
                onClick={() => importInputRef.current?.click()}
                className="cursor-pointer shrink-0"
              >
                <Upload className="h-4 w-4" />
                Import
              </Button>
            </div>

            <div className="flex flex-col gap-3 border rounded-md p-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <Label>Debug Bundle</Label>
                <p className="text-sm text-muted-foreground">
                  Download a zip with your versions, media servers, redacted configuration and recent logs, to attach
                  when reporting a problem. Passwords, tokens, API keys, emails and addresses are replaced.
                </p>
              </div>
              <Button
                variant="outline"
                onClick={handleDebugBundle}
                disabled={creatingBundle}
                className="cursor-pointer shrink-0"
              >
                {creatingBundle ? <Loader2 className="h-4 w-4 animate-spin" /> : <Bug className="h-4 w-4" />}
                Download
              </Button>
            </div>

            <div className="flex items-center justify-between gap-4 border rounded-md p-3">
              <div>
                <Label htmlFor="debug-mode">Debug Mode</Label>
                <p className="text-sm text-muted-foreground">Show extra details for troubleshooting in this browser.</p>
              </div>
              <Switch id="debug-mode" checked={debugEnabled} onCheckedChange={toggleDebugMode} />
            </div>

            <div className="flex items-center justify-between gap-4 border rounded-md p-3">
              <div>
                <Label>App Cache</Label>
                <p className="text-sm text-muted-foreground">
                  Clear the data this browser stores for aura. Your settings are not affected.
                </p>
              </div>
              <ConfirmDestructiveDialogActionButton
                onConfirm={async () => {
                  localStorage.clear();
                  await ClearAllStores();
                  toast.success("App Cache Cleared. Reloading...", { duration: 750 });
                  setTimeout(() => {
                    router.replace("/settings");
                  }, 1000);
                }}
                title="Clear App Cache?"
                description="This will clear all local storage and IndexedDB data. Are you sure you want to continue?"
                confirmText="Yes, Clear Cache"
                cancelText="Cancel"
                variant="ghost"
                className="text-destructive border-1 shadow-none hover:text-red-500 cursor-pointer"
              >
                Clear App Cache
              </ConfirmDestructiveDialogActionButton>
            </div>
          </Card>
        );
      default:
        return null;
    }
  };

  const groups = Array.from(new Set(visibleSections.map((section) => section.group)));
  const ActiveIcon = activeSection.icon;

  return (
    <div className="container mx-auto max-w-6xl p-4 sm:p-6 lg:p-8">
      {loading ? (
        <Loader message="Loading configuration..." />
      ) : error ? (
        <ErrorMessage error={error} />
      ) : (
        <>
          <div className="mb-6">
            <h1 className="text-3xl font-bold">Settings</h1>
          </div>

          <div className="md:grid md:grid-cols-[13rem_minmax(0,1fr)] md:gap-8">
            {/* Desktop: Section list */}
            <nav className="hidden md:block" aria-label="Settings sections">
              <div className="sticky top-20 space-y-5">
                {groups.map((group) => (
                  <div key={group}>
                    <p className="mb-1 px-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                      {group}
                    </p>
                    <div className="space-y-0.5">
                      {visibleSections
                        .filter((section) => section.group === group)
                        .map((section) => {
                          const Icon = section.icon;
                          const indicator = sectionIndicator(section);
                          const active = section.id === activeSection.id;
                          return (
                            <button
                              key={section.id}
                              type="button"
                              onClick={() => selectSection(section.id)}
                              aria-current={active ? "page" : undefined}
                              className={cn(
                                "flex w-full cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-left text-sm transition-colors",
                                active
                                  ? "bg-primary/10 font-medium text-primary"
                                  : "text-muted-foreground hover:bg-muted hover:text-foreground"
                              )}
                            >
                              <Icon className="h-4 w-4 shrink-0" />
                              <span className="flex-1 truncate">{section.label}</span>
                              {indicator && (
                                <span
                                  className={cn(
                                    "h-2 w-2 shrink-0 rounded-full",
                                    indicator === "error" ? "bg-red-500" : "bg-amber-500"
                                  )}
                                  title={indicator === "error" ? "Needs attention" : "Not saved yet"}
                                />
                              )}
                            </button>
                          );
                        })}
                    </div>
                  </div>
                ))}
              </div>
            </nav>

            <div className="min-w-0">
              {/* Mobile: Section picker */}
              <div className="mb-3 md:hidden">
                <Select value={activeSection.id} onValueChange={selectSection}>
                  <SelectTrigger className="h-11 w-full cursor-pointer" aria-label="Settings section">
                    <span className="flex items-center gap-2">
                      <ActiveIcon className="h-4 w-4 text-primary" />
                      <span className="font-medium">{activeSection.label}</span>
                    </span>
                  </SelectTrigger>
                  <SelectContent>
                    {groups.map((group, groupIndex) => (
                      <SelectGroup key={group}>
                        {groupIndex > 0 && <SelectSeparator />}
                        <SelectLabel>{group}</SelectLabel>
                        {visibleSections
                          .filter((section) => section.group === group)
                          .map((section) => {
                            const Icon = section.icon;
                            const indicator = sectionIndicator(section);
                            return (
                              <SelectItem key={section.id} value={section.id} className="cursor-pointer">
                                <span className="flex w-full items-center gap-2">
                                  <Icon className="h-4 w-4" />
                                  {section.label}
                                  {indicator && (
                                    <span
                                      className={cn(
                                        "ml-1 h-2 w-2 rounded-full",
                                        indicator === "error" ? "bg-red-500" : "bg-amber-500"
                                      )}
                                      title={indicator === "error" ? "Needs attention" : "Not saved yet"}
                                    />
                                  )}
                                </span>
                              </SelectItem>
                            );
                          })}
                      </SelectGroup>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {/* Section header */}
              <div className="mb-5 flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <h2 className="hidden text-xl font-semibold md:block">{activeSection.label}</h2>
                  <p className="text-sm text-muted-foreground">{activeSection.description}</p>
                </div>
                {activeConfigSection && (
                  <ConfigSaveStatus
                    className="shrink-0"
                    state={saveState[activeConfigSection]}
                    blockedByErrors={isBlockedByErrors(activeConfigSection)}
                    onRetry={() => void retry(activeConfigSection)}
                  />
                )}
                {activeSection.id === "user-preferences" && (
                  <ConfigSaveStatus
                    className="shrink-0"
                    state={{ status: preferencesSaveStatus, error: preferencesSaveError }}
                    onRetry={retryPreferencesSave}
                  />
                )}
              </div>

              <div ref={contentRef} key={activeSection.id} className="animate-in fade-in-0 duration-200">
                {renderSectionContent()}
              </div>

              {/* Connection settings are saved together with Test & Save */}
              {activeConfigSection && groupChangesPending && (
                <div className="sticky bottom-4 z-30 mt-6">
                  <div className="mx-auto flex w-full flex-col gap-3 rounded-lg border border-amber-500 bg-background/95 px-4 py-3 shadow-lg backdrop-blur sm:flex-row sm:items-center sm:justify-between">
                    <div className="text-sm">
                      <p className="font-medium">Connection changes not saved yet</p>
                      <p className="text-muted-foreground">
                        {activeSectionHasErrors
                          ? "Fix the errors above, then test and save."
                          : "Test the connection and save these changes together."}
                      </p>
                    </div>
                    <div className="flex gap-2">
                      <Button
                        variant="ghost"
                        onClick={() => discardGroup(activeConfigSection)}
                        disabled={groupSaving}
                        className="cursor-pointer"
                      >
                        Discard
                      </Button>
                      <Button
                        onClick={() => handleTestAndSave(activeConfigSection, activeSection.label)}
                        disabled={groupSaving || activeSectionHasErrors}
                        className="cursor-pointer"
                      >
                        {groupSaving && <Loader2 className="h-4 w-4 animate-spin" />}
                        {groupSaving ? "Testing..." : "Test & Save"}
                      </Button>
                    </div>
                  </div>
                </div>
              )}
            </div>
          </div>

          {/* Choose between a redacted or a full export */}
          <Dialog open={exportDialogOpen} onOpenChange={(open) => !exporting && setExportDialogOpen(open)}>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Export Configuration</DialogTitle>
                <DialogDescription>Choose what to include in the file.</DialogDescription>
              </DialogHeader>
              <div className="flex flex-col gap-2">
                {[
                  {
                    redacted: true,
                    icon: ShieldCheck,
                    title: "Redacted",
                    description:
                      "To share when you need help. Passwords, tokens, API keys, emails and addresses are replaced. It can't be imported.",
                  },
                  {
                    redacted: false,
                    icon: DatabaseBackup,
                    title: "Full",
                    description:
                      "To back up or move to another install. It includes your passwords, tokens and API keys, so keep it private.",
                  },
                ].map((option) => (
                  <button
                    key={option.title}
                    type="button"
                    disabled={exporting}
                    onClick={() => handleExport(option.redacted)}
                    className="flex items-start gap-3 rounded-md border p-3 text-left transition hover:bg-muted/50 cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    <option.icon className="h-5 w-5 mt-0.5 shrink-0 text-primary" />
                    <span>
                      <span className="block font-medium">{option.title}</span>
                      <span className="block text-sm text-muted-foreground">{option.description}</span>
                    </span>
                  </button>
                ))}
              </div>
              <DialogFooter>
                <Button
                  variant="ghost"
                  onClick={() => setExportDialogOpen(false)}
                  disabled={exporting}
                  className="cursor-pointer"
                >
                  {exporting && <Loader2 className="h-4 w-4 animate-spin" />}
                  {exporting ? "Exporting..." : "Cancel"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>

          {/* Confirm before replacing the configuration */}
          <Dialog
            open={importFile !== null}
            onOpenChange={(open) => {
              if (!open && !importing) {
                setImportFile(null);
                setImportError(null);
              }
            }}
          >
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Replace your configuration?</DialogTitle>
                <DialogDescription>
                  All settings will be replaced with the ones in <span className="font-medium">{importFile?.name}</span>
                  . Settings that are not in the file are cleared. Your API key is not changed.
                </DialogDescription>
              </DialogHeader>
              {importError && <p className="text-sm text-red-500">{importError}</p>}
              <DialogFooter>
                <Button
                  variant="ghost"
                  onClick={() => {
                    setImportFile(null);
                    setImportError(null);
                  }}
                  disabled={importing}
                  className="cursor-pointer"
                >
                  Cancel
                </Button>
                <Button onClick={handleImportConfirm} disabled={importing} className="cursor-pointer">
                  {importing && <Loader2 className="h-4 w-4 animate-spin" />}
                  {importing ? "Checking..." : "Replace Configuration"}
                </Button>
              </DialogFooter>
            </DialogContent>
          </Dialog>
        </>
      )}
    </div>
  );
}

// useSearchParams needs a Suspense boundary
export default function SettingsPage() {
  return (
    <Suspense fallback={<Loader message="Loading configuration..." />}>
      <SettingsPageContent />
    </Suspense>
  );
}
