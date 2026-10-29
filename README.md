# Lantern

Censorship circumvention tool available for free download on any operating system

![cover page](https://github.com/getlantern/.github/blob/main/resources/cover_page.png)

# Lantern Headless Client

This is a headless client for Lantern, which is a peer-to-peer censorship circumvention tool.
It is designed to be run on a server/nas/router and to be used by a client running on a user's computer.
It's targeted at advanced users who are comfortable with the command line.

## Installation

> All binary releases are available on the [releases page](https://github.com/getlantern/lantern-headless-client/releases) and are signed with cosign.
> You can validate the signature by running `cosign verify-blob -key cosign.pub -signature release_file.tar.gz.sig release_file.tar.gz`.
> The public key is available [here](./cosign.pub).

### From binary

Download the latest release from the [releases page](https://github.com/getlantern/lantern-headless-client/releases/latest).
Extract the tarball and run the binary.

### Debian/Ubuntu

```shell
echo "deb [trusted=yes] https://apt.fury.io/getlantern/ /" > /etc/apt/sources.list.d/fury.list
apt-get update
apt-get install lantern-headless
```

### RedHat/CentOS

```shell
echo -e "[fury]\nname=Gemfury Private Repo\nbaseurl=https://yum.fury.io/getlantern/\nenabled=1\ngpgcheck=0" > /etc/yum.repos.d/fury.repo
yum install lantern-headless
```

### Arch Linux

```
yay -S lantern-headless-bin
```

### Alpine Linux

```shell
echo "https://alpine.fury.io/getlantern/" >> /etc/apk/repositories
apk add lantern-headless
```

### Docker

```shell
docker run --name=lantern-headless \
  --volume=./lantern-data/:/data/ \
  --workdir=/ \
  -p 12345:12345 \
  -p 12346:12346 \
  --restart=always \
  --runtime=runc \
  --detach=true \
  getlantern/lantern-headless  \
  --data-path /data start --http-proxy-addr 0.0.0.0:12345 --socks-proxy-addr 0.0.0.0:12346
```

### Scoop

```shell

scoop bucket add lantern-headless https://github.com/getlantern/lantern-headless-client-scoop-bucket.git`

scoop install lantern-headless-client
```

### Homebrew

```shell
brew tap lantern/cli https://github.com/getlantern/lantern-headless-client-brew-tap.git

brew install lantern/cli/lantern-headless

```

## Usage

`lantern-headless-client --help` will show you the available options.

```shell
Usage: lantern-headless-client [--data-path DATA-PATH] [--log-level LOG-LEVEL] [--json] <command> [<args>]

Options:
  --data-path DATA-PATH
                         Path to store data (config, logs, etc)
  --debug                Enable debug mode
  --raw                  Output logs in raw format
  --quiet                Disable all output
  --auth-url AUTH-URL    Override URL of the authentication server
  --insecure             Whether to skip TLS verification
  --help, -h             display this help and exit
  --version              display version and exit

Commands:
  auth                   Authenticate with Lantern
  start                  Start Lantern
  unbounded              Share your connection as an Unbounded volunteer peer
```

## Authentication

> The application can be used without authentication, but it will be limited to a certain amount of data transfer.

To use the PRO version of Lantern, you need to authenticate with Lantern's servers.
This needs to be done once, and the auth token will be stored in the data directory.
The arguments can be passed via the command line, environment variables or entered interactively.

> Please note that there is no way to pay for PRO account via the headless client. You need to use the desktop/mobile Lantern client for that.

### Sign up

```shell
$ lantern-headless-client auth signup --email <email>
```

### Login

```shell
$ lantern-headless-client auth login --email <email>
```

### Logout

```shell
$ lantern-headless-client auth logout
```

## Sharing your connection (Unbounded)

`lantern-headless unbounded` turns the host into an [Unbounded](https://github.com/getlantern/unbounded) volunteer peer.
Users in censored regions reach it over WebRTC, brokered by Lantern's discovery server, and it relays their traffic
to a Lantern egress server. No account is needed, and the peer opens no inbound ports: it only needs outbound UDP and
HTTPS. It will not help many users from behind a symmetric NAT; the startup NAT check tells you which kind you have.

```shell
$ lantern-headless unbounded --max-peers 5 --bandwidth-limit 5mbit --data-cap 50GB/day
```

```shell
Usage: lantern-headless-client unbounded [--freddie FREDDIE] [--egress EGRESS] [--max-peers MAX-PEERS] [--bandwidth-limit BANDWIDTH-LIMIT] [--data-cap DATA-CAP] [--target-countries TARGET-COUNTRIES] [--tag TAG] [--netstated NETSTATED] [--nat-fail-timeout NAT-FAIL-TIMEOUT] [--stats-interval STATS-INTERVAL] [--skip-nat-check] [--pprof-addr PPROF-ADDR]

Options:
  --freddie FREDDIE      Discovery (signaling) server URL [default: https://freddie.iantem.io, env: UNBOUNDED_FREDDIE]
  --egress EGRESS        Egress server WebSocket URL [default: wss://unbounded.iantem.io:443, env: UNBOUNDED_EGRESS]
  --max-peers MAX-PEERS  How many censored peers to serve at once [default: 5, env: UNBOUNDED_MAX_PEERS]
  --bandwidth-limit BANDWIDTH-LIMIT
                         Byte rate cap per direction, e.g. 5mbit, 500KB/s, 1.5MiB/s (default: unlimited) [env: UNBOUNDED_BANDWIDTH_LIMIT]
  --data-cap DATA-CAP    Pause once this much has been relayed in a period, e.g. 50GB/day, 1TB/month (default: none) [env: UNBOUNDED_DATA_CAP]
  --target-countries TARGET-COUNTRIES
                         Comma-separated ISO 3166-1 alpha-2 countries whose users to prefer, e.g. IR,RU [env: UNBOUNDED_TARGET_COUNTRIES]
  --tag TAG              Label for this peer in the swarm's network view [env: UNBOUNDED_TAG]
  --nat-fail-timeout NAT-FAIL-TIMEOUT
                         How long a WebRTC attempt may spend in NAT traversal [default: 15s, env: UNBOUNDED_NAT_FAIL_TIMEOUT]
  --stats-interval STATS-INTERVAL
                         How often to log relay statistics (0 disables) [default: 60s, env: UNBOUNDED_STATS_INTERVAL]
  --skip-nat-check       Skip the startup NAT mapping probe [env: UNBOUNDED_SKIP_NAT_CHECK]
```

Limits:

- `--max-peers` is how many users are helped at the same time.
- `--bandwidth-limit` caps the byte rate in each direction for the whole process. A lowercase `b` or `bit` means bits
  (`5mbit`, `20Mbps`), an uppercase `B` means bytes (`500KB/s`, `1.5MiB/s`).
- `--data-cap` stops helping once that much has been relayed in the current day, week or month (local time) and
  resumes when the period rolls over. Usage is persisted in the data directory, so a restart does not reset it.

Every option is also an environment variable, which is how the service unit is configured.

### As a Linux service

The packages install an opt-in systemd unit. It runs as an unprivileged dynamic user with its state in
`/var/lib/lantern-unbounded`:

```shell
cp /etc/lantern-headless/unbounded.env.example /etc/lantern-headless/unbounded.env   # then edit the limits
systemctl enable --now /etc/lantern-headless/lantern-unbounded.service
journalctl -u lantern-unbounded -f
```

### In Docker

```shell
docker run --name=lantern-unbounded \
  --volume=./lantern-unbounded/:/data/ \
  --restart=always \
  --detach=true \
  getlantern/lantern-headless \
  --data-path /data unbounded --bandwidth-limit 5mbit --data-cap 50GB/day
```

## Starting Lantern

To start Lantern, run `lantern-headless-client start`.

```shell
$ lantern-headless-client start
```

This will start the HTTP and SOCKS proxies on a random port, and print out the port numbers when it's ready to accept connections.

You can tweak this by supplying custom arguments:

```shell
Usage: lantern-headless-client start [--http-proxy-addr HTTP-PROXY-ADDR] [--socks-proxy-addr SOCKS-PROXY-ADDR] [--sticky-config] [--readable-config] [--proxies-yaml PROXIES-YAML] [--device-id DEVICE-ID]

Options:
  --http-proxy-addr HTTP-PROXY-ADDR
                         Address to bind to and use for HTTP the proxy. Defaults to a random port on localhost [default: 127.0.0.1:0]
  --socks-proxy-addr SOCKS-PROXY-ADDR
                         Address to bind to and use for SOCKS the proxy. Defaults to a random port on localhost [default: 127.0.0.1:0]
  --sticky-config        Whether to use a sticky config
  --readable-config      Whether to use a readable config
```
