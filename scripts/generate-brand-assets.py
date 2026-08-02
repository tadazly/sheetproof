from __future__ import annotations

from pathlib import Path
from PIL import Image, ImageDraw


ROOT = Path(__file__).resolve().parents[1]
BUILD = ROOT / "build"
SITE_BRAND = ROOT / "site" / "public" / "brand"
FRONTEND_PUBLIC = ROOT / "frontend" / "public"

NAVY_TOP = "#184A77"
NAVY_BOTTOM = "#0B1E33"
PAPER = "#F7FAFD"
PAPER_HEADER = "#DDEAF5"
GRID = "#DCE7F1"
BLUE = "#2F6FED"
BLUE_LIGHT = "#6EA3FF"
RED = "#DE5757"
RED_SOFT = "#F8D7D7"
GREEN = "#2DA86B"
GREEN_SOFT = "#D3F1DF"


SVG = '''<?xml version="1.0" encoding="UTF-8"?>
<svg width="1024" height="1024" viewBox="0 0 1024 1024" fill="none" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="bg" x1="170" y1="100" x2="850" y2="930" gradientUnits="userSpaceOnUse">
      <stop stop-color="#184A77"/><stop offset="1" stop-color="#0B1E33"/>
    </linearGradient>
    <linearGradient id="proof" x1="430" y1="420" x2="620" y2="620" gradientUnits="userSpaceOnUse">
      <stop stop-color="#6EA3FF"/><stop offset="1" stop-color="#2F6FED"/>
    </linearGradient>
    <filter id="shadow" x="130" y="150" width="764" height="730" filterUnits="userSpaceOnUse">
      <feDropShadow dx="0" dy="26" stdDeviation="30" flood-color="#020B16" flood-opacity=".32"/>
    </filter>
  </defs>
  <rect x="64" y="64" width="896" height="896" rx="224" fill="url(#bg)"/>
  <path d="M154 212C289 101 475 64 688 64H736C860 64 960 164 960 288V352C797 220 603 164 394 184C307 192 227 211 154 244V212Z" fill="#FFFFFF" fill-opacity=".055"/>
  <g filter="url(#shadow)">
    <rect x="174" y="202" width="314" height="620" rx="64" fill="#F7FAFD"/>
    <rect x="536" y="202" width="314" height="620" rx="64" fill="#F7FAFD"/>
    <path d="M174 266C174 231 203 202 238 202H424C459 202 488 231 488 266V320H174V266Z" fill="#DDEAF5"/>
    <path d="M536 266C536 231 565 202 600 202H786C821 202 850 231 850 266V320H536V266Z" fill="#DDEAF5"/>
    <rect x="218" y="370" width="226" height="74" rx="18" fill="#DCE7F1"/>
    <rect x="218" y="468" width="226" height="82" rx="18" fill="#F8D7D7"/>
    <rect x="218" y="574" width="226" height="74" rx="18" fill="#DCE7F1"/>
    <rect x="218" y="672" width="226" height="74" rx="18" fill="#DCE7F1"/>
    <rect x="580" y="370" width="226" height="74" rx="18" fill="#DCE7F1"/>
    <rect x="580" y="468" width="226" height="82" rx="18" fill="#D3F1DF"/>
    <rect x="580" y="574" width="226" height="74" rx="18" fill="#DCE7F1"/>
    <rect x="580" y="672" width="226" height="74" rx="18" fill="#DCE7F1"/>
    <rect x="218" y="468" width="18" height="82" rx="9" fill="#DE5757"/>
    <rect x="788" y="468" width="18" height="82" rx="9" fill="#2DA86B"/>
    <path d="M420 510L482 572L612 432" stroke="#0B1E33" stroke-opacity=".28" stroke-width="78" stroke-linecap="round" stroke-linejoin="round"/>
    <path d="M420 500L482 562L612 422" stroke="url(#proof)" stroke-width="62" stroke-linecap="round" stroke-linejoin="round"/>
    <path d="M420 500L482 562L612 422" stroke="#FFFFFF" stroke-opacity=".96" stroke-width="22" stroke-linecap="round" stroke-linejoin="round"/>
  </g>
</svg>
'''


def rounded(draw: ImageDraw.ImageDraw, xy: tuple[int, int, int, int], radius: int, fill: str) -> None:
    draw.rounded_rectangle(xy, radius=radius, fill=fill)


def render_icon(size: int) -> Image.Image:
    scale = size / 1024
    image = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(image)

    def s(value: int) -> int:
        return round(value * scale)

    # A subtle vertical blend keeps the mark dimensional without becoming glossy.
    mask = Image.new("L", (size, size), 0)
    mask_draw = ImageDraw.Draw(mask)
    mask_draw.rounded_rectangle((s(64), s(64), s(960), s(960)), radius=s(224), fill=255)
    gradient = Image.new("RGBA", (size, size), NAVY_BOTTOM)
    gradient_pixels = gradient.load()
    top = (24, 74, 119)
    bottom = (11, 30, 51)
    for y in range(size):
        ratio = y / max(1, size - 1)
        color = tuple(round(top[i] * (1 - ratio) + bottom[i] * ratio) for i in range(3)) + (255,)
        for x in range(size):
            gradient_pixels[x, y] = color
    image.paste(gradient, (0, 0), mask)
    draw = ImageDraw.Draw(image)

    # Sheets and large rows intentionally mirror the SVG geometry.
    rounded(draw, (s(174), s(202), s(488), s(822)), s(64), PAPER)
    rounded(draw, (s(536), s(202), s(850), s(822)), s(64), PAPER)
    draw.rectangle((s(174), s(266), s(488), s(320)), fill=PAPER_HEADER)
    draw.rectangle((s(536), s(266), s(850), s(320)), fill=PAPER_HEADER)
    for x1, x2 in ((218, 444), (580, 806)):
        rounded(draw, (s(x1), s(370), s(x2), s(444)), s(18), GRID)
        rounded(draw, (s(x1), s(574), s(x2), s(648)), s(18), GRID)
        rounded(draw, (s(x1), s(672), s(x2), s(746)), s(18), GRID)
    rounded(draw, (s(218), s(468), s(444), s(550)), s(18), RED_SOFT)
    rounded(draw, (s(580), s(468), s(806), s(550)), s(18), GREEN_SOFT)
    rounded(draw, (s(218), s(468), s(236), s(550)), s(9), RED)
    rounded(draw, (s(788), s(468), s(806), s(550)), s(9), GREEN)

    points = [(s(420), s(500)), (s(482), s(562)), (s(612), s(422))]
    draw.line(points, fill=BLUE, width=max(2, s(62)), joint="curve")
    for point in points:
        radius = s(31)
        draw.ellipse((point[0] - radius, point[1] - radius, point[0] + radius, point[1] + radius), fill=BLUE)
    draw.line(points, fill="#FFFFFF", width=max(1, s(20)), joint="curve")
    for point in (points[0], points[-1]):
        radius = s(10)
        draw.ellipse((point[0] - radius, point[1] - radius, point[0] + radius, point[1] + radius), fill="#FFFFFF")
    return image


def main() -> None:
    BUILD.mkdir(parents=True, exist_ok=True)
    (BUILD / "windows").mkdir(parents=True, exist_ok=True)
    SITE_BRAND.mkdir(parents=True, exist_ok=True)
    FRONTEND_PUBLIC.mkdir(parents=True, exist_ok=True)

    (BUILD / "appicon.svg").write_text(SVG, encoding="utf-8", newline="\n")
    (SITE_BRAND / "icon.svg").write_text(SVG, encoding="utf-8", newline="\n")
    (FRONTEND_PUBLIC / "appicon.svg").write_text(SVG, encoding="utf-8", newline="\n")

    icon = render_icon(1024)
    icon.save(BUILD / "appicon.png", optimize=True)
    icon.resize((512, 512), Image.Resampling.LANCZOS).save(SITE_BRAND / "icon-512.png", optimize=True)
    icon.resize((192, 192), Image.Resampling.LANCZOS).save(SITE_BRAND / "icon-192.png", optimize=True)
    icon.save(
        BUILD / "windows" / "icon.ico",
        format="ICO",
        sizes=[(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)],
    )
    icon.save(
        SITE_BRAND / "favicon.ico",
        format="ICO",
        sizes=[(16, 16), (32, 32), (48, 48)],
    )


if __name__ == "__main__":
    main()
