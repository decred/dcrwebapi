dcrwebapi
=========

[![Build Status](https://github.com/decred/dcrwebapi/workflows/Build%20and%20Test/badge.svg)](https://github.com/decred/dcrwebapi/actions)
[![ISC License](https://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)

## Build Docker image

```sh
docker build -t decred/dcrwebapi .
```

## Push to Dockerhub

```sh
docker login
```

Enter your [Docker HUB](https://hub.docker.com/) credentials that has write access to the `decred/dcrwebapi` repository.

```sh
docker push decred/dcrwebapi
```

## Run image

```sh
docker pull decred/dcrwebapi:latest
```
By default, the container exposes port 80.

```sh
docker run --rm -d -p [local port]:80 decred/dcrwebapi
```

## License

dcrwebapi is licensed under the [copyfree](http://copyfree.org) ISC License.
