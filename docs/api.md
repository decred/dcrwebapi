# API

API calls are requested by providing the name of the call as a form parameter.

For example, to request the `vsp` call:

```no-highlight
https://api.decred.org/?c=vsp
```

## Get VSP Info

Collects data from a hard-coded list of Voting Service Providers running
[decred/vspd](https://github.com/decred/vspd).

Example: <https://api.decred.org/?c=vsp>

```json
{
    "teststakepool.decred.org": {
        "network": "testnet",
        "launched": 1590969600,
        "lastupdated": 1596615074,
        "apiversions": [3],
        "feepercentage": 5,
        "closed": false,
        "voting": 3935,
        "voted": 57073,
        "expired": 73,
        "missed": 10,
    },
}
```

## Web Info

Collects data from dcrdata and caches it, serves to JavaScript running on the
homepage of <https://decred.org>.

Example: <https://api.decred.org/?c=webinfo>

```json
{
    "circulatingsupply": 15804577.17784509,
    "ultimatesupply": 20999999.9839432,
    "stakedsupply": 9855286.05084056,
    "blockreward": 8.061013,
    "treasury": 822237.44313611,
    "ticketprice": 268.19271648,
    "height": 837324,
    "lastupdated": 1706092430
}
```

## Price Info

Returns the current USD price of Bitcoin and Decred as reported by dcrdata.

Example: <https://api.decred.org/?c=price>

```json
{
    "bitcoin_usd": 40119.53495,
    "decred_usd": 14.1665886541639,
    "lastupdated": 1706092430
}
```
