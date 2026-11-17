# Historical performance notes

This file originally described an early prototype and is not a performance specification for the current application.

The durable lesson is methodological: sender-side acceptance does not establish delivered throughput. Early sender-only figures were withdrawn because the receiver had not confirmed delivery. Subsequent work used receiver byte counts and, where applicable, full payload hashes.

USB negotiation, raw transport capacity, encrypted VPN goodput and public Internet throughput describe different layers. A result at one layer must not be advertised as a result at another.

The reusable architectural findings now live in the [reverse-tethering engineering guide](reverse-tethering-engineering.md). See [verification scope](verification.md) for current limitations. Detailed device/session records remain local and are excluded from publication.
