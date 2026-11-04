FROM node:22-alpine
RUN corepack enable
WORKDIR /app
COPY package.json pnpm-workspace.yaml .npmrc tsconfig.base.json ./
COPY packages/shared packages/shared
COPY services/advisor services/advisor
RUN pnpm install --filter @udyam/advisor... --no-frozen-lockfile
WORKDIR /app/services/advisor
ENV PORT=8090
EXPOSE 8090
CMD ["pnpm", "start"]
