# syntax=docker/dockerfile:1

# --- deps: install from the lockfile in its own layer ---
FROM node:24-slim AS deps
WORKDIR /app
COPY package.json package-lock.json .npmrc ./
RUN --mount=type=cache,target=/root/.npm npm ci

# --- build: adapter-node output in build/ ---
# No API_URL needed here: src/env.ts skips validation while building and
# reads the real value when the server starts.
FROM deps AS build
COPY . .
RUN npm run build

# --- runtime: the bundled server only ---
# adapter-node bundles every dependency into build/, and all packages are
# devDependencies, so no node_modules is copied. package.json stays for
# "type": "module".
FROM node:24-slim
WORKDIR /app
ENV NODE_ENV=production PORT=3000
COPY --from=build /app/package.json ./
COPY --from=build /app/build ./build
USER node
EXPOSE 3000
CMD ["node", "build"]
