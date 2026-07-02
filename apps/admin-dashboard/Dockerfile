# syntax=docker/dockerfile:1

FROM node:22-alpine AS deps
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci

FROM node:22-alpine AS build
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
# Nuxt SSR build; control plane URL is supplied at runtime via NANCE_ACCELERATOR_URL
ENV NUXT_TELEMETRY_DISABLED=1
RUN npm run build

FROM node:22-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
ENV NUXT_TELEMETRY_DISABLED=1
ENV HOST=0.0.0.0
ENV PORT=3000
ENV NITRO_PORT=3000
ENV NITRO_HOST=0.0.0.0

RUN addgroup --system --gid 1001 nodejs \
  && adduser --system --uid 1001 nuxt

COPY --from=build --chown=nuxt:nodejs /app/.output ./.output
COPY --from=build --chown=nuxt:nodejs /app/package.json ./package.json

USER nuxt
EXPOSE 3000
CMD ["node", ".output/server/index.mjs"]
