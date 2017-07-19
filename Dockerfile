#DOCKER_IMAGE_TAG=decred/dcrwebapi
FROM php:7.1-apache

LABEL description="dcrweb-api"
LABEL version="1.0"
LABEL maintainer "peter@froggle.org"

COPY php.ini /usr/local/etc/php/

# install apcu extension
RUN pecl install apcu && \
    docker-php-ext-enable apcu

# copy document root
COPY index.php /var/www/html/
