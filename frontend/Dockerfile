FROM nginx:stable-alpine3.21

# Create necessary directories
RUN mkdir -p /var/www/html

# Install gettext for envsubst
RUN apk add --no-cache gettext

# Copy nginx configuration and frontend files
COPY nginx.conf /etc/nginx/nginx.conf
COPY ./css /var/www/html/css/

# Set proper permissions
RUN chown -R nginx:nginx /var/www/html && \
    chmod -R 755 /var/www/html && \
    chmod 644 /etc/nginx/nginx.conf

# Expose port 80
EXPOSE 80

# Start Nginx
CMD ["nginx", "-g", "daemon off;"]
