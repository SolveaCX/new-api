FROM nginx:1.27-alpine
ENV APP_CONSOLE_ORIGIN=https://console.flatkey.ai
ENV NGINX_ENVSUBST_FILTER=APP_CONSOLE_ORIGIN
COPY nginx.conf /etc/nginx/templates/default.conf.template
COPY html /usr/share/nginx/html
EXPOSE 8080
