FROM node:24.19.0-alpine3.24 AS dependencies

RUN corepack enable && corepack prepare pnpm@11.25.0 --activate
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json apps/web/package.json
RUN pnpm install --frozen-lockfile --filter @lifehub/web...

FROM dependencies AS build
ARG NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_AUTH_MODE=supabase
ARG NEXT_PUBLIC_SUPABASE_URL
ARG NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL \
    NEXT_PUBLIC_AUTH_MODE=$NEXT_PUBLIC_AUTH_MODE \
    NEXT_PUBLIC_SUPABASE_URL=$NEXT_PUBLIC_SUPABASE_URL \
    NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY=$NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY \
    NEXT_TELEMETRY_DISABLED=1
COPY apps/web apps/web
RUN pnpm --dir apps/web build

FROM node:24.19.0-alpine3.24 AS runtime

ENV HOSTNAME=0.0.0.0 \
    NODE_ENV=production \
    NEXT_TELEMETRY_DISABLED=1 \
    PORT=10000
WORKDIR /app
RUN addgroup -S lifehub && adduser -S -G lifehub lifehub
COPY --from=build --chown=lifehub:lifehub /app/apps/web/.next/standalone ./
COPY --from=build --chown=lifehub:lifehub /app/apps/web/.next/static ./apps/web/.next/static
USER lifehub
EXPOSE 10000
CMD ["node", "apps/web/server.js"]
