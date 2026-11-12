export function scriptPdfName(title: string) {
  return `${(title.trim() || 'Roteiro').replace(/[\\/:*?"<>|]+/g, '_')}.pdf`
}

export async function exportScriptPdf(host: HTMLElement, title: string) {
  const [{ default: html2canvas }, { jsPDF }] = await Promise.all([
    import('html2canvas'),
    import('jspdf'),
  ])
  await document.fonts.ready
  const hidden: { node: HTMLElement; display: string }[] = []
  host
    .querySelectorAll<HTMLElement>(
      '#global-toolbar, .document-actions, .row-remove, .row-drag, .add-row, .ql-toolbar, .script-client-corner',
    )
    .forEach((node) => {
      hidden.push({ node, display: node.style.display })
      node.style.display = 'none'
    })
  let canvas: HTMLCanvasElement
  try {
    host.classList.add('pdf-exporting')
    canvas = await html2canvas(host, {
      backgroundColor: '#ffffff',
      scale: Math.min(2, window.devicePixelRatio || 1.5),
      useCORS: true,
    })
  } finally {
    host.classList.remove('pdf-exporting')
    hidden.forEach(({ node, display }) => {
      node.style.display = display
    })
  }
  const pdf = new jsPDF({ orientation: 'p', unit: 'mm', format: 'a4' })
  const padding = 12
  const pageW = pdf.internal.pageSize.getWidth()
  const pageH = pdf.internal.pageSize.getHeight()
  const imageWidth = pageW - padding * 2
  const pxPerMm = canvas.width / imageWidth
  const sliceMax = Math.max(1, Math.round((pageH - padding * 2) * pxPerMm))
  for (let y = 0; y < canvas.height; y += sliceMax) {
    if (y) pdf.addPage()
    const slice = document.createElement('canvas')
    slice.width = canvas.width
    slice.height = Math.min(sliceMax, canvas.height - y)
    const context = slice.getContext('2d')
    if (!context) throw new Error('Não foi possível gerar o PDF.')
    context.drawImage(canvas, 0, y, canvas.width, slice.height, 0, 0, canvas.width, slice.height)
    pdf.addImage(
      slice.toDataURL('image/png'),
      'PNG',
      padding,
      padding,
      imageWidth,
      slice.height / pxPerMm,
      undefined,
      'FAST',
    )
    pdf.setFontSize(8)
    pdf.text(`${title} • ${new Date().toLocaleString()}`, padding, pageH - 4)
  }
  pdf.save(scriptPdfName(title))
}
