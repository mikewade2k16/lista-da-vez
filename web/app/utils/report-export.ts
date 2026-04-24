function escapeCsvCell(value) {
  const text = String(value ?? "");

  if (/[",;\n]/.test(text)) {
    return `"${text.replace(/"/g, '""')}"`;
  }

  return text;
}

function escapeHtml(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function downloadText(content, filename, mimeType) {
  if (typeof window === "undefined") {
    return;
  }

  const blob = new Blob([content], { type: mimeType });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");

  anchor.href = url;
  anchor.download = filename;
  anchor.click();

  URL.revokeObjectURL(url);
}

function buildCsvContent(reportData) {
  const headers = [
    "Loja",
    "ID atendimento",
    "Data/Hora",
    "Consultor",
    "Desfecho",
    "Valor",
    "Duracao (min)",
    "Espera fila (min)",
    "Modo",
    "Cliente",
    "Telefone",
    "Email",
    "Profissao",
    "Produto visto",
    "Produto fechado",
    "Preenchimento",
    "Motivos",
    "Origens",
    "Motivo fora da vez",
    "Observacoes",
    "Campanhas",
    "Bonus campanha"
  ];
  const lines = [headers.map(escapeCsvCell).join(";")];

  reportData.rows.forEach((row) => {
    lines.push(
      [
        row.storeName,
        row.serviceId,
        row.finishedAtLabel,
        row.consultantName,
        row.outcomeLabel,
        row.saleAmount.toFixed(2),
        Math.round(row.durationMs / 60000),
        Math.round(row.queueWaitMs / 60000),
        row.startModeLabel,
        row.customerName,
        row.customerPhone,
        row.customerEmail,
        row.customerProfession,
        row.productSeen,
        row.productClosed,
        row.completionLabel,
        row.visitReasonsLabel,
        row.customerSourcesLabel,
        row.queueJumpReason,
        row.notes,
        row.campaignNamesLabel,
        row.campaignBonusTotal.toFixed(2)
      ]
        .map(escapeCsvCell)
        .join(";")
    );
  });

  return lines.join("\n");
}

export function exportReportCsv(reportData) {
  const timestamp = new Date().toISOString().slice(0, 19).replace(/[:T]/g, "-");
  const filename = `relatorio-nexo-${timestamp}.csv`;
  const content = buildCsvContent(reportData);

  downloadText(content, filename, "text/csv;charset=utf-8;");
  return true;
}

function buildPdfHtml(reportData) {
  const rows = reportData.rows
    .map(
      (row) => `
        <tr>
          <td>${escapeHtml(row.storeName)}</td>
          <td>${escapeHtml(row.finishedAtLabel)}</td>
          <td>${escapeHtml(row.consultantName)}</td>
          <td>${escapeHtml(row.outcomeLabel)}</td>
          <td>${escapeHtml(row.saleAmountLabel)}</td>
          <td>${escapeHtml(row.durationLabel)}</td>
          <td>${escapeHtml(row.queueWaitLabel)}</td>
          <td>${escapeHtml(row.campaignNamesLabel)}</td>
        </tr>
      `
    )
    .join("");
  const generatedAt = new Intl.DateTimeFormat("pt-BR", { dateStyle: "short", timeStyle: "short" }).format(new Date());

  return `
    <!doctype html>
    <html lang="pt-BR">
      <head>
        <meta charset="UTF-8">
        <title>Relatorio Omni</title>
        <style>
          body { font-family: Arial, sans-serif; margin: 24px; color: #1f2937; }
          h1 { margin: 0 0 8px; font-size: 20px; }
          p { margin: 0 0 14px; font-size: 12px; color: #4b5563; }
          .metrics { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 14px; }
          .metric { border: 1px solid #d1d5db; border-radius: 999px; padding: 4px 10px; font-size: 12px; }
          table { width: 100%; border-collapse: collapse; font-size: 12px; }
          th, td { border: 1px solid #d1d5db; padding: 6px 8px; text-align: left; vertical-align: top; }
          th { background: #f3f4f6; }
          @media print { body { margin: 12px; } }
        </style>
      </head>
      <body>
        <h1>Relatorio operacional Omni</h1>
        <p>Gerado em ${escapeHtml(generatedAt)} | Registros: ${reportData.rows.length}</p>
        <div class="metrics">
          <span class="metric">Conversao: ${reportData.metrics.conversionRate.toFixed(1)}%</span>
          <span class="metric">Valor vendido: ${escapeHtml(reportData.metrics.soldValueLabel)}</span>
          <span class="metric">Ticket medio: ${escapeHtml(reportData.metrics.averageTicketLabel)}</span>
          <span class="metric">Media de atendimento: ${escapeHtml(reportData.metrics.averageDurationLabel)}</span>
          <span class="metric">Media de espera: ${escapeHtml(reportData.metrics.averageQueueWaitLabel)}</span>
          <span class="metric">Bonus campanhas: ${escapeHtml(reportData.metrics.campaignBonusTotalLabel)}</span>
        </div>
        <table>
          <thead>
            <tr>
              <th>Loja</th>
              <th>Data</th>
              <th>Consultor</th>
              <th>Desfecho</th>
              <th>Valor</th>
              <th>Duracao</th>
              <th>Espera</th>
              <th>Campanhas</th>
            </tr>
          </thead>
          <tbody>
            ${
              rows ||
              '<tr><td colspan="8" style="text-align:center;">Sem dados para os filtros selecionados.</td></tr>'
            }
          </tbody>
        </table>
      </body>
    </html>
  `;
}

export function exportReportPdf(reportData) {
  if (typeof window === "undefined") {
    return false;
  }

  const printWindow = window.open("", "_blank", "width=1200,height=900");

  if (!printWindow) {
    return false;
  }

  printWindow.document.open();
  printWindow.document.write(buildPdfHtml(reportData));
  printWindow.document.close();
  printWindow.focus();
  printWindow.print();
  return true;
}
