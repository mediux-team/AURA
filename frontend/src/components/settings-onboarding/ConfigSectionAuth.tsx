import { GenerateAPIKey } from "@/services/auth/api-key";
import { AlertTriangle, Check, Copy, KeyRound } from "lucide-react";
import { toast } from "sonner";

import React, { useEffect, useMemo, useRef, useState } from "react";

import Link from "next/link";

import { PopoverHelp } from "@/components/shared/popover-help";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";

import { cn } from "@/lib/cn";

import type { AppConfigAuth } from "@/types/config/config";

interface ConfigSectionAuthProps {
  value: AppConfigAuth;
  editing: boolean;
  dirtyFields?: { enabled?: boolean; password?: boolean };
  onChange: <K extends keyof AppConfigAuth>(field: K, value: AppConfigAuth[K]) => void;
  errorsUpdate?: (errors: Partial<Record<keyof AppConfigAuth, string>>) => void;
  // Whether a global API key has already been generated. The key itself is never sent to the
  // frontend - only this boolean, computed server-side.
  apiKeyConfigured?: boolean;
}

const hashRegex = /^\$argon2id\$v=\d+\$m=\d+,t=\d+,p=\d+\$[A-Za-z0-9+/=]+\$[A-Za-z0-9+/=]+$/;

const parseCommaList = (v: string): string[] =>
  v
    .split(",")
    .map((s) => s.trim())
    .filter(Boolean);

// CommaListInput edits a list as comma separated text
function CommaListInput({
  id,
  value,
  disabled,
  placeholder,
  onChange,
}: {
  id: string;
  value: string[] | undefined;
  disabled?: boolean;
  placeholder?: string;
  onChange: (list: string[]) => void;
}) {
  const listText = (value || []).join(", ");
  const [text, setText] = useState(listText);
  // Show the saved list again when it changes elsewhere (e.g. after saving)
  useEffect(() => setText(listText), [listText]);
  return (
    <Input
      id={id}
      disabled={disabled}
      placeholder={placeholder}
      value={text}
      onChange={(e) => setText(e.target.value)}
      onBlur={() => {
        const list = parseCommaList(text);
        setText(list.join(", "));
        if (list.join(",") !== (value || []).join(",")) onChange(list);
      }}
    />
  );
}

const SESSION_COOKIE_SECURE_OPTIONS = [
  { value: "auto", label: "Auto", description: "Secure when aura is opened over HTTPS" },
  { value: "always", label: "Always", description: "Always secure (aura is only used over HTTPS)" },
  { value: "never", label: "Never", description: "Never secure (e.g. aura is only used over HTTP)" },
];

export const ConfigSectionAuth: React.FC<ConfigSectionAuthProps> = ({
  value,
  editing,
  dirtyFields = {},
  onChange,
  errorsUpdate,
  apiKeyConfigured = false,
}) => {
  const prevErrorRef = useRef<string>("");

  // API key generation is out-of-band from the editable config diff/save flow - it's its own
  // dedicated endpoint, not a field that gets typed and saved with everything else.
  const [keyConfigured, setKeyConfigured] = useState(apiKeyConfigured);
  const [generatedKey, setGeneratedKey] = useState<string | null>(null);
  const [generating, setGenerating] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    setKeyConfigured(apiKeyConfigured);
  }, [apiKeyConfigured]);

  const handleGenerateKey = async () => {
    setGenerating(true);
    setCopied(false);
    try {
      const resp = await GenerateAPIKey();
      if (resp.status === "error" || !resp.data?.api_key) {
        toast.error(resp.error?.message || "Failed to generate API key");
        return;
      }
      setGeneratedKey(resp.data.api_key);
      setKeyConfigured(true);
      toast.success(keyConfigured ? "API key regenerated - the previous key is now invalid" : "API key generated");
    } finally {
      setGenerating(false);
    }
  };

  const handleCopyKey = async () => {
    if (!generatedKey) return;
    await navigator.clipboard.writeText(generatedKey);
    setCopied(true);
  };

  // Validation
  const errors = useMemo<Partial<Record<keyof AppConfigAuth, string>>>(() => {
    const errs: Partial<Record<keyof AppConfigAuth, string>> = {};
    // Password Errors
    if (value.enabled) {
      const password = value.password.trim();
      if (password.length === 0) {
        errs.password = "Password hash is required when authentication is enabled.";
      } else if (!password.startsWith("***") && !hashRegex.test(password)) {
        // A masked value is the saved hash, which aura keeps
        errs.password = "Invalid Argon2id hash format.";
      }
    }
    return errs;
  }, [value.enabled, value.password]);

  // Emit errors upward
  useEffect(() => {
    if (!errorsUpdate) return;
    const serialized = JSON.stringify(errors);
    if (serialized === prevErrorRef.current) return;
    prevErrorRef.current = serialized;
    errorsUpdate(errors);
  }, [errors, errorsUpdate]);

  const oidc = value.oidc;
  const oidcAllowlistEmpty = (oidc?.allowed_emails?.length ?? 0) === 0 && (oidc?.allowed_domains?.length ?? 0) === 0;

  const updateOidc = <K extends keyof NonNullable<AppConfigAuth["oidc"]>>(
    field: K,
    fieldValue: NonNullable<AppConfigAuth["oidc"]>[K]
  ) => {
    onChange("oidc", { ...oidc, [field]: fieldValue });
  };

  return (
    <Card className={`gap-4 p-4 sm:p-5 ${Object.values(errors).some(Boolean) ? "border-red-500" : "border-muted"}`}>
      <div className="flex items-center justify-between">
        <h2 className="text-xl font-semibold text-blue-500">Authentication</h2>
      </div>

      <div
        className={cn(
          "flex items-center justify-between border rounded-md p-3 transition",
          "border-muted",
          dirtyFields.enabled && "border-amber-500"
        )}
      >
        <Label className="mr-2">Enabled</Label>
        <div className="flex items-center gap-2">
          <Switch disabled={!editing} checked={value.enabled} onCheckedChange={(c) => onChange("enabled", c)} />
          {editing && (
            <PopoverHelp ariaLabel="help-auth-enabled">
              <p>Turn on to enforce authentication. A valid Argon2id password hash must be provided below.</p>
            </PopoverHelp>
          )}
        </div>
      </div>

      <div className="flex">
        <div className={cn("relative flex-1 border rounded-md p-3 space-y-2 transition")}>
          <div>
            <div className="flex items-center justify-between">
              <Label htmlFor="auth-hash">Argon2id Password Hash</Label>
              {editing && (
                <PopoverHelp ariaLabel="help-auth-password-hash">
                  <p className="mb-2">
                    Provide an Argon2id hash. If authentication is enabled this hash must match the user's password.
                  </p>
                  <p>
                    You can use a site like{" "}
                    <Link
                      className="text-primary underline"
                      href="https://argon2.online/"
                      target="_blank"
                      rel="noopener noreferrer"
                    >
                      Argon2.Online
                    </Link>{" "}
                    to generate a hash.
                  </p>
                </PopoverHelp>
              )}
            </div>
            <Input
              id="auth-hash"
              disabled={!editing}
              placeholder="$argon2id$v=19$m=65536,t=3,p=1$..."
              type="text"
              value={value.password}
              onChange={(e) => onChange("password", e.target.value)}
              className={cn("w-full mt-1", dirtyFields.password && "ring-2 ring-amber-500")}
            />
          </div>
          {errors.password && <p className="text-xs text-red-500">{errors.password}</p>}
        </div>
      </div>

      {/* API Key */}
      <div className="border rounded-md p-3 space-y-3 border-muted">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <KeyRound className="h-4 w-4" />
            <Label>API Key</Label>
            <PopoverHelp ariaLabel="help-auth-api-key">
              <p>
                Used for programmatic access to the API. This key is only shown once when generated, copy it
                immediately. If you lose it, you can regenerate a new key which will invalidate the old one.
              </p>
              <p className="mt-2">
                aura comes with built-in{" "}
                <a
                  href="/swagger/index.html"
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-primary hover:underline"
                >
                  API docs
                </a>
                , which you can use to test the API with your key.
              </p>
            </PopoverHelp>
          </div>
          <Button type="button" size="sm" variant="outline" disabled={generating} onClick={handleGenerateKey}>
            {keyConfigured ? "Regenerate" : "Generate"} Key
          </Button>
        </div>

        {!keyConfigured && !generatedKey && (
          <p className="text-xs text-muted-foreground">No API key has been generated yet.</p>
        )}

        {generatedKey && (
          <Alert>
            <AlertTitle>Copy this key now - it won&apos;t be shown again</AlertTitle>
            <AlertDescription>
              <div className="flex items-center gap-2 mt-2">
                <code className="flex-1 break-all rounded bg-muted px-2 py-1 text-xs">{generatedKey}</code>
                <Button type="button" size="sm" variant="outline" onClick={handleCopyKey}>
                  {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
                </Button>
              </div>
            </AlertDescription>
          </Alert>
        )}
      </div>

      {/* OIDC / SSO */}
      <div className="border rounded-md p-3 space-y-3 border-muted">
        <div className="flex items-center justify-between">
          <Label className="mr-2">Single Sign-On (OIDC)</Label>
          <div className="flex items-center gap-2">
            <Switch
              disabled={!editing}
              checked={oidc?.enabled ?? false}
              onCheckedChange={(c) => updateOidc("enabled", c)}
            />
            {editing && (
              <PopoverHelp ariaLabel="help-auth-oidc-enabled">
                <p>Allow signing in via an external OIDC identity provider (e.g. Authelia, Pocket ID, Authentik).</p>
              </PopoverHelp>
            )}
          </div>
        </div>

        {oidc?.enabled && (
          <div className="space-y-3">
            <div>
              <Label htmlFor="oidc-issuer" className="mb-1 block">
                Issuer URL
              </Label>
              <Input
                id="oidc-issuer"
                disabled={!editing}
                placeholder="https://idp.example.com"
                value={oidc.issuer_url || ""}
                onChange={(e) => updateOidc("issuer_url", e.target.value)}
              />
            </div>
            <div>
              <Label htmlFor="oidc-client-id" className="mb-1 block">
                Client ID
              </Label>
              <Input
                id="oidc-client-id"
                disabled={!editing}
                value={oidc.client_id || ""}
                onChange={(e) => updateOidc("client_id", e.target.value)}
              />
            </div>
            <div>
              <Label htmlFor="oidc-client-secret" className="mb-1 block">
                Client Secret
              </Label>
              <Input
                id="oidc-client-secret"
                disabled={!editing}
                type="password"
                value={oidc.client_secret || ""}
                onChange={(e) => updateOidc("client_secret", e.target.value)}
              />
            </div>
            <div>
              <Label htmlFor="oidc-redirect" className="mb-1 block">
                Redirect URL
              </Label>
              <Input
                id="oidc-redirect"
                disabled={!editing}
                placeholder="https://aura.example.com/api/auth/oidc/callback"
                value={oidc.redirect_url || ""}
                onChange={(e) => updateOidc("redirect_url", e.target.value)}
              />
              <p className="text-xs text-muted-foreground mt-1">
                Register this exact URL as an allowed redirect URI with your identity provider.
              </p>
            </div>
            <div>
              <Label htmlFor="oidc-allowed-emails" className="mb-1 block">
                Allowed Emails
              </Label>
              <CommaListInput
                id="oidc-allowed-emails"
                disabled={!editing}
                placeholder="you@example.com, other@example.com"
                value={oidc.allowed_emails}
                onChange={(list) => updateOidc("allowed_emails", list)}
              />
            </div>
            <div>
              <Label htmlFor="oidc-allowed-domains" className="mb-1 block">
                Allowed Domains
              </Label>
              <CommaListInput
                id="oidc-allowed-domains"
                disabled={!editing}
                placeholder="example.com"
                value={oidc.allowed_domains}
                onChange={(list) => updateOidc("allowed_domains", list)}
              />
            </div>

            {oidcAllowlistEmpty && (
              <Alert variant="destructive">
                <AlertTriangle className="h-4 w-4" />
                <AlertTitle>No email/domain restriction configured</AlertTitle>
                <AlertDescription>
                  Any user who can successfully authenticate with this identity provider will be granted full access to
                  this app.
                </AlertDescription>
              </Alert>
            )}
          </div>
        )}
      </div>

      {/* Advanced */}
      <Accordion type="single" collapsible className="rounded-md border px-3">
        <AccordionItem value="advanced" className="border-b-0">
          <AccordionTrigger className="cursor-pointer py-3 text-sm font-medium hover:no-underline">
            Advanced
          </AccordionTrigger>
          <AccordionContent className="space-y-4 pb-3">
            <div>
              <div className="mb-1 flex items-center justify-between">
                <Label htmlFor="session-cookie-secure">Session Cookie Secure</Label>
                {editing && (
                  <PopoverHelp ariaLabel="help-session-cookie-secure">
                    <p className="mb-2">
                      Whether the login session cookie is marked Secure, so the browser only sends it over HTTPS.
                    </p>
                    <p className="text-muted-foreground">
                      Keep Auto unless logging in doesn't stick (e.g. behind a reverse proxy that handles HTTPS).
                    </p>
                  </PopoverHelp>
                )}
              </div>
              <Select
                disabled={!editing}
                value={value.session_cookie_secure || "auto"}
                onValueChange={(v) => onChange("session_cookie_secure", v)}
              >
                <SelectTrigger id="session-cookie-secure" className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {SESSION_COOKIE_SECURE_OPTIONS.map((option) => (
                    <SelectItem key={option.value} value={option.value} className="cursor-pointer">
                      {option.label} <span className="text-muted-foreground">- {option.description}</span>
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {/* Only used with Auto */}
            {(value.session_cookie_secure || "auto") === "auto" && (
              <div className="flex items-center justify-between gap-4">
                <div>
                  <Label htmlFor="trust-proxy-cookie-secure">Trust Reverse Proxy for HTTPS</Label>
                  <p className="text-xs text-muted-foreground">
                    Treat requests as HTTPS when the reverse proxy says so (X-Forwarded-Proto). Only turn this on if
                    aura is behind a reverse proxy that sets this header.
                  </p>
                </div>
                <Switch
                  id="trust-proxy-cookie-secure"
                  disabled={!editing}
                  checked={!!value.trust_proxy_for_cookie_secure}
                  onCheckedChange={(v) => onChange("trust_proxy_for_cookie_secure", v)}
                />
              </div>
            )}

            <div>
              <div className="mb-1 flex items-center justify-between">
                <Label htmlFor="allowed-origins">Allowed Origins</Label>
                {editing && (
                  <PopoverHelp ariaLabel="help-allowed-origins">
                    <p className="mb-2">
                      Other websites allowed to call the aura API with your login (e.g. a dashboard on another domain).
                    </p>
                    <p className="text-muted-foreground">Not needed when using aura normally.</p>
                  </PopoverHelp>
                )}
              </div>
              <CommaListInput
                id="allowed-origins"
                disabled={!editing}
                placeholder="https://dashboard.example.com"
                value={value.allowed_origins}
                onChange={(list) => onChange("allowed_origins", list)}
              />
            </div>
          </AccordionContent>
        </AccordionItem>
      </Accordion>
    </Card>
  );
};
