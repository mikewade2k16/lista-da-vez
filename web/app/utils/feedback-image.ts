const supportedFeedbackImageTypes = new Set(["image/jpeg", "image/png", "image/webp"]);
const feedbackImageTargetBytes = 450 * 1024;
const feedbackImageHardLimitBytes = 1024 * 1024;
const feedbackImageMaxDimension = 1600;
const feedbackImageQualitySteps = [0.82, 0.74, 0.66, 0.58];
const feedbackImageScaleSteps = [1, 0.85, 0.7];

interface EncodedImageCandidate {
  blob: Blob;
  fileName: string;
}

export async function compressFeedbackImage(file: File) {
  if (!supportedFeedbackImageTypes.has(String(file?.type || "").trim().toLowerCase())) {
    throw new Error("Envie uma imagem JPG, PNG ou WebP.");
  }

  if (!import.meta.client) {
    return file;
  }

  const sourceImage = await loadImageElement(file);
  const ratio = Math.min(1, feedbackImageMaxDimension / Math.max(sourceImage.naturalWidth, sourceImage.naturalHeight));
  const baseWidth = Math.max(1, Math.round(sourceImage.naturalWidth * ratio));
  const baseHeight = Math.max(1, Math.round(sourceImage.naturalHeight * ratio));

  let bestCandidate: EncodedImageCandidate | null = null;
  for (const scale of feedbackImageScaleSteps) {
    const width = Math.max(1, Math.round(baseWidth * scale));
    const height = Math.max(1, Math.round(baseHeight * scale));

    for (const mimeType of ["image/webp", "image/jpeg"]) {
      for (const quality of feedbackImageQualitySteps) {
        const blob = await encodeFeedbackImage(sourceImage, width, height, mimeType, quality);
        if (!blob) {
          continue;
        }

        const candidate = {
          blob,
          fileName: replaceFileExtension(file.name, mimeType === "image/webp" ? ".webp" : ".jpg")
        };

        if (!bestCandidate || candidate.blob.size < bestCandidate.blob.size) {
          bestCandidate = candidate;
        }

        if (candidate.blob.size <= feedbackImageTargetBytes) {
          return new File([candidate.blob], candidate.fileName, {
            type: candidate.blob.type || mimeType,
            lastModified: Date.now()
          });
        }
      }
    }
  }

  if (!bestCandidate) {
    throw new Error("Nao foi possivel compactar a imagem selecionada.");
  }

  if (bestCandidate.blob.size > feedbackImageHardLimitBytes) {
    throw new Error("A imagem ficou acima de 1 MB mesmo apos a compactacao. Tente outra imagem.");
  }

  return new File([bestCandidate.blob], bestCandidate.fileName, {
    type: bestCandidate.blob.type || file.type,
    lastModified: Date.now()
  });
}

export function formatFeedbackImageSize(bytes: number) {
  const normalizedBytes = Number(bytes || 0);
  if (normalizedBytes <= 0) {
    return "0 KB";
  }

  if (normalizedBytes < 1024) {
    return `${normalizedBytes} B`;
  }

  if (normalizedBytes < 1024 * 1024) {
    return `${Math.round(normalizedBytes / 1024)} KB`;
  }

  return `${(normalizedBytes / (1024 * 1024)).toFixed(1)} MB`;
}

function loadImageElement(file: File) {
  return new Promise<HTMLImageElement>((resolve, reject) => {
    const objectUrl = URL.createObjectURL(file);
    const image = new Image();

    image.onload = () => {
      URL.revokeObjectURL(objectUrl);
      resolve(image);
    };

    image.onerror = () => {
      URL.revokeObjectURL(objectUrl);
      reject(new Error("Nao foi possivel ler a imagem selecionada."));
    };

    image.src = objectUrl;
  });
}

function encodeFeedbackImage(image: HTMLImageElement, width: number, height: number, mimeType: string, quality: number) {
  return new Promise<Blob | null>((resolve) => {
    const canvas = document.createElement("canvas");
    canvas.width = width;
    canvas.height = height;

    const context = canvas.getContext("2d");
    if (!context) {
      resolve(null);
      return;
    }

    context.drawImage(image, 0, 0, width, height);
    canvas.toBlob((blob) => resolve(blob), mimeType, quality);
  });
}

function replaceFileExtension(fileName: string, nextExtension: string) {
  const normalizedName = String(fileName || "imagem").trim() || "imagem";
  const lastDotIndex = normalizedName.lastIndexOf(".");
  const baseName = lastDotIndex > 0 ? normalizedName.slice(0, lastDotIndex) : normalizedName;
  return `${baseName}${nextExtension}`;
}