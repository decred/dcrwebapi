# dcrwebapi

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
