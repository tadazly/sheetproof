import generated from "./content/generated/content.json";
import type { Locale } from "./i18n";

export const product = generated.facts.product;
export const downloads = {
  releases: generated.facts.downloads.releases,
  windows: generated.facts.downloads["windows-amd64"],
  macos: generated.facts.downloads["macos-universal"],
  checksums: generated.facts.downloads.checksums,
  source: generated.facts.downloads.source,
};

export function localeContent(locale: Locale) { return generated.locales[locale]; }
export function localizedScreenshots(locale: Locale) {
  const metadata = generated.locales[locale].screenshots;
  const facts = generated.facts.screenshots;
  return (Object.keys(facts) as Array<keyof typeof facts>).map((id) => ({ id, src: `/screenshots/${locale}/${facts[id].file}`, width: facts[id].width, height: facts[id].height, alt: metadata[id].alt, caption: metadata[id].caption }));
}
export const generatedReleases = generated.changelog;
