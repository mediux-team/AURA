// @ts-check
import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

// The site is served from https://mediux-team.github.io/AURA
export default defineConfig({
  site: "https://mediux-team.github.io",
  base: "/AURA",
  integrations: [
    starlight({
      title: "aura",
      description: "Browse, preview and apply MediUX image sets to your Plex, Emby, Jellyfin and Silo libraries.",
      logo: {
        light: "./src/assets/aura-logo-light.svg",
        dark: "./src/assets/aura-logo-dark.svg",
        replacesTitle: true,
      },
      favicon: "/favicon.svg",
      social: [
        { icon: "github", label: "GitHub", href: "https://github.com/mediux-team/aura" },
        { icon: "discord", label: "Discord", href: "https://discord.gg/YAKzwKPwyw" },
      ],
      editLink: { baseUrl: "https://github.com/mediux-team/aura/edit/master/docs/" },
      customCss: ["./src/styles/aura.css"],
      // Each group lists the pages in its folder, ordered by sidebar.order in each page
      sidebar: [
        { label: "Overview", link: "/" },
        { label: "Installation", items: [{ autogenerate: { directory: "installation" } }] },
        { label: "How to Use", items: [{ autogenerate: { directory: "how-to-use" } }] },
        { label: "Settings", items: [{ autogenerate: { directory: "settings" } }] },
        {
          label: "Reference",
          items: [
            { label: "Config File & Environment Variables", link: "/config/" },
            { label: "Sonarr Webhook Integration", link: "/sonarr-webhook-integration/" },
          ],
        },
      ],
    }),
  ],
});
