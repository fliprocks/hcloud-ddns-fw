# hcloud-ddns-fw

This tool is a simple microservice which allows you to update your hcloud DNS records and Firewall rules. I built this, because I needed to restrict Firewall rules to specific IPs or networks, which have dynamic IP addresses. So in other words, this is DDNS updater which can set Firewall rules for dynamic IP addresses too.

Besides this, I alway wanted to learn [Go](https://go.dev/), so I thought this could be a great first project.

## Features

### hcloud DNS

It is possible to update one or more **existing** A or AAAA DNS records from one or more DNS zones. If the specified record does not exist in hcloud DNS, the updater won't add it.

### hcloud Firewall

The Firewall updater doesn't update single Firewall rules. It replaces the whole Firewall. Since you can add multiple Firewalls to a single server, I suggest to add an dedicated Firewall for this updater to the desired resources.
Basically, all Firewall features are supported, except port ranges (at the moment).

- Inbound or outbound
- IPv4 and IPv6
- TCP, UDP, ICMP, GRE or ESP
- Port

## Workflow

1. The updater checks your public IP(s) in a given interval against https://ipify.org (or a custom endpoint), or receives them via a [DDNS update URL](#ddns-update-url)
2. An update will be started if a changed IP address is detected

## How to create a hcloud token

To create a hcloud token, follow the steps as described [here](https://docs.hetzner.com/cloud/api/getting-started/generating-api-token/). The token will require **Read & Write** permissions.

‼️ **This token is mighty. Handle with care.** ‼️

## How to retrieve the Firewall id (optional)

### Use a command

Start the Docker container or the Go binary using the following command:

#### `docker run`

```bash
docker run --rm -e HCLOUD_TOKEN=yourSecretHcloudToken ghcr.io/fliprocks/hcloud-ddns-fw:latest list-firewalls
```

#### Go binary (you will need to build this first)

```bash
export HCLOUD_TOKEN=yourSecretHcloudToken
./hcloud-ddns-fw list-firewalls
```

### Have a look at [console.hetzner.com](https://console.hetzner.com)

1. Login at hetzner console
2. Navigate to `Firewalls`
3. Click on desired Firewall
4. Check the Browsers URL
   <br>
   ![](./docs/fwid.png)
   <br>
   The highlighted <b style="color:#22D081;">123456789</b> represents your Firewall id.

## Configuration

See [configuration.md](./configuration.md) and the [env.example](./env.example) file.

## Deploy the services

### Docker compose

The easiest way to deploy the service is using the [compose.yaml](./compose.yaml), [env.example](./env.example) and [config.yaml](./config.yaml) from this repository. Copy `env.example` to `.env` and adjust the settings.

```bash
docker compose up -d
```

### Docker run

Alternatively, you can also use the `docker run` command together with a [config.yaml](./config.yaml) file to deploy the service.

```bash
docker run \
   -e HCLOUD_TOKEN=yourSecretHcloudToken \
   -e UPDATE_INTERVAL=300 \
   -e HTTP_TIMEOUT=10 \
   -e IP_MODE=IPv4 \
   -e LOGLEVEL=INFO \
   -e TZ=Europe/Berlin \
   -v ./config.yaml:/app/config.yaml \
   --restart always \
   -m 32M \
   -d \
   ghcr.io/fliprocks/hcloud-ddns-fw:latest
```

### Go binary (without Docker)

There are several ways on how to run the service without Docker. One might be following solution:

```bash
export HCLOUD_TOKEN=yourSecretHcloudToken
export UPDATE_INTERVAL=300
export HTTP_TIMEOUT=30
export IP_MODE=IPv4
export LOGLEVEL=INFO
export TZ=Europe/Berlin
# export further env vars...
./hcloud-ddns-fw
```

For this, the [config.yaml](./config.yaml) need to be placed in the same directory as the binary.

To run the binary in the background, consider `nohup`.

```bash
nohup ./hcloud-ddns-fw > app.log 2>&1 &
```

## Build

### Build Docker image

Prerequisite: Docker is up and running.

```bash
git clone https://github.com/fliprocks/hcloud-ddns-fw.git
cd hcloud-ddns-fw
git tag
git checkout # <tag you want to build>
docker build -t hcloud-ddns-fw:<your tag> .
```

### Build Go binary

Prerequisite: Go 1.27 ist installed

```bash
git clone https://github.com/fliprocks/hcloud-ddns-fw.git
cd hcloud-ddns-fw
git tag
git checkout # <tag you want to build>
go mod download
go build -o hcloud-ddns-fw
```

## Limitations

### DNS record types

The tool currently only support A and AAAA records.

### Public IP limitations

Both, A records or IPv4 Firewall rules and AAAA records, or IPv6 Firewall rules of course need a public IPv4 or IPv6 address. Otherwise, you will not be able to use the corresponding record type or IP for DNS records and Firewall rules.

### IPv6 with Docker

There are two ways to update AAAA records or IPv6 Firewall rules with the Docker container.

1. Running the container in `network_mode: host` (**easiest**).
2. Configure Docker to work with IPv6 and add the container to a bridge network.

### Docker network mode

`network_mode: host` is not supported on MacOS

## Development

Contributions are welcome. Also see [CODE_OF_CONDUCT](./CODE_OF_CONDUCT.md) for this.

The easiest was is using a dev container. For this, you can use [devcontainer-example.json](./.devcontainer/devcontainer-example.json), customize it according to your needs and rename it do _devcontainer.json_. You will also need to export `HCLOUD_TOKEN` on your host system. Additionally, create a _config-dev.yaml_ file with DNS records and Firewall rules to be used for your development. Make sure `DEVMODE` is set to `true` in [devcontainer-example.json](./.devcontainer/devcontainer-example.json).

### Ideas for future development

- Notifikations e.g. to ntfy
- Trigger further webhooks

## Unaffiliated with Hetzner or FRITZ!

hcloud-ddns-fw is neither related nor supported by [Hetzner](https://hetzner.com) or [FRITZ!](https://fritz.com/).

## Liability

The software is provided "as is", without warranty of any kind, express or implied, including but not limited to the warranties of merchantability, fitness for a particular purpose and noninfringement. In no event shall the authors or copyright holders be liable for any claim, damages or other liability, whether in an action of contract, tort or otherwise, arising from, out of or in connection with the software or the use or other dealings in the software.
