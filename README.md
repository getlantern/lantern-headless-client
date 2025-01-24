# Lantern Headless Client

This is a headless client for Lantern, which is a peer-to-peer censorship circumvention tool. 
It is designed to be run on a server/nas/router, and to be used by a client running on a user's computer.
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

Coming soon

### Homebrew

Coming soon


## Usage
`lantern-headless-client --help` will show you the available options.

```shell
Usage: lantern-headless-client [--data-path DATA-PATH] [--log-level LOG-LEVEL] [--json] <command> [<args>]

Options:
  --data-path DATA-PATH
                         Path to store data (config, logs, etc)
  --log-level LOG-LEVEL
                         Minimum log level to output. Value should be DEBUG, INFO, WARN, or ERROR. [default: INFO, env: LOG_LEVEL]
  --json                 Output logs in JSON format
  --help, -h             display this help and exit

Commands:
  auth                   Authenticate with Lantern
  start                  Start Lantern
```

## Authentication

> TBD: how to get the auth token and uid from GUI app

To use the PRO version of Lantern, you need to authenticate with Lantern's servers.
You can do this by running `lantern-headless-client auth` and supplying the auth token and uid you got from the GUI app.

```shell
$ lantern-headless-client auth --user-id [uid] --token [token]
```

This will store the auth token and uid in the data directory, so you only need to do this once.

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
  --proxies-yaml PROXIES-YAML
                         Path to a custom proxies.yaml file. Assumes --sticky-config
  --device-id DEVICE-ID
                         Custom Device ID

Global options:
  --data-path DATA-PATH
                         Path to store data (config, logs, etc)
  --log-level LOG-LEVEL
                         Minimum log level to output. Value should be DEBUG, INFO, WARN, or ERROR. [default: INFO, env: LOG_LEVEL]
  --json                 Output logs in JSON format
  --help, -h             display this help and exit
```

### Using custom proxies

Instead of using Lantern's default proxies, you can supply your own proxies.yaml file.

```shell
$ lantern-headless-client start --proxies-yaml /path/to/proxies.yaml
```