/* eslint-disable no-unused-vars */

/**
 * Export helpers for dashboard plots.
 * Extracts SVG/PDF from the rendered dashboard DOM.
 */

function escapeXml(value: string): string {
  return value.replace(/[<>&"']/g, (char) => ({
    "<": "&lt;",
    ">": "&gt;",
    "&": "&amp;",
    '"': "&quot;",
    "'": "&apos;",
  })[char] || char);
}

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
  anchor.remove();
  URL.revokeObjectURL(url);
}

function getExportPlotNodes(dashboardRef: HTMLDivElement | null): HTMLElement[] {
  return Array.from(dashboardRef?.querySelectorAll<HTMLElement>("[data-export-plot]") ?? []);
}

export function exportPlotsAsSvg(dashboardRef: HTMLDivElement | null, mounted: boolean): Error | null {
  const plotNodes = getExportPlotNodes(dashboardRef);
  const plotItems = plotNodes
    .map((node) => ({
      title: node.dataset?.exportPlot || "Plot",
      svg: node.querySelector("svg"),
      image: node.querySelector("img"),
    }))
    .filter((item) => Boolean(item.svg || item.image));

  if (plotItems.length === 0) {
    return new Error("No plots are available to export yet.");
  }

  if (!mounted) return null;

  const serializer = new XMLSerializer();
  const width = 900;
  const sectionHeight = 360;
  const body = plotItems.map((item, index) => {
    const y = index * sectionHeight + 48;
    let visual = "";

    if (item.svg) {
      const clone = item.svg.cloneNode(true) as SVGSVGElement;
      const rect = item.svg.getBoundingClientRect();
      const chartWidth = Math.max(1, Math.round(rect.width || 760));
      const chartHeight = Math.max(1, Math.round(rect.height || 260));
      clone.setAttribute("width", String(chartWidth));
      clone.setAttribute("height", String(chartHeight));
      clone.setAttribute("x", "0");
      clone.setAttribute("y", "0");
      visual = serializer.serializeToString(clone);
    } else if (item.image) {
      const rect = item.image.getBoundingClientRect();
      const imageWidth = Math.max(1, Math.round(rect.width || 760));
      const imageHeight = Math.max(1, Math.round(rect.height || 260));
      visual = `<image href="${escapeXml(item.image.src)}" width="${imageWidth}" height="${imageHeight}" preserveAspectRatio="xMidYMid meet" />`;
    }

    return `
      <text x="24" y="${index * sectionHeight + 28}" font-family="Arial, sans-serif" font-size="18" font-weight="700" fill="#0f172a">${escapeXml(item.title)}</text>
      <g transform="translate(24 ${y})">${visual}</g>
    `;
  }).join("");

  const combinedSvg = `
    <svg xmlns="http://www.w3.org/2000/svg" width="${width}" height="${plotItems.length * sectionHeight}" viewBox="0 0 ${width} ${plotItems.length * sectionHeight}">
      <rect width="100%" height="100%" fill="#ffffff"/>
      ${body}
    </svg>
  `.trim();

  downloadBlob(new Blob([combinedSvg], { type: "image/svg+xml;charset=utf-8" }), "insightpilot-plots.svg");
  return null;
}

export function exportPlotsAsPdf(dashboardRef: HTMLDivElement | null): Error | null {
  const plotNodes = getExportPlotNodes(dashboardRef);
  if (plotNodes.length === 0) {
    return new Error("No plots are available to export yet.");
  }

  const sectionHeight = 360;
  const sections = plotNodes.map((node) => {
    const title = node.dataset?.exportPlot || "Plot";
    const svg = node.querySelector("svg");
    const image = node.querySelector("img");
    const visual = svg?.outerHTML || (image ? `<img src="${image.src}" alt="${escapeHtml(title)}" />` : "");
    return `
      <section>
        <h2>${escapeHtml(title)}</h2>
        <div className="plot">${visual}</div>
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
          window.onload = () => {
            window.focus();
            window.print();
          };
        </script>
      </body>
    </html>
  `);
  printWindow.document.close();
  return null;
}
