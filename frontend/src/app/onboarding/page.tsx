"use client";

import { makePlural } from "@/helper/make_plural";
import { SETUP_CODE_REQUIRED_EVENT } from "@/services/api-client";
import { finalizeOnboarding } from "@/services/config/onboarding-finalize";
import { ClearSetupCode, VerifySetupCode } from "@/services/config/setup-code";
import { GetNotificationTemplateVariables } from "@/services/config/template-variables";
import { UpdateAppConfig } from "@/services/config/update";
import * as yaml from "js-yaml";
import {
  AlertCircle,
  ArrowLeft,
  ArrowRight,
  Bell,
  CheckCircle2,
  Circle,
  Clapperboard,
  Flag,
  ImageIcon,
  KeyRound,
  Loader2,
  ScrollText,
  Server,
  ServerCog,
  ShieldCheck,
  Sparkles,
  Tags,
  TriangleAlert,
} from "lucide-react";
import { toast } from "sonner";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import Image from "next/image";

import { ConfigSectionAuth } from "@/components/settings-onboarding/ConfigSectionAuth";
import { ConfigSectionImages, type ImageNameOptions } from "@/components/settings-onboarding/ConfigSectionImages";
import { ConfigSectionLabelsAndTags } from "@/components/settings-onboarding/ConfigSectionLabelsAndTags";
import { ConfigSectionLogging } from "@/components/settings-onboarding/ConfigSectionLogging";
import { ConfigSectionMediaServer } from "@/components/settings-onboarding/ConfigSectionMediaServer";
import { ConfigSectionMediux } from "@/components/settings-onboarding/ConfigSectionMediux";
import {
  ConfigSectionNotifications,
  hasSiloMediaServer,
} from "@/components/settings-onboarding/ConfigSectionNotifications";
import { ConfigSectionOtherMediaServersSetup } from "@/components/settings-onboarding/ConfigSectionOtherMediaServersSetup";
import { ConfigSectionSonarrRadarr } from "@/components/settings-onboarding/ConfigSectionSonarrRadarr";
import { NEW_SERVER_PREFIX } from "@/components/settings-onboarding/OtherMediaServerFields";
import { MediuxLogoIcon } from "@/components/shared/mediux-logo-icon";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { H2, P } from "@/components/ui/typography";

import { cn } from "@/lib/cn";
import { redactSecrets } from "@/lib/redact";
import { useOnboardingStore } from "@/lib/stores/global-store-onboarding";

import type { AppConfig, AppConfigMediaServer, NotificationTemplateVariablesCatalog } from "@/types/config/config";
import { defaultAppConfig } from "@/types/config/config-default-app";

type SectionGroup = "Get Started" | "Required" | "Optional" | "Finish";

interface OnboardingSection {
  id: string;
  label: string;
  description: string;
  icon: React.ComponentType<{ className?: string }>;
  group: SectionGroup;
  configSection?: keyof AppConfig; // Config section the card's errors are reported for
}

const SECTIONS: OnboardingSection[] = [
  {
    id: "welcome",
    label: "Welcome",
    description: "A quick setup to get aura running.",
    icon: Sparkles,
    group: "Get Started",
  },
  {
    id: "mediux",
    label: "MediUX",
    description: "Your MediUX token, used to find and download images.",
    icon: MediuxLogoIcon,
    group: "Required",
    configSection: "mediux",
  },
  {
    id: "media-server",
    label: "Media Server",
    description: "The media server aura applies images to, and the libraries it manages.",
    icon: Server,
    group: "Required",
    configSection: "media_server",
  },
  {
    id: "other-media-servers",
    label: "Other Media Servers",
    description: "Other media servers aura also applies images to. Only the main media server saves images locally.",
    icon: ServerCog,
    group: "Optional",
    configSection: "media_servers",
  },
  {
    id: "authentication",
    label: "Authentication",
    description: "Require a password or Single Sign-On to use aura.",
    icon: ShieldCheck,
    group: "Optional",
    configSection: "auth",
  },
  {
    id: "images",
    label: "Images",
    description: "Image caching, and saving images next to your media.",
    icon: ImageIcon,
    group: "Optional",
    configSection: "images",
  },
  {
    id: "sonarr-radarr",
    label: "Sonarr & Radarr",
    description: "Sonarr and Radarr instances, used for tags.",
    icon: Clapperboard,
    group: "Optional",
    configSection: "sonarr_radarr",
  },
  {
    id: "labels-and-tags",
    label: "Labels & Tags",
    description: "Labels and tags added to or removed from items when images are applied.",
    icon: Tags,
    group: "Optional",
    configSection: "labels_and_tags",
  },
  {
    id: "notifications",
    label: "Notifications",
    description: "Where aura sends notifications, and what they say.",
    icon: Bell,
    group: "Optional",
    configSection: "notifications",
  },
  {
    id: "logging",
    label: "Logging",
    description: "How much aura logs.",
    icon: ScrollText,
    group: "Optional",
    configSection: "logging",
  },
  {
    id: "errors",
    label: "Errors",
    description: "Settings that need to be fixed before finishing.",
    icon: TriangleAlert,
    group: "Finish",
  },
  {
    id: "review",
    label: "Review & Finish",
    description: "Check your configuration, then save it and start aura.",
    icon: Flag,
    group: "Finish",
  },
];

const GROUPS: SectionGroup[] = ["Get Started", "Required", "Optional", "Finish"];

// Sections with a connection that must be checked before moving on
const CONNECTION_SECTIONS = ["mediux", "media-server", "other-media-servers", "sonarr-radarr"] as const;
type ConnectionSectionId = (typeof CONNECTION_SECTIONS)[number];
const isConnectionSection = (sectionId: string): sectionId is ConnectionSectionId =>
  (CONNECTION_SECTIONS as readonly string[]).includes(sectionId);

type SectionState = "complete" | "incomplete" | "error";

// Whether the required settings in a required section are filled in
const requiredFieldsFilled = (sectionId: string, config: AppConfig): boolean => {
  switch (sectionId) {
    case "mediux":
      return !!config.mediux.api_token?.trim();
    case "media-server":
      return (
        !!config.media_server.type?.trim() &&
        !!config.media_server.url?.trim() &&
        !!config.media_server.api_token?.trim() &&
        (config.media_server.libraries?.length ?? 0) > 0
      );
    default:
      return true;
  }
};

// Readable name of a config section in the error list
const sectionLabelForConfig = (configSection: string) =>
  SECTIONS.find((section) => section.configSection === configSection)?.label ??
  configSection.replaceAll("_", " ").replace(/\b\w/g, (letter) => letter.toUpperCase());

const OnboardingPage = () => {
  const { status, fetchStatus } = useOnboardingStore();

  // Hydrate/fetch onboarding status on mount
  useEffect(() => {
    if (!status) fetchStatus();
  }, [status, fetchStatus]);

  // Refresh the status when the setup code is missing or wrong, so it is asked for again
  useEffect(() => {
    const handleSetupCodeRequired = () => void fetchStatus();
    window.addEventListener(SETUP_CODE_REQUIRED_EVENT, handleSetupCodeRequired);
    return () => window.removeEventListener(SETUP_CODE_REQUIRED_EVENT, handleSetupCodeRequired);
  }, [fetchStatus]);

  const [applyLoading, setApplyLoading] = useState(false);
  const [configState, setConfigState] = useState<AppConfig>(() => status?.current_setup || defaultAppConfig());
  const [notificationTemplateVariables, setNotificationTemplateVariables] =
    useState<NotificationTemplateVariablesCatalog | null>(null);
  const [imageNameOptions, setImageNameOptions] = useState<ImageNameOptions | null>(null);
  const [validationErrors, setValidationErrors] = useState<Record<string, Record<string, string>>>({});

  const [activeSectionId, setActiveSectionId] = useState(SECTIONS[0].id);
  // Last section in the flow, to go back to from the error list
  const [lastFlowSectionId, setLastFlowSectionId] = useState(SECTIONS[0].id);
  const [visited, setVisited] = useState<Set<string>>(() => new Set([SECTIONS[0].id]));
  // Sections that have been left. Their errors only show from then on
  const [left, setLeft] = useState<Set<string>>(() => new Set());
  const activeSectionIdRef = useRef(activeSectionId);
  activeSectionIdRef.current = activeSectionId;

  // Whether each section's connection has been checked and works
  const [connectionChecks, setConnectionChecks] = useState<
    Partial<Record<ConnectionSectionId, { verified: boolean; checking: boolean }>>
  >({});
  const connectionReporters = useMemo(
    () =>
      Object.fromEntries(
        CONNECTION_SECTIONS.map((sectionId) => [
          sectionId,
          (verified: boolean, checking: boolean) =>
            setConnectionChecks((prev) =>
              prev[sectionId]?.verified === verified && prev[sectionId]?.checking === checking
                ? prev
                : { ...prev, [sectionId]: { verified, checking } }
            ),
        ])
      ) as Record<ConnectionSectionId, (verified: boolean, checking: boolean) => void>,
    []
  );

  const setupCodeRequired = !!status?.setup_code_required;

  const statusLoaded = !!status;
  useEffect(() => {
    // Wait for the status, as the setup code may be needed first
    if (!statusLoaded || setupCodeRequired) return;
    let mounted = true;

    const fetchTemplateVariables = async () => {
      const response = await GetNotificationTemplateVariables();
      if (!mounted) return;
      if (response.status === "success" && response.data?.variables) {
        setNotificationTemplateVariables(response.data.variables);
        setImageNameOptions({
          variables: response.data.image_name_variables ?? [],
          defaults: response.data.image_name_defaults,
        });
      }
    };

    fetchTemplateVariables();

    return () => {
      mounted = false;
    };
  }, [statusLoaded, setupCodeRequired]);

  // Keep configState in sync with backend status if it changes
  useEffect(() => {
    if (status?.current_setup) {
      setConfigState(status.current_setup);
    }
  }, [status?.current_setup]);

  const updateSectionErrors = useCallback((section: string, errs?: Record<string, string>) => {
    setValidationErrors((prev) => {
      const cleaned = Object.fromEntries(Object.entries(errs ?? {}).filter(([, message]) => !!message));
      if (Object.keys(cleaned).length === 0) {
        if (!prev[section]) return prev;
        const { [section]: _, ...rest } = prev;
        return rest;
      }
      return { ...prev, [section]: cleaned };
    });
  }, []);

  const updateSectionField = useCallback(
    <S extends keyof AppConfig, K extends keyof AppConfig[S]>(section: S, field: K, value: AppConfig[S][K]) => {
      setConfigState(
        (prev) =>
          ({
            ...prev,
            [section]: { ...(prev[section] as object), [field]: value },
          }) as AppConfig
      );
    },
    []
  );

  // The other media servers. The main one stays in media_servers when it is already saved.
  const otherMediaServers = useMemo(
    () =>
      (configState.media_servers ?? []).filter(
        (server) => !configState.main_media_server || server.id !== configState.main_media_server
      ),
    [configState.media_servers, configState.main_media_server]
  );
  const updateOtherMediaServers = useCallback((update: (servers: AppConfigMediaServer[]) => AppConfigMediaServer[]) => {
    setConfigState((prev) => {
      const isMain = (server: AppConfigMediaServer) => !!prev.main_media_server && server.id === prev.main_media_server;
      const all = prev.media_servers ?? [];
      return { ...prev, media_servers: [...all.filter(isMain), ...update(all.filter((server) => !isMain(server)))] };
    });
  }, []);
  const updateOtherMediaServerErrors = useCallback(
    (errs: Record<string, string>) => updateSectionErrors("media_servers", errs),
    [updateSectionErrors]
  );

  const updateImagesField = useCallback(
    <G extends keyof AppConfig["images"], F extends keyof AppConfig["images"][G]>(
      group: G,
      field: F,
      value: AppConfig["images"][G][F]
    ) => {
      setConfigState((prev) => ({
        ...prev,
        images: {
          ...prev.images,
          [group]: {
            ...prev.images[group],
            [field]: value,
          },
        },
      }));
    },
    []
  );

  const stripEmpty = useCallback(function stripEmpty(obj: unknown): unknown {
    if (Array.isArray(obj)) {
      return obj
        .map(stripEmpty)
        .filter(
          (v) =>
            v !== undefined &&
            v !== null &&
            !(typeof v === "string" && v === "") &&
            !(Array.isArray(v) && v.length === 0) &&
            !(typeof v === "object" && v && Object.keys(v).length === 0)
        );
    }
    if (typeof obj === "object" && obj !== null) {
      const out: Record<string, unknown> = {};
      for (const [k, v] of Object.entries(obj)) {
        const cleaned = stripEmpty(v);
        if (
          cleaned !== undefined &&
          cleaned !== null &&
          !(typeof cleaned === "string" && cleaned === "") &&
          !(Array.isArray(cleaned) && cleaned.length === 0) &&
          !(typeof cleaned === "object" && cleaned && Object.keys(cleaned).length === 0)
        ) {
          out[k] = cleaned;
        }
      }
      return out;
    }
    return obj;
  }, []);

  // Memoized YAML representation
  const reviewYaml = useMemo(() => {
    try {
      // Mask secrets so they aren't shown on screen
      const plain = stripEmpty(redactSecrets(JSON.parse(JSON.stringify(configState))));
      return yaml.dump(plain, {
        noRefs: true,
        lineWidth: 100,
        skipInvalid: true,
      });
    } catch {
      return "# Failed to serialize configuration to YAML";
    }
  }, [configState, stripEmpty]);

  // Labels & Tags only apply to Plex, or with Sonarr/Radarr
  const labelsAndTagsAvailable =
    configState.media_server.type === "Plex" ||
    (Array.isArray(configState.sonarr_radarr?.applications) &&
      configState.sonarr_radarr.applications.some((app) => !!app.type));

  const totalErrors = Object.values(validationErrors).reduce((sum, errs) => sum + Object.keys(errs).length, 0);

  // Errors for the sidebar and error list, only for sections that have been left
  const sectionIdForConfig = (configSection: string) =>
    SECTIONS.find((section) => section.configSection === configSection)?.id ?? configSection;
  const shownErrors = Object.fromEntries(
    Object.entries(validationErrors).filter(([configSection]) => left.has(sectionIdForConfig(configSection)))
  );
  const totalShownErrors = Object.values(shownErrors).reduce((sum, errs) => sum + Object.keys(errs).length, 0);

  const visibleSections = SECTIONS.filter(
    (section) =>
      (section.id !== "labels-and-tags" || labelsAndTagsAvailable) && (section.id !== "errors" || totalShownErrors > 0)
  );
  // The flow goes through every section except the error list
  const flowSections = visibleSections.filter((section) => section.id !== "errors");
  const activeSection = visibleSections.find((section) => section.id === activeSectionId) ?? visibleSections[0];
  const flowIndex = flowSections.findIndex((section) => section.id === activeSection.id);
  // Welcome isn't a numbered step
  const numberedSections = flowSections.filter((section) => section.id !== "welcome");
  const stepNumber = numberedSections.findIndex((section) => section.id === activeSection.id) + 1;

  const goToSection = useCallback((sectionId: string) => {
    const previousSectionId = activeSectionIdRef.current;
    if (previousSectionId !== sectionId) {
      setLeft((prev) => (prev.has(previousSectionId) ? prev : new Set(prev).add(previousSectionId)));
    }
    setActiveSectionId(sectionId);
    if (sectionId !== "errors") setLastFlowSectionId(sectionId);
    setVisited((prev) => (prev.has(sectionId) ? prev : new Set(prev).add(sectionId)));
    requestAnimationFrame(() => window.scrollTo({ top: 0, behavior: "smooth" }));
  }, []);

  const sectionState = (section: OnboardingSection): SectionState => {
    if (section.configSection && shownErrors[section.configSection]) return "error";
    if (!visited.has(section.id)) return "incomplete";
    if (section.group === "Required" && !requiredFieldsFilled(section.id, configState)) return "incomplete";
    if (isConnectionSection(section.id) && !connectionChecks[section.id]?.verified) return "incomplete";
    return "complete";
  };

  const requiredSections = visibleSections.filter((section) => section.group === "Required");
  const requiredComplete = requiredSections.every((section) => sectionState(section) === "complete");
  const canFinish = requiredComplete && totalErrors === 0;

  // The current section's errors, including empty required fields.
  // Next is blocked until they're fixed; Back always works.
  const currentSectionErrors = activeSection.configSection
    ? Object.values(validationErrors[activeSection.configSection] ?? {})
    : [];
  // Sections with a connection also wait for it to be checked
  const connectionCheck = isConnectionSection(activeSection.id) ? connectionChecks[activeSection.id] : undefined;
  const waitingForConnection = isConnectionSection(activeSection.id) && !connectionCheck?.verified;
  const blockedByErrors = currentSectionErrors.length > 0 || waitingForConnection;
  const canOpenSection = (sectionId: string) => {
    if (!blockedByErrors || sectionId === "errors") return true;
    const targetIndex = flowSections.findIndex((section) => section.id === sectionId);
    return targetIndex !== -1 && targetIndex <= flowIndex;
  };

  const next = () => {
    if (blockedByErrors) return;
    const nextSection = flowSections[flowIndex + 1];
    if (nextSection) goToSection(nextSection.id);
  };
  const prev = () => {
    // From the error list, go back to the section it was opened from
    if (flowIndex === -1) {
      goToSection(lastFlowSectionId);
      return;
    }
    const previousSection = flowSections[flowIndex - 1];
    if (previousSection) goToSection(previousSection.id);
  };

  const finish = async () => {
    if (!canFinish) {
      // Show every error, including sections that haven't been left yet
      setLeft(new Set(SECTIONS.map((section) => section.id)));
      toast.error(totalErrors > 0 ? "Fix the errors first." : "Complete the required sections first.");
      return;
    }
    setApplyLoading(true);

    try {
      // Media servers added during setup get their ID when they are saved
      const resp = await UpdateAppConfig({
        ...configState,
        media_servers: configState.media_servers?.map((server) =>
          server.id?.startsWith(NEW_SERVER_PREFIX) ? { ...server, id: undefined } : server
        ),
      });
      if (resp.status === "success") {
        if (resp.data?.status) {
          const finalizeResp = await finalizeOnboarding(resp.data.status.current_setup);
          if (finalizeResp.status === "success") {
            // Setup is finished, so the setup code is no longer needed
            ClearSetupCode();
            toast.success("Configuration applied successfully, redirecting...");
            if (configState.auth.enabled) {
              setTimeout(() => (window.location.href = "/login"), 3000);
              return;
            }
            setTimeout(() => (window.location.href = "/"), 50);
          } else {
            toast.error("Failed to finalize onboarding.");
          }
        } else {
          toast.error("Failed to apply configuration.");
        }
      } else if (resp.status === "warn") {
        toast.warning(typeof resp.data === "string" ? resp.data : "Warning occurred while applying configuration.");
      } else {
        toast.error(resp.error?.message || "Failed to apply configuration.");
      }
    } catch {
      toast.error("An error occurred while applying configuration.");
    }

    setApplyLoading(false);
  };

  // Ask for the setup code first, if needed
  if (setupCodeRequired) {
    return <SetupCodeCard onVerified={() => void fetchStatus()} />;
  }

  const renderSectionContent = () => {
    switch (activeSection.id) {
      case "welcome":
        return (
          <div className="space-y-6">
            <div className="flex items-center gap-4">
              <H2 className="text-4xl font-bold">Welcome to</H2>
              <Image src="/aura_word_logo.svg" alt="Aura Logo" width={120} height={120} />
            </div>
            <P className="text-muted-foreground max-w-xl">
              {/* Only when the required settings are entered but have errors */}
              {status?.config_has_errors && (
                <span className="text-destructive">Your configuration has some errors. </span>
              )}
              Start with the required sections (MediUX and your Media Server). The optional sections can be set up now
              or later in Settings. A check mark shows each section that is done.
            </P>
            <Button onClick={next} className="cursor-pointer">
              Get Started
              <ArrowRight className="h-4 w-4" />
            </Button>
          </div>
        );
      case "mediux":
        return (
          <ConfigSectionMediux
            value={configState.mediux}
            editing
            hideEmptyFieldErrors={!left.has("mediux")}
            showActions
            onConnectionVerifiedChange={connectionReporters.mediux}
            configAlreadyLoaded={status?.config_loaded || false}
            dirtyFields={{}}
            onChange={(f, v) => updateSectionField("mediux", f, v)}
            errorsUpdate={(errs) => updateSectionErrors("mediux", errs as Record<string, string>)}
          />
        );
      case "media-server":
        return (
          <ConfigSectionMediaServer
            value={configState.media_server}
            editing
            hideEmptyFieldErrors={!left.has("media-server")}
            autoLoadLibraries
            showActions
            onConnectionVerifiedChange={connectionReporters["media-server"]}
            configAlreadyLoaded={status?.config_loaded || false}
            dirtyFields={{}}
            onChange={(f, v) => updateSectionField("media_server", f, v)}
            errorsUpdate={(errs) => updateSectionErrors("media_server", errs as Record<string, string>)}
          />
        );
      case "other-media-servers":
        return (
          <ConfigSectionOtherMediaServersSetup
            servers={otherMediaServers}
            mainLibraries={configState.media_server.libraries ?? []}
            onChange={updateOtherMediaServers}
            onConnectionVerifiedChange={connectionReporters["other-media-servers"]}
            errorsUpdate={updateOtherMediaServerErrors}
          />
        );
      case "authentication":
        return (
          <ConfigSectionAuth
            value={configState.auth}
            editing
            dirtyFields={{}}
            onChange={(f, v) => updateSectionField("auth", f, v)}
            errorsUpdate={(errs) => updateSectionErrors("auth", errs as Record<string, string>)}
          />
        );
      case "images":
        return (
          <ConfigSectionImages
            value={configState.images}
            editing
            dirtyFields={{}}
            onChange={updateImagesField}
            errorsUpdate={(errs) => updateSectionErrors("images", errs as Record<string, string>)}
            mediaServerType={configState.media_server.type}
            imageNameOptions={imageNameOptions}
          />
        );
      case "sonarr-radarr":
        return (
          <ConfigSectionSonarrRadarr
            value={configState.sonarr_radarr}
            editing
            dirtyFields={{}}
            onChange={(f, v) => updateSectionField("sonarr_radarr", f, v)}
            errorsUpdate={(errs) => updateSectionErrors("sonarr_radarr", errs as Record<string, string>)}
            configAlreadyLoaded={status?.config_loaded || false}
            onConnectionVerifiedChange={connectionReporters["sonarr-radarr"]}
            libraries={configState.media_server.libraries || []}
          />
        );
      case "labels-and-tags":
        return (
          <ConfigSectionLabelsAndTags
            value={configState.labels_and_tags}
            editing
            dirtyFields={{}}
            onChange={(f, v) => updateSectionField("labels_and_tags", f, v)}
            errorsUpdate={(errs) => updateSectionErrors("labels_and_tags", errs as Record<string, string>)}
            mediaServerType={configState.media_server.type}
            srOptions={Array.from(
              new Set(
                (Array.isArray(configState.sonarr_radarr?.applications) ? configState.sonarr_radarr.applications : [])
                  .map((app) => app.type)
                  .filter((type) => !!type)
              )
            )}
          />
        );
      case "notifications":
        return (
          <ConfigSectionNotifications
            value={configState.notifications}
            editing
            dirtyFields={{}}
            onChange={(f, v) => updateSectionField("notifications", f, v)}
            errorsUpdate={(errs) => updateSectionErrors("notifications", errs as Record<string, string>)}
            // Checking a provider sends a test notification, so only check when Test is clicked
            configAlreadyLoaded={false}
            templateVariablesCatalog={notificationTemplateVariables}
            showSiloPlugin={hasSiloMediaServer(configState)}
          />
        );
      case "logging":
        return (
          <ConfigSectionLogging
            value={configState.logging}
            editing
            dirtyFields={{}}
            onChange={(f, v) => updateSectionField("logging", f, v)}
            errorsUpdate={(errs) => updateSectionErrors("logging", errs as Record<string, string>)}
          />
        );
      case "errors":
        return <ErrorList errors={shownErrors} onGoToSection={goToSection} />;
      case "review":
        return (
          <div className="space-y-5">
            <Card className="p-5">
              <h3 className="mb-3 font-semibold">Checklist</h3>
              <ul className="space-y-2">
                {visibleSections
                  .filter((section) => section.group === "Required" || section.group === "Optional")
                  .map((section) => {
                    const state = sectionState(section);
                    return (
                      <li key={section.id} className="flex items-center gap-2 text-sm">
                        <SectionStateIcon state={state} />
                        <button
                          type="button"
                          onClick={() => goToSection(section.id)}
                          className="cursor-pointer hover:underline"
                        >
                          {section.label}
                        </button>
                        <span className="text-xs text-muted-foreground">
                          {section.group}
                          {state === "incomplete" && section.group === "Required" && " · not done yet"}
                          {state === "incomplete" && section.group === "Optional" && " · not visited (defaults)"}
                          {state === "error" && " · has errors"}
                        </span>
                      </li>
                    );
                  })}
              </ul>
            </Card>
            {totalShownErrors > 0 && <ErrorList errors={shownErrors} onGoToSection={goToSection} />}
            <Card className="p-5">
              <h3 className="mb-1 font-semibold">Configuration</h3>
              <p className="mb-3 text-sm text-muted-foreground">Passwords and tokens are hidden.</p>
              <pre className="max-h-96 overflow-auto rounded bg-muted p-4 text-xs">{reviewYaml}</pre>
            </Card>
          </div>
        );
      default:
        return null;
    }
  };

  const isLast = flowIndex === flowSections.length - 1;
  const ActiveIcon = activeSection.icon;

  const navigationButtons = (
    <div className="flex flex-row flex-wrap items-center justify-end gap-2">
      {flowIndex !== 0 && (
        <Button variant="outline" onClick={prev} disabled={applyLoading} className="cursor-pointer">
          <ArrowLeft className="h-4 w-4" />
          Back
        </Button>
      )}
      {!isLast && activeSection.id !== "errors" && (
        <Button onClick={next} disabled={applyLoading || blockedByErrors} className="cursor-pointer">
          Next
          <ArrowRight className="h-4 w-4" />
        </Button>
      )}
      {isLast && (
        <Button onClick={finish} disabled={applyLoading} className="cursor-pointer">
          {applyLoading && <Loader2 className="h-4 w-4 animate-spin" />}
          {applyLoading ? "Applying..." : "Apply & Save"}
        </Button>
      )}
    </div>
  );

  return (
    <div className="mx-auto max-w-6xl p-4 sm:p-6 lg:p-8">
      <div className="mb-6">
        <h1 className="text-3xl font-bold">Setup</h1>
        <p className="text-muted-foreground">
          {requiredComplete
            ? totalShownErrors > 0
              ? `Fix ${totalShownErrors} ${makePlural(totalShownErrors, "error")} to finish.`
              : "The required sections are done. Review and finish when you're ready."
            : "Complete the required sections to finish setting up aura."}
        </p>
      </div>

      <div className="md:grid md:grid-cols-[13rem_minmax(0,1fr)] md:gap-8">
        {/* Desktop: Section list */}
        <nav className="hidden md:block" aria-label="Setup sections">
          <div className="sticky top-20 space-y-5">
            {GROUPS.map((group) => {
              const groupSections = visibleSections.filter((section) => section.group === group);
              if (groupSections.length === 0) return null;
              return (
                <div key={group}>
                  <p className="mb-1 px-3 text-xs font-semibold uppercase tracking-wide text-muted-foreground">
                    {group}
                  </p>
                  <div className="space-y-0.5">
                    {groupSections.map((section) => {
                      const active = section.id === activeSection.id;
                      const Icon = section.icon;
                      const showState = section.group === "Required" || section.group === "Optional";
                      const canOpen = canOpenSection(section.id);
                      return (
                        <button
                          key={section.id}
                          type="button"
                          onClick={() => goToSection(section.id)}
                          disabled={!canOpen}
                          title={canOpen ? undefined : "Fix the errors in this section first"}
                          aria-current={active ? "step" : undefined}
                          className={cn(
                            "flex w-full cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-left text-sm transition-colors",
                            active
                              ? "bg-primary/10 font-medium text-primary"
                              : "text-muted-foreground hover:bg-muted hover:text-foreground",
                            section.id === "errors" && !active && "text-red-500",
                            !canOpen && "cursor-not-allowed opacity-50 hover:bg-transparent hover:text-muted-foreground"
                          )}
                        >
                          <Icon className="h-4 w-4 shrink-0" />
                          <span className="flex-1 truncate">
                            {section.label}
                            {section.id === "errors" && ` (${totalShownErrors})`}
                          </span>
                          {showState && <SectionStateIcon state={sectionState(section)} />}
                        </button>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
        </nav>

        <div className="min-w-0">
          {/* Mobile: Step header and buttons */}
          {activeSection.id !== "welcome" && (
            <div className="mb-4 flex flex-col gap-3 md:hidden">
              <div className="flex items-center gap-2">
                <P className="m-0 text-md text-muted-foreground">
                  {stepNumber > 0
                    ? `Step ${stepNumber} of ${numberedSections.length}: ${activeSection.label}`
                    : activeSection.label}
                </P>
                {activeSection.group === "Optional" && <P className="m-0 text-md text-amber-600">(Optional)</P>}
                {activeSection.group === "Required" && <P className="m-0 text-md text-primary">(Required)</P>}
              </div>
              {navigationButtons}
            </div>
          )}

          {/* Desktop: Section header */}
          {activeSection.id !== "welcome" && (
            <div className="mb-5 hidden items-start justify-between gap-4 md:flex">
              <div>
                <h2 className="flex items-center gap-2 text-xl font-semibold">
                  <ActiveIcon className="h-5 w-5 text-primary" />
                  {activeSection.label}
                  {(activeSection.group === "Required" || activeSection.group === "Optional") && (
                    <span
                      className={cn(
                        "rounded-full px-2 py-0.5 text-xs font-medium",
                        activeSection.group === "Required"
                          ? "bg-primary/10 text-primary"
                          : "bg-amber-500/10 text-amber-600"
                      )}
                    >
                      {activeSection.group}
                    </span>
                  )}
                </h2>
                <p className="text-sm text-muted-foreground">{activeSection.description}</p>
              </div>
            </div>
          )}

          <div key={activeSection.id} className="animate-in fade-in-0 duration-200">
            {renderSectionContent()}
          </div>

          {/* Desktop: Back / Next */}
          {activeSection.id !== "welcome" && <div className="mt-6 hidden md:block">{navigationButtons}</div>}
        </div>
      </div>
    </div>
  );
};

function SectionStateIcon({ state }: { state: SectionState }) {
  if (state === "complete") return <CheckCircle2 className="h-4 w-4 shrink-0 text-green-500" aria-label="Done" />;
  if (state === "error") return <AlertCircle className="h-4 w-4 shrink-0 text-red-500" aria-label="Has errors" />;
  return <Circle className="h-4 w-4 shrink-0 text-muted-foreground/50" aria-label="Not done yet" />;
}

// ErrorList lists every error, grouped by section
function ErrorList({
  errors,
  onGoToSection,
}: {
  errors: Record<string, Record<string, string>>;
  onGoToSection: (sectionId: string) => void;
}) {
  const sections = Object.entries(errors);
  const total = sections.reduce((sum, [, errs]) => sum + Object.keys(errs).length, 0);
  if (total === 0) {
    return (
      <Card className="flex flex-row items-center gap-2 p-5 text-sm text-green-500">
        <CheckCircle2 className="h-4 w-4" />
        No errors
      </Card>
    );
  }

  return (
    <div className="rounded-md border border-destructive/30 bg-destructive/5 p-3">
      <p className="m-0 mb-3 text-sm font-medium text-destructive">
        {total} {makePlural(total, "error")} to fix
      </p>
      <div className="grid gap-2 sm:grid-cols-2">
        {sections.map(([configSection, errs]) => {
          const sectionId = SECTIONS.find((section) => section.configSection === configSection)?.id;
          return (
            <div key={configSection} className="rounded border border-destructive/30 bg-destructive/10 p-2">
              <div className="flex items-center justify-between gap-2">
                <p className="m-0 text-sm font-semibold text-destructive">{sectionLabelForConfig(configSection)}</p>
                {sectionId && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="h-6 cursor-pointer px-2 text-destructive"
                    onClick={() => onGoToSection(sectionId)}
                  >
                    Go to section
                    <ArrowRight className="h-3 w-3" />
                  </Button>
                )}
              </div>
              <ul className="mt-1 space-y-0.5">
                {Object.entries(errs).map(([field, message]) => (
                  <li key={field} className="text-sm text-destructive">
                    {message}
                  </li>
                ))}
              </ul>
            </div>
          );
        })}
      </div>
    </div>
  );
}

// SetupCodeCard asks for the setup code printed in the aura logs
function SetupCodeCard({ onVerified }: { onVerified: () => void }) {
  const [code, setCode] = useState("");
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!code.trim()) return;
    setChecking(true);
    setError(null);
    const response = await VerifySetupCode(code);
    setChecking(false);
    if (response.status === "error") {
      setError(response.error?.message || "Incorrect setup code");
      return;
    }
    onVerified();
  };

  return (
    <div className="mx-auto flex max-w-md flex-col p-6 pt-16">
      <Card className="p-6">
        <div className="mb-2 flex items-center gap-2">
          <KeyRound className="h-5 w-5 text-primary" />
          <h1 className="text-xl font-semibold">Enter your setup code</h1>
        </div>
        <p className="mb-4 text-sm text-muted-foreground">
          To make sure only you can set up aura, enter the setup code from the aura logs (e.g.{" "}
          <span className="font-mono">docker logs aura</span>). Look for the line{" "}
          <span className="italic">Enter this setup code on the setup page</span>.
        </p>
        <form onSubmit={submit} className="space-y-3">
          <div className="space-y-1">
            <Label htmlFor="setup-code">Setup Code</Label>
            <Input
              id="setup-code"
              value={code}
              onChange={(e) => setCode(e.target.value)}
              placeholder="XXXX-XXXX"
              autoComplete="off"
              autoFocus
              className="font-mono uppercase tracking-widest"
            />
            {error && <p className="text-sm text-red-500">{error}</p>}
          </div>
          <Button type="submit" disabled={checking || !code.trim()} className="w-full cursor-pointer">
            {checking && <Loader2 className="h-4 w-4 animate-spin" />}
            {checking ? "Checking..." : "Continue"}
          </Button>
        </form>
      </Card>
    </div>
  );
}

export default OnboardingPage;
