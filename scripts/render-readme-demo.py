#!/usr/bin/env python3
"""Render an annotated GIF from the fixture-only Playwright screenshot tour.

Requires Pillow for documentation generation only. It does not contact a server.
"""
import argparse
import json
from pathlib import Path
from PIL import Image, ImageDraw, ImageFont

parser = argparse.ArgumentParser()
parser.add_argument('--frames', type=Path, default=Path('web/test-results/readme-demo'))
parser.add_argument('--output', type=Path, default=Path('docs/images'))
args = parser.parse_args()
manifest = json.loads((args.frames / 'frames.json').read_text())
args.output.mkdir(parents=True, exist_ok=True)

def font(size):
    for name in ('DejaVuSans.ttf', '/System/Library/Fonts/Supplemental/Arial.ttf'):
        try:
            return ImageFont.truetype(name, size)
        except OSError:
            pass
    return ImageFont.load_default()

width, height = 1280, 768
pages = []
for index, item in enumerate(manifest):
    image = Image.open(args.frames / item['file']).convert('RGB')
    image = image.resize((width, height), Image.Resampling.LANCZOS)
    canvas = Image.new('RGB', (width, height + 82), '#0c1422')
    canvas.paste(image, (0, 0))
    draw = ImageDraw.Draw(canvas)
    draw.rectangle((0, height, width, height + 2), fill='#294151')
    draw.rounded_rectangle((20, height + 19, 64, height + 63), radius=10, fill='#15374a')
    draw.text((29, height + 30), str(index + 1).zfill(2), font=font(20), fill='#85d9d0')
    draw.text((80, height + 15), item['title'], font=font(24), fill='#edf4fc')
    draw.text((80, height + 48), 'Synthetic fixtures  /  Read-only configuration  /  No device telemetry', font=font(15), fill='#a4b9cc')
    pages.append(canvas)

# A shared palette keeps text and theme colors stable between frames.
sample = Image.new('RGB', (320, 213 * len(pages)))
for i, page in enumerate(pages):
    sample.paste(page.resize((320, 213)), (0, 213 * i))
palette = sample.quantize(colors=256, method=Image.Quantize.MEDIANCUT)
frames, durations = [], []
for i, page in enumerate(pages):
    frames.append(page.quantize(palette=palette, dither=Image.Dither.NONE))
    durations.append(manifest[i]['duration'])
    following = pages[(i + 1) % len(pages)]
    for alpha in (0.33, 0.66):
        frames.append(Image.blend(page, following, alpha).quantize(palette=palette, dither=Image.Dither.NONE))
        durations.append(90)

frames[0].save(args.output / 'fabric-explorer-demo.gif', save_all=True,
               append_images=frames[1:], duration=durations, loop=0,
               disposal=2, optimize=True)
pages[0].save(args.output / 'fabric-explorer-overview.png', optimize=True)
# A contact sheet is an ignored local QA artifact, not a distributed recording.
contact = Image.new('RGB', (640 * 2, 425 * ((len(pages) + 1) // 2)), '#0c1422')
for i, page in enumerate(pages):
    contact.paste(page.resize((640, 425)), (640 * (i % 2), 425 * (i // 2)))
contact.save(args.frames / 'contact-sheet.png')
print('Rendered', len(pages), 'actual UI states;', len(frames), 'GIF frames')
print('GIF bytes:', (args.output / 'fabric-explorer-demo.gif').stat().st_size)
