import type { MetadataRoute } from "next";

export const dynamic = "force-static";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "LifeHub · Hal penting hari ini",
    short_name: "LifeHub",
    description: "Satu tempat yang tenang untuk tugas dan hal penting hari ini.",
    start_url: "/",
    display: "standalone",
    background_color: "#f5f6f2",
    theme_color: "#17483d",
    lang: "id",
    categories: ["productivity", "lifestyle"],
    icons: [
      {
        src: "/icon.svg",
        sizes: "any",
        type: "image/svg+xml",
        purpose: "any",
      },
      {
        src: "/icon.svg",
        sizes: "any",
        type: "image/svg+xml",
        purpose: "maskable",
      },
    ],
  };
}
