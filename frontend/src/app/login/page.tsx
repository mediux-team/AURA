"use client";

import { GetAuthMethods } from "@/services/auth/auth-methods";
import { AttemptLogin } from "@/services/auth/login";
import { Eye, EyeOff, Loader2, Lock, ShieldCheck } from "lucide-react";

import { Suspense, useEffect, useState } from "react";

import { useRouter, useSearchParams } from "next/navigation";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

import { useOnboardingStore } from "@/lib/stores/global-store-onboarding";

const OIDC_ERROR_MESSAGES: Record<string, string> = {
  oidc_unavailable: "Single sign-on is not currently available. Try your password instead.",
  oidc_failed: "Single sign-on login failed. Please try again.",
  oidc_not_allowed: "Your identity provider account is not permitted to access this app.",
};

function LoginForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [password, setPassword] = useState("");
  const [showPw, setShowPw] = useState(false);
  const [loading, setLoading] = useState(false);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const [passwordEnabled, setPasswordEnabled] = useState(true);
  const [oidcEnabled, setOidcEnabled] = useState(false);
  const [oidcLoginURL, setOidcLoginURL] = useState("/api/auth/oidc/login");

  // Get Onboarding Status
  const status = useOnboardingStore((state) => state.status);
  const hasHydrated = useOnboardingStore((state) => state.hasHydrated);

  // If onboarding is complete and auth is disabled or AURA_DISABLE_AUTH is set, there's nothing to log in to.
  useEffect(() => {
    if (hasHydrated && status && (status.current_setup.auth.enabled === false || status.auth_disabled_by_env)) {
      router.replace("/");
    }
  }, [hasHydrated, router, status]);

  // Which login methods are available (password / OIDC SSO) - public endpoint, safe pre-login.
  useEffect(() => {
    // Send the address aura is open at, so the identity provider returns to the same one
    setOidcLoginURL(`/api/auth/oidc/login?origin=${encodeURIComponent(window.location.origin)}`);
    void GetAuthMethods().then((resp) => {
      if (resp.status !== "error" && resp.data) {
        setPasswordEnabled(resp.data.password_enabled);
        setOidcEnabled(resp.data.oidc_enabled);
      }
    });
  }, []);

  // Surface any error redirected back from the OIDC callback.
  useEffect(() => {
    const oidcError = searchParams.get("error");
    if (oidcError) {
      let message = OIDC_ERROR_MESSAGES[oidcError] || "Login failed. Please try again.";
      if (oidcError === "oidc_unavailable" && !passwordEnabled) message = "Single sign-on is not currently available.";
      setErrorMsg(message);
    }
  }, [searchParams, passwordEnabled]);

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setErrorMsg(null);
    if (!password) {
      setErrorMsg("Password required.");
      return;
    }
    try {
      setLoading(true);
      const resp = await AttemptLogin(password);
      if (resp.status === "error" || !resp.data?.authenticated) {
        throw new Error(resp.error?.message || "Invalid Password");
      }
      router.replace("/");
    } catch (err: unknown) {
      setErrorMsg((err as { message?: string })?.message || "Login failed. Check password.");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center px-8 pb-20 sm:px-20">
      <Card className="w-full max-w-md shadow-md">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 text-2xl">
            <Lock className="h-6 w-6" /> Sign In
          </CardTitle>
          <CardDescription>Sign in to access aura.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {errorMsg && (
            <Alert variant="destructive">
              <AlertTitle>Error</AlertTitle>
              <AlertDescription>{errorMsg}</AlertDescription>
            </Alert>
          )}

          {oidcEnabled && (
            <a href={oidcLoginURL} className="block">
              <Button type="button" variant="outline" className="w-full">
                <ShieldCheck className="mr-2 h-4 w-4" />
                Sign in with SSO
              </Button>
            </a>
          )}

          {oidcEnabled && passwordEnabled && (
            <div className="flex items-center gap-3 text-xs text-muted-foreground">
              <div className="h-px flex-1 bg-border" />
              or
              <div className="h-px flex-1 bg-border" />
            </div>
          )}

          {passwordEnabled && (
            <form onSubmit={handleSubmit} className="space-y-4">
              <div>
                <Label className="mb-2 font-medium" htmlFor="password">
                  Password
                </Label>
                <div className="relative">
                  <Input
                    id="password"
                    type={showPw ? "text" : "password"}
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    autoComplete="current-password"
                    placeholder="••••••••"
                    disabled={loading}
                    className="pr-10" // add padding so text doesn't run under the icon
                  />
                  <Button
                    variant="ghost"
                    onClick={() => setShowPw(!showPw)}
                    aria-label={showPw ? "Hide password" : "Show password"}
                    className="absolute top-1/2 right-3 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                    disabled={loading}
                    type="button"
                  >
                    {showPw ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </Button>
                </div>
              </div>
              <CardFooter className="flex flex-col gap-3 px-0">
                <Button type="submit" className="w-full" disabled={loading}>
                  {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                  {loading ? "Signing In..." : "Sign In"}
                </Button>
              </CardFooter>
            </form>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense fallback={null}>
      <LoginForm />
    </Suspense>
  );
}
