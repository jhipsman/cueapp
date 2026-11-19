import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "Base",
    short_name: "Base",
    description: "Your home base: an assistant, your memory, your day.",
    start_url: "/",
    display: "standalone",
    background_color: "#111110",
    theme_color: "#111110",
    icons: [
      { src: "/icon", sizes: "512x512", type: "image/png", purpose: "any" },
      { src: "/apple-icon", sizes: "180x180", type: "image/png" },
    ],
  };
}
