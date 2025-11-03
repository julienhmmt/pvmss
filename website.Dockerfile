# Build stage
FROM node:22.21-alpine AS builder

WORKDIR /website

# Install dependencies and copy source code
COPY package.json package-lock.json ./

RUN npm ci

COPY . .

# Build
RUN npm run build

# Runtime stage - Nginx
FROM nginx:stable-alpine-slim

RUN rm /etc/nginx/conf.d/default.conf

COPY <<'NGINX_CONF' /etc/nginx/conf.d/pvmss-site.conf
server {
    listen       80;
    server_name  _;

    root   /usr/share/nginx/html;
    index  index.html;

    location / {
        try_files $uri $uri/ /index.html;
    }

    gzip            on;
    gzip_types      text/plain text/css application/javascript application/json image/svg+xml;
    gzip_vary       on;
}
NGINX_CONF

COPY --from=builder /website/dist /usr/share/nginx/html

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]