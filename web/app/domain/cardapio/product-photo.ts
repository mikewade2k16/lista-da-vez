// Regras do cliente para foto de produto (o back revalida pelo conteudo real).
export const PRODUCT_PHOTO_MAX_BYTES = 20 * 1024 * 1024
export const PRODUCT_PHOTO_ACCEPT = 'image/jpeg,image/png,image/webp,image/gif'

// Devolve a mensagem de erro ou '' quando a foto pode ser enviada.
export function productPhotoError(file: { type: string; size: number }): string {
  if (!PRODUCT_PHOTO_ACCEPT.split(',').includes(file.type)) {
    return 'Envie uma foto em JPG, PNG, WebP ou GIF.'
  }
  if (file.size > PRODUCT_PHOTO_MAX_BYTES) {
    return 'A foto passa de 20MB. Escolha uma foto menor.'
  }
  return ''
}
