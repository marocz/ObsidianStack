# Stage 1 — Build the React app
FROM node:20-alpine AS builder
WORKDIR /app

# Cache node_modules separately from source
COPY package.json package-lock.json ./
RUN npm ci --ignore-scripts

COPY . .
RUN npm run build
# Output: /app/dist

# Stage 2 — Serve with nginx
FROM nginx:1.27-alpine

# Remove default config
RUN rm /etc/nginx/conf.d/default.conf

# Copy our config template (uses envsubst to inject NGINX_SERVER_URL)
COPY nginx.conf /etc/nginx/templates/default.conf.template

# Copy built assets
COPY --from=builder /app/dist /usr/share/nginx/html

# Default server URL — override at runtime with -e NGINX_SERVER_URL=...
ENV NGINX_SERVER_URL=http://obsidianstack-server:8080

EXPOSE 80

# nginx:alpine's entrypoint runs envsubst on *.template files then starts nginx
CMD ["nginx", "-g", "daemon off;"]
