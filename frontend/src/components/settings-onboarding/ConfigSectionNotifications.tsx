"use client";

import { ValidateURL } from "@/helper/validation/validate-url";
import { SendTestNotification } from "@/services/validation/notification";
import { Plus, RotateCcw, TestTube, Trash2 } from "lucide-react";

import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { GetConnectionColor } from "@/components/settings-onboarding/ConfigSectionMediaServer";
import type { ConfigConnectionStatus } from "@/components/settings-onboarding/ConfigSectionSonarrRadarr";
import { CONNECTION_STATUS_COLORS_BG } from "@/components/settings-onboarding/ConfigSectionSonarrRadarr";
import { PopoverHelp } from "@/components/shared/popover-help";
import { VariablePicker, insertAtCursor } from "@/components/shared/variable-picker";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";

import { cn } from "@/lib/cn";
import { log } from "@/lib/logger";

import type {
  AppConfig,
  AppConfigNotificationDiscord,
  AppConfigNotificationGotify,
  AppConfigNotificationProviders,
  AppConfigNotificationPushover,
  AppConfigNotificationTemplate,
  AppConfigNotificationWebhook,
  AppConfigNotifications,
  NotificationTemplateVariablesCatalog,
} from "@/types/config/config";

interface ConfigSectionNotificationsProps {
  value: AppConfigNotifications;
  editing: boolean;
  dirtyFields?: {
    enabled?: boolean;
    providers?: Array<
      Partial<
        Record<
          string,
          | boolean
          | {
              enabled?: boolean;
              webhook?: boolean;
              user_key?: boolean;
              api_token?: boolean;
              url?: boolean;
              headers?: Record<string, boolean>;
            }
        >
      >
    >;
    templates?: Partial<Record<keyof AppConfigNotificationTemplate, boolean>>;
  };
  onChange: <K extends keyof AppConfigNotifications>(field: K, value: AppConfigNotifications[K]) => void;
  errorsUpdate?: (errors: Record<string, string>) => void;
  configAlreadyLoaded: boolean;
  templateVariablesCatalog?: NotificationTemplateVariablesCatalog | null;
  showSiloPlugin?: boolean;
}

const PROVIDER_TYPES = ["Discord", "Pushover", "Gotify", "Webhook"] as const;

const TEMPLATE_TITLES: Partial<Record<keyof AppConfigNotificationTemplate, string>> = {
  app_startup: "App Startup",
  test_notification: "Test Notification",
  autodownload: "Auto Download",
  download_queue: "Download Queue",
  new_item_auto_download: "Auto Download New Item",
  new_sets_available_for_ignored_items: "New Sets Available for Ignored Item",
  check_for_media_item_changes_job: "Check For Media Item Changes Job",
  sonarr_notification: "Sonarr Notification",
  silo_plugin: "Silo Plugin",
};

const TEMPLATE_SUPPORTS_IMAGE: Partial<Record<keyof AppConfigNotificationTemplate, boolean>> = {
  app_startup: false,
  test_notification: false,
  autodownload: true,
  download_queue: true,
  new_item_auto_download: true,
  new_sets_available_for_ignored_items: true,
  check_for_media_item_changes_job: false,
  sonarr_notification: true,
  silo_plugin: true,
};

// hasSiloMediaServer is whether a Silo server is in use, the Silo Plugin template is only for Silo
export const hasSiloMediaServer = (config: AppConfig) =>
  config.media_server?.type === "Silo" ||
  (config.media_servers ?? []).some((server) => server.type === "Silo" && server.enabled !== false);

const TEMPLATE_VAR_REGEX = /\{\{\s*([a-zA-Z0-9_.-]+)\s*\}\}/g;

export const ConfigSectionNotifications: React.FC<ConfigSectionNotificationsProps> = ({
  value,
  editing,
  dirtyFields = {},
  onChange,
  errorsUpdate,
  configAlreadyLoaded,
  templateVariablesCatalog,
  showSiloPlugin = false,
}) => {
  const prevErrorsRef = useRef<string>("");
  const hasRunInitialValidation = useRef(false);

  // The title and message fields of each template, to add variables at the cursor
  const templateFieldRefs = useRef<
    Partial<
      Record<
        keyof AppConfigNotificationTemplate,
        { title?: HTMLInputElement | null; message?: HTMLTextAreaElement | null }
      >
    >
  >({});

  // Local select state for adding providers
  const [newProviderType, setNewProviderType] = useState<string>("Discord");

  // State to track editing header keys
  const [editingHeaderKeys, setEditingHeaderKeys] = useState<Record<number, Record<string, string>>>({});

  const providers = useMemo(() => (Array.isArray(value.providers) ? value.providers : []), [value.providers]);

  const templates = useMemo(() => value.templates || {}, [value.templates]);

  // State to track app connection testing
  const [appConnectionStatus, setAppConnectionStatus] = useState<Record<number, ConfigConnectionStatus>>({});
  const [remoteTokenErrors, setRemoteTokenErrors] = useState<Record<number, string | null>>({});

  const templateKeys = useMemo(
    () =>
      (Object.keys(TEMPLATE_TITLES) as Array<keyof AppConfigNotificationTemplate>).filter(
        (key) => key !== "silo_plugin" || showSiloPlugin
      ),
    [showSiloPlugin]
  );
  const templateVariables = useMemo(() => templateVariablesCatalog, [templateVariablesCatalog]);

  const getUsedVars = (text: string) => {
    const set = new Set<string>();
    for (const m of text.matchAll(TEMPLATE_VAR_REGEX)) {
      if (m[0]) set.add(m[0]);
    }
    return [...set];
  };

  // ----- Validation -----
  const errors = useMemo(() => {
    const errs: Record<string, string> = {};

    if (value.enabled) {
      // At least one enabled provider recommended
      if (!providers.some((p) => p.enabled)) {
        errs["Providers"] = "Enable at least one provider or disable notifications.";
      }
    }

    providers.forEach((p, idx) => {
      if (value.enabled && p.enabled) {
        if (p.provider === "Discord") {
          const discord = p.discord;
          const webhook = (discord?.webhook || "").trim();
          if (!webhook) {
            errs[`Providers.[${idx}].Discord.Webhook`] = "Webhook URL required.";
          } else if (!/^https?:\/\//i.test(webhook)) {
            errs[`Providers.[${idx}].Discord.Webhook`] = "Webhook must start with http(s)://";
          }
          if (remoteTokenErrors[idx]) {
            errs[`Providers.[${idx}].Discord.Webhook`] = remoteTokenErrors[idx] || "Connection failed.";
          }
        } else if (p.provider === "Pushover") {
          const push = p.pushover;
          if (!(push?.user_key || "").trim()) {
            errs[`Providers.[${idx}].Pushover.UserKey`] = "User key required.";
          }
          if (!(push?.api_token || "").trim()) {
            errs[`Providers.[${idx}].Pushover.ApiToken`] = "App token required.";
          }
          if (remoteTokenErrors[idx]) {
            errs[`Providers.[${idx}].Pushover.ApiToken`] = remoteTokenErrors[idx] || "Connection failed.";
          }
        } else if (p.provider === "Gotify") {
          const gotify = p.gotify;
          const rawURL = (gotify?.url || "").trim();
          if (!rawURL) {
            errs[`Providers.[${idx}].Gotify.URL`] = "URL required.";
          } else {
            const urlErr = ValidateURL(rawURL);
            if (urlErr) errs[`Providers.[${idx}].Gotify.URL`] = urlErr;
          }
          if (!(gotify?.api_token || "").trim()) {
            errs[`Providers.[${idx}].Gotify.ApiToken`] = "App token required.";
          }
          if (remoteTokenErrors[idx]) {
            errs[`Providers.[${idx}].Gotify.ApiToken`] = remoteTokenErrors[idx] || "Connection failed.";
          }
        } else if (p.provider === "Webhook") {
          const webhook = p.webhook;
          const rawURL = (webhook?.url || "").trim();
          if (!rawURL) {
            errs[`Providers.[${idx}].Webhook.URL`] = "URL required.";
          } else {
            const urlErr = ValidateURL(rawURL);
            if (urlErr) errs[`Providers.[${idx}].Webhook.URL`] = urlErr;
          }
        }
      }
    });

    return errs;
  }, [value.enabled, providers, remoteTokenErrors]);

  // Emit errors
  useEffect(() => {
    if (!errorsUpdate) return;
    const serialized = JSON.stringify(errors);
    if (serialized === prevErrorsRef.current) return;
    prevErrorsRef.current = serialized;
    errorsUpdate(errors);
  }, [errors, errorsUpdate]);

  // ----- Mutators -----
  const setProviders = (next: AppConfigNotificationProviders[]) => onChange("providers", next);

  const addProvider = () => {
    if (!editing) return;
    const type = newProviderType;
    let newEntry: AppConfigNotificationProviders;
    if (type === "Discord") {
      newEntry = {
        provider: "Discord",
        enabled: true,
        discord: { webhook: "" },
      };
    } else if (type === "Pushover") {
      newEntry = {
        provider: "Pushover",
        enabled: true,
        pushover: { user_key: "", api_token: "" },
      };
    } else if (type === "Gotify") {
      newEntry = {
        provider: "Gotify",
        enabled: true,
        gotify: { url: "", api_token: "" },
      };
    } else if (type === "Webhook") {
      newEntry = {
        provider: "Webhook",
        enabled: true,
        webhook: { url: "", headers: {} },
      };
    } else {
      return;
    }
    setProviders([...providers, newEntry]);
  };

  const removeProvider = (idx: number) => {
    if (!editing) return;
    const next = providers.slice();
    next.splice(idx, 1);
    setProviders(next);
  };

  const updateProvider = <K extends keyof AppConfigNotificationProviders>(
    idx: number,
    field: K,
    val: AppConfigNotificationProviders[K]
  ) => {
    const next = providers.slice();
    next[idx] = { ...next[idx], [field]: val };
    setProviders(next);
  };

  const updateDiscord = <K extends keyof AppConfigNotificationDiscord>(
    idx: number,
    field: K,
    val: AppConfigNotificationDiscord[K]
  ) => {
    const prov = providers[idx];
    if (!prov.discord) return;
    const next = providers.slice();
    next[idx] = {
      ...prov,
      discord: { ...prov.discord, [field]: val },
    };
    setProviders(next);
  };

  const updatePushover = <K extends keyof AppConfigNotificationPushover>(
    idx: number,
    field: K,
    val: AppConfigNotificationPushover[K]
  ) => {
    const prov = providers[idx];
    if (!prov.pushover) return;
    const next = providers.slice();
    next[idx] = {
      ...prov,
      pushover: { ...prov.pushover, [field]: val },
    };
    setProviders(next);
  };

  const updateGotify = <K extends keyof AppConfigNotificationGotify>(
    idx: number,
    field: K,
    val: AppConfigNotificationGotify[K]
  ) => {
    const prov = providers[idx];
    if (!prov.gotify) return;
    const next = providers.slice();
    next[idx] = {
      ...prov,
      gotify: { ...prov.gotify, [field]: val },
    };
    setProviders(next);
  };

  const updateWebhook = <K extends keyof AppConfigNotificationWebhook>(
    idx: number,
    field: K,
    val: AppConfigNotificationWebhook[K]
  ) => {
    const prov = providers[idx];
    if (!prov.webhook) return;
    const next = providers.slice();
    next[idx] = {
      ...prov,
      webhook: { ...prov.webhook, [field]: val },
    };
    setProviders(next);
  };

  const updateTemplate = (
    key: keyof AppConfigNotificationTemplate,
    patch: Partial<AppConfigNotificationTemplate[keyof AppConfigNotificationTemplate]>
  ) => {
    const current = templates[key] || {
      enabled: false,
      title: "",
      message: "",
      include_image: false,
    };

    onChange("templates", {
      ...templates,
      [key]: {
        ...current,
        ...patch,
      },
    });
  };

  // insertTemplateVariable puts the variable at the cursor of the title or message, or at the end
  const insertTemplateVariable = (
    key: keyof AppConfigNotificationTemplate,
    variable: string,
    target: "title" | "message"
  ) => {
    const tpl = templates[key] || {
      enabled: false,
      title: "",
      message: "",
      include_image: false,
    };
    const nextText = insertAtCursor(templateFieldRefs.current[key]?.[target], tpl[target] || "", variable);
    updateTemplate(key, { [target]: nextText } as Partial<
      AppConfigNotificationTemplate[keyof AppConfigNotificationTemplate]
    >);
  };

  const runRemoteValidation = useCallback(
    async (idx: number, showToast = true) => {
      const provider = providers[idx];
      if (!provider || !provider.enabled) return;

      // Set to unknown while testing
      setAppConnectionStatus((s) => ({
        ...s,
        [idx]: { status: "unknown", color: GetConnectionColor("unknown") },
      }));

      try {
        const start = Date.now();
        const { ok, message } = await SendTestNotification(
          provider,
          "test_notification",
          value.templates?.test_notification || {
            enabled: false,
            title: "Test Notification",
            message: "This is a test notification from aura.",
            include_image: false,
          },
          showToast
        );
        const elapsed = Date.now() - start;
        const minDelay = 400;

        if (elapsed < minDelay) {
          await new Promise((resolve) => setTimeout(resolve, minDelay - elapsed));
        }

        if (ok) {
          setRemoteTokenErrors((s) => ({ ...s, [idx]: null }));
          setAppConnectionStatus((s) => ({ ...s, [idx]: { status: "ok", color: GetConnectionColor("ok") } }));
        } else {
          setRemoteTokenErrors((s) => ({ ...s, [idx]: message || "Connection failed" }));
          setAppConnectionStatus((s) => ({
            ...s,
            [idx]: { status: "error", color: GetConnectionColor("error") },
          }));
        }
      } catch {
        setRemoteTokenErrors((s) => ({ ...s, [idx]: "Connection failed" }));
        setAppConnectionStatus((s) => ({
          ...s,
          [idx]: { status: "error", color: GetConnectionColor("error") },
        }));
      }
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [providers]
  );

  useEffect(() => {
    if (configAlreadyLoaded && !hasRunInitialValidation.current) {
      // Run remote validation for all apps that have URL and API Token set
      providers.forEach((p, idx) => {
        if (p.enabled) {
          if (p.provider === "Discord" && p.discord?.webhook) {
            setTimeout(() => runRemoteValidation(idx, false), 200 * idx);
          } else if (p.provider === "Pushover" && p.pushover?.user_key && p.pushover?.api_token) {
            setTimeout(() => runRemoteValidation(idx, false), 200 * idx);
          } else if (p.provider === "Gotify" && p.gotify?.url && p.gotify?.api_token) {
            setTimeout(() => runRemoteValidation(idx, false), 200 * idx);
          } else if (p.provider === "Webhook" && p.webhook?.url) {
            setTimeout(() => runRemoteValidation(idx, false), 200 * idx);
          }
        }
      });
      hasRunInitialValidation.current = true;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [configAlreadyLoaded, runRemoteValidation]);

  return (
    <Card className={`gap-4 p-4 sm:p-5 ${Object.values(errors).some(Boolean) ? "border-red-500" : "border-muted"}`}>
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold text-blue-500">Notifications</h2>
      </div>

      {/* Global Enabled */}
      <div
        className={cn(
          "flex items-center justify-between border rounded-md p-3 transition",
          "border-muted",
          dirtyFields.enabled && "border-amber-500"
        )}
      >
        <Label>Enabled</Label>
        <div className="flex items-center gap-2">
          <Switch disabled={!editing} checked={value.enabled} onCheckedChange={(v) => onChange("enabled", v)} />
          {editing && (
            <PopoverHelp ariaLabel="help-notifications-enabled">
              <p className="mb-2">
                Turn on to send events through enabled providers (Discord, Pushover, Gotify, Custom Webhook).
              </p>
              <p className="text-muted-foreground">Each provider can also be enabled individually.</p>
            </PopoverHelp>
          )}
        </div>
      </div>

      {/* Providers */}
      <div className={cn("space-y-4")}>
        <div className="flex items-center justify-end">
          {editing && (
            <div className="flex items-center gap-2">
              <Select value={newProviderType} onValueChange={(v) => setNewProviderType(v)}>
                <SelectTrigger className="h-8 w-36">
                  <SelectValue placeholder="Type" />
                </SelectTrigger>
                <SelectContent>
                  {PROVIDER_TYPES.map((p) => (
                    <SelectItem key={p} value={p}>
                      {p}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button type="button" variant="outline" size="sm" onClick={addProvider}>
                <Plus className="h-4 w-4 mr-1" />
                Add
              </Button>
            </div>
          )}
        </div>

        {providers.length === 0 && <p className="text-sm text-muted-foreground">No notification providers added.</p>}

        {providers.map((p, idx) => {
          const providerDirty = dirtyFields.providers?.[idx];
          const discordDirty =
            typeof providerDirty?.discord === "object" && providerDirty?.discord !== null ? providerDirty.discord : {};
          const pushoverDirty =
            typeof providerDirty?.pushover === "object" && providerDirty?.pushover !== null
              ? providerDirty.pushover
              : {};
          const gotifyDirty =
            typeof providerDirty?.gotify === "object" && providerDirty?.gotify !== null ? providerDirty.gotify : {};
          const webhookDirty =
            typeof providerDirty?.webhook === "object" && providerDirty?.webhook !== null ? providerDirty.webhook : {};
          const providerErrorEntries = Object.entries(errors).filter(([k]) => k.startsWith(`Providers.[${idx}]`));
          const providerErrors = providerErrorEntries.map(([, msg]) => msg);
          const connStatus = appConnectionStatus[idx] || {
            status: "unknown",
            color: "gray-500",
          };
          return (
            <div
              key={idx}
              className={cn(
                "space-y-3 rounded-md border p-3 transition",
                providerErrors.length ? "border-red-500" : "border-muted"
              )}
            >
              <div className="flex items-center justify-between">
                <div
                  className={cn(
                    "flex items-center gap-3",
                    providerDirty?.Enabled && "rounded-md border border-amber-500"
                  )}
                >
                  <h2 className={`text-xl font-semibold text-${connStatus.color}`}>{p.provider}</h2>
                  <span
                    className={`h-2 w-2 rounded-full ${CONNECTION_STATUS_COLORS_BG[connStatus.status]} animate-pulse`}
                    title={`Connection status: ${connStatus.status}`}
                  />

                  <Switch
                    disabled={!editing}
                    checked={p.enabled}
                    onCheckedChange={(v) => updateProvider(idx, "enabled", v)}
                  />
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={
                      !value.enabled ||
                      !p.enabled ||
                      (p.provider === "Discord" && !p.discord?.webhook) ||
                      (p.provider === "Pushover" && (!p.pushover?.user_key || !p.pushover?.api_token)) ||
                      (p.provider === "Gotify" && (!p.gotify?.url || !p.gotify?.api_token)) ||
                      (p.provider === "Webhook" && !p.webhook?.url)
                    }
                    hidden={
                      ((p.provider === "Discord" && !p.discord?.webhook) ||
                        (p.provider === "Pushover" && (!p.pushover?.user_key || !p.pushover?.api_token)) ||
                        (p.provider === "Gotify" && (!p.gotify?.url || !p.gotify?.api_token)) ||
                        (p.provider === "Webhook" && !p.webhook?.url)) &&
                      !editing
                    }
                    onClick={() => {
                      runRemoteValidation(idx);
                    }}
                    aria-label="test-app-connection"
                  >
                    <TestTube className="h-4 w-4 mr-1" />{" "}
                  </Button>
                  {editing && (
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={() => removeProvider(idx)}
                      aria-label="help-apps-remove-app"
                      className="bg-red-700"
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  )}
                </div>
              </div>

              {/* Discord Fields */}
              {p.provider === "Discord" && p.enabled && (
                <div className={cn("space-y-1")}>
                  <div className="flex items-center justify-between">
                    <Label>Webhook URL</Label>
                    {editing && (
                      <PopoverHelp ariaLabel="help-notifications-discord-webhook">
                        <p className="mb-2 font-medium">Discord Webhook</p>
                        <ol className="list-decimal ml-4 space-y-1 ">
                          <li>Server Settings &gt; Integrations &gt; Webhooks</li>
                          <li>Create Webhook (choose channel)</li>
                          <li>Copy the webhook URL and paste here</li>
                        </ol>
                        <p className="mt-2 text-muted-foreground">
                          Must begin with https://discord.com/api/webhooks/...
                        </p>
                      </PopoverHelp>
                    )}
                  </div>
                  <Input
                    disabled={!editing}
                    placeholder="https://discord.com/api/webhooks/..."
                    value={p.discord?.webhook || ""}
                    onChange={(e) => {
                      const val = e.target.value;
                      updateDiscord(idx, "webhook", val);
                    }}
                    className={cn(discordDirty?.webhook && "border border-amber-500 p-3")}
                  />
                  {providerErrorEntries
                    .filter(([k]) => k.endsWith("Discord.Webhook"))
                    .map(([, msg], i) => (
                      <p key={i} className="text-xs text-red-500">
                        {msg}
                      </p>
                    ))}
                </div>
              )}

              {/* Pushover Fields */}
              {p.provider === "Pushover" && p.enabled && (
                <div className="grid gap-3 md:grid-cols-2">
                  <div className={cn("space-y-1")}>
                    <div className="flex items-center justify-between">
                      <Label>User Key</Label>
                      {editing && (
                        <PopoverHelp ariaLabel="help-notification-pushover-user-key">
                          <p className="mb-1 font-medium">Pushover User Key</p>
                          <p className=" mb-2">Found on your Pushover dashboard after logging in.</p>
                          <p className="text-muted-foreground">https://pushover.net/</p>
                        </PopoverHelp>
                      )}
                    </div>
                    <Input
                      disabled={!editing}
                      placeholder="User key"
                      value={p.pushover?.user_key || ""}
                      onChange={(e) => updatePushover(idx, "user_key", e.target.value)}
                      className={cn(pushoverDirty?.user_key && "border border-amber-500 p-3")}
                    />
                    {providerErrorEntries
                      .filter(([k]) => k.endsWith("Pushover.UserKey"))
                      .map(([, msg], i) => (
                        <p key={i} className="text-xs text-red-500">
                          {msg}
                        </p>
                      ))}
                  </div>
                  <div className={cn("space-y-1")}>
                    <div className="flex items-center justify-between">
                      <Label>App Token</Label>
                      {editing && (
                        <PopoverHelp ariaLabel="help-notifications-pushover-app-token">
                          <p className="mb-1 font-medium">Pushover App Token</p>
                          <p className=" mb-2">Create or view under "Your Applications" in Pushover.</p>
                          <p className="text-muted-foreground">Needed to send messages via the API.</p>
                        </PopoverHelp>
                      )}
                    </div>
                    <Input
                      disabled={!editing}
                      placeholder="App token"
                      value={p.pushover?.api_token || ""}
                      onChange={(e) => updatePushover(idx, "api_token", e.target.value)}
                      className={cn(pushoverDirty?.api_token && "border border-amber-500 p-3")}
                    />
                    {providerErrorEntries
                      .filter(([k]) => k.endsWith("Pushover.Token"))
                      .map(([, msg], i) => (
                        <p key={i} className="text-xs text-red-500">
                          {msg}
                        </p>
                      ))}
                  </div>
                </div>
              )}

              {/* Gotify Fields */}
              {p.provider === "Gotify" && p.enabled && (
                <div className="grid gap-3 md:grid-cols-2">
                  <div className={cn("space-y-1")}>
                    <div className="flex items-center justify-between">
                      <Label>URL</Label>
                      {editing && (
                        <PopoverHelp ariaLabel="help-notifications-gotify-url">
                          <p className="mb-1 font-medium">Gotify URL</p>
                          <p className=" mb-2">
                            The base URL of your Gotify server. Domains may omit port. IPv4 addresses must include a
                            port
                          </p>
                          <p>Examples:</p>
                          <ul className="list-disc ml-4 space-y-1 text-muted-foreground">
                            <li>https://gotify.domain.com</li>
                            <li>http://192.168.1.10:8070</li>
                            <li>http://gotify:8070</li>
                          </ul>
                        </PopoverHelp>
                      )}
                    </div>
                    <Input
                      disabled={!editing}
                      placeholder="URL"
                      value={p.gotify?.url || ""}
                      onChange={(e) => updateGotify(idx, "url", e.target.value)}
                      className={cn(gotifyDirty?.url && "border border-amber-500 p-3")}
                    />
                    {providerErrorEntries
                      .filter(([k]) => k.endsWith("Gotify.URL"))
                      .map(([, msg], i) => (
                        <p key={i} className="text-xs text-red-500">
                          {msg}
                        </p>
                      ))}
                  </div>
                  <div className={cn("space-y-1")}>
                    <div className="flex items-center justify-between">
                      <Label>App Token</Label>
                      {editing && (
                        <PopoverHelp ariaLabel="help-notifications-gotify-app-token">
                          <p className="mb-1 font-medium">
                            <span className="font-mono">Gotify App Token</span>
                          </p>
                          <p className=" mb-2">
                            Generate or view your app token under <span className="font-semibold">Apps</span> in Gotify.
                          </p>
                          <p className=" text-muted-foreground">This token is required to send messages via the API.</p>
                        </PopoverHelp>
                      )}
                    </div>
                    <Input
                      disabled={!editing}
                      placeholder="App token"
                      value={p.gotify?.api_token || ""}
                      onChange={(e) => updateGotify(idx, "api_token", e.target.value)}
                      className={cn(gotifyDirty?.api_token && "border border-amber-500 p-3")}
                    />
                    {providerErrorEntries
                      .filter(([k]) => k.endsWith("Gotify.Token"))
                      .map(([, msg], i) => (
                        <p key={i} className="text-xs text-red-500">
                          {msg}
                        </p>
                      ))}
                  </div>
                </div>
              )}

              {/* Webhook Fields */}
              {p.provider === "Webhook" && p.enabled && (
                <div className={cn("space-y-1")}>
                  <div className="flex items-center justify-between">
                    <Label>URL</Label>
                    {editing && (
                      <PopoverHelp ariaLabel="help-notifications-webhook-url">
                        <p className="mb-2 font-medium">Custom Webhook URL</p>
                        <p className="text-muted-foreground">The URL to send POST requests to for notifications.</p>
                      </PopoverHelp>
                    )}
                  </div>
                  <Input
                    disabled={!editing}
                    placeholder="https://example.com/webhook"
                    value={p.webhook?.url || ""}
                    onChange={(e) => {
                      const val = e.target.value;
                      updateWebhook(idx, "url", val);
                    }}
                    className={cn(webhookDirty?.url && "border border-amber-500 p-3")}
                  />
                  {providerErrorEntries
                    .filter(([k]) => k.endsWith("Webhook.url"))
                    .map(([, msg], i) => (
                      <p key={i} className="text-xs text-red-500">
                        {msg}
                      </p>
                    ))}

                  {/* Headers Input */}
                  <div className="flex items-center justify-between mt-2">
                    <Label>Custom Headers</Label>
                    {editing && (
                      <PopoverHelp ariaLabel="help-notifications-webhook-headers">
                        <p className="mb-2 font-medium">Custom Headers</p>
                        <p className="text-muted-foreground">
                          Add any custom headers to include in the webhook POST request. Enter as key/value pairs.
                        </p>
                      </PopoverHelp>
                    )}
                  </div>
                  <div className="space-y-2">
                    {Object.entries(p.webhook?.headers || {}).map(([key, value], i) => (
                      <div key={key + i} className="flex gap-2 items-center justify-between">
                        <Input
                          disabled={!editing}
                          placeholder="Header Name"
                          value={editingHeaderKeys[idx]?.[key] !== undefined ? editingHeaderKeys[idx][key] : key}
                          onChange={(e) => {
                            const val = e.target.value;
                            setEditingHeaderKeys((prev) => ({
                              ...prev,
                              [idx]: {
                                ...(prev[idx] || {}),
                                [key]: val,
                              },
                            }));
                          }}
                          onBlur={(e) => {
                            const rawKey = e.target.value.trim();
                            const newKey = rawKey.replace(/\s+/g, "_");
                            if (newKey && newKey !== key) {
                              const headers = { ...(p.webhook?.headers || {}) };
                              headers[newKey] = headers[key];
                              delete headers[key];
                              updateWebhook(idx, "headers", headers);
                              // Clean up local state for this key
                              setEditingHeaderKeys((prev) => {
                                const next = { ...(prev[idx] || {}) };
                                delete next[key];
                                return { ...prev, [idx]: next };
                              });
                            } else {
                              // Clean up local state if unchanged
                              setEditingHeaderKeys((prev) => {
                                const next = { ...(prev[idx] || {}) };
                                delete next[key];
                                return { ...prev, [idx]: next };
                              });
                            }
                          }}
                          className={cn("w-1/2", webhookDirty?.headers?.[key] && "border border-amber-500 p-3")}
                        />
                        <Input
                          disabled={!editing}
                          placeholder="Header Value"
                          value={value}
                          onChange={(e) => {
                            const headers = { ...(p.webhook?.headers || {}) };
                            headers[key] = e.target.value;
                            updateWebhook(idx, "headers", headers);
                          }}
                          className={cn("w-1/2", webhookDirty?.headers?.[key] && "border border-amber-500 p-3")}
                        />
                        {editing && (
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => {
                              const headers = { ...(p.webhook?.headers || {}) };
                              delete headers[key];
                              updateWebhook(idx, "headers", headers);
                            }}
                            aria-label="Remove header"
                            className="bg-red-700"
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        )}
                      </div>
                    ))}
                    {editing && (
                      <Button
                        type="button"
                        variant="outline"
                        size="sm"
                        onClick={() => {
                          const headers = { ...(p.webhook?.headers || {}) };
                          let i = 1;
                          const newKey = "Header";
                          while (headers[newKey + i]) i++;
                          headers[newKey + i] = "";
                          updateWebhook(idx, "headers", headers);
                        }}
                      >
                        <Plus className="h-4 w-4 mr-1" />
                        Add Header
                      </Button>
                    )}
                  </div>
                </div>
              )}
            </div>
          );
        })}

        {/* Notification Templates */}
        <div className="space-y-3">
          <div>
            <h3 className="text-lg font-semibold text-blue-500">Custom Notification Templates</h3>
            <p className="text-sm text-muted-foreground">
              Override title/message per event. Use variables like <span className="font-mono">{"{{AppName}}"}</span>.
            </p>
          </div>

          <Accordion type="multiple" className="w-full">
            {templateKeys.map((key) => {
              const tpl = templates[key] || {
                enabled: false,
                title: "",
                message: "",
                include_image: false,
              };

              const allowedVars = templateVariables?.template_variables?.[String(key)] || [];
              const usedVars = [...getUsedVars(tpl.title || ""), ...getUsedVars(tpl.message || "")];
              const unknownVars = templateVariables
                ? [...new Set(usedVars.filter((v) => !allowedVars.includes(v)))]
                : [];
              const supportsImage = TEMPLATE_SUPPORTS_IMAGE[key] ?? true;
              const defaultTpl = templateVariables?.default_templates?.[key];
              const isDefault =
                !defaultTpl ||
                ((tpl.title || "") === defaultTpl.title &&
                  (tpl.message || "") === defaultTpl.message &&
                  (!supportsImage || !!tpl.include_image === !!defaultTpl.include_image));

              return (
                <AccordionItem key={String(key)} value={String(key)} className="rounded-md border px-3 mb-1">
                  <AccordionTrigger>
                    <div className="flex items-center gap-2 text-left">
                      <span>{TEMPLATE_TITLES[key] || key}</span>
                      {tpl.enabled ? (
                        <span className="text-xs text-green-500">Enabled</span>
                      ) : (
                        <span className="text-xs text-muted-foreground">Disabled</span>
                      )}
                    </div>
                  </AccordionTrigger>

                  <AccordionContent className="space-y-3 pt-1">
                    <div className="flex items-center justify-between">
                      <Label>Enabled</Label>

                      <div className="flex items-center">
                        {/* Restore Default Button */}
                        {editing && !isDefault && defaultTpl && (
                          <Button
                            className="mr-2"
                            variant="outline"
                            size="sm"
                            onClick={() =>
                              updateTemplate(key, {
                                title: defaultTpl.title,
                                message: defaultTpl.message,
                                include_image: defaultTpl.include_image,
                              })
                            }
                          >
                            <RotateCcw className="h-4 w-4" />
                            Restore Default
                          </Button>
                        )}

                        {/* Test Button */}
                        {value.enabled &&
                          tpl.enabled &&
                          providers.filter((p) => p.enabled).length > 0 &&
                          unknownVars.length === 0 && (
                            <Button
                              className="mr-2"
                              variant="outline"
                              size="sm"
                              disabled={
                                !value.enabled ||
                                !tpl.enabled ||
                                providers.filter((p) => p.enabled).length === 0 ||
                                unknownVars.length > 0
                              }
                              onClick={() => {
                                log("INFO", "Settings", "Notifications", "Testing notification with template", {
                                  ...tpl,
                                  title: tpl.title || "Test Title",
                                  message: tpl.message || "Test Message",
                                });
                                for (const p of providers.filter((p) => p.enabled)) {
                                  SendTestNotification(p, key, tpl, true);
                                }
                              }}
                            >
                              <TestTube className="h-4 w-4" />
                            </Button>
                          )}
                        <Switch
                          disabled={!editing}
                          checked={!!tpl.enabled}
                          onCheckedChange={(v) => updateTemplate(key, { enabled: v })}
                        />
                      </div>
                    </div>

                    <div className="space-y-1">
                      <Label>Title</Label>
                      <div className="flex items-center gap-2">
                        <VariablePicker
                          variables={allowedVars}
                          disabled={!editing || !tpl.enabled}
                          label={`the ${TEMPLATE_TITLES[key] || key} title`}
                          onPick={(variable) => insertTemplateVariable(key, variable, "title")}
                        />
                        <Input
                          ref={(el) => {
                            templateFieldRefs.current[key] = { ...templateFieldRefs.current[key], title: el };
                          }}
                          disabled={!editing || !tpl.enabled}
                          value={tpl.title || ""}
                          onChange={(e) => updateTemplate(key, { title: e.target.value })}
                          placeholder="Notification title"
                          className="h-8 sm:h-9 text-xs sm:text-sm"
                        />
                      </div>
                    </div>

                    <div className="space-y-1">
                      <Label>Message</Label>
                      <div className="flex items-start gap-2">
                        <VariablePicker
                          variables={allowedVars}
                          disabled={!editing || !tpl.enabled}
                          label={`the ${TEMPLATE_TITLES[key] || key} message`}
                          onPick={(variable) => insertTemplateVariable(key, variable, "message")}
                        />
                        <Textarea
                          ref={(el) => {
                            templateFieldRefs.current[key] = { ...templateFieldRefs.current[key], message: el };
                          }}
                          disabled={!editing || !tpl.enabled}
                          value={tpl.message || ""}
                          onChange={(e) => updateTemplate(key, { message: e.target.value })}
                          placeholder="Notification message"
                          className="w-full min-h-[80px] rounded-md border px-3 py-2 text-xs sm:text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 resize-vertical"
                        />
                      </div>
                    </div>

                    {supportsImage && (
                      <div className="flex items-center justify-between">
                        <Label>Include Image</Label>
                        <Switch
                          disabled={!editing || !tpl.enabled}
                          checked={!!tpl.include_image}
                          onCheckedChange={(v) => updateTemplate(key, { include_image: v })}
                        />
                      </div>
                    )}

                    {unknownVars.length > 0 && (
                      <p className="text-xs text-red-500">Unknown variable(s): {unknownVars.join(", ")}</p>
                    )}
                  </AccordionContent>
                </AccordionItem>
              );
            })}
          </Accordion>
        </div>

        {errors["Providers"] && <p className="text-xs text-red-500">{errors["Providers"]}</p>}
      </div>
    </Card>
  );
};
