# gb

`gb` is a tool for creating and restoring git bundles. Like, _a lot_ of git bundles. Super fast 🏃💨

## Minimum requirements

* macOS/Linux/WSL
* [git](https://git-scm.com)

## Recommended requirements

* [mise](https://mise.jdx.dev/)
* [go 1.24+](https://go.dev)
* [taskfile](https://taskfile.dev)
* [ko](https://ko.build)
* [docker](https://www.docker.com)
  * Shoutout to [orbstack](https://orbstack.dev/) if you're on macOS.

## Quickstart

> [!NOTE]
> This project is a work in progress (i.e., pre-alpha). Builds are not yet available.

~~Download the latest release from the [releases](https://github.com/pythoninthegrass/gb/releases) page.~~

## Development

Install the [recommended requirements](#recommended-requirements).

Check the available tasks:

```bash
task --list-all
```

Build the binary:

```bash
task build
```

Run the binary:

```bash
# general help
./bin/gb -h

# backup
./bin/gb backup -h

# restore
./bin/gb restore -h
```

## TODO

* See: [TODO.md](TODO.md)
