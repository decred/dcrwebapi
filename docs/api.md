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
        "revoked": 83
    },
}
```

## Get VSP Info (legacy)

Collects data from a hard-coded list of Voting Service Providers running
[decred/dcrstakepool](https://github.com/decred/dcrstakepool).

Example: <https://api.decred.org/?c=gsd>

```json
{
    "Alpha":{
        "APIEnabled":true,
        "APIVersionsSupported":[1, 2],
        "Network":"testnet",
        "URL":"https://teststakepool.decred.org",
        "Launched":1516579200,
        "LastUpdated":1598266266,
        "Immature":0,
        "Live":1,
        "Voted":616,
        "Missed":10,
        "PoolFees":1,
        "ProportionLive":0.00015578750584203146,
        "ProportionMissed":0.01597444089456869,
        "UserCount":7,
        "UserCountActive":5,
        "Version":"1.5.0-pre"
    },
}
```
