FROM node:22.19.0-alpine AS build
RUN corepack enable
WORKDIR /src/apps/web
COPY apps/web/package.json apps/web/pnpm-lock.yaml ./
RUN pnpm ci
COPY apps/web ./
RUN pnpm run build

FROM nginx:1.27.5-alpine
COPY deploy/nginx/default.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/apps/web/dist /usr/share/nginx/html
