# dcrwebapi

[![Build Status](https://github.com/decred/dcrwebapi/workflows/Build%20and%20Test/badge.svg)](https://github.com/decred/dcrwebapi/actions)
[![ISC License](https://img.shields.io/badge/license-ISC-blue.svg)](http://copyfree.org)

dcrwebapi implements a simple HTTP API which provides summary data about the
Decred blockchain and ecosystem.
Some data such as a list of Voting Service Providers is hard-coded, and some is
collected from external sources such as GitHub and
[dcrdata](https://github.com/decred/dcrdata).

## Voting Service Providers

Data from dcrwebapi is used to populate the VSP list of both
[decred.org](https://decred.org/vsp/) and
[Decrediton](https://github.com/decred/decrediton).

To add a new VSP to the API, VSP operators must open a pull request on this
respository after following the [operator guidelines](https://docs.decred.org/advanced/operating-a-vsp/)
and coordinating with the [Decred community](https://decred.org/community/).

## API

API calls are documented in [api.md](./docs/api.md).

## Docker

To build the image:

```sh
docker build -t decred/dcrwebapi .
```

By default, the container exposes port 80.
To run the image:

```sh
docker run --rm -d -p [local port]:80 decred/dcrwebapi
```

## License

dcrwebapi is licensed under the [copyfree](http://copyfree.org) ISC License.
