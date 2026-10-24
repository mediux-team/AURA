"use client";

import { FaDiscord, FaGithub } from "react-icons/fa";

import Image from "next/image";
import Link from "next/link";

import { ReleaseNotesDialog } from "@/components/layout/app-release-notes";

import { BUILD_COMMIT_SHORT, BUILD_INFO } from "@/lib/build-info";

import { useAppVersion } from "@/hooks/app-version";

type AppFooterProps = {
  version?: string;
};

export function AppFooter({ version = "dev" }: AppFooterProps) {
  // App Version Hooks
  const { latestVersion, showReleaseNotes, changelog, isNewerVersion, handleCloseReleaseNotes } =
    useAppVersion(version);

  return (
    <footer className="border-t bg-background/80 backdrop-blur-sm w-full py-3 px-4 md:px-12">
      <div className="flex flex-col space-y-3 md:flex-row md:justify-between md:items-center md:space-y-0">
        {/* Copyright - Line 1 on mobile */}
        <div className="text-sm text-muted-foreground text-center md:text-left">
          © {new Date().getFullYear()} MediUX. All rights reserved.
        </div>

        {/* Links row - Line 2 on mobile */}
        <div className="flex justify-center space-x-4 items-center">
          <Link
            href="https://mediux.io"
            target="_blank"
            rel="noopener noreferrer"
            className="group flex items-center hover:text-primary transition-colors active:scale-95 hover:brightness-120"
          >
            MediUX
            <div className="relative ml-1 w-[16px] h-[16px] rounded-t-md overflow-hidden">
              <Image src="/mediux_logo.svg" alt="Logo" fill className="object-contain" />
            </div>
          </Link>

          <Link
            href="https://discord.gg/YAKzwKPwyw"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center hover:text-primary transition-colors active:scale-95 hover:brightness-120"
          >
            Discord <FaDiscord className="ml-1 h-3 w-3" />
          </Link>

          <Link
            href="https://github.com/mediux-team/aura"
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center hover:text-primary transition-colors active:scale-95 hover:brightness-120"
          >
            GitHub <FaGithub className="ml-1 h-3 w-3" />
          </Link>
        </div>

        {/* Version - Line 3 on mobile */}
        <div className="flex flex-col items-center md:flex-row md:justify-end gap-2 hover:brightness-120 active:scale-95 transition">
          <Link
            href={`/change-log?currentVersion=${encodeURIComponent(version)}`}
            className="text-sm py-1 px-2 bg-muted rounded-md hover:text-primary transition-colors"
            title={
              BUILD_COMMIT_SHORT
                ? `View change log and build details (commit ${BUILD_COMMIT_SHORT})`
                : "View change log"
            }
          >
            App Version: {version}
            {BUILD_INFO.buildNumber && ` · Build ${BUILD_INFO.buildNumber}`}
          </Link>
          {latestVersion && isNewerVersion(latestVersion, version) && (
            <Link
              href={`/change-log?currentVersion=${encodeURIComponent(version)}&updates=true&latestVersion=${encodeURIComponent(latestVersion)}`}
              className="text-sm py-1 px-2 rounded-md bg-yellow-500/40 border border-yellow-500 hover:bg-yellow-500/90 hover:text-black transition-colors animate-pulse"
              title={`Change log for latest version ${latestVersion} available`}
            >
              Update Available: {latestVersion}
            </Link>
          )}
        </div>
      </div>
      <ReleaseNotesDialog open={showReleaseNotes} onOpenChange={handleCloseReleaseNotes} changelog={changelog} />
    </footer>
  );
}

export default AppFooter;
