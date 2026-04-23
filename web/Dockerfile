FROM node:24.11.1-bookworm-slim AS deps

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

FROM deps AS build

COPY . .

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
