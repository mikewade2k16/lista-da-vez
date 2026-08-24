FROM node:24.11.1-bookworm-slim AS deps

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

FROM deps AS build

COPY . .

# O config do Nuxt e avaliado antes de o CLI normalizar NODE_ENV. Fixar aqui
# garante que o build realmente use as opcoes de producao (incluindo sourcemaps
# desligados) sem alterar o target dev abaixo.
ENV NODE_ENV=production

# O Nitro ultrapassa 4 GB ao empacotar o servidor. O ARG permite ajuste pelo
# deploy sem mudar a imagem final; 5120 MB cabe no Docker Desktop atual (~6 GB)
# quando o deploy pausa temporariamente a stack local.
ARG WEB_BUILD_HEAP_MB=5120
ENV NODE_OPTIONS="--max-old-space-size=${WEB_BUILD_HEAP_MB}"
RUN npm run build

FROM deps AS dev

COPY . .

ENV NODE_ENV=development
ENV NITRO_HOST=0.0.0.0
ENV PORT=3003

EXPOSE 3003

CMD ["npm", "run", "dev"]

FROM node:24.11.1-bookworm-slim

WORKDIR /app

ENV NODE_ENV=production
ENV NITRO_HOST=0.0.0.0
ENV PORT=3003

COPY --from=build /app/.output ./.output

EXPOSE 3003

CMD ["node", ".output/server/index.mjs"]
