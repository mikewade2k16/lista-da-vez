# Imagem do agent-runner para uso futuro em VPS (profile meta-ads-assistant).
# Modo PRIMARIO local e rodar no HOST (npm start), porque o OAuth do MCP da Meta
# fica cacheado nas credenciais do Claude do host (~/.claude). Ver README.md.
FROM node:22-alpine

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci --omit=dev --no-fund --no-audit

COPY src ./src

ENV META_ADS_ASSISTANT_PORT=8765
EXPOSE 8765

USER node

CMD ["npm", "start"]
