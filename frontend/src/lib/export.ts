/* eslint-disable no-unused-vars */

function escapeHtml(value: string): string {
  return value.replace(/[<>&"']/g, (char) => ({
    "<": "&lt;",
    ">": "&gt;",
    "&": "&amp;",
    '"': "&quot;",
    "'": "&#039;",
  })[char] || char);
}

function downloadBlob(blob: Blob, filename: string): void {
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  setTimeout(() => {
    anchor.remove();
    URL.revokeObjectURL(url);
  }, 200);
}

function resolveCssVar(value: string): string {
  const match = value.match(/^var\((--[^,]+)(?:,\s*(.*))?\)$/);
  if (!match) return value;
  const prop = match[1];
  const fallback = (match[2] || "").trim();
  try {
    const resolved = getComputedStyle(document.documentElement).getPropertyValue(prop).trim();
    return resolved || fallback;
  } catch {
    return fallback;
  }
}

// Find the chart SVG — picks the largest SVG in the element tree,
// skipping tiny icon SVGs from PinButton / DownloadDropdown (lucide-react 16x16).
// Requires the SVG to be at least 100x100 rendered pixels to avoid picking up
// toolbar / icon SVGs when the chart is img-based (e.g. PythonPlot).
function findChartSvg(element: HTMLElement): SVGSVGElement | null {
  const svgs = element.querySelectorAll<SVGSVGElement>("svg");
  let best: SVGSVGElement | null = null;
  let bestArea = 0;
  const MIN_AREA = 100 * 100; // ignore SVGs smaller than 100x100
  for (const s of svgs) {
    const rect = s.getBoundingClientRect();
    const area = rect.width * rect.height;
    if (area > bestArea && area >= MIN_AREA) {
      bestArea = area;
      best = s;
    }
  }
  return best;
}

// Read the intrinsic dimensions from an SVG element (attributes or viewBox).
function getSvgDimensions(svg: SVGSVGElement): { w: number; h: number } {
  const attrW = svg.getAttribute("width");
  const attrH = svg.getAttribute("height");
  if (attrW && attrH) {
    const w = parseFloat(attrW);
    const h = parseFloat(attrH);
    if (w > 0 && h > 0) return { w, h };
  }
  const vb = svg.getAttribute("viewBox");
  if (vb) {
    const parts = vb.split(/[\s,]+/).map(Number);
    if (parts.length === 4 && parts[2] > 0 && parts[3] > 0) {
      return { w: parts[2], h: parts[3] };
    }
  }
  // Fallback to rendered size
  const rect = svg.getBoundingClientRect();
  return {
    w: Math.max(1, Math.round(rect.width || 600)),
    h: Math.max(1, Math.round(rect.height || 300)),
  };
}

function serializeSvg(svg: SVGSVGElement): string | null {
  const { w, h } = getSvgDimensions(svg);
  const viewBox = svg.getAttribute("viewBox") || `0 0 ${w} ${h}`;

  const clone = svg.cloneNode(true) as SVGSVGElement;
  clone.setAttribute("xmlns", "http://www.w3.org/2000/svg");
  clone.setAttribute("width", String(w));
  clone.setAttribute("height", String(h));
  clone.setAttribute("viewBox", viewBox);

  // Resolve CSS var() in fill and stroke attributes on every element
  const all = clone.querySelectorAll("*");
  for (let i = 0; i < all.length; i++) {
    const el = all[i];
    for (const attr of ["fill", "stroke", "color", "stop-color"]) {
      const val = el.getAttribute(attr);
      if (val && val.includes("var(")) {
        el.setAttribute(attr, resolveCssVar(val));
      }
    }
    // Handle inline style attributes
    const styleVal = el.getAttribute("style");
    if (styleVal && styleVal.includes("var(")) {
      el.setAttribute("style", styleVal.replace(/var\(--[^,]+(?:,\s*[^)]+)?\)/g, (m) => resolveCssVar(m)));
    }
  }

  const serializer = new XMLSerializer();
  return '<?xml version="1.0" encoding="UTF-8"?>\n' + serializer.serializeToString(clone);
}

// Fetch an image as a blob to avoid CORS / tainted-canvas issues.
// Returns a blob URL that can be used safely in <img> and <canvas>.
async function fetchImageAsBlob(src: string): Promise<Blob> {
  const resp = await fetch(src);
  if (!resp.ok) throw new Error(`Failed to fetch image: ${resp.status}`);
  return resp.blob();
}

async function fetchImageAsBlobUrl(src: string): Promise<string> {
  const blob = await fetchImageAsBlob(src);
  return URL.createObjectURL(blob);
}

// --- Per-chart download helpers ---

export async function downloadChartPng(element: HTMLElement, filename: string): Promise<void> {
  return downloadChartAsRaster(element, filename, "png");
}

export async function downloadChartJpeg(element: HTMLElement, filename: string): Promise<void> {
  return downloadChartAsRaster(element, filename, "jpeg");
}

async function downloadChartAsRaster(element: HTMLElement, filename: string, ext: "png" | "jpeg"): Promise<void> {
  const svg = findChartSvg(element);
  const img = element.querySelector("img") as HTMLImageElement | null;

  if (!svg && !img) {
    throw new Error("No chart content found to download");
  }

  // For img-based charts (Python plots), skip the canvas pipeline when the
  // source is already PNG — just download the raw image blob directly.
  if (!svg && img && ext === "png") {
    const blob = await fetchImageAsBlob(img.src);
    downloadBlob(blob, `${filename}.png`);
    return;
  }

  let w: number;
  let h: number;

  if (svg) {
    const dims = getSvgDimensions(svg);
    w = dims.w;
    h = dims.h;
  } else {
    w = img!.naturalWidth || Math.round(img!.getBoundingClientRect().width) || 800;
    h = img!.naturalHeight || Math.round(img!.getBoundingClientRect().height) || 450;
  }

  const canvas = document.createElement("canvas");
  const scale = window.devicePixelRatio || 2;
  canvas.width = w * scale;
  canvas.height = h * scale;
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("Could not get canvas 2D context");

  ctx.scale(scale, scale);
  ctx.fillStyle = "#ffffff";
  ctx.fillRect(0, 0, w, h);

  let imageEl: HTMLImageElement;

  if (svg) {
    const svgStr = serializeSvg(svg);
    if (!svgStr) return;
    const blob = new Blob([svgStr], { type: "image/svg+xml;charset=utf-8" });
    const url = URL.createObjectURL(blob);

    try {
      imageEl = await new Promise<HTMLImageElement>((resolve, reject) => {
        const el = new Image();
        el.onload = () => resolve(el);
        el.onerror = () => reject(new Error("SVG failed to render as image"));
        el.src = url;
      });
    } finally {
      URL.revokeObjectURL(url);
    }
  } else {
    // Img-based chart (Python plot) — fetch as blob to avoid tainted canvas
    const blobUrl = await fetchImageAsBlobUrl(img!.src);
    try {
      imageEl = await new Promise<HTMLImageElement>((resolve, reject) => {
        const el = new Image();
        el.onload = () => resolve(el);
        el.onerror = () => reject(new Error("Image failed to load: " + img!.src));
        el.src = blobUrl;
      });
    } finally {
      URL.revokeObjectURL(blobUrl);
    }
  }

  ctx.drawImage(imageEl, 0, 0, w, h);

  const mimeType = ext === "png" ? "image/png" : "image/jpeg";
  const blob = await new Promise<Blob | null>((resolve) =>
    canvas.toBlob((b) => resolve(b), mimeType, ext === "jpeg" ? 0.92 : undefined)
  );

  if (blob) {
    downloadBlob(blob, `${filename}.${ext}`);
  } else {
    throw new Error("canvas.toBlob returned null — the canvas may be tainted");
  }
}

function getExportPlotNodes(dashboardRef: HTMLDivElement | null): HTMLElement[] {
  return Array.from(dashboardRef?.querySelectorAll<HTMLElement>("[data-export-plot]") ?? []);
}

export function exportPlotsAsPdf(dashboardRef: HTMLDivElement | null): Error | null {
  const plotNodes = getExportPlotNodes(dashboardRef);
  if (plotNodes.length === 0) {
    return new Error("No plots are available to export yet.");
  }

  const sections = plotNodes.map((node) => {
    const title = node.dataset?.exportPlot || "Plot";
    const svgEl = findChartSvg(node);
    const imgEl = node.querySelector("img");
    let visual = "";

    if (svgEl) {
      const clone = svgEl.cloneNode(true) as SVGSVGElement;
      clone.setAttribute("xmlns", "http://www.w3.org/2000/svg");
      const all = clone.querySelectorAll("*");
      for (let i = 0; i < all.length; i++) {
        const el = all[i];
        for (const attr of ["fill", "stroke", "color", "stop-color"]) {
          const val = el.getAttribute(attr);
          if (val && val.includes("var(")) el.setAttribute(attr, resolveCssVar(val));
        }
      }
      visual = new XMLSerializer().serializeToString(clone);
    } else if (imgEl) {
      visual = `<img src="${escapeHtml(imgEl.src)}" alt="${escapeHtml(title)}" />`;
    }

    return `
      <section>
        <h2>${escapeHtml(title)}</h2>
        <div class="plot">${visual}</div>
      </section>
    `;
  }).join("");

  const printWindow = window.open("", "_blank");
  if (!printWindow) {
    return new Error("Could not open the PDF export window. Allow popups and try again.");
  }

  printWindow.document.write(`
    <!doctype html>
    <html>
      <head>
        <title>InsightPilot Plots</title>
        <style>
          body { margin: 32px; color: #0f172a; font-family: Arial, sans-serif; }
          header { margin-bottom: 24px; }
          h1 { margin: 0 0 6px; font-size: 24px; }
          p { margin: 0; color: #64748b; }
          section { break-inside: avoid; page-break-inside: avoid; margin-bottom: 28px; }
          h2 { font-size: 16px; margin: 0 0 12px; }
          .plot { border: 1px solid #e2e8f0; border-radius: 12px; padding: 18px; }
          svg, img { display: block; max-width: 100%; height: auto; margin: 0 auto; }
          @page { margin: 18mm; }
        </style>
      </head>
      <body>
        <header>
          <h1>InsightPilot Plot Export</h1>
          <p>${new Date().toLocaleString()}</p>
        </header>
        ${sections}
        <script>
          window.onload = () => { window.focus(); window.print(); };
        </script>
      </body>
    </html>
  `);
  printWindow.document.close();
  return null;
}
