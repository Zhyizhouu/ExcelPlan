"""Renders the ExcelPlan app icon: forest-green rounded-square badge with a
spreadsheet grid + checkmark mark, matching frontend/src/index.css theme
tokens. Draws each target size at high supersample factor for clean
anti-aliased edges, using a simplified 2x2 grid at small sizes (3x3 becomes
illegible below ~48px) per the approved design proof.
"""
from PIL import Image, ImageDraw
import os, shutil

DESKTOP = r"D:\Abner\!Programs\excelplan\backend\cmd\desktop"
OUT = os.path.join(DESKTOP, "icon")  # tracked design source, not build output
os.makedirs(OUT, exist_ok=True)

FOREST = (45, 106, 79, 255)      # --color-accent (light)
OFFWHITE = (248, 246, 240, 255)  # --color-canvas (light) -- warm paper white
SAGE = (122, 184, 148, 255)      # --color-accent (dark)

SS = 8  # supersample factor


def rounded_square(size, radius_frac, fill):
    s = size * SS
    img = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)
    d.rounded_rectangle([0, 0, s - 1, s - 1], radius=int(s * radius_frac), fill=fill)
    return img, d, s


def checkmark_points(x0, y0, x1, y1, overflow=0.18):
    """A bold check-tick's three vertices, sized to overflow its cell
    slightly (bottom + upper-right), matching the approved mockup."""
    w, h = x1 - x0, y1 - y0
    a = (x0 - w * 0.05, y0 + h * 0.52)
    b = (x0 + w * 0.40, y1 + h * overflow)
    c = (x1 + w * overflow, y0 - h * overflow * 1.1)
    return a, b, c


def draw_check(d, x0, y0, x1, y1, color, weight_frac=0.30):
    a, b, c = checkmark_points(x0, y0, x1, y1)
    w = (x1 - x0) * weight_frac
    d.line([a, b], fill=color, width=int(w))
    d.line([b, c], fill=color, width=int(w))
    for p in (a, b, c):
        d.ellipse([p[0] - w / 2, p[1] - w / 2, p[0] + w / 2, p[1] + w / 2], fill=color)


def grid_icon(size, cells, mark_color=OFFWHITE, bg=FOREST, radius_frac=0.225):
    img, d, s = rounded_square(size, radius_frac, bg)

    grid_frac = 0.62
    gsize = s * grid_frac
    gx0 = (s - gsize) / 2
    gy0 = (s - gsize) / 2
    gx1, gy1 = gx0 + gsize, gy0 + gsize
    line_w = max(1, int(s * 0.018))

    cell = gsize / cells
    for i in range(cells + 1):
        x = gx0 + i * cell
        d.line([(x, gy0), (x, gy1)], fill=mark_color, width=line_w)
        y = gy0 + i * cell
        d.line([(gx0, y), (gx1, y)], fill=mark_color, width=line_w)

    # Checkmark over the bottom-right cell, drawn last so it sits on top of
    # the grid lines it overlaps.
    cx0, cy0 = gx0 + (cells - 1) * cell, gy0 + (cells - 1) * cell
    cx1, cy1 = gx0 + cells * cell, gy0 + cells * cell
    draw_check(d, cx0, cy0, cx1, cy1, mark_color, weight_frac=0.34)

    return img.resize((size, size), Image.LANCZOS)


def save(img, name):
    path = os.path.join(OUT, name)
    img.save(path)
    print("wrote", path)


# Large sizes keep the richer 3x3 grid; it reads fine once the badge is
# bigger than a favicon.
for size in (1024, 256, 128, 64, 48):
    save(grid_icon(size, cells=3), f"icon-{size}.png")

def check_only_icon(size, mark_color=OFFWHITE, bg=FOREST, radius_frac=0.225):
    """No grid at all -- just the checkmark, bigger and centered. At 16px
    even a 2x2 grid dissolves into noise; the checkmark alone is what
    actually survives favicon scale."""
    img, d, s = rounded_square(size, radius_frac, bg)
    pad = s * 0.24
    draw_check(d, pad, pad, s - pad, s - pad, mark_color, weight_frac=0.30)
    return img.resize((size, size), Image.LANCZOS)


# 32px keeps a simplified 2x2 grid; it's still legible there.
save(grid_icon(32, cells=2), "icon-32.png")

# 16px (and anything smaller a favicon might request) drops the grid
# entirely -- the 3x3 and 2x2 lines both mush together at this size.
for size in (16, 24):
    save(check_only_icon(size), f"icon-{size}.png")

# Dark-mode-flavoured alternate (sage mark), kept as a spare, not wired up.
save(grid_icon(512, cells=3, mark_color=SAGE), "icon-sage-512.png")

# Publish the two files Wails actually reads for a Windows build:
# build/appicon.png (generic source) and build/windows/icon.ico (multi-res).
build_dir = os.path.join(DESKTOP, "build")
os.makedirs(os.path.join(build_dir, "windows"), exist_ok=True)
shutil.copy(os.path.join(OUT, "icon-1024.png"), os.path.join(build_dir, "appicon.png"))
print("wrote", os.path.join(build_dir, "appicon.png"))

import subprocess
ico_sizes = ["icon-16.png", "icon-24.png", "icon-32.png", "icon-48.png",
             "icon-64.png", "icon-128.png", "icon-256.png"]
ico_out = os.path.join(build_dir, "windows", "icon.ico")
subprocess.run(["magick", *[os.path.join(OUT, n) for n in ico_sizes], ico_out], check=True)
print("wrote", ico_out)
