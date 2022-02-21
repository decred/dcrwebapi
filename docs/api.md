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
